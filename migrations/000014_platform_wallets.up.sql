ALTER TABLE wallets DROP CONSTRAINT wallets_currency_chk;
ALTER TABLE wallets ADD CONSTRAINT wallets_currency_chk CHECK (currency IN ('UGX', 'USD', 'EUR'));

ALTER TABLE wallets ADD COLUMN kind TEXT NOT NULL DEFAULT 'operator';
ALTER TABLE wallets ADD CONSTRAINT wallets_kind_chk CHECK (kind IN ('operator', 'bureau', 'personal', 'ova', 'fee_fx', 'fee_transaction'));

ALTER TABLE wallets ADD COLUMN balance NUMERIC(20, 2) NOT NULL DEFAULT 0;
ALTER TABLE wallets ADD CONSTRAINT wallets_balance_chk CHECK (balance >= 0);

ALTER TABLE wallets ADD COLUMN user_id UUID REFERENCES users (id);
ALTER TABLE wallets ALTER COLUMN organisation_id DROP NOT NULL;
ALTER TABLE wallets ADD CONSTRAINT wallets_owner_chk CHECK (
    (organisation_id IS NOT NULL AND user_id IS NULL AND kind IN ('operator', 'bureau'))
    OR (organisation_id IS NULL AND user_id IS NOT NULL AND kind = 'personal')
    OR (organisation_id IS NULL AND user_id IS NULL AND kind IN ('ova', 'fee_fx', 'fee_transaction'))
);

CREATE UNIQUE INDEX wallets_user_currency_idx ON wallets (user_id, currency) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX wallets_platform_kind_currency_idx ON wallets (kind, currency) WHERE organisation_id IS NULL AND user_id IS NULL;

INSERT INTO wallets (id, kind, currency, balance, is_default)
SELECT gen_random_uuid(), kind, currency, 0, FALSE
FROM (VALUES
    ('ova', 'USD'), ('ova', 'EUR'), ('ova', 'UGX'),
    ('fee_fx', 'USD'), ('fee_fx', 'EUR'), ('fee_fx', 'UGX'),
    ('fee_transaction', 'USD'), ('fee_transaction', 'EUR'), ('fee_transaction', 'UGX')
) AS seed(kind, currency);

CREATE TABLE wallet_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL,
    kind TEXT NOT NULL,
    wallet_id UUID NOT NULL REFERENCES wallets (id),
    direction TEXT NOT NULL,
    amount NUMERIC(20, 2) NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wallet_movements_kind_chk CHECK (kind IN ('collection', 'withdraw', 'p2p', 'conversion')),
    CONSTRAINT wallet_movements_direction_chk CHECK (direction IN ('in', 'out')),
    CONSTRAINT wallet_movements_amount_chk CHECK (amount > 0)
);

CREATE INDEX wallet_movements_group_idx ON wallet_movements (group_id);

CREATE TABLE transaction_fees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL,
    currency TEXT NOT NULL,
    flat NUMERIC(20, 2) NOT NULL DEFAULT 0,
    markup_bps INTEGER NOT NULL DEFAULT 0,
    updated_by UUID REFERENCES users (id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT transaction_fees_kind_chk CHECK (kind IN ('collection', 'withdraw', 'p2p')),
    CONSTRAINT transaction_fees_currency_chk CHECK (currency IN ('USD', 'EUR', 'UGX')),
    CONSTRAINT transaction_fees_flat_chk CHECK (flat >= 0),
    CONSTRAINT transaction_fees_bps_chk CHECK (markup_bps >= 0),
    CONSTRAINT transaction_fees_kind_currency_unique UNIQUE (kind, currency)
);

INSERT INTO transaction_fees (kind, currency)
SELECT kind, currency
FROM (VALUES
    ('collection', 'USD'), ('collection', 'EUR'), ('collection', 'UGX'),
    ('withdraw', 'USD'), ('withdraw', 'EUR'), ('withdraw', 'UGX'),
    ('p2p', 'USD'), ('p2p', 'EUR'), ('p2p', 'UGX')
) AS seed(kind, currency);
