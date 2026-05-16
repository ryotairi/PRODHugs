-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN promoted_until TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN promotion_message TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN promoted_until;
ALTER TABLE users DROP COLUMN promotion_message;
-- +goose StatementEnd
