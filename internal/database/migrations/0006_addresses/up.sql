CREATE TABLE
    IF NOT EXISTS addresses (
        id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
        unit_number VARCHAR(10),
        street VARCHAR(255) NOT NULL,
        district VARCHAR(255) NOT NULL,
        city VARCHAR(255) NOT NULL,
        province VARCHAR(255) NOT NULL,
        country VARCHAR(255) NOT NULL,
        postal_code VARCHAR(20) NOT NULL,
        latitude FLOAT (9, 6),
        longitude FLOAT (9, 6),
        google_place_id TEXT,
        property_id UUID NOT NULL UNIQUE REFERENCES properties (id),
        created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    )