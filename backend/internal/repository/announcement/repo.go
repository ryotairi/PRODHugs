package announcement

import (
	"context"

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

func (r *repo) GetActiveForUser(ctx context.Context, userID uuid.UUID) (*models.Announcement, error) {
	q := repository.Queries(ctx, r.q)

	row, err := q.GetActiveAnnouncementForUser(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &models.Announcement{
		ID:        row.ID,
		Message:   row.Message,
		CreatedAt: row.CreatedAt.Time,
		CreatedBy: row.CreatedBy,
		Active:    true,
	}, nil
}

func (r *repo) GetActive(ctx context.Context) (*models.Announcement, error) {
	q := repository.Queries(ctx, r.q)

	row, err := q.GetActiveAnnouncement(ctx)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &models.Announcement{
		ID:        row.ID,
		Message:   row.Message,
		CreatedAt: row.CreatedAt.Time,
		CreatedBy: row.CreatedBy,
		Active:    row.Active,
	}, nil
}

func (r *repo) Create(ctx context.Context, message string, createdBy uuid.UUID) (*models.Announcement, error) {
	q := repository.Queries(ctx, r.q)

	row, err := q.CreateAnnouncement(ctx, storage.CreateAnnouncementParams{
		Message:   message,
		CreatedBy: createdBy,
	})
	if err != nil {
		return nil, err
	}

	return &models.Announcement{
		ID:        row.ID,
		Message:   row.Message,
		CreatedAt: row.CreatedAt.Time,
		CreatedBy: row.CreatedBy,
		Active:    row.Active,
	}, nil
}

func (r *repo) Deactivate(ctx context.Context, id uuid.UUID) error {
	q := repository.Queries(ctx, r.q)
	return q.DeactivateAnnouncement(ctx, id)
}

func (r *repo) Dismiss(ctx context.Context, announcementID, userID uuid.UUID) error {
	q := repository.Queries(ctx, r.q)
	return q.DismissAnnouncement(ctx, storage.DismissAnnouncementParams{
		AnnouncementID: announcementID,
		UserID:         userID,
	})
}
