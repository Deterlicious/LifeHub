-- +goose Up
ALTER TABLE lifehub.profiles
    ALTER COLUMN timezone DROP DEFAULT,
    ALTER COLUMN timezone DROP NOT NULL;

-- +goose Down
UPDATE lifehub.profiles
SET timezone = 'Asia/Jakarta'
WHERE timezone IS NULL;

ALTER TABLE lifehub.profiles
    ALTER COLUMN timezone SET DEFAULT 'Asia/Jakarta',
    ALTER COLUMN timezone SET NOT NULL;
