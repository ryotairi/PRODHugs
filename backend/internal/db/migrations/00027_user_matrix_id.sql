-- +goose Up
ALTER TABLE users ADD COLUMN matrix_id TEXT;
ALTER TABLE users ADD COLUMN matrix_room_id TEXT;
CREATE UNIQUE INDEX users_matrix_id_key ON users (matrix_id) WHERE matrix_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS users_matrix_id_key;
ALTER TABLE users DROP COLUMN IF EXISTS matrix_room_id;
ALTER TABLE users DROP COLUMN IF EXISTS matrix_id;
