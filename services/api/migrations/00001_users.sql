-- +goose Up
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    firebase_uid  text NOT NULL UNIQUE,
    email         text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    theme         text NOT NULL DEFAULT 'system' CHECK (theme IN ('system', 'dark', 'light')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
