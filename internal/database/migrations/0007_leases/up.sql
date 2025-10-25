CREATE TYPE lease_status AS ENUM ('active', 'inactive', 'expired');

CREATE TABLE
    IF NOT EXISTS leases (
        id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
        start_date DATE NOT NULL,
        end_date DATE,
        monthly_rent NUMERIC(12, 2) NOT NULL,
        deposit NUMERIC(12, 2) NOT NULL,
        status lease_status NOT NULL,
        property_id UUID NOT NULL REFERENCES properties (id),
        tenant_id UUID NOT NULL REFERENCES users (id),
        created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    )