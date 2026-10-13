-- Money, ledger, and unused supporting tables are out of this slice.
-- People, organisations, memberships, invites, and email tokens stay.

DROP TABLE IF EXISTS simulation_events;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS ussd_sessions;
DROP TABLE IF EXISTS insurance_contributions;
DROP TABLE IF EXISTS tips;
DROP TABLE IF EXISTS qr_badges;
DROP TABLE IF EXISTS park_stays;
DROP TABLE IF EXISTS cash_draws;
DROP TABLE IF EXISTS payouts;
DROP TABLE IF EXISTS split_rules;
DROP TABLE IF EXISTS trip_budget_lines;
DROP TABLE IF EXISTS trip_tourists;
DROP TABLE IF EXISTS trips;
DROP TABLE IF EXISTS funding_intents;
DROP TABLE IF EXISTS fee_schedules;
DROP TABLE IF EXISTS fx_quotes;
DROP TABLE IF EXISTS fx_rates;
DROP TRIGGER IF EXISTS journal_entries_balanced ON journal_entries;
DROP FUNCTION IF EXISTS enforce_journal_balance();
DROP TABLE IF EXISTS journal_entries;
DROP TABLE IF EXISTS journals;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS beneficiaries;

DROP TYPE IF EXISTS simulation_kind;
DROP TYPE IF EXISTS park_stay_status;
DROP TYPE IF EXISTS cash_draw_purpose;
DROP TYPE IF EXISTS payout_status;
DROP TYPE IF EXISTS split_trigger;
DROP TYPE IF EXISTS split_basis;
DROP TYPE IF EXISTS budget_category;
DROP TYPE IF EXISTS trip_status;
DROP TYPE IF EXISTS funding_status;
DROP TYPE IF EXISTS fee_type;
DROP TYPE IF EXISTS entry_direction;
DROP TYPE IF EXISTS journal_type;
DROP TYPE IF EXISTS account_kind;
DROP TYPE IF EXISTS account_owner_type;
DROP TYPE IF EXISTS payout_method;
