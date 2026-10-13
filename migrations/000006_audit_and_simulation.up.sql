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
