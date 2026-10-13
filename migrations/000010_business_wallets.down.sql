DROP TABLE IF EXISTS fx_rates;
DROP TABLE IF EXISTS wallets;
ALTER TABLE organisations DROP COLUMN IF EXISTS status;
DROP TYPE IF EXISTS organisation_status;
