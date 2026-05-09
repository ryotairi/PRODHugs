package user

import (
	"context"
	"time"

	"go-service-template/internal/db/sqlc/storage"
	"go-service-template/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *repo) CreateSudokuCaptcha(ctx context.Context, userID uuid.UUID, puzzle []byte, solution []byte, expiresAt time.Time) (storage.SudokuCaptcha, error) {
	q := repository.Queries(ctx, r.q)
	
	exp := pgtype.Timestamptz{Time: expiresAt, Valid: true}
	return q.CreateSudokuCaptcha(ctx, storage.CreateSudokuCaptchaParams{
		UserID:    userID,
		Puzzle:    puzzle,
		Solution:  solution,
		ExpiresAt: exp,
	})
}

func (r *repo) SetSudokuCooldown(ctx context.Context, userID uuid.UUID, cooldownUntil time.Time) error {
	q := repository.Queries(ctx, r.q)
	cd := pgtype.Timestamptz{Time: cooldownUntil, Valid: true}
	return q.SetSudokuCooldown(ctx, storage.SetSudokuCooldownParams{
		ID:                  userID,
		SudokuCooldownUntil: cd,
	})
}

func (r *repo) GetSudokuCaptcha(ctx context.Context, id uuid.UUID) (storage.SudokuCaptcha, error) {
	q := repository.Queries(ctx, r.q)
	return q.GetSudokuCaptcha(ctx, id)
}

func (r *repo) IncrementSudokuErrors(ctx context.Context, id uuid.UUID) (storage.SudokuCaptcha, error) {
	q := repository.Queries(ctx, r.q)
	return q.IncrementSudokuErrors(ctx, id)
}

func (r *repo) MarkSudokuPassed(ctx context.Context, id uuid.UUID) (storage.SudokuCaptcha, error) {
	q := repository.Queries(ctx, r.q)
	return q.MarkSudokuPassed(ctx, id)
}

func (r *repo) DeleteSudokuCaptcha(ctx context.Context, id uuid.UUID) error {
	q := repository.Queries(ctx, r.q)
	return q.DeleteSudokuCaptcha(ctx, id)
}
