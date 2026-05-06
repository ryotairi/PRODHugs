package hug

import (
	"context"

	"go-service-template/internal/transport/http/middleware"
	v1 "go-service-template/internal/transport/http/v1"

	"github.com/google/uuid"
)

func (h *HugHandler) GetActiveAnnouncement(ctx context.Context, _ v1.GetActiveAnnouncementRequestObject) (v1.GetActiveAnnouncementResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	a, err := h.annSvc.GetActiveAnnouncement(ctx, userID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return v1.GetActiveAnnouncement204Response{}, nil
	}

	return v1.GetActiveAnnouncement200JSONResponse{
		Id:        a.ID,
		Message:   a.Message,
		CreatedAt: a.CreatedAt,
	}, nil
}

func (h *HugHandler) DismissAnnouncement(ctx context.Context, req v1.DismissAnnouncementRequestObject) (v1.DismissAnnouncementResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	err := h.annSvc.DismissAnnouncement(ctx, userID, req.AnnouncementId)
	if err != nil {
		return nil, err
	}

	return v1.DismissAnnouncement200JSONResponse{Message: "dismissed"}, nil
}
