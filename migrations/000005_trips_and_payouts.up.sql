-- A trip has many tourists, so membership lives in trip_tourists.
-- split_rules.value is minor units when basis is fixed, and basis points when basis is percent.
-- The column is trigger_on because TRIGGER is a reserved word.

CREATE TYPE trip_status AS ENUM ('draft', 'confirmed', 'in_progress', 'completed', 'cancelled');

CREATE TYPE budget_category AS ENUM (
    'lodge',
    'driver',
    'fuel',
    'guide',
    'park_fee_cash',
    'other'
);

CREATE TYPE split_basis AS ENUM ('fixed', 'percent');

CREATE TYPE split_trigger AS ENUM ('on_funding', 'on_milestone', 'manual');

CREATE TYPE payout_status AS ENUM ('pending', 'submitted', 'succeeded', 'failed');

CREATE TYPE cash_draw_purpose AS ENUM ('park_fee');

CREATE TYPE park_stay_status AS ENUM ('quoted', 'confirmed', 'cancelled');

CREATE TABLE trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_organisation_id UUID NOT NULL REFERENCES organisations (id),
    title TEXT NOT NULL,
    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,
    currency TEXT NOT NULL,
    budget_total BIGINT NOT NULL,
    status trip_status NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT trips_dates_chk CHECK (ends_on >= starts_on),
    CONSTRAINT trips_budget_chk CHECK (budget_total >= 0),
    CONSTRAINT trips_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE TABLE trip_tourists (
    trip_id UUID NOT NULL REFERENCES trips (id),
    user_id UUID NOT NULL REFERENCES users (id),
    PRIMARY KEY (trip_id, user_id)
);

CREATE TABLE trip_budget_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips (id),
    category budget_category NOT NULL,
    planned_amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    beneficiary_id UUID REFERENCES beneficiaries (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT trip_budget_lines_amount_chk CHECK (planned_amount >= 0),
    CONSTRAINT trip_budget_lines_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX trip_budget_lines_trip_id_idx ON trip_budget_lines (trip_id);

CREATE TABLE split_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips (id),
    beneficiary_id UUID NOT NULL REFERENCES beneficiaries (id),
    basis split_basis NOT NULL,
    value BIGINT NOT NULL,
    trigger_on split_trigger NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT split_rules_value_chk CHECK (value > 0)
);

CREATE INDEX split_rules_trip_id_idx ON split_rules (trip_id);

CREATE TABLE payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips (id),
    beneficiary_id UUID NOT NULL REFERENCES beneficiaries (id),
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    fee BIGINT NOT NULL DEFAULT 0,
    markup BIGINT NOT NULL DEFAULT 0,
    fx_quote_id UUID REFERENCES fx_quotes (id),
    status payout_status NOT NULL DEFAULT 'pending',
    provider_ref TEXT,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payouts_amount_chk CHECK (amount > 0),
    CONSTRAINT payouts_fee_chk CHECK (fee >= 0 AND markup >= 0),
    CONSTRAINT payouts_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX payouts_trip_id_idx ON payouts (trip_id);

CREATE TABLE cash_draws (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips (id),
    operator_organisation_id UUID NOT NULL REFERENCES organisations (id),
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    purpose cash_draw_purpose NOT NULL,
    receipt_ref TEXT,
    receipt_url TEXT,
    reconciled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT cash_draws_amount_chk CHECK (amount > 0),
    CONSTRAINT cash_draws_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX cash_draws_trip_id_idx ON cash_draws (trip_id);

CREATE TABLE park_stays (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID REFERENCES trips (id),
    permit_no TEXT NOT NULL,
    park TEXT NOT NULL,
    visitors INTEGER NOT NULL,
    fee_amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    status park_stay_status NOT NULL DEFAULT 'quoted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT park_stays_permit_unique UNIQUE (permit_no),
    CONSTRAINT park_stays_visitors_chk CHECK (visitors > 0),
    CONSTRAINT park_stays_fee_chk CHECK (fee_amount >= 0),
    CONSTRAINT park_stays_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE TABLE qr_badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guide_user_id UUID NOT NULL REFERENCES users (id),
    code TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT qr_badges_code_unique UNIQUE (code)
);

CREATE INDEX qr_badges_guide_user_id_idx ON qr_badges (guide_user_id);

CREATE TABLE tips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guide_user_id UUID NOT NULL REFERENCES users (id),
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    tipper_user_id UUID REFERENCES users (id),
    qr_badge_id UUID NOT NULL REFERENCES qr_badges (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tips_amount_chk CHECK (amount > 0),
    CONSTRAINT tips_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);
