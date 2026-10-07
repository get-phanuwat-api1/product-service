# Product Service

A Product REST API in Go, built with **Clean Architecture**, **constructor-based dependency injection**, **PostgreSQL**, and **Swagger** docs.

| Method | Path            | Description                                         |
|--------|-----------------|-----------------------------------------------------|
| POST   | `/product`      | Create a product                                    |
| PATCH  | `/product/{id}` | Partially update a product (only the fields you send) |
| GET    | `/api-docs`     | Swagger UI                                          |
| GET    | `/health`       | Health check                                        |

---

## How to start the service

### Option 1: Docker Compose (simplest; needs only Docker)

```bash
docker compose up --build
```

- API: http://localhost:8080
- Swagger UI: http://localhost:8080/api-docs

### Option 2: Run locally with Go (Go 1.25+)

```bash
make db        # start PostgreSQL in Docker (port 5432)
make run       # go run ./cmd/api
```

The `products` table is created automatically on startup (embedded migration).

Environment variables:

| Name           | Default                                                              |
|----------------|----------------------------------------------------------------------|
| `HTTP_PORT`    | `8080`                                                               |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/product?sslmode=disable` |

---

## API

### Create Product: `POST /product`

```json
{
  "name": "iPhone 17",           // string, required
  "description": "Latest model", // string, nullable
  "sale_price": 39900,           // double, nullable
  "price": 42900                 // double, required
}
```

Response `201`:

```json
{
  "successful": true,
  "error_code": "",
  "data": {
    "id": 1,
    "name": "iPhone 17",
    "description": "Latest model",
    "price": 42900,
    "sale_price": 39900,
    "created_at": "2026-10-07T12:00:00Z",
    "updated_at": "2026-10-07T12:00:00Z"
  }
}
```

On error, `data` is `null`: `{"successful": false, "error_code": "INVALID_PRICE", "data": null}`

### Patch Product: `PATCH /product/{id}`

Overwrites **only the keys present in the body**:

| Body                         | Result                                   |
|------------------------------|------------------------------------------|
| key omitted (undefined)      | keeps the current value                  |
| `"description": null`        | sets `description` to NULL               |
| `"sale_price": null`         | sets `sale_price` to NULL                |
| `"name": null` / `"price": null` | `400` (these fields are not nullable) |

```bash
curl -X PATCH localhost:8080/product/1 -H 'Content-Type: application/json' \
  -d '{"name":"iPhone 17 Pro","sale_price":null}'
```

Response `200`: `{"successful": true, "error_code": ""}`

### Error codes

| error_code              | HTTP | Cause                                         |
|-------------------------|------|-----------------------------------------------|
| `INVALID_REQUEST_BODY`  | 400  | Malformed JSON / wrong types / empty body     |
| `INVALID_PRODUCT_ID`    | 400  | `id` is not a positive integer                |
| `INVALID_NAME`          | 400  | name missing, null, blank, or > 255 characters |
| `INVALID_PRICE`         | 400  | price missing, null, or negative              |
| `INVALID_SALE_PRICE`    | 400  | sale_price negative or greater than price     |
| `PRODUCT_NOT_FOUND`     | 404  | No product with this id                       |
| `INTERNAL_SERVER_ERROR` | 500  | Unexpected error                              |

---

## Architecture

```
cmd/api/main.go                  # entrypoint: config, DB, graceful shutdown
internal/
├── app/                         # composition root (dependency injection wiring)
├── config/                      # env config
├── domain/                      # entity, business rules, error codes, ports (interfaces)
├── usecase/                     # application orchestration (depends only on domain)
├── repository/postgres/         # PostgreSQL adapter (pgx) + transactor + migrations
├── delivery/http/               # Gin handlers, DTOs, router, swagger route
├── pkg/optional/                # Field[T]: distinguishes undefined / null / value for PATCH
└── testutil/                    # testcontainers PostgreSQL helper
test/component/                  # component tests (E2E within the service)
docs/                            # generated Swagger (swag)
```

Dependency direction: `delivery → usecase → domain ← repository`

- `domain` declares the `ProductRepository` and `Transactor` interfaces. The infrastructure layer implements them, so the inner layers depend on no frameworks.
- Every dependency is injected through constructors (`NewProductUsecase(repo, tx)`, `NewProductHandler(uc, logger)`). The wiring lives in [`internal/app/app.go`](internal/app/app.go).
- PATCH runs inside a transaction with `SELECT ... FOR UPDATE`, so concurrent patches cannot overwrite each other.

---

## Tests

| Level | Location | What it tests | Needs Docker? |
|-------|----------|---------------|---------------|
| Service/Domain unit test (function) | `internal/domain/*_test.go`, `internal/pkg/optional/*_test.go` | validation and patch-merge rules | No |
| Usecase unit test (orchestrator) | `internal/usecase/*_test.go` | flow, transaction use, error paths (mocked repository) | No |
| Repository unit test (integration) | `internal/repository/postgres/*_test.go` | real SQL against PostgreSQL (testcontainers), commit and rollback | Yes |
| Component test (E2E within service) | `test/component/*_test.go` | HTTP → handler → usecase → repository → PostgreSQL, plus Swagger | Yes |

```bash
make test        # run everything (Docker must be running)
make test-unit   # domain + usecase only (-short), no Docker needed
make cover       # coverage
```

To use an existing database instead of testcontainers, set `TEST_DATABASE_URL=postgres://...`

Regenerate Swagger after changing annotations:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
make swag
```
