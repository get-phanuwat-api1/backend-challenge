.PHONY: up down db run migrate-up migrate-down migrate-status migrate-create test test-unit test-repo test-component cover swag proto lint postman

# Development-only secret used by `make run` when JWT_SECRET is not set.
DEV_JWT_SECRET ?= dev-only-secret-change-me-0123456789abcdef
PROTOC_INCLUDE ?= $(shell brew --prefix protobuf@3 2>/dev/null)/include

## --- Run ---------------------------------------------------------------

up:            ## start mongo + api (api auto-migrates on start)
	docker compose up --build -d

down:
	docker compose down

db:            ## start only mongo and wait until it is healthy
	docker compose up -d --wait mongo

run: db       ## start mongo, then run the api locally (auto-migrates on start)
	JWT_SECRET=$${JWT_SECRET:-$(DEV_JWT_SECRET)} go run ./cmd/api

## --- Migration ---------------------------------------------------------
# The API applies pending migrations on every start (database.auto_migrate),
# i.e. on every deploy. These targets are for running them by hand.

migrate-up:
	JWT_SECRET=$${JWT_SECRET:-$(DEV_JWT_SECRET)} go run ./cmd/migrate up

migrate-down:  ## roll back N steps: make migrate-down N=1
	JWT_SECRET=$${JWT_SECRET:-$(DEV_JWT_SECRET)} go run ./cmd/migrate down $(or $(N),1)

migrate-status: ## list applied + pending migrations (schema_migrations history)
	JWT_SECRET=$${JWT_SECRET:-$(DEV_JWT_SECRET)} go run ./cmd/migrate status

migrate-create: ## make migrate-create NAME=add_phone_index
	go run ./cmd/migrate create $(NAME)

## --- Test --------------------------------------------------------------

test:          ## all tests with the race detector (repository + component need Docker)
	go test -race ./... -count=1

test-unit:     ## unit tests only (no Docker), with the race detector
	go test -race ./... -short -count=1

test-repo:
	go test ./internal/infrastructure/mongodb/... -count=1 -v

test-component:
	go test ./test/component/... -count=1 -v

cover:
	go test ./... -count=1 -coverpkg=./internal/...,./config/...,./pkg/... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

## --- Tools -------------------------------------------------------------

postman:       ## run the Postman collection against a running service (needs Node.js)
	npx -y newman@6 run postman/user-service.postman_collection.json -e postman/local.postman_environment.json

swag:          ## regenerate swagger (go install github.com/swaggo/swag/cmd/swag@latest)
	swag init -g cmd/api/main.go -o docs --parseInternal

proto:         ## regenerate gRPC code (needs protoc, protoc-gen-go, protoc-gen-go-grpc)
	protoc -I proto -I $(PROTOC_INCLUDE) \
		--go_out=proto --go_opt=paths=source_relative \
		--go-grpc_out=proto --go-grpc_opt=paths=source_relative \
		user/v1/user.proto

lint:
	go vet ./...
