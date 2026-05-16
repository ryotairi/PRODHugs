package user

import (
	"context"
	"time"

	"go-service-template/internal/db/sqlc/storage"
	"go-service-template/internal/models"
	"go-service-template/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *repo) UpdateSettings(ctx context.Context, id uuid.UUID, gender *string, displayName *string, tag *string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	var g pgtype.Text
	if gender != nil {
		g = pgtype.Text{String: *gender, Valid: true}
	}

	var dn pgtype.Text
	if displayName != nil {
		dn = pgtype.Text{String: *displayName, Valid: true}
	}

	var t pgtype.Text
	if tag != nil {
		t = pgtype.Text{String: *tag, Valid: true}
	}

	u, err := q.UpdateUserSettings(ctx, storage.UpdateUserSettingsParams{
		ID:          id,
		Gender:      g,
		DisplayName: dn,
		Tag:         t,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) GetTelegramID(ctx context.Context, userID uuid.UUID) (*int64, error) {
	q := repository.Queries(ctx, r.q)

	tid, err := q.GetUserTelegramID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !tid.Valid {
		return nil, nil
	}
	return &tid.Int64, nil
}

// SetTelegramID stores the given telegram ID for the user.
func (r *repo) SetTelegramID(ctx context.Context, userID uuid.UUID, telegramID int64) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	uid, err := q.SetUserTelegramID(ctx, storage.SetUserTelegramIDParams{ID: userID, TelegramID: pgtype.Int8{Int64: telegramID, Valid: true}})
	if err != nil {
		return nil, err
	}
	return toModelUser(uid), nil
}

// ClearTelegramID removes the Telegram ID from the user.
func (r *repo) ClearTelegramID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	uid, err := q.ClearUserTelegramID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toModelUser(uid), nil
}

// IsTelegramIDTaken checks if a telegram ID is already bound to another account.
func (r *repo) IsTelegramIDTaken(ctx context.Context, telegramID int64, excludeUserID uuid.UUID) (bool, error) {
	q := repository.Queries(ctx, r.q)

	taken, err := q.IsTelegramIDTaken(ctx, storage.IsTelegramIDTakenParams{TelegramID: pgtype.Int8{Int64: telegramID, Valid: true}, ID: excludeUserID})
	if err != nil {
		return false, err
	}
	return taken, nil
}

func (r *repo) UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error {
	q := repository.Queries(ctx, r.q)

	return q.UpdateUserPassword(ctx, storage.UpdateUserPasswordParams{
		ID:       id,
		Password: hashedPassword,
	})
}

func (r *repo) PromoteUser(ctx context.Context, id uuid.UUID, promotedUntil time.Time, message *string, bid int32) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	var msg pgtype.Text
	if message != nil {
		msg = pgtype.Text{String: *message, Valid: true}
	}

	u, err := q.PromoteUser(ctx, storage.PromoteUserParams{
		ID:            id,
		PromotedUntil: pgtype.Timestamptz{Time: promotedUntil, Valid: true},
		PromotionMessage: msg,
		PromotionBid:  bid,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}
