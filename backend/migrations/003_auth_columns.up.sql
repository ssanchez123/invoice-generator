-- Migration 003: Add auth-related columns to users table
-- The users table already has: id, tenant_id, email, role, password_hash, created_at
-- We add first_name and last_name for display purposes and ensure password_hash is NOT NULL.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS first_name TEXT,
    ADD COLUMN IF NOT EXISTS last_name TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Ensure password_hash is NOT NULL (it should already be, but enforce it)
-- This is safe if all existing rows have a non-null password_hash.
-- If there are null values, they need to be updated first.
UPDATE users SET password_hash = '' WHERE password_hash IS NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;