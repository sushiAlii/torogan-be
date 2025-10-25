CREATE TABLE
    IF NOT EXISTS auth_providers (
        id SERIAL PRIMARY KEY,
        name VARCHAR(100) NOT NULL,
    )