-- Restores the tables removed by 000008. Users and organisations already exist.

CREATE TABLE idempotency_keys (
    idempotency_key TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    response_body BYTEA NOT NULL DEFAULT ''::bytea,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TYPE payout_method AS ENUM ('mobile_money', 'bank', 'wallet');

CREATE TABLE beneficiaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name TEXT NOT NULL,
    organisation_id UUID REFERENCES organisations (id),
    user_id UUID REFERENCES users (id),
    payout_method payout_method NOT NULL,
    payout_account TEXT NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT beneficiaries_owner_chk CHECK (
        (organisation_id IS NOT NULL AND user_id IS NULL)
        OR (organisation_id IS NULL AND user_id IS NOT NULL)
    ),
    CONSTRAINT beneficiaries_currency_chk CHECK (currency IN ('UGX', 'USD')),
    CONSTRAINT beneficiaries_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX beneficiaries_organisation_id_idx ON beneficiaries (organisation_id);
CREATE INDEX beneficiaries_user_id_idx ON beneficiaries (user_id);

-- Ledger. Platform accounts (revenue, insurance, clearing, FX, tips, cash float)
-- use owner_type platform and a null owner_id: they are not owned by a user or organisation.
-- Amounts are int8 minor units. USD and CAD use cents. UGX uses whole shillings (ISO exponent 0).

CREATE TYPE account_owner_type AS ENUM ('user', 'organisation', 'platform');

CREATE TYPE account_kind AS ENUM (
    'virungapass',
    'virungatrip',
    'beneficiary',
    'revenue_fx',
    'revenue_fees',
    'insurance_fund',
    'tips',
    'cash_float',
    'provider_clearing',
    'fx_position'
);

CREATE TYPE journal_type AS ENUM (
    'funding',
    'payout',
    'fx',
    'fee',
    'insurance',
    'tip',
    'cash_draw',
    'adjustment'
);

CREATE TYPE entry_direction AS ENUM ('debit', 'credit');

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type account_owner_type NOT NULL,
    owner_id UUID,
    kind account_kind NOT NULL,
    currency TEXT NOT NULL,
    cached_balance BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT accounts_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT accounts_owner_chk CHECK (
        (owner_type = 'platform' AND owner_id IS NULL)
        OR (owner_type <> 'platform' AND owner_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX accounts_owner_kind_currency_uidx
    ON accounts (owner_type, owner_id, kind, currency)
    WHERE owner_id IS NOT NULL;

CREATE UNIQUE INDEX accounts_platform_kind_currency_uidx
    ON accounts (kind, currency)
    WHERE owner_type = 'platform';

CREATE TABLE journals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type journal_type NOT NULL,
    idempotency_key TEXT,
    reference TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT journals_idempotency_key_unique UNIQUE (idempotency_key)
);

CREATE TABLE journal_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_id UUID NOT NULL REFERENCES journals (id),
    account_id UUID NOT NULL REFERENCES accounts (id),
    direction entry_direction NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    CONSTRAINT journal_entries_amount_chk CHECK (amount > 0),
    CONSTRAINT journal_entries_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX journal_entries_journal_id_idx ON journal_entries (journal_id);
CREATE INDEX journal_entries_account_id_idx ON journal_entries (account_id);

-- A row check cannot see sibling lines. This deferred constraint trigger runs at
-- commit, after every line of the journal has been written.
CREATE OR REPLACE FUNCTION enforce_journal_balance() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    jid UUID;
    unbalanced INT;
BEGIN
    jid := COALESCE(NEW.journal_id, OLD.journal_id);

    IF EXISTS (
        SELECT 1
        FROM journal_entries AS entry
        JOIN accounts AS account ON account.id = entry.account_id
        WHERE entry.journal_id = jid
          AND entry.currency <> account.currency
    ) THEN
        RAISE EXCEPTION 'journal entry currency does not match account'
            USING ERRCODE = '23514';
    END IF;

    SELECT COUNT(*) INTO unbalanced
    FROM (
        SELECT currency
        FROM journal_entries
        WHERE journal_id = jid
        GROUP BY currency
        HAVING SUM(CASE WHEN direction = 'debit' THEN amount ELSE 0 END)
            <> SUM(CASE WHEN direction = 'credit' THEN amount ELSE 0 END)
    ) AS bad;

    IF unbalanced > 0 THEN
        RAISE EXCEPTION 'unbalanced journal %', jid
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER journal_entries_balanced
AFTER INSERT OR UPDATE OR DELETE ON journal_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION enforce_journal_balance();

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

CREATE TYPE simulation_kind AS ENUM ('failure', 'delay');

CREATE TABLE insurance_contributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_id UUID NOT NULL REFERENCES journals (id),
    fund_account_id UUID NOT NULL REFERENCES accounts (id),
    member_organisation_id UUID NOT NULL REFERENCES organisations (id),
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT insurance_contributions_amount_chk CHECK (amount > 0),
    CONSTRAINT insurance_contributions_currency_iso_chk CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX insurance_contributions_member_organisation_id_idx
    ON insurance_contributions (member_organisation_id);

CREATE TABLE ussd_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    msisdn TEXT NOT NULL,
    state TEXT NOT NULL,
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ussd_sessions_msisdn_idx ON ussd_sessions (msisdn);

CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users (id),
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    before JSONB,
    after JSONB,
    ip TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id);
CREATE INDEX audit_log_actor_user_id_idx ON audit_log (actor_user_id);

CREATE TABLE simulation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users (id),
    kind simulation_kind NOT NULL,
    target TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
