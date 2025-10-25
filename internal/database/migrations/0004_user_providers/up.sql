CREATE TABLE
    IF NOT EXISTS user_providers (
        user_id UUID NOT NULL REFERENCES users (id),
        provider_id INTEGER NOT NULL REFERENCES auth_providers (id),
        sub_id VARCHAR(100) NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (user_id, provider_id),
    );