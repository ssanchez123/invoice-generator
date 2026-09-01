# Integration tests via docker-compose
# Usage:
#   make test              — run unit tests (no Docker needed)
#   make test-integration  — run integration tests with PostgreSQL in Docker
#   make test-all          — run both

.PHONY: test test-integration test-all

test:
	cd backend && go test ./... -count=1

test-integration:
	docker compose -f deploy/docker/docker-compose.test.yml build
	docker compose -f deploy/docker/docker-compose.test.yml run --rm integration-tests

test-all: test test-integration