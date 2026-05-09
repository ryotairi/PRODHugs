package sudoku

import (
	"context"
	"go-service-template/internal/db/sqlc/storage"
	"go-service-template/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Queries(ctx context.Context) *storage.Queries {
	return repository.Queries(ctx, r.db)
}

func (r *Repository) CreateSudokuCaptcha(ctx context.Context, userID uuid.UUID, puzzle []byte, solution []byte, expiresAt pgtype.Timestamptz) (storage.SudokuCaptcha, error) {
	return r.Queries(ctx).CreateSudokuCaptcha(ctx, storage.CreateSudokuCaptchaParams{
		UserID:    userID,
		Puzzle:    puzzle,
		Solution:  solution,
		ExpiresAt: expiresAt,
	})
}

func (r *Repository) GetSudokuCaptcha(ctx context.Context, id uuid.UUID) (storage.SudokuCaptcha, error) {
	return r.Queries(ctx).GetSudokuCaptcha(ctx, id)
}

func (r *Repository) IncrementSudokuErrors(ctx context.Context, id uuid.UUID) (storage.SudokuCaptcha, error) {
	return r.Queries(ctx).IncrementSudokuErrors(ctx, id)
}

func (r *Repository) MarkSudokuPassed(ctx context.Context, id uuid.UUID) (storage.SudokuCaptcha, error) {
	return r.Queries(ctx).MarkSudokuPassed(ctx, id)
}

func (r *Repository) DeleteSudokuCaptcha(ctx context.Context, id uuid.UUID) error {
	return r.Queries(ctx).DeleteSudokuCaptcha(ctx, id)
}
