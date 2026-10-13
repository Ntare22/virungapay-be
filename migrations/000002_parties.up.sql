-- Parties: people, organisations, and payout beneficiaries.
-- Drivers are beneficiaries of a transport organisation. The login role enum has no
-- driver value, so a driver does not get a users row.

CREATE TYPE user_role AS ENUM (
    'tourist',
    'operator_owner',
    'operator_staff',
    'guide',
    'auto_admin',
    'uhoa_admin',
    'va_ops',
    'va_finance',
    'va_superadmin'
);

CREATE TYPE user_status AS ENUM ('invited', 'active', 'suspended');

CREATE TYPE kyc_status AS ENUM ('none', 'pending', 'verified', 'rejected');

CREATE TYPE organisation_type AS ENUM (
    'operator',
    'lodge',
    'fuel_station',
    'transport_provider'
);

CREATE TYPE association_code AS ENUM ('AUTO', 'UHOA');

CREATE TYPE payout_method AS ENUM ('mobile_money', 'bank', 'wallet');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role user_role NOT NULL,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT,
    password_hash TEXT NOT NULL,
    status user_status NOT NULL DEFAULT 'active',
    kyc_status kyc_status NOT NULL DEFAULT 'none',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_phone_unique UNIQUE (phone)
);

CREATE TABLE organisations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type organisation_type NOT NULL,
    name TEXT NOT NULL,
    country TEXT NOT NULL,
    association_member_of association_code,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT organisations_country_chk CHECK (country ~ '^[A-Z]{2}$')
);

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
