-- name: CreateUser :one
INSERT INTO users (username, password, role, gender, created_at)
VALUES (
    $1, $2, $3, $4, NOW()
)
RETURNING *;

-- name: GetUserByUsername :one
SELECT 
    u.*, 
    COALESCE(b.amount, 0)::int AS balance,
    COALESCE((
        SELECT AVG(EXTRACT(EPOCH FROM (h.accepted_at - h.created_at)))
        FROM hugs h
        WHERE h.receiver_id = u.id AND h.status = 'completed'
    ), -1)::float AS avg_response_time
FROM users u
LEFT JOIN balances b ON b.user_id = u.id
WHERE u.username = $1;

-- name: GetUserByID :one
SELECT 
    u.*, 
    COALESCE(b.amount, 0)::int AS balance,
    COALESCE((
        SELECT AVG(EXTRACT(EPOCH FROM (h.accepted_at - h.created_at)))
        FROM hugs h
        WHERE h.receiver_id = u.id AND h.status = 'completed'
    ), -1)::float AS avg_response_time
FROM users u
LEFT JOIN balances b ON b.user_id = u.id
WHERE u.id = $1;

-- name: SearchUsers :many
SELECT 
    u.id, u.username, u.role, u.gender, u.display_name, u.tag, u.special_tag,
    (u.telegram_id IS NOT NULL)::bool AS is_telegram_linked,
    u.promoted_until, u.promotion_message,
    COALESCE((
        SELECT AVG(EXTRACT(EPOCH FROM (h.accepted_at - h.created_at)))
        FROM hugs h
        WHERE h.receiver_id = u.id AND h.status = 'completed'
    ), -1)::float AS avg_response_time
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
ORDER BY 
    (u.promoted_until > NOW()) DESC,
    (
        SELECT AVG(EXTRACT(EPOCH FROM (h.accepted_at - h.created_at)))
        FROM hugs h
        WHERE h.receiver_id = u.id AND h.status = 'completed'
    ) ASC NULLS LAST,
    COALESCE(rt.last_visit, u.created_at) DESC NULLS LAST
LIMIT @lim::int OFFSET @off::int;

-- name: ListAllUsers :many
SELECT 
    u.id, u.username, u.role, u.gender, u.display_name, u.tag, u.special_tag,
    (u.telegram_id IS NOT NULL)::bool AS is_telegram_linked,
    u.promoted_until, u.promotion_message,
    COALESCE((
        SELECT AVG(EXTRACT(EPOCH FROM (h.accepted_at - h.created_at)))
        FROM hugs h
        WHERE h.receiver_id = u.id AND h.status = 'completed'
    ), -1)::float AS avg_response_time
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
ORDER BY 
    (u.promoted_until > NOW()) DESC,
    (
        SELECT AVG(EXTRACT(EPOCH FROM (h.accepted_at - h.created_at)))
        FROM hugs h
        WHERE h.receiver_id = u.id AND h.status = 'completed'
    ) ASC NULLS LAST,
    COALESCE(rt.last_visit, u.created_at) DESC NULLS LAST
LIMIT @lim::int OFFSET @off::int;

-- name: GetLeaderboard :many
SELECT
    u.id,
    u.username,
    u.gender,
    u.display_name,
    u.tag,
    u.special_tag,
    COALESCE(b.amount, 0)::int AS balance
FROM users u
LEFT JOIN balances b ON b.user_id = u.id
WHERE u.banned_at IS NULL
ORDER BY balance DESC
LIMIT @lim::int OFFSET @off::int;

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
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

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
SELECT 
    u.*, 
    COALESCE(b.amount, 0)::int AS balance,
    COALESCE((
        SELECT AVG(EXTRACT(EPOCH FROM (h.accepted_at - h.created_at)))
        FROM hugs h
        WHERE h.receiver_id = u.id AND h.status = 'completed'
    ), -1)::float AS avg_response_time
FROM users u
LEFT JOIN balances b ON b.user_id = u.id
WHERE u.telegram_id = $1;

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
       u.promoted_until, u.promotion_message,
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
       u.promoted_until, u.promotion_message,
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
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

-- name: AdminUpdateGender :one
UPDATE users
SET gender = $2
WHERE id = $1
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

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
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

-- name: AdminUpdateTag :one
UPDATE users
SET tag = $2
WHERE id = $1
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

-- name: AdminUpdateSpecialTag :one
UPDATE users
SET special_tag = $2
WHERE id = $1
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

-- name: AdminUpdateCaptchaType :one
UPDATE users
SET captcha_type = $2
WHERE id = $1
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

-- name: SetCaptchaCooldown :exec
UPDATE users
SET captcha_cooldown_until = $2
WHERE id = $1;

-- name: AdminDeleteUser :execrows
DELETE FROM users
WHERE id = $1 AND role != 'admin';

-- name: AdminClearPromotion :one
UPDATE users
SET promoted_until = NULL, promotion_message = NULL
WHERE id = $1
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;

-- name: PromoteUser :one
UPDATE users
SET promoted_until = $2, promotion_message = $3
WHERE id = $1
RETURNING *, (SELECT COALESCE(amount, 0) FROM balances WHERE user_id = users.id)::int AS balance;
