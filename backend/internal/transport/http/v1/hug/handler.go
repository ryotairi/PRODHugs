package hug

import (
	"context"
	"go-service-template/internal/models"
	hug "go-service-template/internal/service/hug"

	"github.com/google/uuid"
)

type service interface {
	SuggestHug(ctx context.Context, giverID, receiverID uuid.UUID, hugType string, comment *string, captchaToken *string) (*models.Hug, *models.User, error)
	AcceptHug(ctx context.Context, hugID, receiverID uuid.UUID) (*models.Hug, error)
	DeclineHug(ctx context.Context, hugID, receiverID uuid.UUID) error
	CancelHug(ctx context.Context, hugID, giverID uuid.UUID) error
	GetCooldownInfo(ctx context.Context, userA, userB uuid.UUID) (*hug.CooldownInfoResult, error)
	UpgradeCooldown(ctx context.Context, payerID, otherUserID uuid.UUID) (*models.HugCooldown, error)
	GetBalance(ctx context.Context, userID uuid.UUID) (*models.Balance, error)
	GetHugDetail(ctx context.Context, hugID, requesterID uuid.UUID, isAdmin bool) (*models.HugDetail, error)
	GetOutgoingHugs(ctx context.Context, userID uuid.UUID) ([]*models.OutgoingPendingHug, *models.SlotInfo, error)
	BuyHugSlot(ctx context.Context, userID uuid.UUID) (*models.SlotInfo, int32, error)
	GetHugHistory(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*models.HugFeedItem, error)
	GetRecentFeed(ctx context.Context, limit, offset int32) ([]*models.HugFeedItem, error)
	GetHugActivity(ctx context.Context) ([]*models.HugActivityItem, error)
	GetLeaderboard(ctx context.Context, limit, offset int32) ([]*models.LeaderboardEntry, error)
	GetUserStats(ctx context.Context, userID uuid.UUID, gender *string) (*models.UserStats, error)
	GetUserProfile(ctx context.Context, userID uuid.UUID, viewerID *uuid.UUID) (*models.User, *models.UserStats, *models.Balance, *models.MutualHugStats, bool, *models.IntimacyInfo, error)
	SearchUsers(ctx context.Context, query string, viewerID uuid.UUID, limit, offset int32) ([]*models.User, error)
	ClaimDailyReward(ctx context.Context, userID uuid.UUID) (int32, int32, int32, bool, error)
	GetPendingInbox(ctx context.Context, userID uuid.UUID) ([]*models.PendingHugInboxItem, error)
	GetInboxCount(ctx context.Context, userID uuid.UUID) (int64, error)
	BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error
	UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error
	GetBlockedUsers(ctx context.Context, userID uuid.UUID) ([]*models.BlockedUser, error)
	GetPairIntimacy(ctx context.Context, userA, userB uuid.UUID) (*models.IntimacyInfo, error)
	GetUserConnections(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*models.ConnectionItem, error)
	GetIntimacyLeaderboard(ctx context.Context, limit, offset int32) ([]*models.LeaderboardPairEntry, error)
	// Streak methods
	GetPairStreak(ctx context.Context, userA, userB uuid.UUID) (*models.StreakInfo, error)
	GetUserTopStreaks(ctx context.Context, userID uuid.UUID, limit int32) ([]*models.TopStreakEntry, error)
	GetPairStreakCalendar(ctx context.Context, userA, userB uuid.UUID) ([]*models.StreakCalendarDay, error)
}

type announcementService interface {
	GetActiveAnnouncement(ctx context.Context, userID uuid.UUID) (*models.Announcement, error)
	DismissAnnouncement(ctx context.Context, userID, announcementID uuid.UUID) error
}

type HugHandler struct {
	svc    service
	annSvc announcementService
}

func New(svc service, annSvc announcementService) *HugHandler {
	return &HugHandler{svc: svc, annSvc: annSvc}
}
