CREATE TYPE property_type AS ENUM ('house', 'apartment', 'condo');

CREATE TABLE
    IF NOT EXISTS properties (
        id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
        nickname VARCHAR(255) NOT NULL,
        size_sq_m INTEGER NOT NULL,
        type property_type NOT NULL,
        description VARCHAR(255),
        price NUMERIC(10, 2) NOT NULL,
        min_contract_months INTEGER NOT NULL,
        owner_id UUID NOT NULL REFERENCES users (id),
        created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    )