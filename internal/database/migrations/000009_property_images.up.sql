CREATE TABLE
    IF NOT EXISTS property_images (
        id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
        property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
        url TEXT NOT NULL,
        is_main BOOLEAN NOT NULL DEFAULT false,
        position INT NOT NULL DEFAULT 0,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );

CREATE INDEX IF NOT EXISTS property_images_property_id_idx ON property_images (property_id);

CREATE UNIQUE INDEX IF NOT EXISTS property_images_one_main_idx ON property_images (property_id)
WHERE
    is_main;
