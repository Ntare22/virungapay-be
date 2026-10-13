DROP INDEX IF EXISTS itineraries_tourist_idx;

ALTER TABLE itineraries DROP COLUMN IF EXISTS tourist_user_id;
