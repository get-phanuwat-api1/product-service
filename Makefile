.PHONY: up down db run migrate-up migrate-down migrate-status migrate-create test test-unit test-repo test-component cover swag lint fuzz mocks postman

## --- Run ---------------------------------------------------------------

up:            ## start postgres + api (api auto-migrates on start)
	docker compose up --build -d

down:
	docker compose down

db:            ## start only postgres and wait until it is healthy
	docker compose up -d --wait postgres

run: db       ## start postgres, then run the api locally (auto-migrates on start)
	go run ./cmd/api

## --- Migration ---------------------------------------------------------

migrate-up:
	go run ./cmd/migrate up

migrate-down:  ## roll back N steps: make migrate-down N=1
	go run ./cmd/migrate down $(or $(N),1)

migrate-status: ## list applied + pending migrations (schema_migrations history)
	go run ./cmd/migrate status

migrate-create: ## make migrate-create NAME=add_sku_to_products
	go run ./cmd/migrate create $(NAME)

## --- Test --------------------------------------------------------------

test:          ## all tests with the race detector (repository + component need Docker)
	go test -race ./... -count=1

test-unit:     ## unit tests only (no Docker), with the race detector
	go test -race ./... -short -count=1

FUZZTIME ?= 30s
fuzz:          ## run every fuzz test for FUZZTIME each: make fuzz FUZZTIME=2m
	go test ./pkg/optional -run '^$$' -fuzz FuzzField_MatchesPointerDecoding -fuzztime $(FUZZTIME)
	go test ./internal/model -run '^$$' -fuzz FuzzProduct_ApplyPatch -fuzztime $(FUZZTIME)

test-repo:
	go test ./internal/repository/... -count=1 -v

test-component:
	go test ./test/component/... -count=1 -v

cover:
	go test ./... -count=1 -coverpkg=./internal/...,./config/...,./pkg/... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

## --- Tools -------------------------------------------------------------

postman:       ## run the Postman collection against a running service (needs Node.js)
	npx -y newman@6 run postman/product-service.postman_collection.json -e postman/local.postman_environment.json

mocks:         ## regenerate mocks from interfaces (.mockery.yml)
	go run github.com/vektra/mockery/v3@v3.8.0

swag:          ## regenerate swagger (go install github.com/swaggo/swag/cmd/swag@latest)
	swag init -g cmd/api/main.go -o docs --parseInternal

lint:
	go vet ./...
