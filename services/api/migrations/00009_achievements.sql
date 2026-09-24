-- +goose Up
-- La distinción es histórica, no un estado: dice que llegaste, no que seguís
-- ahí (docs/sdd-distinciones.md §2). El pilar del mapa muestra cómo estás hoy y
-- puede empeorar; esta tabla no.
--
-- Por eso no hay UPDATE ni DELETE en ningún lado y la clave primaria alcanza
-- como toda la lógica: se inserta con ON CONFLICT DO NOTHING, así evaluar mil
-- veces gana lo mismo que evaluar una y oxidarse no borra nada.
CREATE TABLE achievements (
    user_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code      text NOT NULL,
    earned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code)
);

-- +goose Down
DROP TABLE achievements;
