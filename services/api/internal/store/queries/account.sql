-- name: DeleteUser :execrows
-- Borra la cuenta. El resto lo hace el ON DELETE CASCADE de las migraciones:
-- cada tabla con datos de una persona referencia users(id). La única excepción
-- es ai_answers, que es una caché global por hash de prompt y no tiene user_id;
-- está dicho en la página de privacidad.
DELETE FROM users WHERE id = $1;

-- Las consultas de exportación traen la fila entera a propósito: exportar tiene
-- que devolver todo lo que la app guarda de esa persona, así que agregar una
-- columna a una tabla la agrega también a la exportación sin que haya que
-- acordarse de tocar acá.

-- name: ListAccountPlacementRuns :many
SELECT * FROM placement_runs WHERE user_id = $1 ORDER BY started_at, id;

-- name: ListAccountAttempts :many
SELECT * FROM attempts WHERE user_id = $1 ORDER BY id;

-- name: ListAccountCards :many
SELECT * FROM cards WHERE user_id = $1 ORDER BY item_id;

-- name: ListAccountSkillMastery :many
SELECT * FROM skill_mastery WHERE user_id = $1 ORDER BY skill_id;

-- name: ListAccountLessonProgress :many
SELECT * FROM lesson_progress WHERE user_id = $1 ORDER BY skill_id;

-- name: ListAccountDailyLog :many
SELECT * FROM daily_log WHERE user_id = $1 ORDER BY day;

-- name: ListAccountUsage :many
SELECT * FROM usage_daily WHERE user_id = $1 ORDER BY day, kind;
