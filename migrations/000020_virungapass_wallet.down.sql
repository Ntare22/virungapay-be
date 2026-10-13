DROP INDEX IF EXISTS wallets_one_virungapass_idx;

DELETE FROM wallets WHERE kind = 'virungapass';

ALTER TABLE wallets DROP CONSTRAINT wallets_owner_chk;
ALTER TABLE wallets ADD CONSTRAINT wallets_owner_chk CHECK (
    (organisation_id IS NOT NULL AND user_id IS NULL AND kind IN ('operator', 'bureau'))
    OR (organisation_id IS NULL AND user_id IS NOT NULL AND kind = 'personal')
    OR (organisation_id IS NULL AND user_id IS NULL AND kind IN ('ova', 'fee_fx', 'fee_transaction'))
);

ALTER TABLE wallets DROP CONSTRAINT wallets_kind_chk;
ALTER TABLE wallets ADD CONSTRAINT wallets_kind_chk CHECK (
    kind IN ('operator', 'bureau', 'personal', 'ova', 'fee_fx', 'fee_transaction')
);

ALTER TABLE wallets DROP COLUMN IF EXISTS name;
