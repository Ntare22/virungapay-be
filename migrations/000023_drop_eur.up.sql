DELETE FROM fx_rates WHERE pair = 'EUR/UGX';

DELETE FROM transaction_fees WHERE currency = 'EUR';

DELETE FROM wallets WHERE currency = 'EUR';

ALTER TABLE wallets DROP CONSTRAINT wallets_currency_chk;
ALTER TABLE wallets ADD CONSTRAINT wallets_currency_chk CHECK (currency IN ('UGX', 'USD'));

ALTER TABLE fx_rates DROP CONSTRAINT fx_rates_pair_chk;
ALTER TABLE fx_rates ADD CONSTRAINT fx_rates_pair_chk CHECK (pair = 'USD/UGX');

ALTER TABLE transaction_fees DROP CONSTRAINT transaction_fees_currency_chk;
ALTER TABLE transaction_fees ADD CONSTRAINT transaction_fees_currency_chk CHECK (currency IN ('USD', 'UGX'));

ALTER TABLE itineraries DROP CONSTRAINT itineraries_currency_chk;
ALTER TABLE itineraries ADD CONSTRAINT itineraries_currency_chk CHECK (currency IN ('', 'USD'));
