CREATE TABLE
    IF NOT EXISTS addresses(
        id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
        street_address VARCHAR(255) NOT NULL,
        extended_address VARCHAR(255),
        city VARCHAR(100) NOT NULL,
        state VARCHAR(100) NOT NULL,
        country_code VARCHAR(2) NOT NULL,
        latitude DECIMAL(10,8) NOT NULL,
        longitude DECIMAL(11,8) NOT NULL,
        google_place_id TEXT,
        property_id UUID REFERENCES properties(id)
    );
