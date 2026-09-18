-- name: GetOpenPlacementRun :one
SELECT * FROM placement_runs
WHERE user_id = $1 AND part = $2 AND status = 'in_progress';

-- name: CreatePlacementRun :one
INSERT INTO placement_runs (user_id, part, content_version)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPlacementRunForUpdate :one
SELECT * FROM placement_runs
WHERE id = $1 AND user_id = $2
FOR UPDATE;

-- name: GetPlacementRun :one
SELECT * FROM placement_runs
WHERE id = $1 AND user_id = $2;

-- name: ListLatestPlacementRuns :many
-- La corrida más reciente de cada parte.
SELECT DISTINCT ON (part) *
FROM placement_runs
WHERE user_id = $1
ORDER BY part, started_at DESC;

-- name: FinishPlacementRun :exec
UPDATE placement_runs
SET status = 'done', finished_at = now()
WHERE id = $1;

-- name: ListRunAttempts :many
SELECT item_id, skill_id, correct, consulted
FROM attempts
WHERE placement_run_id = $1
ORDER BY id;

-- name: InsertAttempt :one
INSERT INTO attempts (user_id, item_id, skill_id, context, placement_run_id, response, correct, latency_ms, consulted)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;

-- name: GetCard :one
SELECT * FROM cards WHERE user_id = $1 AND item_id = $2;

-- name: UpsertCard :exec
INSERT INTO cards (user_id, item_id, due, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id, item_id) DO UPDATE
SET due = EXCLUDED.due,
    stability = EXCLUDED.stability,
    difficulty = EXCLUDED.difficulty,
    elapsed_days = EXCLUDED.elapsed_days,
    scheduled_days = EXCLUDED.scheduled_days,
    reps = EXCLUDED.reps,
    lapses = EXCLUDED.lapses,
    state = EXCLUDED.state,
    last_review = EXCLUDED.last_review;

-- name: UpsertSkillMastery :exec
INSERT INTO skill_mastery (user_id, skill_id, mastery, state, source, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (user_id, skill_id) DO UPDATE
SET mastery = EXCLUDED.mastery,
    state = EXCLUDED.state,
    source = EXCLUDED.source,
    updated_at = now();

-- name: ListSkillMastery :many
SELECT * FROM skill_mastery WHERE user_id = $1;

-- name: ListDueCards :many
SELECT item_id, due FROM cards
WHERE user_id = $1 AND due <= now()
ORDER BY due
LIMIT $2;

-- name: CountDueCards :one
SELECT count(*) FROM cards WHERE user_id = $1 AND due <= now();

-- name: ListRecentSkillAttempts :many
-- Los últimos intentos de una habilidad, del más nuevo al más viejo.
SELECT correct, consulted, created_at
FROM attempts
WHERE user_id = $1 AND skill_id = $2 AND context <> 'placement'
ORDER BY created_at DESC
LIMIT $3;

-- name: ListAnsweredToday :many
SELECT DISTINCT item_id FROM attempts
WHERE user_id = $1 AND created_at >= date_trunc('day', now());

-- name: GetLessonProgress :one
SELECT * FROM lesson_progress WHERE user_id = $1 AND skill_id = $2;

-- name: ListLessonProgress :many
SELECT * FROM lesson_progress WHERE user_id = $1;

-- name: StartLesson :one
INSERT INTO lesson_progress (user_id, skill_id)
VALUES ($1, $2)
ON CONFLICT (user_id, skill_id) DO UPDATE SET skill_id = EXCLUDED.skill_id
RETURNING *;

-- name: CompleteLesson :one
INSERT INTO lesson_progress (user_id, skill_id, status, completed_at)
VALUES ($1, $2, 'done', now())
ON CONFLICT (user_id, skill_id) DO UPDATE
SET status = 'done', completed_at = COALESCE(lesson_progress.completed_at, now())
RETURNING *;

-- name: AddToDailyLog :one
INSERT INTO daily_log (user_id, day, colada, answers, seconds)
VALUES ($1, current_date, $2, 1, $3)
ON CONFLICT (user_id, day) DO UPDATE
SET colada = daily_log.colada + EXCLUDED.colada,
    answers = daily_log.answers + 1,
    seconds = daily_log.seconds + EXCLUDED.seconds
RETURNING *;

-- name: GetTodayLog :one
SELECT * FROM daily_log WHERE user_id = $1 AND day = current_date;

-- name: ListRecentDays :many
-- Días con actividad, del más nuevo al más viejo: con esto se calcula el jornal.
SELECT day, colada, answers, seconds FROM daily_log
WHERE user_id = $1
ORDER BY day DESC
LIMIT $2;

-- name: SumColada :one
SELECT COALESCE(sum(colada), 0)::bigint FROM daily_log WHERE user_id = $1;

-- name: CountAnsweredItemsBySkill :one
-- Cuántos ítems distintos de una habilidad se respondieron alguna vez fuera del
-- diagnóstico: con eso se sabe si ya se recorrió el banco del tema.
SELECT count(DISTINCT item_id) FROM attempts
WHERE user_id = $1 AND skill_id = $2 AND context <> 'placement';
