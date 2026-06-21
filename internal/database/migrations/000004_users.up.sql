CREATE TABLE
    IF NOT EXISTS users (
        id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
        email VARCHAR(100),
        password BYTEA NOT NULL,
        avatar_url TEXT,
        role_id INTEGER REFERENCES roles(id) UNIQUE
    )

