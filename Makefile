.PHONY: up down db run migrate-up migrate-down migrate-version migrate-create test test-unit test-repo test-component cover swag lint fuzz mocks

## --- Run ---------------------------------------------------------------

up:            ## start postgres + api (api auto-migrates on start)
	docker compose up --build -d

down:
	docker compose down

db:            ## start only postgres (for `make run`)
	docker compose up -d postgres

run:           ## run the api locally (reads config/config.yaml, auto-migrates on start)
	go run ./cmd/api

## --- Migration ---------------------------------------------------------

migrate-up:
	go run ./cmd/migrate up

migrate-down:  ## roll back N steps: make migrate-down N=1
	go run ./cmd/migrate down $(or $(N),1)

migrate-version:
	go run ./cmd/migrate version

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

mocks:         ## regenerate mocks from interfaces (.mockery.yml)
	go run github.com/vektra/mockery/v3@v3.8.0

swag:          ## regenerate swagger (go install github.com/swaggo/swag/cmd/swag@latest)
	swag init -g cmd/api/main.go -o docs --parseInternal

lint:
	go vet ./...
