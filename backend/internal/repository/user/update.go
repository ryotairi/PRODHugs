package user

import (
	"context"
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

// GetMatrixID returns the matrix ID and DM room ID for the given user, or (nil, nil) if not linked.
func (r *repo) GetMatrixID(ctx context.Context, userID uuid.UUID) (*string, *string, error) {
	q := repository.Queries(ctx, r.q)

	row, err := q.GetUserMatrixID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	var mid *string
	if row.MatrixID.Valid {
		mid = &row.MatrixID.String
	}
	var rid *string
	if row.MatrixRoomID.Valid {
		rid = &row.MatrixRoomID.String
	}
	return mid, rid, nil
}

// SetMatrixID stores the given matrix ID (and optional DM room ID) for the user.
func (r *repo) SetMatrixID(ctx context.Context, userID uuid.UUID, matrixID string, roomID string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	var rid pgtype.Text
	if roomID != "" {
		rid = pgtype.Text{String: roomID, Valid: true}
	}

	u, err := q.SetUserMatrixID(ctx, storage.SetUserMatrixIDParams{
		ID:           userID,
		MatrixID:     pgtype.Text{String: matrixID, Valid: true},
		MatrixRoomID: rid,
	})
	if err != nil {
		return nil, err
	}
	return toModelUser(u), nil
}

// ClearMatrixID removes the Matrix ID and room from the user.
func (r *repo) ClearMatrixID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.ClearUserMatrixID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toModelUser(u), nil
}

// IsMatrixIDTaken checks if a matrix ID is already bound to another account.
func (r *repo) IsMatrixIDTaken(ctx context.Context, matrixID string, excludeUserID uuid.UUID) (bool, error) {
	q := repository.Queries(ctx, r.q)

	taken, err := q.IsMatrixIDTaken(ctx, storage.IsMatrixIDTakenParams{MatrixID: pgtype.Text{String: matrixID, Valid: true}, ID: excludeUserID})
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
