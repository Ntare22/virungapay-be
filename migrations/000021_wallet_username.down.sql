DROP INDEX IF EXISTS wallets_username_idx;
ALTER TABLE wallets DROP COLUMN IF EXISTS username;
