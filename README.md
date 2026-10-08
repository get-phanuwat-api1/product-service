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

This starts PostgreSQL and then the API. **The API applies pending migrations automatically on every start**, so you do not need a separate migrate step.

- API: http://localhost:8080
- Swagger UI: http://localhost:8080/api-docs

### Option 2: Run locally with Go (Go 1.25+)

```bash
make run   # starts PostgreSQL in Docker if needed, then the API (auto-migrates first)
```

### Option 3: Debug in VS Code

Open the `product-service` folder in VS Code, install the recommended **Go** extension, then go to **Run and Debug** (F5):

| Configuration | What it does |
|---|---|
| **API: debug** | Starts PostgreSQL (and stops the dockerized API to free port 8080), then runs the API under the debugger. Set breakpoints in handler, service, or repository and call the API from Swagger or curl. |
| Migrate: version | Debugs the migration CLI |
| Test: current package | Debugs the tests of the open file's package |
| Test: selected function | Select a `TestXxx` name, then run it |
| Test: component (E2E within service) | Debugs the component tests (needs Docker) |
| Attach: running process | Attaches to an already running Go process |

You can also use the **debug test** link that the Go extension shows above each `func TestXxx`.

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
| `database.auto_migrate` | `DATABASE_AUTO_MIGRATE` | `true` |
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

### Auto-migrate on startup

With `database.auto_migrate: true` (the default), the API runs `migrate up` every time it starts, before it opens the HTTP port:

```
INFO running database migrations
INFO database schema is up to date version=2
INFO server started addr=:8080
```

- **Idempotent:** if nothing is pending, startup simply continues.
- **Safe with multiple instances:** golang-migrate holds a PostgreSQL advisory lock, so only one instance applies the changes and the others wait. This is covered by `TestMigrateUp_ConcurrentStartsAreSafe`.
- **Fails fast:** if a migration fails, or the schema is left dirty, the API exits instead of serving on a half-migrated schema.
- **Turning it off:** set `DATABASE_AUTO_MIGRATE=false`, for example in production when migrations run as a separate deploy step. In that case, use the CLI below.

### Manual CLI

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

### Postman

[`postman/`](postman/) contains a ready-to-use collection and a local environment:

1. In Postman, choose **Import**, then select both files in `postman/`.
2. Select the **Product Service - Local** environment (`baseUrl = http://localhost:8080`).
3. Start the service, then **Run collection**, or send requests one by one. **Create product (all fields)** saves `productId` for the PATCH requests.

| Folder | Requests |
|---|---|
| Health check | `GET /health` |
| Create Product - POST /product | all fields, required only, explicit nulls |
| Create Product - errors | `INVALID_NAME`, `INVALID_PRICE`, `INVALID_SALE_PRICE`, `INVALID_REQUEST_BODY` |
| Patch Product - PATCH /product/{id} | only name, price + sale_price, clear description, clear sale_price, empty body |
| Patch Product - errors | `PRODUCT_NOT_FOUND`, `INVALID_PRODUCT_ID`, null name/price, sale_price above price |

Every request has test scripts that check the status, `successful`, and `error_code`, and that `X-Request-ID` is echoed back (20 requests, 62 assertions). Each request also sends `X-User-ID: {{userId}}`, so changes appear in `audit_logs` with actor `postman`.

You can run the same collection from the command line with [Newman](https://github.com/postmanlabs/newman):

```bash
make postman
```

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
make test        # run everything with -race (Docker must be running)
make test-unit   # unit tests only with -race, no Docker needed
make fuzz        # run fuzz tests (FUZZTIME=30s each by default)
make cover       # coverage
make mocks       # regenerate mocks after changing an interface
```

To use an existing database instead of testcontainers, set `TEST_DATABASE_URL=postgres://...`

### Unit test conventions

- **Table-driven tests with subtests.** Pure unit tests run with `t.Parallel()`. Tests that share a database (repository, component) run sequentially.
- **Mocks are generated** by [mockery](https://github.com/vektra/mockery) from the interfaces listed in [`.mockery.yml`](.mockery.yml) into `internal/mocks/`. Use the typed API: `repo.EXPECT().CreateProduct(...).Return(nil)`. Mocks built with `mocks.NewXxx(t)` fail the test automatically if an expectation is not met.
  - `mocks.FakeTransactor` is a hand-written fake, because it only needs to run `fn`.
- **No monkey patching.** Every dependency is an interface injected through a constructor, so tests never touch global state.
- **Fuzz tests** check invariants over random inputs:
  - `FuzzField_MatchesPointerDecoding`: `optional.Field` decodes JSON exactly like `*T`, plus key presence.
  - `FuzzProduct_ApplyPatch`: a failed patch never mutates the product, and a successful patch always yields a valid product that keeps every undefined field.

  The seed corpus runs as part of `go test`.
