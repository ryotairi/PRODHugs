package middleware

import (
	"context"
	"fmt"
	"go-service-template/internal/jwt"
	"go-service-template/internal/transport/http/authctx"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
)

// UserIDContextKey is re-exported from authctx for backward compatibility
// with existing callers (v1 handlers). New code should import authctx
// directly to avoid pulling in the full middleware package.
const (
	UserIDContextKey   = authctx.UserIDContextKey
	UserRoleContextKey = authctx.UserRoleContextKey
)

func NewAuthenticator(jwtManager *jwt.Manager) openapi3filter.AuthenticationFunc {
	return func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
		if input.SecuritySchemeName != "BearerAuth" {
			return fmt.Errorf("unsupported security scheme: %s", input.SecuritySchemeName)
		}

		authHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
		if authHeader == "" {
			return fmt.Errorf("missing Authorization header")
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return fmt.Errorf("invalid Authorization header format")
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		userID, userRole, tokenType, _, _, err := jwtManager.ParseToken(tokenString)
		if err != nil {
			return fmt.Errorf("invalid token: %w", err)
		}

		if tokenType != "access" {
			return fmt.Errorf("invalid token type: expected access token")
		}

		requiredScopes := input.Scopes

		if len(requiredScopes) > 0 {
			hasAccess := false
			for _, scope := range requiredScopes {
				if scope == userRole {
					hasAccess = true
					break
				}
			}
			if !hasAccess {
				return fmt.Errorf("insufficient permissions: required scopes %v, but user role is %s", requiredScopes, userRole)
			}
		}

		req := input.RequestValidationInput.Request
		ctxWithUser := context.WithValue(req.Context(), authctx.UserIDContextKey, userID)
		ctxWithUser = context.WithValue(ctxWithUser, authctx.UserRoleContextKey, userRole)

		*req = *req.WithContext(ctxWithUser)

		return nil
	}
}
