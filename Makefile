ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: help run-api run-scraper build test vet lint tidy fmt migrate-up migrate-down up down logs ps clean

# Tunables overridable on the command line: `make run-api API_PORT=9090`
DATABASE_URL ?= postgres://ottawa:ottawa@localhost:5432/ottawa_waittimes?sslmode=disable
MIGRATIONS_DIR ?= migrations

help:
	@echo "Targets:"
	@echo "  run-api          Run the REST API binary (cmd/api)"
	@echo "  run-scraper      Run the scraper daemon (cmd/scraper)"
	@echo "  build            Compile all packages"
	@echo "  test             Run tests with -race"
	@echo "  vet              go vet ./..."
	@echo "  lint             golangci-lint run"
	@echo "  fmt              gofmt -s -w ."
	@echo "  tidy             go mod tidy"
	@echo "  migrate-up       Apply database migrations"
	@echo "  migrate-down     Roll back last migration"
	@echo "  up               docker compose up -d"
	@echo "  down             docker compose down"
	@echo "  logs             docker compose logs -f"
	@echo "  ps               docker compose ps"

run-api:
	go run ./cmd/api

run-scraper:
	go run ./cmd/scraper

build:
	go build ./...

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	go tool golangci-lint run

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

migrate-up:
	go tool migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	go tool migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

clean:
	go clean -cache -testcache
