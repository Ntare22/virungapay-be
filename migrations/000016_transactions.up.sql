CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL UNIQUE,
    kind TEXT NOT NULL,
    currency TEXT NOT NULL,
    gross NUMERIC(20, 2) NOT NULL,
    fee NUMERIC(20, 2) NOT NULL,
    net NUMERIC(20, 2) NOT NULL,
    organisation_id UUID REFERENCES organisations (id),
    from_name TEXT NOT NULL DEFAULT '',
    to_name TEXT NOT NULL DEFAULT '',
    counterparty_email TEXT NOT NULL DEFAULT '',
    itinerary_id UUID REFERENCES itineraries (id),
    invoice_number TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT transactions_kind_chk CHECK (kind IN ('collection', 'withdraw', 'p2p')),
    CONSTRAINT transactions_gross_chk CHECK (gross > 0),
    CONSTRAINT transactions_fee_chk CHECK (fee >= 0 AND net >= 0)
);

CREATE INDEX transactions_created_idx ON transactions (created_at DESC);
CREATE INDEX transactions_organisation_idx ON transactions (organisation_id);

INSERT INTO transactions (
    group_id, kind, currency, gross, fee, net, organisation_id,
    from_name, to_name, counterparty_email, itinerary_id, invoice_number, created_at
)
SELECT
    COALESCE(matched.group_id, gen_random_uuid()),
    'collection',
    itinerary.currency,
    itinerary.amount_due,
    COALESCE(matched.fee, 0),
    COALESCE(matched.net, itinerary.amount_due),
    itinerary.organisation_id,
    itinerary.customer_name,
    org.name,
    itinerary.customer_email,
    itinerary.id,
    COALESCE(itinerary.invoice_number, ''),
    COALESCE(itinerary.paid_at, itinerary.updated_at)
FROM itineraries AS itinerary
JOIN organisations AS org ON org.id = itinerary.organisation_id
LEFT JOIN LATERAL (
    SELECT movement.group_id, movement.amount AS net, fee_movement.amount AS fee
    FROM wallet_movements AS movement
    JOIN wallets AS wallet ON wallet.id = movement.wallet_id
    LEFT JOIN wallet_movements AS fee_movement
        ON fee_movement.group_id = movement.group_id
        AND fee_movement.direction = 'in'
        AND fee_movement.wallet_id IN (SELECT id FROM wallets WHERE kind = 'fee_transaction' AND currency = itinerary.currency)
    WHERE wallet.organisation_id = itinerary.organisation_id
      AND movement.kind = 'collection'
      AND movement.direction = 'in'
      AND movement.currency = itinerary.currency
      AND itinerary.paid_at IS NOT NULL
      AND movement.created_at BETWEEN itinerary.paid_at - INTERVAL '2 minutes' AND itinerary.paid_at + INTERVAL '2 minutes'
    ORDER BY ABS(EXTRACT(EPOCH FROM (movement.created_at - itinerary.paid_at)))
    LIMIT 1
) AS matched ON TRUE
WHERE itinerary.status = 'paid'
  AND itinerary.currency IN ('USD', 'EUR');
