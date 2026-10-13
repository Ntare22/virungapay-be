-- Invited drivers can sign in, so user_role gains driver.
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'driver';

ALTER TABLE users
    ADD COLUMN email_verified_at TIMESTAMPTZ;

CREATE TABLE organisation_members (
    organisation_id UUID NOT NULL REFERENCES organisations (id),
    user_id UUID NOT NULL REFERENCES users (id),
    member_role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organisation_id, user_id),
    CONSTRAINT organisation_members_user_unique UNIQUE (user_id),
    CONSTRAINT organisation_members_role_chk CHECK (member_role IN ('operator_owner', 'guide', 'driver'))
);

CREATE INDEX organisation_members_organisation_id_idx ON organisation_members (organisation_id);

CREATE TABLE company_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations (id),
    user_id UUID NOT NULL REFERENCES users (id),
    role TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT company_invites_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT company_invites_role_chk CHECK (role IN ('guide', 'driver'))
);

CREATE INDEX company_invites_organisation_id_idx ON company_invites (organisation_id);

CREATE TABLE password_resets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id),
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT password_resets_token_hash_unique UNIQUE (token_hash)
);

CREATE INDEX password_resets_user_id_idx ON password_resets (user_id);

CREATE TABLE email_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id),
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_verifications_token_hash_unique UNIQUE (token_hash)
);

CREATE INDEX email_verifications_user_id_idx ON email_verifications (user_id);
