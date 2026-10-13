ALTER TABLE fx_rates DROP CONSTRAINT fx_rates_pair_chk;
ALTER TABLE fx_rates ADD CONSTRAINT fx_rates_pair_chk CHECK (pair IN ('USD/UGX', 'EUR/UGX'));
