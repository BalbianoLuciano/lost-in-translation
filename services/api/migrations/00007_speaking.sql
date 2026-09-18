-- +goose Up
-- Los drills orales se guardan como cualquier otro intento, con su propio contexto.
ALTER TABLE attempts DROP CONSTRAINT attempts_context_check;
ALTER TABLE attempts ADD CONSTRAINT attempts_context_check
    CHECK (context IN ('placement', 'review', 'lesson', 'speaking'));

-- +goose Down
DELETE FROM attempts WHERE context = 'speaking';
ALTER TABLE attempts DROP CONSTRAINT attempts_context_check;
ALTER TABLE attempts ADD CONSTRAINT attempts_context_check
    CHECK (context IN ('placement', 'review', 'lesson'));
