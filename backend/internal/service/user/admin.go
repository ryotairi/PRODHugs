package user

import (
	"context"
	"fmt"
	"go-service-template/internal/models"
	"go-service-template/pkg/crypto"
	"time"

	"github.com/google/uuid"
)

func (s *service) GetAdminStats(ctx context.Context) (*models.AdminStats, error) {
	total, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	banned, err := s.repo.CountBannedUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count banned users: %w", err)
	}

	return &models.AdminStats{
		TotalUsers:  total,
		BannedUsers: banned,
	}, nil
}

func (s *service) ListUsersAdmin(ctx context.Context, limit, offset int32) ([]*models.AdminUser, error) {
	return s.repo.ListUsersAdmin(ctx, limit, offset)
}

func (s *service) SearchUsersAdmin(ctx context.Context, query string, limit, offset int32) ([]*models.AdminUser, error) {
	return s.repo.SearchUsersAdmin(ctx, query, limit, offset)
}

func (s *service) BanUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.repo.BanUser(ctx, id)
}

func (s *service) UnbanUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.repo.UnbanUser(ctx, id)
}

func (s *service) AdminUpdateUsername(ctx context.Context, id uuid.UUID, username string) (*models.User, error) {
	return s.repo.AdminUpdateUsername(ctx, id, username)
}

func (s *service) AdminUpdateGender(ctx context.Context, id uuid.UUID, gender *string) (*models.User, error) {
	return s.repo.AdminUpdateGender(ctx, id, gender)
}

func (s *service) AdminUpdatePassword(ctx context.Context, id uuid.UUID, newPassword string) error {
	hash, err := crypto.GenerateHash(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.repo.AdminUpdatePassword(ctx, id, hash)
}

func (s *service) AdminUpdateDisplayName(ctx context.Context, id uuid.UUID, displayName *string) (*models.User, error) {
	return s.repo.AdminUpdateDisplayName(ctx, id, displayName)
}

func (s *service) AdminUpdateTag(ctx context.Context, id uuid.UUID, tag *string) (*models.User, error) {
	return s.repo.AdminUpdateTag(ctx, id, tag)
}

func (s *service) AdminUpdateSpecialTag(ctx context.Context, id uuid.UUID, specialTag *string) (*models.User, error) {
	return s.repo.AdminUpdateSpecialTag(ctx, id, specialTag)
}

func (s *service) AdminDeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.repo.AdminDeleteUser(ctx, id)
}

func (s *service) ClearExpiredPromotions(ctx context.Context) (int64, error) {
	vips, err := s.repo.ListVIPUsers(ctx)
	if err != nil {
		return 0, err
	}

	var expiredCount int64
	for i, u := range vips {
		// Only first 3 are considered active and lose time
		if i < 3 {
			newRemaining := u.VipRemainingSeconds - 60
			if newRemaining <= 0 {
				// Expired!
				_, err := s.repo.AdminClearPromotion(ctx, u.ID)
				if err == nil {
					// Set 6h cooldown and reset budget for next time
					_, _ = s.repo.SetVipCooldown(ctx, u.ID, time.Now().Add(6*time.Hour), 86400)
					expiredCount++
				}
			} else {
				// Deduct time
				_, _ = s.repo.UpdateVipBudget(ctx, u.ID, newRemaining)
			}
		}
	}

	if expiredCount > 0 && s.onPromotionUpdated != nil {
		s.onPromotionUpdated()
	}
	return expiredCount, nil
}

func (s *service) AdminUpdateBalance(ctx context.Context, id uuid.UUID, amount int32) (*models.Balance, error) {
	return s.balanceRepo.AdminSetBalance(ctx, id, amount)
}

func (s *service) AdminUpdateCaptchaType(ctx context.Context, id uuid.UUID, captchaType string) (*models.User, error) {
	return s.repo.AdminUpdateCaptchaType(ctx, id, captchaType)
}

func (s *service) AdminClearPromotion(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u, err := s.repo.AdminClearPromotion(ctx, id)
	if err == nil && s.onPromotionUpdated != nil {
		s.onPromotionUpdated()
	}
	return u, err
}
