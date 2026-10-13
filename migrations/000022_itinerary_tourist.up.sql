ALTER TABLE itineraries ADD COLUMN tourist_user_id UUID REFERENCES users (id);

CREATE INDEX itineraries_tourist_idx ON itineraries (tourist_user_id) WHERE tourist_user_id IS NOT NULL;
