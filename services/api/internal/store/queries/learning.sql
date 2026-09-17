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
