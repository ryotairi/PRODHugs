-- +goose Up
-- +goose StatementBegin

ALTER TABLE users
ADD COLUMN requires_sudoku BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN sudoku_cooldown_until TIMESTAMP WITH TIME ZONE;

CREATE TABLE sudoku_captchas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    puzzle JSONB NOT NULL,
    solution JSONB NOT NULL,
    errors INTEGER NOT NULL DEFAULT 0,
    passed BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sudoku_captchas_user_id ON sudoku_captchas(user_id);
CREATE INDEX idx_sudoku_captchas_expires_at ON sudoku_captchas(expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS sudoku_captchas;

ALTER TABLE users
DROP COLUMN requires_sudoku,
DROP COLUMN sudoku_cooldown_until;

-- +goose StatementEnd
