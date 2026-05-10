-- +goose Up
-- +goose StatementBegin

ALTER TABLE users
ADD COLUMN captcha_type TEXT NOT NULL DEFAULT 'none';

UPDATE users SET captcha_type = 'sudoku' WHERE requires_sudoku = TRUE;

ALTER TABLE users
DROP COLUMN requires_sudoku;

ALTER TABLE users
RENAME COLUMN sudoku_cooldown_until TO captcha_cooldown_until;

CREATE TABLE casino_captchas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    passed BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_casino_captchas_user_id ON casino_captchas(user_id);
CREATE INDEX idx_casino_captchas_expires_at ON casino_captchas(expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS casino_captchas;

ALTER TABLE users
RENAME COLUMN captcha_cooldown_until TO sudoku_cooldown_until;

ALTER TABLE users
ADD COLUMN requires_sudoku BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE users SET requires_sudoku = TRUE WHERE captcha_type = 'sudoku';

ALTER TABLE users
DROP COLUMN captcha_type;

-- +goose StatementEnd
