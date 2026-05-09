-- name: CreateSudokuCaptcha :one
INSERT INTO sudoku_captchas (user_id, puzzle, solution, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSudokuCaptcha :one
SELECT *
FROM sudoku_captchas
WHERE id = $1;

-- name: IncrementSudokuErrors :one
UPDATE sudoku_captchas
SET errors = errors + 1
WHERE id = $1
RETURNING *;

-- name: MarkSudokuPassed :one
UPDATE sudoku_captchas
SET passed = TRUE
WHERE id = $1
RETURNING *;

-- name: DeleteSudokuCaptcha :exec
DELETE FROM sudoku_captchas
WHERE id = $1;
