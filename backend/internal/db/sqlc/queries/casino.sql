-- name: CreateCasinoCaptcha :one
INSERT INTO casino_captchas (user_id, expires_at)
VALUES ($1, $2)
RETURNING *;

-- name: GetCasinoCaptcha :one
SELECT * FROM casino_captchas WHERE id = $1;

-- name: MarkCasinoPassed :one
UPDATE casino_captchas
SET passed = TRUE
WHERE id = $1
RETURNING *;

-- name: DeleteCasinoCaptcha :exec
DELETE FROM casino_captchas WHERE id = $1;
