package v2

import (
	"context"

	"go-service-template/internal/transport/http/authctx"

	"github.com/google/uuid"
)

// userIDFromCtx reads the authenticated user ID injected by the OpenAPI
// security middleware (shared with v1 via the authctx package).
func userIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(authctx.UserIDContextKey).(uuid.UUID)
	return v, ok
}
