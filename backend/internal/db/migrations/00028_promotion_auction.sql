-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN promotion_bid INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN promotion_bid;
-- +goose StatementEnd
