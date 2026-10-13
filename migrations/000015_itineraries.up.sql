CREATE TABLE packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations (id),
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT packages_status_chk CHECK (status IN ('draft', 'published'))
);

CREATE INDEX packages_organisation_idx ON packages (organisation_id);

CREATE TABLE package_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id UUID NOT NULL REFERENCES packages (id) ON DELETE CASCADE,
    day_number INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    CONSTRAINT package_days_number_chk CHECK (day_number > 0),
    CONSTRAINT package_days_unique UNIQUE (package_id, day_number)
);

CREATE TABLE package_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id UUID NOT NULL REFERENCES packages (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    body TEXT NOT NULL,
    position INTEGER NOT NULL,
    CONSTRAINT package_items_kind_chk CHECK (kind IN ('inclusion', 'exclusion', 'note')),
    CONSTRAINT package_items_position_chk CHECK (position >= 0)
);

CREATE INDEX package_items_package_idx ON package_items (package_id, position);

CREATE TABLE inquiries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations (id),
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,
    party_size INTEGER NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT inquiries_dates_chk CHECK (ends_on >= starts_on),
    CONSTRAINT inquiries_party_chk CHECK (party_size > 0)
);

CREATE INDEX inquiries_organisation_idx ON inquiries (organisation_id, created_at DESC);

CREATE TABLE itineraries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations (id),
    package_id UUID REFERENCES packages (id),
    inquiry_id UUID REFERENCES inquiries (id),
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,
    party_size INTEGER NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    currency TEXT NOT NULL DEFAULT '',
    amount_due NUMERIC(20, 2) NOT NULL DEFAULT 0,
    invoice_number TEXT,
    pay_token_hash TEXT,
    status TEXT NOT NULL DEFAULT 'draft',
    sent_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT itineraries_status_chk CHECK (status IN ('draft', 'sent', 'paid')),
    CONSTRAINT itineraries_currency_chk CHECK (currency IN ('', 'USD', 'EUR')),
    CONSTRAINT itineraries_amount_chk CHECK (amount_due >= 0),
    CONSTRAINT itineraries_dates_chk CHECK (ends_on >= starts_on),
    CONSTRAINT itineraries_party_chk CHECK (party_size > 0)
);

CREATE UNIQUE INDEX itineraries_invoice_idx ON itineraries (organisation_id, invoice_number) WHERE invoice_number IS NOT NULL;
CREATE UNIQUE INDEX itineraries_pay_token_idx ON itineraries (pay_token_hash) WHERE pay_token_hash IS NOT NULL;
CREATE INDEX itineraries_organisation_idx ON itineraries (organisation_id, created_at DESC);

CREATE TABLE itinerary_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    itinerary_id UUID NOT NULL REFERENCES itineraries (id) ON DELETE CASCADE,
    day_number INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    CONSTRAINT itinerary_days_number_chk CHECK (day_number > 0),
    CONSTRAINT itinerary_days_unique UNIQUE (itinerary_id, day_number)
);

CREATE TABLE itinerary_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    itinerary_id UUID NOT NULL REFERENCES itineraries (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    body TEXT NOT NULL,
    position INTEGER NOT NULL,
    CONSTRAINT itinerary_items_kind_chk CHECK (kind IN ('inclusion', 'exclusion', 'note')),
    CONSTRAINT itinerary_items_position_chk CHECK (position >= 0)
);

CREATE INDEX itinerary_items_itinerary_idx ON itinerary_items (itinerary_id, position);
