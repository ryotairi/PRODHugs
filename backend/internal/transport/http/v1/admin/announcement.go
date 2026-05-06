package admin

import (
	"context"

	"go-service-template/internal/transport/http/middleware"
	v1 "go-service-template/internal/transport/http/v1"

	"github.com/google/uuid"
)

func (h *AdminHandler) CreateAnnouncement(ctx context.Context, req v1.CreateAnnouncementRequestObject) (v1.CreateAnnouncementResponseObject, error) {
	adminID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	a, err := h.svc.CreateAnnouncement(ctx, adminID, req.Body.Message)
	if err != nil {
		return nil, err
	}

	return v1.CreateAnnouncement201JSONResponse{
		Id:        a.ID,
		Message:   a.Message,
		CreatedAt: a.CreatedAt,
	}, nil
}

func (h *AdminHandler) DeleteAnnouncement(ctx context.Context, req v1.DeleteAnnouncementRequestObject) (v1.DeleteAnnouncementResponseObject, error) {
	err := h.svc.DeactivateAnnouncement(ctx, req.AnnouncementId)
	if err != nil {
		return nil, err
	}

	return v1.DeleteAnnouncement200JSONResponse{Message: "Announcement removed"}, nil
}
