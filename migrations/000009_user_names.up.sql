ALTER TABLE users ADD COLUMN first_name TEXT;
ALTER TABLE users ADD COLUMN last_name TEXT;

UPDATE users
SET first_name = split_part(btrim(name), ' ', 1),
    last_name = CASE
        WHEN strpos(btrim(name), ' ') = 0 THEN ''
        ELSE btrim(substr(btrim(name), strpos(btrim(name), ' ') + 1))
    END;

ALTER TABLE users ALTER COLUMN first_name SET NOT NULL;
ALTER TABLE users ALTER COLUMN last_name SET NOT NULL;
ALTER TABLE users DROP COLUMN name;
