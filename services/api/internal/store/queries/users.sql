-- name: UpsertUser :one
INSERT INTO users (firebase_uid, email, display_name)
VALUES ($1, $2, $3)
ON CONFLICT (firebase_uid) DO UPDATE
SET email        = EXCLUDED.email,
    display_name = EXCLUDED.display_name,
    last_seen_at = now()
RETURNING *;

-- name: UpdateUserTheme :one
UPDATE users
SET theme = $2
WHERE firebase_uid = $1
RETURNING *;

-- name: GetUserByFirebaseUID :one
SELECT * FROM users WHERE firebase_uid = $1;
