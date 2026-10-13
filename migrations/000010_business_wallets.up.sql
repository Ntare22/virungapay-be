CREATE TYPE organisation_status AS ENUM ('pending', 'approved');

ALTER TABLE organisations
    ADD COLUMN status organisation_status NOT NULL DEFAULT 'pending';

CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations (id),
    currency TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wallets_currency_chk CHECK (currency IN ('UGX', 'USD')),
    CONSTRAINT wallets_org_currency_unique UNIQUE (organisation_id, currency)
);

CREATE UNIQUE INDEX wallets_one_default_idx ON wallets (organisation_id) WHERE is_default;

CREATE TABLE fx_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pair TEXT NOT NULL DEFAULT 'USD/UGX',
    quote_on DATE NOT NULL,
    integrator_rate NUMERIC(20, 8) NOT NULL,
    markup_bps INTEGER NOT NULL,
    customer_rate NUMERIC(20, 8) NOT NULL,
    set_by UUID NOT NULL REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fx_rates_pair_chk CHECK (pair = 'USD/UGX'),
    CONSTRAINT fx_rates_positive_chk CHECK (integrator_rate > 0 AND customer_rate > 0),
    CONSTRAINT fx_rates_markup_chk CHECK (markup_bps >= 0),
    CONSTRAINT fx_rates_pair_day_unique UNIQUE (pair, quote_on)
);
