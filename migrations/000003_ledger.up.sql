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
