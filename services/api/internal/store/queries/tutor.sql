-- name: GetCachedAnswer :one
SELECT answer, model FROM ai_answers WHERE prompt_hash = $1;

-- name: SaveAnswer :exec
INSERT INTO ai_answers (prompt_hash, question, context, answer, model)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (prompt_hash) DO NOTHING;
