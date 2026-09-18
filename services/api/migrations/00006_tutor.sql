-- +goose Up
-- Respuestas del profesor de IA, cacheadas: la misma pregunta no se paga dos veces.
CREATE TABLE ai_answers (
    id           bigserial PRIMARY KEY,
    prompt_hash  text NOT NULL UNIQUE,
    question     text NOT NULL,
    context      text NOT NULL DEFAULT '',
    answer       text NOT NULL,
    model        text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- Cuántas preguntas por día: el free tier de Groq no es infinito.
CREATE TABLE ai_usage (
    user_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day      date NOT NULL,
    asks     integer NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);

-- +goose Down
DROP TABLE ai_usage;
DROP TABLE ai_answers;
