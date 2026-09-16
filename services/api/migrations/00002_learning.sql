-- +goose Up
-- El contenido (habilidades e ítems) no vive en la base: la API lo embebe desde
-- content/ compilado. Acá sólo queda lo que hace cada usuario.

CREATE TABLE placement_runs (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    part             text NOT NULL,
    content_version  text NOT NULL,
    status           text NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'done')),
    started_at       timestamptz NOT NULL DEFAULT now(),
    finished_at      timestamptz
);

-- Una sola corrida abierta por parte.
CREATE UNIQUE INDEX placement_runs_one_open ON placement_runs (user_id, part) WHERE status = 'in_progress';

CREATE TABLE attempts (
    id                bigserial PRIMARY KEY,
    user_id           uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id           text NOT NULL,
    skill_id          text NOT NULL,
    context           text NOT NULL CHECK (context IN ('placement', 'review', 'lesson')),
    placement_run_id  uuid REFERENCES placement_runs (id) ON DELETE CASCADE,
    response          jsonb NOT NULL,
    correct           boolean NOT NULL,
    latency_ms        integer,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX attempts_by_run ON attempts (placement_run_id, id);
CREATE INDEX attempts_by_user_skill ON attempts (user_id, skill_id, created_at);

-- Estado FSRS por usuario e ítem.
CREATE TABLE cards (
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id         text NOT NULL,
    due             timestamptz NOT NULL,
    stability       double precision NOT NULL,
    difficulty      double precision NOT NULL,
    elapsed_days    integer NOT NULL,
    scheduled_days  integer NOT NULL,
    reps            integer NOT NULL,
    lapses          integer NOT NULL,
    state           smallint NOT NULL,
    last_review     timestamptz,
    PRIMARY KEY (user_id, item_id)
);

CREATE INDEX cards_due ON cards (user_id, due);

-- Dominio por habilidad: define si la pieza está en plano, suspendida, calzada u oxidada.
CREATE TABLE skill_mastery (
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    skill_id    text NOT NULL,
    mastery     real NOT NULL CHECK (mastery >= 0 AND mastery <= 1),
    state       text NOT NULL CHECK (state IN ('plano', 'suspendida', 'calzada', 'oxidada')),
    source      text NOT NULL CHECK (source IN ('placement', 'practice')),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, skill_id)
);

-- +goose Down
DROP TABLE skill_mastery;
DROP TABLE cards;
DROP TABLE attempts;
DROP TABLE placement_runs;
