package user

import (
	"time"

	"go-service-template/internal/db/sqlc/storage"
	"go-service-template/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type repo struct {
	q *storage.Queries
}

func New(db *pgxpool.Pool) *repo {
	return &repo{
		q: storage.New(db),
	}
}

func toModelUser(u storage.User) *models.User {
	var gender *string
	if u.Gender.Valid {
		gender = &u.Gender.String
	}
	var bannedAt *time.Time
	if u.BannedAt.Valid {
		bannedAt = &u.BannedAt.Time
	}
	var createdAt *time.Time
	if u.CreatedAt.Valid {
		createdAt = &u.CreatedAt.Time
	}
	var displayName *string
	if u.DisplayName.Valid {
		displayName = &u.DisplayName.String
	}
	var telegramID *int64
	if u.TelegramID.Valid {
		telegramID = &u.TelegramID.Int64
	}
	var matrixID *string
	if u.MatrixID.Valid {
		matrixID = &u.MatrixID.String
	}
	var matrixRoomID *string
	if u.MatrixRoomID.Valid {
		matrixRoomID = &u.MatrixRoomID.String
	}
	var tag *string
	if u.Tag.Valid {
		tag = &u.Tag.String
	}
	var specialTag *string
	if u.SpecialTag.Valid {
		specialTag = &u.SpecialTag.String
	}
	var captchaCooldownUntil *time.Time
	if u.CaptchaCooldownUntil.Valid {
		captchaCooldownUntil = &u.CaptchaCooldownUntil.Time
	}
	return &models.User{
		ID:                   u.ID,
		Username:             u.Username,
		Role:                 u.Role,
		HashedPassword:       u.Password,
		Gender:               gender,
		DisplayName:          displayName,
		Tag:                  tag,
		SpecialTag:           specialTag,
		TelegramID:           telegramID,
		MatrixID:             matrixID,
		MatrixRoomID:         matrixRoomID,
		BannedAt:             bannedAt,
		CreatedAt:            createdAt,
		CaptchaType:          u.CaptchaType,
		CaptchaCooldownUntil: captchaCooldownUntil,
	}
}

func toAdminUser(u storage.ListUsersAdminRow) *models.AdminUser {
	var gender *string
	if u.Gender.Valid {
		gender = &u.Gender.String
	}
	var bannedAt *time.Time
	if u.BannedAt.Valid {
		bannedAt = &u.BannedAt.Time
	}
	var createdAt *time.Time
	if u.CreatedAt.Valid {
		createdAt = &u.CreatedAt.Time
	}
	var displayName *string
	if u.DisplayName.Valid {
		displayName = &u.DisplayName.String
	}
	var lastVisitAt *time.Time
	if u.LastVisitAt.Valid {
		lastVisitAt = &u.LastVisitAt.Time
	}
	var tag *string
	if u.Tag.Valid {
		tag = &u.Tag.String
	}
	var specialTag *string
	if u.SpecialTag.Valid {
		specialTag = &u.SpecialTag.String
	}
	var captchaCooldownUntil *time.Time
	if u.CaptchaCooldownUntil.Valid {
		captchaCooldownUntil = &u.CaptchaCooldownUntil.Time
	}
	return &models.AdminUser{
		ID:                   u.ID,
		Username:             u.Username,
		Role:                 u.Role,
		Gender:               gender,
		DisplayName:          displayName,
		Tag:                  tag,
		SpecialTag:           specialTag,
		BannedAt:             bannedAt,
		CreatedAt:            createdAt,
		Balance:              u.Balance,
		LastVisitAt:          lastVisitAt,
		CaptchaType:          u.CaptchaType,
		CaptchaCooldownUntil: captchaCooldownUntil,
	}
}

func toAdminUserFromSearch(u storage.SearchUsersAdminRow) *models.AdminUser {
	var gender *string
	if u.Gender.Valid {
		gender = &u.Gender.String
	}
	var bannedAt *time.Time
	if u.BannedAt.Valid {
		bannedAt = &u.BannedAt.Time
	}
	var createdAt *time.Time
	if u.CreatedAt.Valid {
		createdAt = &u.CreatedAt.Time
	}
	var displayName *string
	if u.DisplayName.Valid {
		displayName = &u.DisplayName.String
	}
	var lastVisitAt *time.Time
	if u.LastVisitAt.Valid {
		lastVisitAt = &u.LastVisitAt.Time
	}
	var tag *string
	if u.Tag.Valid {
		tag = &u.Tag.String
	}
	var specialTag *string
	if u.SpecialTag.Valid {
		specialTag = &u.SpecialTag.String
	}
	var captchaCooldownUntil *time.Time
	if u.CaptchaCooldownUntil.Valid {
		captchaCooldownUntil = &u.CaptchaCooldownUntil.Time
	}
	return &models.AdminUser{
		ID:                   u.ID,
		Username:             u.Username,
		Role:                 u.Role,
		Gender:               gender,
		DisplayName:          displayName,
		Tag:                  tag,
		SpecialTag:           specialTag,
		BannedAt:             bannedAt,
		CreatedAt:            createdAt,
		Balance:              u.Balance,
		LastVisitAt:          lastVisitAt,
		CaptchaType:          u.CaptchaType,
		CaptchaCooldownUntil: captchaCooldownUntil,
	}
}
