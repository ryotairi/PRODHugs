package user

import (
	"context"
	"errors"

	"go-service-template/internal/errorz"
	"go-service-template/internal/transport/http/middleware"
	v1 "go-service-template/internal/transport/http/v1"

	"github.com/google/uuid"
)

// CreateMatrixLink validates the MXID, asks the bot to DM the user with a
// confirmation request, and returns the bot URL/MXID. The frontend polls
// GET /users/me until matrix_id is non-null.
func (h *UserHandler) CreateMatrixLink(ctx context.Context, req v1.CreateMatrixLinkRequestObject) (v1.CreateMatrixLinkResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	if req.Body == nil || req.Body.MatrixId == "" {
		return v1.CreateMatrixLink400JSONResponse{
			BadRequestJSONResponse: v1.BadRequestJSONResponse{
				Code:    v1.INVALIDMATRIXID,
				Message: "matrix_id is required",
			},
		}, nil
	}

	botUserID, botURL, err := h.svc.RequestMatrixLink(ctx, userID, req.Body.MatrixId)
	if err != nil {
		switch {
		case errors.Is(err, errorz.ErrInvalidMatrixID):
			return v1.CreateMatrixLink400JSONResponse{
				BadRequestJSONResponse: v1.BadRequestJSONResponse{
					Code:    v1.INVALIDMATRIXID,
					Message: "Неверный формат MXID. Должно быть @user:server",
				},
			}, nil
		case errors.Is(err, errorz.ErrMatrixIDTaken):
			return v1.CreateMatrixLink400JSONResponse{
				BadRequestJSONResponse: v1.BadRequestJSONResponse{
					Code:    v1.MATRIXIDTAKEN,
					Message: "Этот Matrix аккаунт уже привязан к другому пользователю",
				},
			}, nil
		}
		// Bot not configured / unreachable — surface as 503.
		if !h.svc.MatrixEnabled() {
			return v1.CreateMatrixLink503JSONResponse{
				Code:    v1.MATRIXLOGINFAILED,
				Message: "Matrix integration is not configured",
			}, nil
		}
		return nil, err
	}

	return v1.CreateMatrixLink200JSONResponse{
		MatrixId:  req.Body.MatrixId,
		BotUserId: botUserID,
		BotUrl:    botURL,
	}, nil
}

// UnlinkMatrix removes the user's Matrix linkage from the database. The bot
// also handles unlinks initiated from inside the chat via `!unlink`.
func (h *UserHandler) UnlinkMatrix(ctx context.Context, req v1.UnlinkMatrixRequestObject) (v1.UnlinkMatrixResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	u, err := h.svc.UnlinkMatrix(ctx, userID)
	if err != nil {
		return nil, err
	}
	return v1.UnlinkMatrix200JSONResponse(toV1User(u)), nil
}
