-- name: GetActiveAnnouncement :one
SELECT id, message, created_at, created_by, active
FROM announcements
WHERE active = TRUE
ORDER BY created_at DESC
LIMIT 1;

-- name: GetActiveAnnouncementForUser :one
SELECT a.id, a.message, a.created_at, a.created_by
FROM announcements a
WHERE a.active = TRUE
  AND NOT EXISTS (
    SELECT 1 FROM announcement_dismissals ad
    WHERE ad.announcement_id = a.id AND ad.user_id = @user_id::uuid
  )
ORDER BY a.created_at DESC
LIMIT 1;

-- name: CreateAnnouncement :one
WITH deactivated AS (
    UPDATE announcements SET active = FALSE WHERE active = TRUE
)
INSERT INTO announcements (message, created_by)
VALUES (@message::text, @created_by::uuid)
RETURNING *;

-- name: DeactivateAnnouncement :exec
UPDATE announcements SET active = FALSE WHERE id = $1;

-- name: DismissAnnouncement :exec
INSERT INTO announcement_dismissals (announcement_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
