#!/bin/sh
# Run pending database migrations in order.
# Expects migration files as /migrations/NNN_*.up.sql (sorted lexicographically).
# Idempotent: skips already-applied migrations via a tracking table.
set -e

DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

# Ensure the schema_migrations tracking table exists
psql "$DB_URL" <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version  TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

# Apply each .up.sql migration that hasn't been applied yet
for file in /migrations/*.up.sql; do
    [ -f "$file" ] || continue
    version=$(basename "$file" .up.sql)

    applied=$(psql -tAc "SELECT 1 FROM schema_migrations WHERE version = '$version'" "$DB_URL" 2>/dev/null || echo "")
    if [ "$applied" = "1" ]; then
        echo "migration $version: already applied, skipping"
        continue
    fi

    echo "migration $version: applying..."
    psql -v ON_ERROR_STOP=1 -f "$file" "$DB_URL"
    psql -c "INSERT INTO schema_migrations (version) VALUES ('$version')" "$DB_URL"
    echo "migration $version: done"
done

echo "all migrations applied"