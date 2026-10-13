-- +migrate NoTransaction
ALTER TYPE organisation_status ADD VALUE IF NOT EXISTS 'disabled';
