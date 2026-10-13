ALTER TABLE users ADD COLUMN name TEXT;

UPDATE users
SET name = btrim(first_name || ' ' || last_name);

ALTER TABLE users ALTER COLUMN name SET NOT NULL;
ALTER TABLE users DROP COLUMN first_name;
ALTER TABLE users DROP COLUMN last_name;
