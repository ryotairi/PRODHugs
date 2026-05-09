-- name: GetCooldown :one
SELECT user_a_id, user_b_id, last_hug_at, cooldown_seconds, decline_cooldown_until
FROM hug_cooldowns
WHERE user_a_id = LEAST($1::UUID, $2::UUID)
  AND user_b_id = GREATEST($1::UUID, $2::UUID);

-- name: UpsertCooldown :one
INSERT INTO hug_cooldowns (user_a_id, user_b_id, cooldown_seconds)
VALUES (LEAST($1::UUID, $2::UUID), GREATEST($1::UUID, $2::UUID), $3)
ON CONFLICT (user_a_id, user_b_id)
DO UPDATE SET last_hug_at = now()
RETURNING *;

-- name: ReduceCooldown :one
UPDATE hug_cooldowns
SET cooldown_seconds = GREATEST(cooldown_seconds - @reduction::INTEGER, 300)
WHERE user_a_id = LEAST($1::UUID, $2::UUID)
  AND user_b_id = GREATEST($1::UUID, $2::UUID)
RETURNING *;

-- name: SetDeclineCooldown :exec
INSERT INTO hug_cooldowns (user_a_id, user_b_id, decline_cooldown_until, cooldown_seconds, last_hug_at)
VALUES (LEAST($1::UUID, $2::UUID), GREATEST($1::UUID, $2::UUID), $3, 3600, '2000-01-01'::timestamptz)
ON CONFLICT (user_a_id, user_b_id)
DO UPDATE SET decline_cooldown_until = $3;

-- name: SetSudokuPenaltyCooldown :exec
INSERT INTO hug_cooldowns (user_a_id, user_b_id, decline_cooldown_until, cooldown_seconds, last_hug_at)
VALUES (LEAST($1::UUID, $2::UUID), GREATEST($1::UUID, $2::UUID), $3, 3600, '2000-01-01'::timestamptz)
ON CONFLICT (user_a_id, user_b_id)
DO UPDATE SET decline_cooldown_until = $3;
