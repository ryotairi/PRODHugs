-- name: CreateUser :one
INSERT INTO users (username, password, role, gender, created_at)
VALUES (
    $1, $2, $3, $4, NOW()
)
RETURNING *;

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = $1;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: SearchUsers :many
SELECT u.id, u.username, u.role, u.gender, u.display_name, u.tag, u.special_tag
FROM users u
LEFT JOIN LATERAL (
    SELECT MAX(created_at) AS last_visit
    FROM refresh_tokens
    WHERE user_id = u.id
) rt ON true
WHERE (u.username ILIKE '%' || @query::text || '%' OR u.display_name ILIKE '%' || @query::text || '%')
  AND u.banned_at IS NULL
  AND u.id NOT IN (
    SELECT blocked_id FROM user_blocks WHERE blocker_id = @viewer_id::uuid
    UNION
    SELECT blocker_id FROM user_blocks WHERE blocked_id = @viewer_id::uuid
  )
ORDER BY COALESCE(rt.last_visit, u.created_at) DESC NULLS LAST
LIMIT @lim::int OFFSET @off::int;

-- name: ListAllUsers :many
SELECT u.id, u.username, u.role, u.gender, u.display_name, u.tag, u.special_tag
FROM users u
LEFT JOIN LATERAL (
    SELECT MAX(created_at) AS last_visit
    FROM refresh_tokens
    WHERE user_id = u.id
) rt ON true
WHERE u.banned_at IS NULL
  AND u.id NOT IN (
    SELECT blocked_id FROM user_blocks WHERE blocker_id = @viewer_id::uuid
    UNION
    SELECT blocker_id FROM user_blocks WHERE blocked_id = @viewer_id::uuid
  )
ORDER BY COALESCE(rt.last_visit, u.created_at) DESC NULLS LAST
LIMIT @lim::int OFFSET @off::int;

-- name: GetLeaderboard :many
SELECT
    u.id,
    u.username,
    u.role,
    u.gender,
    u.display_name,
    u.tag,
    u.special_tag,
    COALESCE(given.cnt, 0) + COALESCE(received.cnt, 0) AS total_hugs,
    COALESCE(given.cnt, 0) AS hugs_given,
    COALESCE(received.cnt, 0) AS hugs_received
FROM users u
LEFT JOIN (
    SELECT giver_id, COUNT(*) AS cnt FROM hugs WHERE status = 'completed' GROUP BY giver_id
) given ON given.giver_id = u.id
LEFT JOIN (
    SELECT receiver_id, COUNT(*) AS cnt FROM hugs WHERE status = 'completed' GROUP BY receiver_id
) received ON received.receiver_id = u.id
WHERE u.banned_at IS NULL
ORDER BY total_hugs DESC
LIMIT @lim::int OFFSET @off::int;

-- name: GetUserStats :one
SELECT
    COUNT(*) FILTER (WHERE giver_id = @user_id::uuid)::bigint AS hugs_given,
    COUNT(*) FILTER (WHERE receiver_id = @user_id::uuid)::bigint AS hugs_received,
    COUNT(*)::bigint AS total_hugs
FROM hugs
WHERE (giver_id = @user_id::uuid OR receiver_id = @user_id::uuid)
  AND status = 'completed';

-- name: GetRecentHugsFeed :many
SELECT
    h.id,
    h.giver_id,
    h.receiver_id,
    COALESCE(h.accepted_at, h.created_at) AS created_at,
    h.hug_type,
    (h.comment IS NOT NULL)::bool AS has_comment,
    h.streak_tier,
    g.username AS giver_username,
    r.username AS receiver_username,
    g.gender AS giver_gender,
    g.display_name AS giver_display_name,
    r.display_name AS receiver_display_name
FROM hugs h
JOIN users g ON g.id = h.giver_id
JOIN users r ON r.id = h.receiver_id
WHERE h.status = 'completed'
ORDER BY COALESCE(h.accepted_at, h.created_at) DESC
LIMIT @lim::int OFFSET @off::int;

-- name: UpdateUserSettings :one
UPDATE users
SET gender = $2, display_name = $3, tag = $4
WHERE id = $1
RETURNING *;

-- name: GetUserTelegramID :one
SELECT telegram_id FROM users WHERE id = $1;

-- name: SetUserTelegramID :one
UPDATE users
SET telegram_id = $2
WHERE id = $1
RETURNING *;

-- name: ClearUserTelegramID :one
UPDATE users
SET telegram_id = NULL
WHERE id = $1
RETURNING *;

-- name: IsTelegramIDTaken :one
SELECT EXISTS(
    SELECT 1 FROM users WHERE telegram_id = $1 AND id != $2
) AS taken;

-- name: GetUserByTelegramID :one
SELECT * FROM users WHERE telegram_id = $1;

-- name: GetUserMatrixID :one
SELECT matrix_id, matrix_room_id FROM users WHERE id = $1;

-- name: SetUserMatrixID :one
UPDATE users
SET matrix_id = $2, matrix_room_id = $3
WHERE id = $1
RETURNING *;

-- name: ClearUserMatrixID :one
UPDATE users
SET matrix_id = NULL, matrix_room_id = NULL
WHERE id = $1
RETURNING *;

-- name: IsMatrixIDTaken :one
SELECT EXISTS(
    SELECT 1 FROM users WHERE matrix_id = $1 AND id != $2
) AS taken;

-- name: GetUserByMatrixID :one
SELECT * FROM users WHERE matrix_id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password = $2
WHERE id = $1;

-- name: BanUser :one
UPDATE users
SET banned_at = NOW()
WHERE id = $1 AND role != 'admin'
RETURNING *;

-- name: UnbanUser :one
UPDATE users
SET banned_at = NULL
WHERE id = $1
RETURNING *;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CountBannedUsers :one
SELECT COUNT(*) FROM users WHERE banned_at IS NOT NULL;

-- name: ListUsersAdmin :many
SELECT u.id, u.username, u.role, u.gender, u.display_name, u.tag, u.special_tag, u.banned_at, u.created_at, u.captcha_type, u.captcha_cooldown_until,
       COALESCE(b.amount, 0)::int AS balance,
       COALESCE(rt.last_visit, u.created_at)::timestamptz AS last_visit_at
FROM users u
LEFT JOIN balances b ON b.user_id = u.id
LEFT JOIN LATERAL (
    SELECT MAX(created_at) AS last_visit
    FROM refresh_tokens
    WHERE user_id = u.id
) rt ON true
ORDER BY last_visit_at DESC NULLS LAST
LIMIT @lim::int OFFSET @off::int;

-- name: SearchUsersAdmin :many
SELECT u.id, u.username, u.role, u.gender, u.display_name, u.tag, u.special_tag, u.banned_at, u.created_at, u.captcha_type, u.captcha_cooldown_until,
       COALESCE(b.amount, 0)::int AS balance,
       COALESCE(rt.last_visit, u.created_at)::timestamptz AS last_visit_at
FROM users u
LEFT JOIN balances b ON b.user_id = u.id
LEFT JOIN LATERAL (
    SELECT MAX(created_at) AS last_visit
    FROM refresh_tokens
    WHERE user_id = u.id
) rt ON true
WHERE (u.username ILIKE '%' || @query::text || '%' OR u.display_name ILIKE '%' || @query::text || '%')
ORDER BY last_visit_at DESC NULLS LAST
LIMIT @lim::int OFFSET @off::int;

-- name: AdminUpdateUsername :one
UPDATE users
SET username = $2
WHERE id = $1
RETURNING *;

-- name: AdminUpdateGender :one
UPDATE users
SET gender = $2
WHERE id = $1
RETURNING *;

-- name: AdminUpdatePassword :exec
UPDATE users
SET password = $2
WHERE id = $1;

-- name: GetUserSlots :one
SELECT hug_slots FROM users WHERE id = $1;

-- name: IncrementUserSlots :one
UPDATE users
SET hug_slots = hug_slots + 1
WHERE id = $1 AND hug_slots < 5
RETURNING hug_slots;

-- name: AdminUpdateDisplayName :one
UPDATE users
SET display_name = $2
WHERE id = $1
RETURNING *;

-- name: AdminUpdateTag :one
UPDATE users
SET tag = $2
WHERE id = $1
RETURNING *;

-- name: AdminUpdateSpecialTag :one
UPDATE users
SET special_tag = $2
WHERE id = $1
RETURNING *;

-- name: AdminUpdateCaptchaType :one
UPDATE users
SET captcha_type = $2
WHERE id = $1
RETURNING *;

-- name: SetCaptchaCooldown :exec
UPDATE users
SET captcha_cooldown_until = $2
WHERE id = $1;

-- name: AdminDeleteUser :execrows
DELETE FROM users
WHERE id = $1 AND role != 'admin';
