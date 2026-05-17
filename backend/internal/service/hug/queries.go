package hug

import (
	"context"
	"fmt"
	"time"

	"go-service-template/internal/errorz"
	"go-service-template/internal/models"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

func (s *service) GetBalance(ctx context.Context, userID uuid.UUID) (*models.Balance, error) {
	return s.balanceRepo.GetBalance(ctx, userID)
}

func (s *service) GetHugDetail(ctx context.Context, hugID, requesterID uuid.UUID, isAdmin bool) (*models.HugDetail, error) {
	detail, err := s.hugRepo.GetHugDetail(ctx, hugID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, errorz.ErrHugNotFound
	}
	// Only sender, receiver, or admin can see details
	if !isAdmin && detail.GiverID != requesterID && detail.ReceiverID != requesterID {
		return nil, errorz.ErrHugNotFound // treat as not found for privacy
	}
	return detail, nil
}

func (s *service) GetHugHistory(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*models.HugFeedItem, error) {
	return s.hugRepo.ListHugsByUser(ctx, userID, limit, offset)
}

func (s *service) GetRecentFeed(ctx context.Context, limit, offset int32) ([]*models.HugFeedItem, error) {
	return s.hugRepo.GetRecentFeed(ctx, limit, offset)
}

func (s *service) GetHugActivity(ctx context.Context) ([]*models.HugActivityItem, error) {
	const cacheKey = "activity"
	if cached, ok := s.activityCache.Get(cacheKey); ok {
		return cached, nil
	}

	items, err := s.hugRepo.GetHugActivity(ctx)
	if err != nil {
		return nil, err
	}

	s.activityCache.Set(cacheKey, items)
	return items, nil
}

func (s *service) GetLeaderboard(ctx context.Context, limit, offset int32) ([]*models.LeaderboardEntry, error) {
	cacheKey := fmt.Sprintf("%d:%d", limit, offset)
	if cached, ok := s.leaderboardCache.Get(cacheKey); ok {
		return cached, nil
	}

	entries, err := s.hugRepo.GetLeaderboard(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	s.leaderboardCache.Set(cacheKey, entries)
	return entries, nil
}

func (s *service) GetUserStats(ctx context.Context, userID uuid.UUID, gender *string) (*models.UserStats, error) {
	return s.hugRepo.GetUserStats(ctx, userID, gender)
}

func (s *service) GetUserProfile(ctx context.Context, userID uuid.UUID, viewerID *uuid.UUID) (*models.User, *models.UserStats, *models.Balance, *models.MutualHugStats, bool, *models.IntimacyInfo, error) {
	var (
		user      *models.User
		stats     *models.UserStats
		balance   *models.Balance
		mutual    *models.MutualHugStats
		isBlocked bool
		intimacy  *models.IntimacyInfo
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		user, err = s.userRepo.GetByID(gCtx, userID)
		return err
	})

	g.Go(func() error {
		var err error
		// Gender not yet available (user fetched in parallel); pass nil and fix rank after g.Wait().
		stats, err = s.hugRepo.GetUserStats(gCtx, userID, nil)
		return err
	})

	g.Go(func() error {
		var err error
		balance, err = s.balanceRepo.GetBalance(gCtx, userID)
		return err
	})

	if viewerID != nil && *viewerID != userID {
		vid := *viewerID
		g.Go(func() error {
			var err error
			mutual, err = s.hugRepo.CountMutualHugs(gCtx, userID, vid)
			return err
		})
		g.Go(func() error {
			var err error
			isBlocked, err = s.blockRepo.IsBlockedByEither(gCtx, vid, userID)
			return err
		})
		g.Go(func() error {
			pair, err := s.intimacyRepo.GetPairIntimacy(gCtx, vid, userID)
			if err != nil {
				return err
			}
			rawScore := 0
			if pair != nil {
				rawScore = pair.RawScore
			}
			info := models.ComputeIntimacyInfo(rawScore)
			intimacy = &info
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, nil, nil, false, nil, err
	}

	// Recompute rank with the user's gender now that both are available.
	if stats != nil && user != nil {
		stats.Rank = models.GetRank(stats.TotalHugs, user.Gender)
	}

	return user, stats, balance, mutual, isBlocked, intimacy, nil
}

func (s *service) SearchUsers(ctx context.Context, query string, viewerID uuid.UUID, limit, offset int32) ([]*models.User, error) {
	return s.hugRepo.SearchUsers(ctx, query, viewerID, limit, offset)
}

func (s *service) BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if blockerID == blockedID {
		return errorz.ErrCannotBlockSelf
	}
	// Verify target exists
	_, err := s.userRepo.GetByID(ctx, blockedID)
	if err != nil {
		return err
	}
	return s.blockRepo.Block(ctx, blockerID, blockedID)
}

func (s *service) UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	return s.blockRepo.Unblock(ctx, blockerID, blockedID)
}

func (s *service) GetBlockedUsers(ctx context.Context, userID uuid.UUID) ([]*models.BlockedUser, error) {
	return s.blockRepo.GetBlockedUsers(ctx, userID)
}

func (s *service) ExpirePendingHugs(ctx context.Context) error {
	return s.hugRepo.ExpirePendingHugs(ctx)
}

func (s *service) GetPendingInbox(ctx context.Context, userID uuid.UUID) ([]*models.PendingHugInboxItem, error) {
	return s.hugRepo.GetPendingHugsForUser(ctx, userID)
}

func (s *service) GetOutgoingHugs(ctx context.Context, userID uuid.UUID) ([]*models.OutgoingPendingHug, *models.SlotInfo, error) {
	hugs, err := s.hugRepo.GetOutgoingPendingHugs(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	slots, err := s.userRepo.GetUserSlots(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	slotInfo := &models.SlotInfo{
		TotalSlots: slots,
		UsedSlots:  int32(len(hugs)),
	}
	if slots < models.MaxHugSlots {
		cost := models.SlotCost(slots + 1)
		slotInfo.NextSlotCost = &cost
	}

	return hugs, slotInfo, nil
}

func (s *service) BuyHugSlot(ctx context.Context, userID uuid.UUID) (*models.SlotInfo, int32, error) {
	var (
		slotInfo   *models.SlotInfo
		newBalance int32
	)

	err := s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		// Get current slot count
		currentSlots, err := s.userRepo.GetUserSlots(txCtx, userID)
		if err != nil {
			return err
		}
		if currentSlots >= models.MaxHugSlots {
			return errorz.ErrMaxSlotsReached
		}

		// Calculate cost for the next slot
		cost := models.SlotCost(currentSlots + 1)

		// Deduct balance
		bal, err := s.balanceRepo.DeductBalance(txCtx, userID, cost)
		if err != nil {
			return err
		}
		if bal == nil {
			return errorz.ErrInsufficientBalance
		}
		newBalance = bal.Amount

		// Increment slots
		newSlots, err := s.userRepo.IncrementUserSlots(txCtx, userID)
		if err != nil {
			return err
		}
		if newSlots == 0 {
			// Shouldn't happen since we checked above, but safety
			return errorz.ErrMaxSlotsReached
		}

		// Count current outgoing hugs for the response
		outgoing, err := s.hugRepo.GetOutgoingPendingHugs(txCtx, userID)
		if err != nil {
			return err
		}

		slotInfo = &models.SlotInfo{
			TotalSlots: newSlots,
			UsedSlots:  int32(len(outgoing)),
		}
		if newSlots < models.MaxHugSlots {
			nextCost := models.SlotCost(newSlots + 1)
			slotInfo.NextSlotCost = &nextCost
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return slotInfo, newBalance, nil
}

func (s *service) GetInboxCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.hugRepo.CountPendingHugsForUser(ctx, userID)
}

func (s *service) ClaimDailyReward(ctx context.Context, userID uuid.UUID) (int32, int32, int32, bool, error) {
	var (
		amount         int32
		streakDays     int32
		balAmount      int32
		alreadyClaimed bool
	)

	// Wrap check + claim + balance update in a transaction to prevent double-claiming.
	err := s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		// Check if already claimed today
		existing, err := s.dailyRepo.GetDailyReward(txCtx, userID)
		if err != nil {
			return err
		}

		if existing != nil && existing.LastClaimedAt.UTC().Format("2006-01-02") == models.Today() {
			// Already claimed today
			bal, err := s.balanceRepo.GetBalance(txCtx, userID)
			if err != nil {
				return err
			}
			alreadyClaimed = true
			streakDays = existing.StreakDays
			balAmount = bal.Amount
			return nil
		}

		// Claim
		reward, err := s.dailyRepo.ClaimDailyReward(txCtx, userID)
		if err != nil {
			return err
		}

		// Calculate reward amount: base 5 + min(streak-1, 5)
		bonus := reward.StreakDays - 1
		if bonus > 5 {
			bonus = 5
		}
		amount = int32(5) + bonus

		bal, err := s.balanceRepo.AddBalance(txCtx, userID, amount)
		if err != nil {
			return err
		}

		streakDays = reward.StreakDays
		balAmount = bal.Amount
		return nil
	})
	if err != nil {
		return 0, 0, 0, false, err
	}

	return amount, streakDays, balAmount, alreadyClaimed, nil
}

// GetDailyRewardStatus returns the current daily-reward status for the user.
// canClaim is true when the reward has not been claimed today (UTC).
// nextClaimAt is the next UTC midnight after the last claim (or "now" if can claim).
// streakDays is the user's current streak; lastClaimedAt is nil if never claimed.
func (s *service) GetDailyRewardStatus(ctx context.Context, userID uuid.UUID) (bool, time.Time, int32, *time.Time, error) {
	existing, err := s.dailyRepo.GetDailyReward(ctx, userID)
	if err != nil {
		return false, time.Time{}, 0, nil, err
	}

	now := time.Now().UTC()

	if existing == nil {
		// Never claimed — can claim now; streak resets to 1 on claim.
		return true, now, 0, nil, nil
	}

	lastClaimed := existing.LastClaimedAt.UTC()
	lastClaimedDay := lastClaimed.Format("2006-01-02")
	canClaim := lastClaimedDay != models.Today()

	// Next claim is the next UTC midnight after the last claim.
	nextClaimAt := time.Date(lastClaimed.Year(), lastClaimed.Month(), lastClaimed.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	if canClaim {
		nextClaimAt = now
	}

	return canClaim, nextClaimAt, existing.StreakDays, &lastClaimed, nil
}

// GetPairIntimacy returns the intimacy info for a user pair.
func (s *service) GetPairIntimacy(ctx context.Context, userA, userB uuid.UUID) (*models.IntimacyInfo, error) {
	pair, err := s.intimacyRepo.GetPairIntimacy(ctx, userA, userB)
	if err != nil {
		return nil, err
	}

	rawScore := 0
	if pair != nil {
		rawScore = pair.RawScore
	}

	info := models.ComputeIntimacyInfo(rawScore)
	return &info, nil
}

// GetUserConnections returns a user's connections sorted by intimacy.
func (s *service) GetUserConnections(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*models.ConnectionItem, error) {
	return s.intimacyRepo.GetUserConnections(ctx, userID, limit, offset)
}

// GetIntimacyLeaderboard returns the top pairs by intimacy score.
func (s *service) GetIntimacyLeaderboard(ctx context.Context, limit, offset int32) ([]*models.LeaderboardPairEntry, error) {
	return s.intimacyRepo.GetLeaderboard(ctx, limit, offset)
}

// ApplyIntimacyDecay runs the decay job for stale pairs.
func (s *service) ApplyIntimacyDecay(ctx context.Context) error {
	return s.intimacyRepo.ApplyDecay(ctx)
}

// GetPairStreak returns the streak info for a pair of users.
func (s *service) GetPairStreak(ctx context.Context, userA, userB uuid.UUID) (*models.StreakInfo, error) {
	streak, err := s.hugRepo.GetPairStreak(ctx, userA, userB)
	if err != nil {
		return nil, err
	}

	info := models.ComputeStreakInfo(streak)
	return &info, nil
}

// GetUserTopStreaks returns the user's top active pair streaks.
func (s *service) GetUserTopStreaks(ctx context.Context, userID uuid.UUID, limit int32) ([]*models.TopStreakEntry, error) {
	return s.hugRepo.GetUserTopStreaks(ctx, userID, limit)
}

// GetPairStreakCalendar returns the hug calendar for a pair of users.
func (s *service) GetPairStreakCalendar(ctx context.Context, userA, userB uuid.UUID) ([]*models.StreakCalendarDay, error) {
	// Get last 90 days of data for the calendar
	since := time.Now().UTC().AddDate(0, -3, 0)
	return s.hugRepo.GetPairStreakCalendar(ctx, userA, userB, since)
}
