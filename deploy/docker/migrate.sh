#!/bin/sh
# Run pending database migrations in order.
# Expects migration files as /migrations/NNN_*.up.sql (sorted lexicographically).
# Idempotent: skips already-applied migrations via a tracking table.
set -e

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"

DB_SSLMODE="${DB_SSLMODE:-disable}"
case "$DB_SSLMODE" in
    disable|require|verify-ca|verify-full) ;;
    *) echo "invalid DB_SSLMODE" >&2; exit 1 ;;
esac
if [ "${ENVIRONMENT:-development}" = "production" ]; then
    if [ "$DB_SSLMODE" = "disable" ] || [ "$DB_PASSWORD" = "invoicegen_dev" ]; then
        echo "production migrations require database TLS and a non-development password" >&2
        exit 1
    fi
fi

export PGPASSWORD="$DB_PASSWORD"
export PGSSLMODE="$DB_SSLMODE"
if [ -n "${DB_SSLROOTCERT:-}" ]; then
    export PGSSLROOTCERT="$DB_SSLROOTCERT"
fi
run_psql() {
    psql --host="$DB_HOST" --port="$DB_PORT" --username="$DB_USER" --dbname="$DB_NAME" "$@"
}

# Ensure the schema_migrations tracking table exists
run_psql <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version  TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

# Apply each .up.sql migration that hasn't been applied yet
for file in /migrations/*.up.sql; do
    [ -f "$file" ] || continue
    version=$(basename "$file" .up.sql)

    applied=$(run_psql -tAc "SELECT 1 FROM schema_migrations WHERE version = '$version'" 2>/dev/null || echo "")
    if [ "$applied" = "1" ]; then
        echo "migration $version: already applied, skipping"
        continue
    fi

    echo "migration $version: applying..."
    run_psql -v ON_ERROR_STOP=1 -f "$file"
    run_psql -c "INSERT INTO schema_migrations (version) VALUES ('$version')"
    echo "migration $version: done"
done

echo "all migrations applied"