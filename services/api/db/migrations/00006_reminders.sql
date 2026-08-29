-- +goose Up
CREATE TABLE lifehub.reminder_definitions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    source_kind text NOT NULL,
    source_id uuid NOT NULL,
    schedule_kind text NOT NULL,
    minutes_before integer,
    days_before integer,
    time_local time without time zone,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT reminder_definitions_source_kind CHECK (source_kind IN ('task', 'event', 'bill', 'document')),
    CONSTRAINT reminder_definitions_schedule_shape CHECK (
        (
            schedule_kind = 'before_moment'
            AND minutes_before BETWEEN 0 AND 525600
            AND days_before IS NULL
            AND time_local IS NULL
        )
        OR
        (
            schedule_kind = 'before_date'
            AND minutes_before IS NULL
            AND days_before BETWEEN 0 AND 3650
            AND time_local IS NOT NULL
        )
    )
);

CREATE INDEX reminder_definitions_source_idx
    ON lifehub.reminder_definitions (user_id, source_kind, source_id, created_at, id);

CREATE TABLE lifehub.reminder_schedules (
    id uuid PRIMARY KEY,
    reminder_id uuid NOT NULL REFERENCES lifehub.reminder_definitions(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    generation bigint NOT NULL,
    fire_at timestamptz NOT NULL,
    state text NOT NULL DEFAULT 'scheduled',
    river_job_id bigint,
    invalidated_at timestamptz,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT reminder_schedules_generation CHECK (generation > 0),
    CONSTRAINT reminder_schedules_state CHECK (state IN ('scheduled', 'delivered', 'invalidated')),
    CONSTRAINT reminder_schedules_state_shape CHECK (
        (state = 'scheduled' AND invalidated_at IS NULL AND delivered_at IS NULL)
        OR (state = 'delivered' AND invalidated_at IS NULL AND delivered_at IS NOT NULL)
        OR (state = 'invalidated' AND invalidated_at IS NOT NULL AND delivered_at IS NULL)
    ),
    UNIQUE (reminder_id, generation),
    UNIQUE (river_job_id)
);

CREATE UNIQUE INDEX reminder_schedules_active_idx
    ON lifehub.reminder_schedules (reminder_id)
    WHERE state = 'scheduled';

CREATE INDEX reminder_schedules_due_idx
    ON lifehub.reminder_schedules (state, fire_at, id);

CREATE TABLE lifehub.notifications (
    id uuid PRIMARY KEY,
    schedule_id uuid NOT NULL,
    generation bigint NOT NULL,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    source_kind text NOT NULL,
    source_id uuid NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notifications_generation CHECK (generation > 0),
    CONSTRAINT notifications_source_kind CHECK (source_kind IN ('task', 'event', 'bill', 'document')),
    CONSTRAINT notifications_title_length CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT notifications_body_length CHECK (char_length(btrim(body)) BETWEEN 1 AND 500),
    UNIQUE (schedule_id, generation)
);

CREATE INDEX notifications_feed_idx
    ON lifehub.notifications (user_id, created_at DESC, id DESC);

CREATE INDEX notifications_unread_idx
    ON lifehub.notifications (user_id, created_at DESC, id DESC)
    WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS lifehub.notifications;
DROP TABLE IF EXISTS lifehub.reminder_schedules;
DROP TABLE IF EXISTS lifehub.reminder_definitions;
