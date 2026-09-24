-- name: GrantAchievements :exec
-- Todos los códigos en un solo INSERT, dentro de la transacción que escribió el
-- progreso que los causó. El ON CONFLICT DO NOTHING es la regla del dominio: lo
-- ganado no se vuelve a ganar ni se pierde, así que earned_at es el día de la
-- primera vez y no se toca nunca más.
INSERT INTO achievements (user_id, code)
SELECT @user_id::uuid, unnest(@codes::text[])
ON CONFLICT (user_id, code) DO NOTHING;

-- name: ListAchievements :many
SELECT * FROM achievements WHERE user_id = $1;
