-- Seed data for development: one tenant, one customer, and one admin user.
-- Run after migrations.
-- NOTE: must run AFTER 002_auth_columns (seeded user uses first_name/last_name).

-- Temporarily bypass RLS for seeding
SET LOCAL row_security = off;

INSERT INTO tenants (id, name, address, tax_id, settings, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Acme Corp',
    '{"line1":"Calle 123","city":"Bogotá","country":"CO","postcode":"110111"}'::jsonb,
    '900123456-7',
    '{"default_currency":"COP","invoice_prefix":"INV"}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO customers (id, tenant_id, name, email, phone, address, tax_id, metadata, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000001',
    'Cliente Demo S.A.S',
    'demo@cliente.com',
    '+57 300 123 4567',
    '{"line1":"Av Cr 7 #100-20","city":"Medellín","country":"CO","postcode":"050010"}'::jsonb,
    '901234567-8',
    '{}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;
-- Seed admin user for development
-- Email: admin@acme.com  Password: Password123!
INSERT INTO users (id, tenant_id, email, role, password_hash, first_name, last_name, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000100',
    '00000000-0000-0000-0000-000000000001',
    'admin@acme.com',
    'admin',
    '$2a$10$98DhV48j.U7xXZI9yBQhTumJhjPGq/3Yzwmkbh0XlNDLtIdDQ93SS',
    'Admin',
    'User',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;
