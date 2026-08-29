-- +goose Up
CREATE TABLE lifehub.documents (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    name text NOT NULL,
    category text NOT NULL,
    notes text,
    expires_on date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT documents_name_length CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    CONSTRAINT documents_category CHECK (category IN ('identity', 'license', 'insurance', 'education', 'work', 'other')),
    CONSTRAINT documents_notes_length CHECK (notes IS NULL OR char_length(notes) <= 5000)
);

CREATE INDEX documents_expiry_idx
    ON lifehub.documents (user_id, expires_on, created_at, id);

-- +goose Down
DROP TABLE IF EXISTS lifehub.documents;
