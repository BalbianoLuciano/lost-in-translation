-- +goose Up
-- Marca si se consultó el glosario mientras se respondía ese ítem: un acierto
-- consultado cuenta menos que uno de memoria.
ALTER TABLE attempts ADD COLUMN consulted boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE attempts DROP COLUMN consulted;
