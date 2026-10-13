-- Rates and quotes are numeric, never float.
-- fee_schedules.markup_bps is VirungaPay's markup. The operator's own markup is not stored here.

CREATE TYPE fee_type AS ENUM ('card_funding', 'wallet_outflow', 'insurance');

CREATE TYPE funding_status AS ENUM (
    'created',
    'quoted',
    'authorized',
    'captured',
    'failed',
    'cancelled'
);

CREATE TABLE fx_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pair TEXT NOT NULL,
    mid NUMERIC(20, 8) NOT NULL,
    buy NUMERIC(20, 8) NOT NULL,
    sell NUMERIC(20, 8) NOT NULL,
    source TEXT NOT NULL,
    valid_from TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fx_rates_pair_chk CHECK (pair ~ '^[A-Z]{3}/[A-Z]{3}$'),
    CONSTRAINT fx_rates_positive_chk CHECK (mid > 0 AND buy > 0 AND sell > 0)
);

CREATE INDEX fx_rates_pair_valid_from_idx ON fx_rates (pair, valid_from DESC);

CREATE TABLE fx_quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id),
    pair TEXT NOT NULL,
    rate NUMERIC(20, 8) NOT NULL,
    spread_bps INTEGER NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    locked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fx_quotes_pair_chk CHECK (pair ~ '^[A-Z]{3}/[A-Z]{3}$'),
    CONSTRAINT fx_quotes_rate_chk CHECK (rate > 0),
    CONSTRAINT fx_quotes_spread_chk CHECK (spread_bps >= 0)
);

CREATE INDEX fx_quotes_user_id_idx ON fx_quotes (user_id);

CREATE TABLE fee_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type fee_type NOT NULL,
    flat BIGINT NOT NULL,
    percent_bps INTEGER NOT NULL,
    markup_bps INTEGER NOT NULL,
    currency TEXT NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fee_schedules_non_negative_chk CHECK (
        flat >= 0 AND percent_bps >= 0 AND markup_bps >= 0
    ),
    CONSTRAINT fee_schedules_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT fee_schedules_type_currency_effective_unique UNIQUE (type, currency, effective_from)
);

CREATE TABLE funding_intents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tourist_user_id UUID NOT NULL REFERENCES users (id),
    card_currency TEXT NOT NULL,
    amount BIGINT NOT NULL,
    fx_quote_id UUID REFERENCES fx_quotes (id),
    status funding_status NOT NULL DEFAULT 'created',
    provider_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT funding_intents_amount_chk CHECK (amount > 0),
    CONSTRAINT funding_intents_currency_iso_chk CHECK (card_currency ~ '^[A-Z]{3}$')
);

CREATE INDEX funding_intents_tourist_user_id_idx ON funding_intents (tourist_user_id);
