-- name: GetUserNote :one
SELECT author_id, target_id, content, updated_at
FROM user_notes
WHERE author_id = $1 AND target_id = $2;

-- name: UpsertUserNote :one
INSERT INTO user_notes (author_id, target_id, content, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (author_id, target_id) DO UPDATE
    SET content = EXCLUDED.content,
        updated_at = NOW()
RETURNING author_id, target_id, content, updated_at;

-- name: DeleteUserNote :exec
DELETE FROM user_notes
WHERE author_id = $1 AND target_id = $2;

-- name: ListUserNotes :many
SELECT n.author_id, n.target_id, n.content, n.updated_at,
       u.username AS target_username, u.display_name AS target_display_name
FROM user_notes n
JOIN users u ON u.id = n.target_id
WHERE n.author_id = $1
ORDER BY n.updated_at DESC
LIMIT @lim::int OFFSET @off::int;
