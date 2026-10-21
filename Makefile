# Integration tests via docker-compose (PostgreSQL and local S3 protocol tests)
# Usage:
#   make test              — run unit tests (no Docker needed)
#   make test-integration  — run PostgreSQL integration tests and local S3 protocol tests
#   make test-all          — run both

.PHONY: test test-integration test-all

COMPOSE ?= docker compose

test:
	cd backend && go test ./... -count=1

test-integration:
	$(COMPOSE) -f deploy/docker/docker-compose.test.yml build
	$(COMPOSE) -f deploy/docker/docker-compose.test.yml run --rm integration-tests

test-all: test test-integration