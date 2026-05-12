package user

import (
	"context"
	"go-service-template/internal/jwt"
	"go-service-template/internal/matrix"
	"go-service-template/internal/models"
	userService "go-service-template/internal/service/user"
	"go-service-template/internal/telegram"
	v1 "go-service-template/internal/transport/http/v1"
	"time"

	"github.com/google/uuid"
)

type service interface {
	Create(ctx context.Context, input *models.CreateUser) (*models.User, string, string, error)
	Login(ctx context.Context, username string, password string) (*models.User, string, string, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	UpdateSettings(ctx context.Context, id uuid.UUID, gender *string, displayName *string, tag *string) (*models.User, error)
	ChangePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error
	GenerateLinkToken(ctx context.Context, userID uuid.UUID) (string, string, error)
	UnlinkTelegram(ctx context.Context, userID uuid.UUID) (*models.User, error)
	RequestMatrixLink(ctx context.Context, userID uuid.UUID, matrixID string) (string, string, error)
	UnlinkMatrix(ctx context.Context, userID uuid.UUID) (*models.User, error)
	MatrixEnabled() bool
	MatrixBotUserID() string
	MatrixBotDeepLink() string
	SaveRefreshToken(ctx context.Context, jti string, userID uuid.UUID, expiresAtUnix int64) error
	IsRefreshTokenActive(ctx context.Context, jti string) (bool, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error
	GenerateSudokuCaptcha(ctx context.Context, userID uuid.UUID) (uuid.UUID, [][]int, error)
	VerifySudokuCell(ctx context.Context, captchaID uuid.UUID, userID uuid.UUID, row, col, value int) (*userService.CaptchaResult, error)
	CompleteSudoku(ctx context.Context, captchaID uuid.UUID, userID uuid.UUID) (string, error)
	GenerateCasinoCaptcha(ctx context.Context, userID uuid.UUID) (uuid.UUID, time.Time, error)
	SpinCasino(ctx context.Context, captchaID uuid.UUID, userID uuid.UUID) (*userService.CasinoSpinResult, error)
}

type UserHandler struct {
	svc              service
	jwtManager       *jwt.Manager
	cookieSecure     bool
	loginStore       *telegram.LoginStore
	botUsername      string
	matrixLoginStore *matrix.LoginStore
	matrixBotUserID  string
}

func New(svc service, jwtManager *jwt.Manager, cookieSecure bool) *UserHandler {
	return &UserHandler{svc: svc, jwtManager: jwtManager, cookieSecure: cookieSecure}
}

// SetTelegramLoginStore configures the login store and bot username for
// the Telegram login endpoints. Called after construction.
func (h *UserHandler) SetTelegramLoginStore(store *telegram.LoginStore, botUsername string) {
	h.loginStore = store
	h.botUsername = botUsername
}

// SetMatrixLoginStore configures the Matrix signup login store and bot MXID
// for the Matrix login endpoints. Called after construction.
func (h *UserHandler) SetMatrixLoginStore(store *matrix.LoginStore, botUserID string) {
	h.matrixLoginStore = store
	h.matrixBotUserID = botUserID
}

func toV1User(u *models.User) v1.User {
	user := v1.User{
		Id:                   u.ID,
		Username:             u.Username,
		Role:                 v1.UserRole(u.Role),
		DisplayName:          u.DisplayName,
		Tag:                  u.Tag,
		SpecialTag:           u.SpecialTag,
		TelegramId:           u.TelegramID,
		MatrixId:             u.MatrixID,
		CaptchaType:          v1.CaptchaType(u.CaptchaType),
		CaptchaCooldownUntil: u.CaptchaCooldownUntil,
	}
	if u.Gender != nil {
		g := v1.Gender(*u.Gender)
		user.Gender = &g
	}
	return user
}
