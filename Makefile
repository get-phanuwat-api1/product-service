.PHONY: run up down db test test-unit test-repo test-component cover swag lint

# Start postgres + api with Docker Compose
up:
	docker compose up --build -d

down:
	docker compose down

# Start only postgres (for running the api locally with `make run`)
db:
	docker compose up -d postgres

run:
	go run ./cmd/api

# All tests (repository + component tests need Docker for testcontainers)
test:
	go test ./... -count=1

# Fast tests only: domain/service + usecase (no Docker needed)
test-unit:
	go test ./... -short -count=1

test-repo:
	go test ./internal/repository/... -count=1 -v

test-component:
	go test ./test/component/... -count=1 -v

cover:
	go test ./... -count=1 -coverpkg=./internal/... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

# Regenerate Swagger docs (go install github.com/swaggo/swag/cmd/swag@latest)
swag:
	swag init -g cmd/api/main.go -o docs --parseInternal

lint:
	go vet ./...
