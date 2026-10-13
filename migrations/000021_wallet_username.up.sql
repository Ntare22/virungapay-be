ALTER TABLE wallets ADD COLUMN username TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX wallets_username_idx ON wallets (username) WHERE username <> '';

UPDATE wallets
SET username = left(trim(both '-' from regexp_replace(lower(
        COALESCE(
            NULLIF(trim(concat_ws(' ', (SELECT first_name FROM users WHERE users.id = wallets.user_id), (SELECT last_name FROM users WHERE users.id = wallets.user_id))), ''),
            NULLIF(trim((SELECT name FROM organisations WHERE organisations.id = wallets.organisation_id)), ''),
            kind
        )
    ), '[^a-z0-9]+', '-', 'g')), 20)
    || '-' || lower(currency)
    || '-' || left(replace(id::text, '-', ''), 4)
WHERE username = '';
