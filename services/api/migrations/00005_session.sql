-- +goose Up
-- Lecciones leídas y actividad diaria: lo que sostiene la sesión de todos los días.

CREATE TABLE lesson_progress (
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    skill_id      text NOT NULL,
    status        text NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'done')),
    started_at    timestamptz NOT NULL DEFAULT now(),
    completed_at  timestamptz,
    PRIMARY KEY (user_id, skill_id)
);

-- Un día de obra: los adobes (colada) y los minutos que le pusiste.
CREATE TABLE daily_log (
    user_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day       date NOT NULL,
    colada    integer NOT NULL DEFAULT 0,
    answers   integer NOT NULL DEFAULT 0,
    seconds   integer NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);

-- +goose Down
DROP TABLE daily_log;
DROP TABLE lesson_progress;
