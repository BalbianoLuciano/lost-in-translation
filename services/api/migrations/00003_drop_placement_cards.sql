-- +goose Up
-- El diagnóstico dejó de crear tarjetas de repaso: si sus ítems volvieran en la
-- sesión diaria, la próxima medición sería de memoria y no de nivel. Esto borra
-- las tarjetas que quedaron de ítems que sólo se respondieron en el diagnóstico.
-- Los intentos no se tocan: el historial queda.
DELETE FROM cards c
WHERE NOT EXISTS (
    SELECT 1 FROM attempts a
    WHERE a.user_id = c.user_id
      AND a.item_id = c.item_id
      AND a.context <> 'placement'
);

-- +goose Down
-- No se puede reconstruir el estado FSRS borrado; no hay vuelta atrás.
SELECT 1;
