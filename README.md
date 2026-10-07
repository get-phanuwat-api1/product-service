# Product Service

A Product REST API in Go with a layered architecture (**handler → service → repository**), dependency injection, PostgreSQL, versioned migrations, a YAML config file, and Swagger docs.

| Method | Path            | Description                                           |
|--------|-----------------|-------------------------------------------------------|
| POST   | `/product`      | Create a product                                      |
| PATCH  | `/product/{id}` | Partially update a product (only the fields you send) |
| GET    | `/api-docs`     | Swagger UI                                            |
| GET    | `/health`       | Health check                                          |

---

## How to start the service

### Option 1: Docker Compose (needs only Docker)

```bash
docker compose up --build
```

This starts PostgreSQL, runs migrations (`migrate up`), and then starts the API.

- API: http://localhost:8080
- Swagger UI: http://localhost:8080/api-docs

### Option 2: Run locally with Go (Go 1.25+)

```bash
make db           # start PostgreSQL in Docker (port 5432)
make migrate-up   # apply migrations
make run          # start the API
```

---

## Project structure

```
product-service/
├── cmd/
│   ├── api/main.go                  # API entrypoint: load config → DB → container → HTTP server
│   └── migrate/main.go              # migration CLI: up / down / version / force / create
├── config/
│   ├── config.yaml                  # default config (local)
│   └── config.go                    # load YAML + env override (Viper)
├── migrations/                      # SQL migrations (golang-migrate format, embedded in binary)
│   ├── 000001_create_products_table.{up,down}.sql
│   └── 000002_create_audit_logs_table.{up,down}.sql
├── internal/
│   ├── handler/                     # HTTP layer (controller): parse request → call service → write response
│   ├── service/                     # business logic / orchestration (transaction, patch flow, audit)
│   ├── repository/                  # data access (SQL, pgx): product, audit log, transactor
│   ├── model/                       # entity + business rules (validate, apply patch, audit snapshot)
│   ├── dto/                         # request / response structs (JSON shape)
│   ├── middleware/                  # request ID, actor, access log, panic recovery
│   ├── apperror/                    # error codes + HTTP status mapping
│   ├── router/                      # route + middleware + swagger
│   ├── container/                   # dependency injection: wires repository → service → handler → router
│   └── mocks/                       # testify mocks for unit tests
├── pkg/                             # reusable, project-agnostic packages
│   ├── database/                    # pgx pool + migrator
│   ├── logger/                      # slog JSON logger, adds request_id/actor from context
│   ├── requestctx/                  # request ID / actor stored in context.Context
│   └── optional/                    # Field[T]: undefined vs null vs value (for PATCH)
├── test/
│   ├── component/                   # component tests (E2E within service)
│   └── testutil/                    # testcontainers PostgreSQL helper
├── docs/                            # generated Swagger (swag)
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

Request flow:

```
HTTP → router → handler → service → repository → PostgreSQL
                  │          │           │
                 dto       model     model
```

- Each layer depends only on the **interface** of the layer below it (`service.ProductService`, `repository.ProductRepository`), so every layer can be tested with mocks.
- Every dependency is injected through constructors and wired in one place: [`internal/container/container.go`](internal/container/container.go).

---

## Config

Defaults live in [`config/config.yaml`](config/config.yaml). **Any environment variable overrides the file**: replace `.` with `_` and use upper case.

| YAML key            | Env var             | Default     |
|---------------------|---------------------|-------------|
| `server.port`       | `SERVER_PORT`       | `8080`      |
| `database.host`     | `DATABASE_HOST`     | `localhost` |
| `database.port`     | `DATABASE_PORT`     | `5432`      |
| `database.user`     | `DATABASE_USER`     | `postgres`  |
| `database.password` | `DATABASE_PASSWORD` | `postgres`  |
| `database.name`     | `DATABASE_NAME`     | `product`   |
| `log.level`         | `LOG_LEVEL`         | `info`      |
| `log.format`        | `LOG_FORMAT`        | `json`      |

To use a different config file, set `CONFIG_PATH=/path/to/config.yaml`.

---

## Logging

The service uses structured JSON logs via `log/slog` ([`pkg/logger`](pkg/logger/logger.go)).

- **`request_id` and `actor` are added to every log line automatically** from the request context, so all logs from one request can be traced together.
  - `X-Request-ID`: sent by the client, or generated when omitted. It is echoed in the response header.
  - `X-User-ID`: identifies who is calling. It defaults to `anonymous`, since the service has no authentication yet.
- **Access log:** one line per request with method, route, status, latency, client IP, and `error_code`. 5xx is logged at `ERROR`, 4xx at `WARN`, and everything else at `INFO`. `/health` and `/api-docs` are skipped.
- **Panics** are recovered, logged with a stack trace, and answered with `INTERNAL_SERVER_ERROR`.

```json
{"time":"...","level":"INFO","msg":"http request","service":"product-service","method":"PATCH",
 "route":"/product/:id","status":200,"latency_ms":3,"request_id":"demo-1","actor":"bob"}
```

## Audit log

Every successful business change is written to the `audit_logs` table **in the same transaction** as the change. If the audit row cannot be written, the change is rolled back, so no change goes unaudited.

| Column | Example |
|---|---|
| `action` | `PRODUCT_CREATED`, `PRODUCT_UPDATED` |
| `entity_type` / `entity_id` | `product` / `1` |
| `actor` | from `X-User-ID` |
| `request_id` | matches `request_id` in the logs |
| `before_data` / `after_data` | full JSONB snapshots (`before_data` is null on create) |

The following are **not** audited: failed requests (they appear in the access log instead), an empty PATCH, and a PATCH whose values are all identical to the current ones.

```sql
SELECT action, actor, request_id, before_data, after_data, created_at
FROM audit_logs WHERE entity_type = 'product' AND entity_id = 1 ORDER BY id;
```

---

## Migration

The project uses [golang-migrate](https://github.com/golang-migrate/migrate). Each migration is a pair of `{version}_{name}.up.sql` / `.down.sql` files, and the applied version is stored in the `schema_migrations` table.

```bash
make migrate-up                          # apply all pending migrations
make migrate-down N=1                    # roll back 1 step
make migrate-version                     # show current version
make migrate-create NAME=add_sku_column  # create a new up/down file pair
```

---

## API

### Create Product: `POST /product`

```json
{ "name": "iPhone 17", "description": "Latest model", "sale_price": 39900, "price": 42900 }
```

`description` and `sale_price` are nullable. Response `201`:

```json
{
  "successful": true,
  "error_code": "",
  "data": { "id": 1, "name": "iPhone 17", "description": "Latest model",
            "price": 42900, "sale_price": 39900,
            "created_at": "...", "updated_at": "..." }
}
```

### Patch Product: `PATCH /product/{id}`

Overwrites **only the keys present in the body**:

| Body                             | Result                                |
|----------------------------------|---------------------------------------|
| key omitted (undefined)          | keeps the current value               |
| `"description": null`            | sets `description` to NULL            |
| `"sale_price": null`             | sets `sale_price` to NULL             |
| `"name": null` / `"price": null` | `400` (these fields are not nullable) |

Response `200`: `{"successful": true, "error_code": ""}`

### Error codes

| error_code              | HTTP | Cause                                          |
|-------------------------|------|------------------------------------------------|
| `INVALID_REQUEST_BODY`  | 400  | Malformed JSON / wrong types / empty body      |
| `INVALID_PRODUCT_ID`    | 400  | `id` is not a positive integer                 |
| `INVALID_NAME`          | 400  | name missing, null, blank, or > 255 characters |
| `INVALID_PRICE`         | 400  | price missing, null, or negative               |
| `INVALID_SALE_PRICE`    | 400  | sale_price negative or greater than price      |
| `PRODUCT_NOT_FOUND`     | 404  | No product with this id                        |
| `INTERNAL_SERVER_ERROR` | 500  | Unexpected error                               |

---

## Tests

| Requirement | Location | Approach | Docker |
|---|---|---|---|
| Service/Domain unit test (function) | `internal/model/*_test.go` | pure functions: validation, patch merge | – |
| Usecase unit test (orchestrator) | `internal/service/*_test.go` | mocked repositories + transactor, incl. audit before/after | – |
| Repository unit test (integration) | `internal/repository/*_test.go` | real PostgreSQL (testcontainers): product + audit log | ✔ |
| Component test (E2E within service) | `test/component/*_test.go` | HTTP → handler → service → repository → DB | ✔ |
| (extra) Handler unit test | `internal/handler/*_test.go` | mocked service, checks status / error_code | – |
| (extra) Middleware / logger tests | `internal/middleware`, `pkg/logger` | request ID, actor, access log, recovery | – |

```bash
make test        # run everything (Docker must be running)
make test-unit   # no Docker needed
make cover       # coverage
```

To use an existing database instead of testcontainers, set `TEST_DATABASE_URL=postgres://...`
