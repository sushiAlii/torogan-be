CREATE TABLE
    IF NOT EXISTS features (
        id SERIAL PRIMARY KEY,
        name VARCHAR(100),
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );

INSERT INTO
    features (name)
VALUES
    ('Swimming Pool'),
    ('Gym'),
    ('Sauna'),
    ('Parking'),
    ('Air Conditioning'),
    ('WiFi'),
    ('Balcony'),
    ('Elevator'),
    ('24/7 Security'),
    ('Pet Friendly'),
    ('Furnished'),
    ('Backup Power');
