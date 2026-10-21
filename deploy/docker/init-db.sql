-- Initial database setup
-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Application role (non-superuser).
-- The backend MUST connect with this role: superusers bypass Row-Level
-- Security entirely (ADR-004), so connecting as the bootstrap superuser
-- would silently disable tenant isolation at the database level.
-- Migrations and seeds still run as the admin (bootstrap) user.
CREATE ROLE invoicegen_app LOGIN PASSWORD 'invoicegen_dev' NOSUPERUSER;
CREATE ROLE invoicegen_operator NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;

GRANT CONNECT ON DATABASE invoicegen TO invoicegen_app;
GRANT USAGE ON SCHEMA public TO invoicegen_app;
GRANT CONNECT ON DATABASE invoicegen TO invoicegen_operator;
GRANT USAGE ON SCHEMA public TO invoicegen_operator;

-- Tables/sequences are created later by migrations (run as the admin user),
-- so grant future objects via default privileges...
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO invoicegen_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO invoicegen_app;