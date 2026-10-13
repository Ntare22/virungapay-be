ALTER TABLE transactions DROP CONSTRAINT transactions_kind_chk;
ALTER TABLE transactions ADD CONSTRAINT transactions_kind_chk CHECK (kind IN ('collection', 'withdraw', 'p2p', 'conversion'));

ALTER TABLE transactions ADD COLUMN quote_currency TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN quote_amount NUMERIC(20, 2);
