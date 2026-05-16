package user

import (
	"context"
	"fmt"
	"go-service-template/internal/errorz"
	"go-service-template/internal/models"
	"go-service-template/pkg/crypto"

	"github.com/google/uuid"
)

func (s *service) Login(ctx context.Context, username string, password string) (*models.User, string, string, error) {
	u, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, "", "", err
	}

	ok, err := crypto.ComparePasswordAndHash(password, u.HashedPassword)
	if err != nil {
		return nil, "", "", err
	}
	if !ok {
		return nil, "", "", errorz.ErrInvalidCredentials
	}

	if u.BannedAt != nil {
		return nil, "", "", errorz.ErrUserBanned
	}

	accessToken, _, err := s.jwtManager.GenerateAccessToken(u.ID, u.Role)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, jti, expUnix, err := s.jwtManager.GenerateRefreshToken(u.ID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	if err := s.refreshTokenRepo.SaveRefreshToken(ctx, jti, u.ID, expUnix); err != nil {
		return nil, "", "", fmt.Errorf("failed to persist refresh token: %w", err)
	}

	return u, accessToken, refreshToken, nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.repo.GetByUsername(ctx, username)
}

func (s *service) ListVIPUsers(ctx context.Context) ([]*models.User, error) {
	return s.repo.ListVIPUsers(ctx)
}
