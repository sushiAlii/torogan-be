CREATE TABLE
    IF NOT EXISTS properties (
        id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
        title VARCHAR(255),
        type VARCHAR(50),
        size_sq_m DECIMAL(8,2),
        description TEXT,
        bedrooms INTEGER,
        bathrooms NUMERIC(3,1),
        price NUMERIC(12,2),
        owner_id UUID REFERENCES users(id),
        expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
        is_rented BOOLEAN NOT NULL DEFAULT false,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        deleted_at TIMESTAMP WITH TIME ZONE
    );

CREATE INDEX IF NOT EXISTS idx_properties_owner_id ON properties (owner_id);

-- Serves the public browse filter, which excludes rented and expired
-- listings in one WHERE clause on both columns together.
CREATE INDEX IF NOT EXISTS idx_properties_availability ON properties (is_rented, expires_at);
