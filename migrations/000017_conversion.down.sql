ALTER TABLE transactions DROP COLUMN IF EXISTS quote_amount;
ALTER TABLE transactions DROP COLUMN IF EXISTS quote_currency;

ALTER TABLE transactions DROP CONSTRAINT transactions_kind_chk;
ALTER TABLE transactions ADD CONSTRAINT transactions_kind_chk CHECK (kind IN ('collection', 'withdraw', 'p2p'));
