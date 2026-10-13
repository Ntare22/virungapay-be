DROP TRIGGER IF EXISTS journal_entries_balanced ON journal_entries;
DROP FUNCTION IF EXISTS enforce_journal_balance();
DROP TABLE IF EXISTS journal_entries;
DROP TABLE IF EXISTS journals;
DROP TABLE IF EXISTS accounts;
DROP TYPE IF EXISTS entry_direction;
DROP TYPE IF EXISTS journal_type;
DROP TYPE IF EXISTS account_kind;
DROP TYPE IF EXISTS account_owner_type;
