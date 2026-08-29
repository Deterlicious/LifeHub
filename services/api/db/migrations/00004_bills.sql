-- +goose Up
CREATE TABLE lifehub.bills (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES lifehub.profiles(user_id) ON DELETE CASCADE,
    title text NOT NULL,
    notes text,
    amount bigint NOT NULL,
    currency text NOT NULL DEFAULT 'IDR',
    due_at timestamptz NOT NULL,
    paid_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT bills_title_length CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT bills_notes_length CHECK (notes IS NULL OR char_length(notes) <= 5000),
    CONSTRAINT bills_amount_range CHECK (amount BETWEEN 1 AND 9007199254740991),
    CONSTRAINT bills_currency_format CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX bills_unpaid_due_idx
    ON lifehub.bills (user_id, due_at, id)
    WHERE paid_at IS NULL;

CREATE INDEX bills_paid_idx
    ON lifehub.bills (user_id, paid_at, id)
    WHERE paid_at IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS lifehub.bills;
