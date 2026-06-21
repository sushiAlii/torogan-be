CREATE TABLE
    IF NOT EXISTS properties (
        id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
        title VARCHAR(255),
        size_sq_m DECIMAL(8,2),
        description TEXT,
        price NUMERIC(12,2),
        owner_id UUID REFERENCES users(id)
    );
