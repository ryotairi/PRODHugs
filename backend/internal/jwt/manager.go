package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Manager struct {
	secret               []byte
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
}

func NewManager(secret string, accessDuration, refreshDuration time.Duration) *Manager {
	return &Manager{
		secret:               []byte(secret),
		accessTokenDuration:  accessDuration,
		refreshTokenDuration: refreshDuration,
	}
}

// GenerateAccessToken creates a short-lived JWT for API authentication.
func (m *Manager) GenerateAccessToken(userID uuid.UUID, role string) (string, int64, error) {
	now := time.Now()
	exp := now.Add(m.accessTokenDuration)

	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"role": role,
		"type": "access",
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", 0, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, int64(m.accessTokenDuration.Seconds()), nil
}

// GenerateRefreshToken creates a long-lived JWT stored in an HttpOnly cookie.
func (m *Manager) GenerateRefreshToken(userID uuid.UUID) (string, string, int64, error) {
	now := time.Now()
	exp := now.Add(m.refreshTokenDuration)
	jti := uuid.NewString()

	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "refresh",
		"jti":  jti,
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, jti, exp.Unix(), nil
}

// RefreshTokenDuration returns the configured refresh token lifetime.
func (m *Manager) RefreshTokenDuration() time.Duration {
	return m.refreshTokenDuration
}

// GenerateCaptchaToken creates a short-lived JWT for completed captchas.
func (m *Manager) GenerateCaptchaToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	exp := now.Add(5 * time.Minute)

	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "captcha",
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ParseCaptchaToken validates a captcha token.
func (m *Manager) ParseCaptchaToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "captcha" {
			return uuid.Nil, fmt.Errorf("invalid token type")
		}

		userIDStr, ok := claims["sub"].(string)
		if !ok {
			return uuid.Nil, fmt.Errorf("invalid token claims: missing sub")
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid user ID in token: %w", err)
		}

		return userID, nil
	}

	return uuid.Nil, fmt.Errorf("invalid token")
}
func (m *Manager) ParseToken(tokenString string) (uuid.UUID, string, string, string, int64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return uuid.Nil, "", "", "", 0, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDStr, ok := claims["sub"].(string)
		if !ok {
			return uuid.Nil, "", "", "", 0, fmt.Errorf("invalid token claims: missing sub")
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return uuid.Nil, "", "", "", 0, fmt.Errorf("invalid user ID in token: %w", err)
		}

		role, _ := claims["role"].(string) // role is empty for refresh tokens

		tokenType, ok := claims["type"].(string)
		if !ok {
			return uuid.Nil, "", "", "", 0, fmt.Errorf("invalid token claims: missing type")
		}

		jti, _ := claims["jti"].(string)

		expFloat, ok := claims["exp"].(float64)
		if !ok {
			return uuid.Nil, "", "", "", 0, fmt.Errorf("invalid token claims: missing exp")
		}

		return userID, role, tokenType, jti, int64(expFloat), nil
	}

	return uuid.Nil, "", "", "", 0, fmt.Errorf("invalid token")
}
