CREATE TABLE
    IF NOT EXISTS users (
        id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
        email VARCHAR(100) NOT NULL UNIQUE,
        password TEXT NOT NULL,
        avatar_url TEXT,
        role_id INTEGER NOT NULL REFERENCES roles (id),
        created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    )