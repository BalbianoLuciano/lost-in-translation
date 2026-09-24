-- +goose Up
-- Un solo lugar para "cuánto gastó esta persona hoy", por tipo de gasto.
--
-- Antes sólo se contaban las preguntas al profesor; la voz, que es lo más caro,
-- no se contaba en ningún lado. Unificar las dos en una tabla permite además
-- preguntar por el gasto del día entero, que es lo que protege la clave del
-- proveedor cuando hay más de un usuario.
CREATE TABLE usage_daily (
    user_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day      date NOT NULL,
    kind     text NOT NULL CHECK (kind IN ('ask', 'speaking')),
    n        integer NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day, kind)
);

-- El gasto del día de todos: sostiene el tope global.
CREATE INDEX usage_daily_by_day ON usage_daily (day, kind);

INSERT INTO usage_daily (user_id, day, kind, n)
SELECT user_id, day, 'ask', asks FROM ai_usage;

DROP TABLE ai_usage;

-- +goose Down
CREATE TABLE ai_usage (
    user_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day      date NOT NULL,
    asks     integer NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);

INSERT INTO ai_usage (user_id, day, asks)
SELECT user_id, day, n FROM usage_daily WHERE kind = 'ask';

DROP TABLE usage_daily;
