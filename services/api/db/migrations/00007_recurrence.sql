-- +goose Up
CREATE TABLE lifehub.recurrence_series (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    source_kind text NOT NULL,
    frequency text NOT NULL,
    interval integer NOT NULL,
    anchor_on date NOT NULL,
    ends_on date,
    timezone text NOT NULL,
    generation bigint NOT NULL DEFAULT 1,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT recurrence_series_source_kind CHECK (source_kind IN ('task', 'event', 'bill')),
    CONSTRAINT recurrence_series_frequency CHECK (frequency IN ('daily', 'weekly', 'monthly', 'yearly')),
    CONSTRAINT recurrence_series_interval CHECK (interval BETWEEN 1 AND 365),
    CONSTRAINT recurrence_series_ends CHECK (ends_on IS NULL OR ends_on >= anchor_on),
    CONSTRAINT recurrence_series_timezone CHECK (char_length(btrim(timezone)) BETWEEN 1 AND 100),
    CONSTRAINT recurrence_series_generation CHECK (generation > 0)
);

CREATE INDEX recurrence_series_owner_active_idx
    ON lifehub.recurrence_series (user_id, active, source_kind, anchor_on, id);

CREATE TABLE lifehub.recurrence_task_templates (
    series_id uuid PRIMARY KEY REFERENCES lifehub.recurrence_series(id) ON DELETE CASCADE,
    title text NOT NULL,
    notes text,
    priority text NOT NULL,
    due_time time without time zone NOT NULL,
    CONSTRAINT recurrence_task_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT recurrence_task_notes CHECK (notes IS NULL OR char_length(notes) <= 5000),
    CONSTRAINT recurrence_task_priority CHECK (priority IN ('low', 'normal', 'high'))
);

CREATE TABLE lifehub.recurrence_event_templates (
    series_id uuid PRIMARY KEY REFERENCES lifehub.recurrence_series(id) ON DELETE CASCADE,
    title text NOT NULL,
    notes text,
    location text,
    all_day boolean NOT NULL,
    starts_time time without time zone,
    duration_seconds bigint,
    all_day_span integer,
    CONSTRAINT recurrence_event_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT recurrence_event_notes CHECK (notes IS NULL OR char_length(notes) <= 5000),
    CONSTRAINT recurrence_event_location CHECK (location IS NULL OR char_length(location) <= 500),
    CONSTRAINT recurrence_event_shape CHECK (
        (all_day AND starts_time IS NULL AND duration_seconds IS NULL AND all_day_span >= 0)
        OR
        (NOT all_day AND starts_time IS NOT NULL AND (duration_seconds IS NULL OR duration_seconds > 0) AND all_day_span IS NULL)
    )
);

CREATE TABLE lifehub.recurrence_bill_templates (
    series_id uuid PRIMARY KEY REFERENCES lifehub.recurrence_series(id) ON DELETE CASCADE,
    title text NOT NULL,
    notes text,
    amount bigint NOT NULL,
    currency text NOT NULL,
    due_time time without time zone NOT NULL,
    CONSTRAINT recurrence_bill_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT recurrence_bill_notes CHECK (notes IS NULL OR char_length(notes) <= 5000),
    CONSTRAINT recurrence_bill_amount CHECK (amount BETWEEN 1 AND 9007199254740991),
    CONSTRAINT recurrence_bill_currency CHECK (currency ~ '^[A-Z]{3}$')
);

ALTER TABLE lifehub.tasks
    ADD COLUMN series_id uuid REFERENCES lifehub.recurrence_series(id),
    ADD COLUMN occurrence_on date,
    ADD COLUMN series_generation bigint,
    ADD COLUMN is_exception boolean NOT NULL DEFAULT false,
    ADD COLUMN excluded_at timestamptz,
    ADD CONSTRAINT tasks_recurrence_shape CHECK (
        (series_id IS NULL AND occurrence_on IS NULL AND series_generation IS NULL AND NOT is_exception AND excluded_at IS NULL)
        OR
        (series_id IS NOT NULL AND occurrence_on IS NOT NULL AND series_generation > 0)
    );

CREATE UNIQUE INDEX tasks_series_occurrence_idx
    ON lifehub.tasks (series_id, occurrence_on)
    WHERE series_id IS NOT NULL;

ALTER TABLE lifehub.events
    ADD COLUMN series_id uuid REFERENCES lifehub.recurrence_series(id),
    ADD COLUMN occurrence_on date,
    ADD COLUMN series_generation bigint,
    ADD COLUMN is_exception boolean NOT NULL DEFAULT false,
    ADD COLUMN excluded_at timestamptz,
    ADD CONSTRAINT events_recurrence_shape CHECK (
        (series_id IS NULL AND occurrence_on IS NULL AND series_generation IS NULL AND NOT is_exception AND excluded_at IS NULL)
        OR
        (series_id IS NOT NULL AND occurrence_on IS NOT NULL AND series_generation > 0)
    );

CREATE UNIQUE INDEX events_series_occurrence_idx
    ON lifehub.events (series_id, occurrence_on)
    WHERE series_id IS NOT NULL;

ALTER TABLE lifehub.bills
    ADD COLUMN series_id uuid REFERENCES lifehub.recurrence_series(id),
    ADD COLUMN occurrence_on date,
    ADD COLUMN series_generation bigint,
    ADD COLUMN is_exception boolean NOT NULL DEFAULT false,
    ADD COLUMN excluded_at timestamptz,
    ADD CONSTRAINT bills_recurrence_shape CHECK (
        (series_id IS NULL AND occurrence_on IS NULL AND series_generation IS NULL AND NOT is_exception AND excluded_at IS NULL)
        OR
        (series_id IS NOT NULL AND occurrence_on IS NOT NULL AND series_generation > 0)
    );

CREATE UNIQUE INDEX bills_series_occurrence_idx
    ON lifehub.bills (series_id, occurrence_on)
    WHERE series_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS lifehub.bills_series_occurrence_idx;
ALTER TABLE lifehub.bills
    DROP CONSTRAINT IF EXISTS bills_recurrence_shape,
    DROP COLUMN IF EXISTS excluded_at,
    DROP COLUMN IF EXISTS is_exception,
    DROP COLUMN IF EXISTS series_generation,
    DROP COLUMN IF EXISTS occurrence_on,
    DROP COLUMN IF EXISTS series_id;

DROP INDEX IF EXISTS lifehub.events_series_occurrence_idx;
ALTER TABLE lifehub.events
    DROP CONSTRAINT IF EXISTS events_recurrence_shape,
    DROP COLUMN IF EXISTS excluded_at,
    DROP COLUMN IF EXISTS is_exception,
    DROP COLUMN IF EXISTS series_generation,
    DROP COLUMN IF EXISTS occurrence_on,
    DROP COLUMN IF EXISTS series_id;

DROP INDEX IF EXISTS lifehub.tasks_series_occurrence_idx;
ALTER TABLE lifehub.tasks
    DROP CONSTRAINT IF EXISTS tasks_recurrence_shape,
    DROP COLUMN IF EXISTS excluded_at,
    DROP COLUMN IF EXISTS is_exception,
    DROP COLUMN IF EXISTS series_generation,
    DROP COLUMN IF EXISTS occurrence_on,
    DROP COLUMN IF EXISTS series_id;

DROP TABLE IF EXISTS lifehub.recurrence_bill_templates;
DROP TABLE IF EXISTS lifehub.recurrence_event_templates;
DROP TABLE IF EXISTS lifehub.recurrence_task_templates;
DROP TABLE IF EXISTS lifehub.recurrence_series;
