package admin

import (
	"context"
	"go-service-template/internal/models"
	v1 "go-service-template/internal/transport/http/v1"

	"github.com/google/uuid"
)

type service interface {
	GetAdminStats(ctx context.Context) (*models.AdminStats, error)
	ListUsersAdmin(ctx context.Context, limit, offset int32) ([]*models.AdminUser, error)
	SearchUsersAdmin(ctx context.Context, query string, limit, offset int32) ([]*models.AdminUser, error)
	BanUser(ctx context.Context, id uuid.UUID) (*models.User, error)
	UnbanUser(ctx context.Context, id uuid.UUID) (*models.User, error)
	AdminUpdateUsername(ctx context.Context, id uuid.UUID, username string) (*models.User, error)
	AdminUpdateGender(ctx context.Context, id uuid.UUID, gender *string) (*models.User, error)
	AdminUpdatePassword(ctx context.Context, id uuid.UUID, newPassword string) error
	AdminUpdateBalance(ctx context.Context, id uuid.UUID, amount int32) (*models.Balance, error)
	AdminUpdateDisplayName(ctx context.Context, id uuid.UUID, displayName *string) (*models.User, error)
	AdminUpdateTag(ctx context.Context, id uuid.UUID, tag *string) (*models.User, error)
	AdminUpdateSpecialTag(ctx context.Context, id uuid.UUID, specialTag *string) (*models.User, error)
	AdminUpdateCaptchaType(ctx context.Context, id uuid.UUID, captchaType string) (*models.User, error)
	AdminClearPromotion(ctx context.Context, id uuid.UUID) (*models.User, error)
	AdminDeleteUser(ctx context.Context, id uuid.UUID) error
	CreateAnnouncement(ctx context.Context, adminID uuid.UUID, message string) (*models.Announcement, error)
	DeactivateAnnouncement(ctx context.Context, id uuid.UUID) error
}

type AdminHandler struct {
	svc service
}

func New(svc service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func ptr[T any](v T) *T {
	return &v
}

func toV1User(u *models.User) v1.User {
	bal := int(u.Balance)
	user := v1.User{
		Id:                   u.ID,
		Username:             u.Username,
		Role:                 v1.UserRole(u.Role),
		DisplayName:          u.DisplayName,
		Tag:                  u.Tag,
		SpecialTag:           u.SpecialTag,
		TelegramId:           u.TelegramID,
		CaptchaType:          v1.CaptchaType(u.CaptchaType),
		CaptchaCooldownUntil: u.CaptchaCooldownUntil,
		PromotedUntil:        u.PromotedUntil,
		PromotionMessage:     u.PromotionMessage,
		PromotionBid:         ptr(int(u.PromotionBid)),
		VipRemainingSeconds:  ptr(int(u.VipRemainingSeconds)),
		VipCooldownUntil:     u.VipCooldownUntil,
		Balance:              &bal,
	}
	if u.Gender != nil {
		g := v1.Gender(*u.Gender)
		user.Gender = &g
	}
	return user
}

func toV1AdminUser(u *models.User) v1.AdminUser {
	au := v1.AdminUser{
		Id:                   u.ID,
		Username:             u.Username,
		Role:                 v1.AdminUserRole(u.Role),
		Balance:              int(u.Balance),
		DisplayName:          u.DisplayName,
		Tag:                  u.Tag,
		SpecialTag:           u.SpecialTag,
		CaptchaType:          v1.CaptchaType(u.CaptchaType),
		CaptchaCooldownUntil: u.CaptchaCooldownUntil,
		PromotedUntil:        u.PromotedUntil,
		PromotionMessage:     u.PromotionMessage,
		PromotionBid:         ptr(int(u.PromotionBid)),
		VipRemainingSeconds:  ptr(int(u.VipRemainingSeconds)),
		VipCooldownUntil:     u.VipCooldownUntil,
	}
	if u.Gender != nil {
		g := v1.Gender(*u.Gender)
		au.Gender = &g
	}
	if u.BannedAt != nil {
		au.BannedAt = u.BannedAt
	}
	return au
}

func toV1AdminUserFromAdmin(u *models.AdminUser) v1.AdminUser {
	au := v1.AdminUser{
		Id:                   u.ID,
		Username:             u.Username,
		Role:                 v1.AdminUserRole(u.Role),
		Balance:              int(u.Balance),
		CreatedAt:            u.CreatedAt,
		DisplayName:          u.DisplayName,
		Tag:                  u.Tag,
		SpecialTag:           u.SpecialTag,
		LastVisitAt:          u.LastVisitAt,
		CaptchaType:          v1.CaptchaType(u.CaptchaType),
		CaptchaCooldownUntil: u.CaptchaCooldownUntil,
		PromotedUntil:        u.PromotedUntil,
		PromotionMessage:     u.PromotionMessage,
		PromotionBid:         ptr(int(u.PromotionBid)),
		VipRemainingSeconds:  ptr(int(u.VipRemainingSeconds)),
		VipCooldownUntil:     u.VipCooldownUntil,
	}
	if u.Gender != nil {
		g := v1.Gender(*u.Gender)
		au.Gender = &g
	}
	if u.BannedAt != nil {
		au.BannedAt = u.BannedAt
	}
	return au
}
