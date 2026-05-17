package user

import (
	"context"
	"database/sql"
	"errors"
	"go-service-template/internal/db/sqlc/storage"
	"go-service-template/internal/errorz"
	"go-service-template/internal/models"
	"go-service-template/internal/repository"
	"go-service-template/pkg/dberrors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *repo) BanUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.BanUser(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorz.ErrCannotBanAdmin
		}
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) UnbanUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.UnbanUser(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorz.ErrUserNotFound
		}
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) CountUsers(ctx context.Context) (int64, error) {
	q := repository.Queries(ctx, r.q)
	return q.CountUsers(ctx)
}

func (r *repo) CountBannedUsers(ctx context.Context) (int64, error) {
	q := repository.Queries(ctx, r.q)
	return q.CountBannedUsers(ctx)
}

func (r *repo) ListUsersAdmin(ctx context.Context, limit, offset int32) ([]*models.AdminUser, error) {
	q := repository.Queries(ctx, r.q)

	rows, err := q.ListUsersAdmin(ctx, storage.ListUsersAdminParams{
		Lim: limit,
		Off: offset,
	})
	if err != nil {
		return nil, err
	}

	users := make([]*models.AdminUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, toAdminUser(row))
	}
	return users, nil
}

func (r *repo) SearchUsersAdmin(ctx context.Context, query string, limit, offset int32) ([]*models.AdminUser, error) {
	q := repository.Queries(ctx, r.q)

	rows, err := q.SearchUsersAdmin(ctx, storage.SearchUsersAdminParams{
		Query: query,
		Lim:   limit,
		Off:   offset,
	})
	if err != nil {
		return nil, err
	}

	users := make([]*models.AdminUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, toAdminUserFromSearch(row))
	}
	return users, nil
}

func (r *repo) AdminUpdateUsername(ctx context.Context, id uuid.UUID, username string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.AdminUpdateUsername(ctx, storage.AdminUpdateUsernameParams{
		ID:       id,
		Username: username,
	})
	if err != nil {
		if dberrors.IsUniqueViolation(err) {
			return nil, errorz.ErrUserAlreadyExists
		}
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) AdminUpdateGender(ctx context.Context, id uuid.UUID, gender *string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	var g pgtype.Text
	if gender != nil {
		g = pgtype.Text{String: *gender, Valid: true}
	}

	u, err := q.AdminUpdateGender(ctx, storage.AdminUpdateGenderParams{
		ID:     id,
		Gender: g,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) AdminUpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error {
	q := repository.Queries(ctx, r.q)

	return q.AdminUpdatePassword(ctx, storage.AdminUpdatePasswordParams{
		ID:       id,
		Password: hashedPassword,
	})
}

func (r *repo) AdminDeleteUser(ctx context.Context, id uuid.UUID) error {
	q := repository.Queries(ctx, r.q)

	rows, err := q.AdminDeleteUser(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errorz.ErrCannotDeleteAdmin
	}
	return nil
}

func (r *repo) ClearExpiredPromotions(ctx context.Context) (int64, error) {
	q := repository.Queries(ctx, r.q)
	return q.ClearExpiredPromotions(ctx)
}

func (r *repo) AdminUpdateTag(ctx context.Context, id uuid.UUID, tag *string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	var t pgtype.Text
	if tag != nil {
		t = pgtype.Text{String: *tag, Valid: true}
	}

	u, err := q.AdminUpdateTag(ctx, storage.AdminUpdateTagParams{
		ID:  id,
		Tag: t,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) AdminUpdateDisplayName(ctx context.Context, id uuid.UUID, displayName *string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	var dn pgtype.Text
	if displayName != nil {
		dn = pgtype.Text{String: *displayName, Valid: true}
	}

	u, err := q.AdminUpdateDisplayName(ctx, storage.AdminUpdateDisplayNameParams{
		ID:          id,
		DisplayName: dn,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) AdminUpdateSpecialTag(ctx context.Context, id uuid.UUID, specialTag *string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	var t pgtype.Text
	if specialTag != nil {
		t = pgtype.Text{String: *specialTag, Valid: true}
	}

	u, err := q.AdminUpdateSpecialTag(ctx, storage.AdminUpdateSpecialTagParams{
		ID:         id,
		SpecialTag: t,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) AdminUpdateCaptchaType(ctx context.Context, id uuid.UUID, captchaType string) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.AdminUpdateCaptchaType(ctx, storage.AdminUpdateCaptchaTypeParams{
		ID:          id,
		CaptchaType: captchaType,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorz.ErrUserNotFound
		}
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) AdminClearPromotion(ctx context.Context, id uuid.UUID) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.AdminClearPromotion(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorz.ErrUserNotFound
		}
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) SetVipCooldown(ctx context.Context, id uuid.UUID, cooldownUntil time.Time, remainingSeconds int32) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.SetVipCooldown(ctx, storage.SetVipCooldownParams{
		ID:                  id,
		VipCooldownUntil:    pgtype.Timestamptz{Time: cooldownUntil, Valid: true},
		VipRemainingSeconds: remainingSeconds,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}

func (r *repo) UpdateVipBudget(ctx context.Context, id uuid.UUID, remainingSeconds int32) (*models.User, error) {
	q := repository.Queries(ctx, r.q)

	u, err := q.UpdateVipBudget(ctx, storage.UpdateVipBudgetParams{
		ID:                  id,
		VipRemainingSeconds: remainingSeconds,
	})
	if err != nil {
		return nil, err
	}

	return toModelUser(u), nil
}
