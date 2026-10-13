DROP TABLE IF EXISTS email_verifications;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS company_invites;
DROP TABLE IF EXISTS organisation_members;

ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;

-- PostgreSQL cannot drop a single enum value. user_role 'driver' stays.
