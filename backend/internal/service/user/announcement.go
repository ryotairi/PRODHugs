package user

import (
	"context"

	"go-service-template/internal/models"

	"github.com/google/uuid"
)

// GetActiveAnnouncement returns the active announcement for a user (nil if none or dismissed).
func (s *service) GetActiveAnnouncement(ctx context.Context, userID uuid.UUID) (*models.Announcement, error) {
	if s.announcementRepo == nil {
		return nil, nil
	}
	return s.announcementRepo.GetActiveForUser(ctx, userID)
}

// CreateAnnouncement creates a new announcement (admin). Deactivates any previous one.
func (s *service) CreateAnnouncement(ctx context.Context, adminID uuid.UUID, message string) (*models.Announcement, error) {
	if s.announcementRepo == nil {
		return nil, nil
	}

	a, err := s.announcementRepo.Create(ctx, message, adminID)
	if err != nil {
		return nil, err
	}

	if s.onAnnouncementCreated != nil {
		s.onAnnouncementCreated(a)
	}

	return a, nil
}

// DeactivateAnnouncement removes the announcement for everyone.
func (s *service) DeactivateAnnouncement(ctx context.Context, id uuid.UUID) error {
	if s.announcementRepo == nil {
		return nil
	}

	err := s.announcementRepo.Deactivate(ctx, id)
	if err != nil {
		return err
	}

	if s.onAnnouncementRemoved != nil {
		s.onAnnouncementRemoved(id)
	}

	return nil
}

// DismissAnnouncement dismisses the announcement for a single user.
func (s *service) DismissAnnouncement(ctx context.Context, userID, announcementID uuid.UUID) error {
	if s.announcementRepo == nil {
		return nil
	}
	return s.announcementRepo.Dismiss(ctx, announcementID, userID)
}
