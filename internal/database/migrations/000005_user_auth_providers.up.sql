CREATE TABLE
    IF NOT EXISTS user_auth_providers (
        user_id UUID REFERENCES users(id) NOT NULL,
        auth_provider_id INTEGER REFERENCES auth_providers(id) NOT NULL,
        sub_id VARCHAR(255),
        PRIMARY KEY (user_id, auth_provider_id)
    );
