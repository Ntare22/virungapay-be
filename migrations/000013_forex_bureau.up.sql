-- +migrate NoTransaction
ALTER TYPE organisation_type ADD VALUE IF NOT EXISTS 'forex_bureau';
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'forex_bureau_owner';
