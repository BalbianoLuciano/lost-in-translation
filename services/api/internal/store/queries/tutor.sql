-- name: GetCachedAnswer :one
SELECT answer, model FROM ai_answers WHERE prompt_hash = $1;

-- name: SaveAnswer :exec
INSERT INTO ai_answers (prompt_hash, question, context, answer, model)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (prompt_hash) DO NOTHING;

-- name: CountAsksToday :one
SELECT COALESCE((SELECT asks FROM ai_usage WHERE user_id = $1 AND day = current_date), 0)::int;

-- name: AddAsk :exec
INSERT INTO ai_usage (user_id, day, asks)
VALUES ($1, current_date, 1)
ON CONFLICT (user_id, day) DO UPDATE SET asks = ai_usage.asks + 1;
