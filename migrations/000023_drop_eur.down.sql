ALTER TABLE wallets DROP CONSTRAINT wallets_currency_chk;
ALTER TABLE wallets ADD CONSTRAINT wallets_currency_chk CHECK (currency IN ('UGX', 'USD', 'EUR'));

ALTER TABLE fx_rates DROP CONSTRAINT fx_rates_pair_chk;
ALTER TABLE fx_rates ADD CONSTRAINT fx_rates_pair_chk CHECK (pair IN ('USD/UGX', 'EUR/UGX'));

ALTER TABLE transaction_fees DROP CONSTRAINT transaction_fees_currency_chk;
ALTER TABLE transaction_fees ADD CONSTRAINT transaction_fees_currency_chk CHECK (currency IN ('USD', 'EUR', 'UGX'));

ALTER TABLE itineraries DROP CONSTRAINT itineraries_currency_chk;
ALTER TABLE itineraries ADD CONSTRAINT itineraries_currency_chk CHECK (currency IN ('', 'USD', 'EUR'));

INSERT INTO wallets (id, kind, currency, balance, is_default)
SELECT gen_random_uuid(), kind, currency, 0, FALSE
FROM (VALUES
    ('ova', 'EUR'),
    ('fee_fx', 'EUR'),
    ('fee_transaction', 'EUR')
) AS seed(kind, currency)
WHERE NOT EXISTS (
    SELECT 1 FROM wallets WHERE wallets.kind = seed.kind AND wallets.currency = seed.currency
);

INSERT INTO transaction_fees (kind, currency)
SELECT kind, currency
FROM (VALUES
    ('collection', 'EUR'),
    ('withdraw', 'EUR'),
    ('p2p', 'EUR')
) AS seed(kind, currency)
WHERE NOT EXISTS (
    SELECT 1 FROM transaction_fees WHERE transaction_fees.kind = seed.kind AND transaction_fees.currency = seed.currency
);
