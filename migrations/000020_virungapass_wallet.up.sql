ALTER TABLE wallets ADD COLUMN name TEXT NOT NULL DEFAULT '';

ALTER TABLE wallets DROP CONSTRAINT wallets_kind_chk;
ALTER TABLE wallets ADD CONSTRAINT wallets_kind_chk CHECK (
    kind IN ('operator', 'bureau', 'personal', 'virungapass', 'ova', 'fee_fx', 'fee_transaction')
);

ALTER TABLE wallets DROP CONSTRAINT wallets_owner_chk;
ALTER TABLE wallets ADD CONSTRAINT wallets_owner_chk CHECK (
    (organisation_id IS NOT NULL AND user_id IS NULL AND kind IN ('operator', 'bureau'))
    OR (organisation_id IS NULL AND user_id IS NOT NULL AND kind IN ('personal', 'virungapass'))
    OR (organisation_id IS NULL AND user_id IS NULL AND kind IN ('ova', 'fee_fx', 'fee_transaction'))
);

CREATE UNIQUE INDEX wallets_one_virungapass_idx ON wallets (user_id) WHERE kind = 'virungapass';

INSERT INTO wallets (id, user_id, kind, currency, name, balance, is_default)
SELECT gen_random_uuid(), users.id, 'virungapass', 'UGX', 'VirungaPass Wallet', 0, TRUE
FROM users
WHERE users.role = 'tourist'
  AND NOT EXISTS (
      SELECT 1 FROM wallets WHERE wallets.user_id = users.id AND wallets.kind = 'virungapass'
  );
