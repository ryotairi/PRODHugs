-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_notes (
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL CHECK (char_length(content) > 0 AND char_length(content) <= 256),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (author_id, target_id)
);

-- Lookup-by-author for the "my notes" listing.
CREATE INDEX idx_user_notes_author_updated ON user_notes (author_id, updated_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_notes;
-- +goose StatementEnd
