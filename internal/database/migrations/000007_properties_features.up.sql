CREATE TABLE
    IF NOT EXISTS properties_features (
        property_id UUID REFERENCES properties(id) NOT NULL,
        feature_id INTEGER REFERENCES features(id) NOT NULL,
        PRIMARY KEY(property_id, feature_id)
    );
