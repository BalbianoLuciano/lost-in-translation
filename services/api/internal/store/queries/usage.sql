-- name: CountUsageToday :one
SELECT COALESCE((
    SELECT n FROM usage_daily
    WHERE user_id = $1 AND day = current_date AND kind = $2
), 0)::int;

-- name: CountUsageTodayAll :one
SELECT COALESCE(sum(n), 0)::int FROM usage_daily
WHERE day = current_date AND kind = $1;

-- name: AddUsage :exec
INSERT INTO usage_daily (user_id, day, kind, n)
VALUES ($1, current_date, $2, 1)
ON CONFLICT (user_id, day, kind) DO UPDATE SET n = usage_daily.n + 1;
