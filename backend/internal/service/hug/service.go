package hug

import (
	"context"
	"time"

	"go-service-template/internal/cache"
	"go-service-template/internal/models"

	"github.com/google/uuid"
)

type hugRepo interface {
	InsertHug(ctx context.Context, giverID, receiverID uuid.UUID, status, hugType string, comment *string) (*models.Hug, error)
	AcceptHug(ctx context.Context, hugID, receiverID uuid.UUID, streakTier string) (*models.Hug, error)
	DeclineHug(ctx context.Context, hugID, receiverID uuid.UUID) (*models.Hug, error)
	CancelHug(ctx context.Context, hugID, giverID uuid.UUID) (*models.Hug, error)
	GetHugByID(ctx context.Context, hugID uuid.UUID) (*models.Hug, error)
	ListHugsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*models.HugFeedItem, error)
	GetPendingHugsForUser(ctx context.Context, userID uuid.UUID) ([]*models.PendingHugInboxItem, error)
	GetOutgoingPendingHugs(ctx context.Context, userID uuid.UUID) ([]*models.OutgoingPendingHug, error)
	CountPendingHugsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	HasOutgoingPendingHug(ctx context.Context, userID uuid.UUID) (bool, error)
	HasPendingHugForPair(ctx context.Context, giverID, receiverID uuid.UUID) (bool, error)
	CheckSuggestEligibility(ctx context.Context, giverID, receiverID uuid.UUID) (outgoingCount int32, pairPending, reversePending bool, err error)
	GetCooldown(ctx context.Context, userA, userB uuid.UUID) (*models.HugCooldown, error)
	UpsertCooldown(ctx context.Context, userA, userB uuid.UUID, cooldownSeconds int32) (*models.HugCooldown, error)
	ReduceCooldown(ctx context.Context, userA, userB uuid.UUID, reduction int32) (*models.HugCooldown, error)
	SetDeclineCooldown(ctx context.Context, userA, userB uuid.UUID, until time.Time) error
	GetRecentFeed(ctx context.Context, limit, offset int32) ([]*models.HugFeedItem, error)
	GetHugActivity(ctx context.Context) ([]*models.HugActivityItem, error)
	GetLeaderboard(ctx context.Context, limit, offset int32) ([]*models.LeaderboardEntry, error)
	GetUserStats(ctx context.Context, userID uuid.UUID, gender *string) (*models.UserStats, error)
	CountMutualHugs(ctx context.Context, userA, userB uuid.UUID) (*models.MutualHugStats, error)
	SearchUsers(ctx context.Context, query string, viewerID uuid.UUID, limit, offset int32) ([]*models.User, error)
	GetHugDetail(ctx context.Context, hugID uuid.UUID) (*models.HugDetail, error)
	ExpirePendingHugs(ctx context.Context) error
	// Streak methods
	GetPairStreak(ctx context.Context, userA, userB uuid.UUID) (*models.PairStreak, error)
	UpsertPairStreak(ctx context.Context, streak *models.PairStreak) (*models.PairStreak, error)
	GetUserTopStreaks(ctx context.Context, userID uuid.UUID, limit int32) ([]*models.TopStreakEntry, error)
	GetPairStreakCalendar(ctx context.Context, userA, userB uuid.UUID, since time.Time) ([]*models.StreakCalendarDay, error)
}

type balanceRepo interface {
	GetBalance(ctx context.Context, userID uuid.UUID) (*models.Balance, error)
	AddBalance(ctx context.Context, userID uuid.UUID, delta int32) (*models.Balance, error)
	DeductBalance(ctx context.Context, userID uuid.UUID, delta int32) (*models.Balance, error)
	EnsureBalance(ctx context.Context, userID uuid.UUID) error
}

type dailyRewardRepo interface {
	GetDailyReward(ctx context.Context, userID uuid.UUID) (*models.DailyReward, error)
	ClaimDailyReward(ctx context.Context, userID uuid.UUID) (*models.DailyReward, error)
}

type userRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserSlots(ctx context.Context, userID uuid.UUID) (int32, error)
	IncrementUserSlots(ctx context.Context, userID uuid.UUID) (int32, error)
}

type blockRepo interface {
	IsBlockedByEither(ctx context.Context, userA, userB uuid.UUID) (bool, error)
	Block(ctx context.Context, blockerID, blockedID uuid.UUID) error
	Unblock(ctx context.Context, blockerID, blockedID uuid.UUID) error
	GetBlockedUsers(ctx context.Context, userID uuid.UUID) ([]*models.BlockedUser, error)
}

type intimacyRepo interface {
	GetPairIntimacy(ctx context.Context, userA, userB uuid.UUID) (*models.PairIntimacy, error)
	UpsertPairIntimacy(ctx context.Context, userA, userB uuid.UUID) (*models.PairIntimacy, error)
	ApplyDecay(ctx context.Context) error
	GetUserConnections(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*models.ConnectionItem, error)
	GetLeaderboard(ctx context.Context, limit, offset int32) ([]*models.LeaderboardPairEntry, error)
}

type transactor interface {
	RunInTx(ctx context.Context, fn func(context.Context) error) error
}

// Callback types for WebSocket integration
type HugCompletedCallback func(item *models.HugFeedItem, bonusCoins int32, comment *string)
type HugSuggestionCallback func(targetUserID uuid.UUID, item *models.PendingHugInboxItem, comment *string)
type HugDeclinedCallback func(targetUserID uuid.UUID, hugID uuid.UUID, receiverID uuid.UUID)
type HugCancelledCallback func(targetUserID uuid.UUID, hugID uuid.UUID)

type jwtManager interface {
	ParseCaptchaToken(tokenString string) (uuid.UUID, error)
}

type service struct {
	hugRepo      hugRepo
	balanceRepo  balanceRepo
	dailyRepo    dailyRewardRepo
	userRepo     userRepo
	blockRepo    blockRepo
	intimacyRepo intimacyRepo
	jwtManager   jwtManager
	tx           transactor

	onHugCompleted  HugCompletedCallback
	onHugSuggestion HugSuggestionCallback
	onHugDeclined   HugDeclinedCallback
	onHugCancelled  HugCancelledCallback

	// In-memory caches for hot, stale-tolerant data.
	leaderboardCache *cache.TTL[string, []*models.LeaderboardEntry]
	activityCache    *cache.TTL[string, []*models.HugActivityItem]
}

func New(hugRepo hugRepo, balanceRepo balanceRepo, dailyRepo dailyRewardRepo, userRepo userRepo, blockRepo blockRepo, intimacyRepo intimacyRepo, jwtManager jwtManager, tx transactor) *service {
	return &service{
		hugRepo:          hugRepo,
		balanceRepo:      balanceRepo,
		dailyRepo:        dailyRepo,
		userRepo:         userRepo,
		blockRepo:        blockRepo,
		intimacyRepo:     intimacyRepo,
		jwtManager:       jwtManager,
		tx:               tx,
		leaderboardCache: cache.New[string, []*models.LeaderboardEntry](30 * time.Second),
		activityCache:    cache.New[string, []*models.HugActivityItem](2 * time.Minute),
	}
}

func (s *service) SetHugCompletedCallback(cb HugCompletedCallback) {
	s.onHugCompleted = cb
}

func (s *service) SetHugSuggestionCallback(cb HugSuggestionCallback) {
	s.onHugSuggestion = cb
}

func (s *service) SetHugDeclinedCallback(cb HugDeclinedCallback) {
	s.onHugDeclined = cb
}

func (s *service) SetHugCancelledCallback(cb HugCancelledCallback) {
	s.onHugCancelled = cb
}
