#!/bin/bash
# Dev setup script: starts PostgreSQL, runs migrations, and starts API
set -e

echo "=== Invoice Generator — Dev Setup ==="

# Start dependencies
echo "1. Starting Docker services..."
docker compose -f deploy/docker/docker-compose.yml up -d postgres

# Wait for Postgres
echo "2. Waiting for PostgreSQL..."
until docker compose -f deploy/docker/docker-compose.yml exec -T postgres pg_isready -U invoicegen 2>/dev/null; do
  sleep 1
done
echo "   PostgreSQL is ready."

# Run migrations
echo "3. Running migrations..."
# In a real setup: migrate -path backend/migrations -database "postgres://..." up
echo "   (migrations would run here)"

# Start API
echo "4. Starting API server (port 8080)..."
cd backend && go run cmd/api/main.go