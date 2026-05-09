-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN IF NOT EXISTS sudoku_cooldown_until TIMESTAMP WITH TIME ZONE;
ALTER TABLE sudoku_captchas DROP COLUMN IF EXISTS target_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS sudoku_cooldown_until;
ALTER TABLE sudoku_captchas ADD COLUMN IF NOT EXISTS target_id UUID REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd
