package user

import (
	"context"
	"fmt"
	"regexp"

	"go-service-template/internal/errorz"
	"go-service-template/internal/models"

	"github.com/google/uuid"
)

// matrixIDPattern matches a reasonable-looking MXID: @localpart:servername.
// Localpart allows the characters the spec permits for modern MXIDs
// ([a-z0-9._=/-]+); historical MXIDs may include other chars but we keep
// validation lenient — the real check is the homeserver's response.
var matrixIDPattern = regexp.MustCompile(`^@[A-Za-z0-9._=/\-+]+:[^\s@]+$`)

// RequestMatrixLink validates the MXID, asks the bot to open a confirmation DM
// with the user, and returns the bot URL. The linking is only completed once
// the user accepts in the DM (via reaction or `!accept`).
func (s *service) RequestMatrixLink(ctx context.Context, userID uuid.UUID, matrixID string) (botUserID, botURL string, err error) {
	if s.matrixBot == nil || !s.matrixBot.Enabled() || s.matrixBotUserID == "" {
		return "", "", fmt.Errorf("matrix linking not configured")
	}
	if !matrixIDPattern.MatchString(matrixID) {
		return "", "", errorz.ErrInvalidMatrixID
	}

	taken, err := s.repo.IsMatrixIDTaken(ctx, matrixID, userID)
	if err != nil {
		return "", "", fmt.Errorf("check matrix id: %w", err)
	}
	if taken {
		return "", "", errorz.ErrMatrixIDTaken
	}

	if _, _, err := s.matrixBot.InitiateLink(ctx, userID, matrixID); err != nil {
		return "", "", fmt.Errorf("initiate matrix link: %w", err)
	}

	return s.matrixBotUserID, "https://matrix.to/#/" + s.matrixBotUserID, nil
}

// UnlinkMatrix removes the user's Matrix account linkage.
func (s *service) UnlinkMatrix(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return s.repo.ClearMatrixID(ctx, userID)
}

// MatrixBotDeepLink returns the matrix.to URL for the bot (or empty if bot is disabled).
func (s *service) MatrixBotDeepLink() string {
	if s.matrixBotUserID == "" {
		return ""
	}
	return "https://matrix.to/#/" + s.matrixBotUserID
}
