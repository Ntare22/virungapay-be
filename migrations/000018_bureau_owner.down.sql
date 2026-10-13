ALTER TABLE company_invites DROP CONSTRAINT company_invites_role_chk;
ALTER TABLE company_invites ADD CONSTRAINT company_invites_role_chk CHECK (role IN ('guide', 'driver'));

ALTER TABLE organisation_members DROP CONSTRAINT organisation_members_role_chk;
ALTER TABLE organisation_members ADD CONSTRAINT organisation_members_role_chk CHECK (member_role IN ('operator_owner', 'guide', 'driver'));
