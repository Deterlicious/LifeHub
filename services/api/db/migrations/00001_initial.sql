-- +goose Up
CREATE SCHEMA IF NOT EXISTS lifehub;

CREATE TABLE lifehub.profiles (
    user_id uuid PRIMARY KEY,
    timezone text,
    locale text NOT NULL DEFAULT 'id-ID',
    currency text NOT NULL DEFAULT 'IDR',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT profiles_timezone_length CHECK (timezone IS NULL OR char_length(timezone) BETWEEN 1 AND 100),
    CONSTRAINT profiles_locale_length CHECK (char_length(locale) BETWEEN 2 AND 20),
    CONSTRAINT profiles_currency_format CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE TABLE lifehub.tasks (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    title text NOT NULL,
    notes text,
    priority text NOT NULL DEFAULT 'normal',
    due_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tasks_title_length CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT tasks_notes_length CHECK (notes IS NULL OR char_length(notes) <= 5000),
    CONSTRAINT tasks_priority CHECK (priority IN ('low', 'normal', 'high'))
);

CREATE INDEX tasks_open_due_idx
    ON lifehub.tasks (user_id, due_at, id)
    WHERE completed_at IS NULL AND due_at IS NOT NULL;

CREATE INDEX tasks_open_anytime_idx
    ON lifehub.tasks (user_id, created_at, id)
    WHERE completed_at IS NULL AND due_at IS NULL;

CREATE INDEX tasks_completed_idx
    ON lifehub.tasks (user_id, completed_at, id)
    WHERE completed_at IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS lifehub.tasks;
DROP TABLE IF EXISTS lifehub.profiles;
