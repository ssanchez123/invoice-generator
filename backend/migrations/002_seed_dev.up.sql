-- Seed data for development: one tenant and one customer.
-- Run after migrations.

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