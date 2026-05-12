package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-service-template/internal/errorz"
	"go-service-template/internal/matrix"
	"go-service-template/internal/models"
	"go-service-template/pkg/crypto"
)

// LoginViaMatrix authenticates an existing user by their Matrix ID or
// auto-registers a new one. Mirrors LoginViaTelegram.
func (s *service) LoginViaMatrix(ctx context.Context, info *matrix.MatrixUserInfo) (*models.User, error) {
	// Try to find an existing user with this Matrix ID.
	u, err := s.repo.GetByMatrixID(ctx, info.MatrixID)
	if err == nil {
		if u.BannedAt != nil {
			return nil, errorz.ErrUserBanned
		}
		// Refresh the stored DM room ID if it changed.
		if u.MatrixRoomID == nil || *u.MatrixRoomID != info.RoomID {
			if _, err := s.repo.SetMatrixID(ctx, u.ID, info.MatrixID, info.RoomID); err != nil {
				return nil, fmt.Errorf("update matrix room id: %w", err)
			}
			u.MatrixRoomID = &info.RoomID
		}
		return u, nil
	}
	if !errors.Is(err, errorz.ErrUserNotFound) {
		return nil, fmt.Errorf("lookup by matrix id: %w", err)
	}

	// Auto-register from MXID.
	username, err := s.generateUniqueUsernameFromMatrix(ctx, info)
	if err != nil {
		return nil, fmt.Errorf("generate username: %w", err)
	}

	randomPassword, err := generateRandomPassword()
	if err != nil {
		return nil, fmt.Errorf("generate random password: %w", err)
	}

	hash, err := crypto.GenerateHash(randomPassword)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	input := &models.CreateUser{
		Username:       username,
		Password:       randomPassword,
		HashedPassword: hash,
		Role:           "user",
	}

	u, err = s.repo.Create(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	u, err = s.repo.SetMatrixID(ctx, u.ID, info.MatrixID, info.RoomID)
	if err != nil {
		return nil, fmt.Errorf("set matrix id: %w", err)
	}

	// Copy the Matrix display name into our profile if one was provided.
	if strings.TrimSpace(info.DisplayName) != "" {
		dn := strings.TrimSpace(info.DisplayName)
		if len(dn) > 32 {
			dn = dn[:32]
		}
		if updated, err := s.repo.UpdateSettings(ctx, u.ID, nil, &dn, nil); err == nil {
			u = updated
		}
	}

	return u, nil
}

// generateUniqueUsernameFromMatrix derives a username from the MXID localpart,
// falling back to a random one if nothing sanitizes cleanly.
func (s *service) generateUniqueUsernameFromMatrix(ctx context.Context, info *matrix.MatrixUserInfo) (string, error) {
	// Extract localpart (@localpart:server).
	localpart := info.MatrixID
	if at := strings.Index(localpart, "@"); at == 0 {
		localpart = localpart[1:]
	}
	if colon := strings.Index(localpart, ":"); colon >= 0 {
		localpart = localpart[:colon]
	}

	if base := sanitizeUsername(localpart); base != "" {
		if username, err := s.findAvailableUsername(ctx, base); err == nil {
			return username, nil
		}
	}

	if info.DisplayName != "" {
		if base := sanitizeUsername(info.DisplayName); base != "" {
			if username, err := s.findAvailableUsername(ctx, base); err == nil {
				return username, nil
			}
		}
	}

	suffix, err := randomHex(4)
	if err != nil {
		return "", err
	}
	return "user_" + suffix, nil
}
