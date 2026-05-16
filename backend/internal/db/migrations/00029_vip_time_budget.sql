-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN vip_remaining_seconds INTEGER NOT NULL DEFAULT 86400;
ALTER TABLE users ADD COLUMN vip_cooldown_until TIMESTAMPTZ;

-- Reset existing promotions to the new system
UPDATE users SET promoted_until = NOW() + interval '100 years' WHERE promoted_until > NOW();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN vip_remaining_seconds;
ALTER TABLE users DROP COLUMN vip_cooldown_until;
-- +goose StatementEnd
