// Package note is the repository for per-user private notes.
package note

import (
	"context"
	"errors"

	"go-service-template/internal/db/sqlc/storage"
	"go-service-template/internal/models"
	"go-service-template/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repo struct {
	q *storage.Queries
}

func New(db *pgxpool.Pool) *repo {
	return &repo{q: storage.New(db)}
}

// GetNote returns the caller's note about target, or nil if none exists.
func (r *repo) GetNote(ctx context.Context, authorID, targetID uuid.UUID) (*models.UserNote, error) {
	q := repository.Queries(ctx, r.q)
	row, err := q.GetUserNote(ctx, storage.GetUserNoteParams{
		AuthorID: authorID,
		TargetID: targetID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &models.UserNote{
		AuthorID:  row.AuthorID,
		TargetID:  row.TargetID,
		Content:   row.Content,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

// UpsertNote creates or replaces the caller's note about target. content must
// already be validated (non-empty, <=256 chars) by the service layer.
func (r *repo) UpsertNote(ctx context.Context, authorID, targetID uuid.UUID, content string) (*models.UserNote, error) {
	q := repository.Queries(ctx, r.q)
	row, err := q.UpsertUserNote(ctx, storage.UpsertUserNoteParams{
		AuthorID: authorID,
		TargetID: targetID,
		Content:  content,
	})
	if err != nil {
		return nil, err
	}
	return &models.UserNote{
		AuthorID:  row.AuthorID,
		TargetID:  row.TargetID,
		Content:   row.Content,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

// DeleteNote removes the caller's note about target. No-op if none exists.
func (r *repo) DeleteNote(ctx context.Context, authorID, targetID uuid.UUID) error {
	q := repository.Queries(ctx, r.q)
	return q.DeleteUserNote(ctx, storage.DeleteUserNoteParams{
		AuthorID: authorID,
		TargetID: targetID,
	})
}

// ListNotes returns the caller's notes ordered by most-recently-updated.
func (r *repo) ListNotes(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]*models.UserNote, error) {
	q := repository.Queries(ctx, r.q)
	rows, err := q.ListUserNotes(ctx, storage.ListUserNotesParams{
		AuthorID: authorID,
		Lim:      limit,
		Off:      offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*models.UserNote, len(rows))
	for i, row := range rows {
		n := &models.UserNote{
			AuthorID:       row.AuthorID,
			TargetID:       row.TargetID,
			Content:        row.Content,
			UpdatedAt:      row.UpdatedAt.Time,
			TargetUsername: row.TargetUsername,
		}
		if row.TargetDisplayName.Valid {
			s := row.TargetDisplayName.String
			n.TargetDisplayName = &s
		}
		result[i] = n
	}
	return result, nil
}
