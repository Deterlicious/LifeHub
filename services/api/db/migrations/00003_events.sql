-- +goose Up
CREATE TABLE lifehub.events (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    title text NOT NULL,
    notes text,
    location text,
    all_day boolean NOT NULL,
    timezone text NOT NULL,
    starts_at timestamptz,
    ends_at timestamptz,
    starts_on date,
    ends_on date,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT events_title_length CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT events_notes_length CHECK (notes IS NULL OR char_length(notes) <= 5000),
    CONSTRAINT events_location_length CHECK (location IS NULL OR char_length(location) <= 500),
    CONSTRAINT events_timezone_length CHECK (char_length(timezone) BETWEEN 1 AND 100),
    CONSTRAINT events_time_shape CHECK (
        (
            all_day
            AND starts_at IS NULL
            AND ends_at IS NULL
            AND starts_on IS NOT NULL
            AND (ends_on IS NULL OR ends_on >= starts_on)
        )
        OR
        (
            NOT all_day
            AND starts_at IS NOT NULL
            AND (ends_at IS NULL OR ends_at > starts_at)
            AND starts_on IS NULL
            AND ends_on IS NULL
        )
    )
);

CREATE INDEX events_timed_today_idx
    ON lifehub.events (user_id, starts_at, ends_at, id)
    WHERE all_day = false;

CREATE INDEX events_all_day_today_idx
    ON lifehub.events (user_id, starts_on, ends_on, id)
    WHERE all_day = true;

-- +goose Down
DROP TABLE IF EXISTS lifehub.events;
