package hug

import (
	"context"
	"time"

	"go-service-template/internal/errorz"
	"go-service-template/internal/models"

	"github.com/google/uuid"
)

const (
	defaultCooldownSeconds      = 3600 // 1 hour
	cooldownReductionPerUpgrade = 600  // 10 minutes
	upgradeCost                 = 5    // balance cost per upgrade
	minCooldownSeconds          = 300  // 5 minutes minimum
	declineCooldownSeconds      = 300  // 5 minutes
)

// SuggestHug creates a pending hug suggestion (replaces old SendHug).
// Returns the created hug and the receiver's user data (for the response).
func (s *service) SuggestHug(ctx context.Context, giverID, receiverID uuid.UUID, hugType string, comment *string, captchaToken *string) (*models.Hug, *models.User, error) {
	if giverID == receiverID {
		return nil, nil, errorz.ErrCannotHugSelf
	}

	// Default to standard if empty
	if hugType == "" {
		hugType = models.HugTypeStandard
	}
	if !models.ValidHugType(hugType) {
		return nil, nil, errorz.ErrHugTypeLocked
	}

	// Check if either user has blocked the other
	blocked, err := s.blockRepo.IsBlockedByEither(ctx, giverID, receiverID)
	if err != nil {
		return nil, nil, err
	}
	if blocked {
		return nil, nil, errorz.ErrUserBlocked
	}

	// Verify receiver exists (can be done outside tx — user won't disappear)
	receiver, err := s.userRepo.GetByID(ctx, receiverID)
	if err != nil {
		return nil, nil, err
	}

	giver, err := s.userRepo.GetByID(ctx, giverID)
	if err != nil {
		return nil, nil, err
	}

	if giver.CaptchaType != "none" {
		if captchaToken == nil || *captchaToken == "" {
			return nil, nil, errorz.ErrCaptchaRequired
		}

		tokenUserID, err := s.jwtManager.ParseCaptchaToken(*captchaToken)
		if err != nil || tokenUserID != giverID {
			return nil, nil, errorz.ErrCaptchaFailed
		}
	}

	// Check intimacy-gated hug type
	if hugType != models.HugTypeStandard {
		intimacy, err := s.intimacyRepo.GetPairIntimacy(ctx, giverID, receiverID)
		if err != nil {
			return nil, nil, err
		}
		rawScore := 0
		if intimacy != nil {
			rawScore = intimacy.RawScore
		}
		if !models.IsHugTypeUnlocked(rawScore, hugType) {
			return nil, nil, errorz.ErrHugTypeLocked
		}
	}

	var h *models.Hug

	// Wrap all checks + insert in a transaction to prevent TOCTOU races
	// (e.g., two concurrent requests both passing the pending check before either inserts).
	err = s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		// Combined eligibility check — count + 2 EXISTS in a single DB round-trip.
		outgoingCount, pairPending, reversePending, err := s.hugRepo.CheckSuggestEligibility(txCtx, giverID, receiverID)
		if err != nil {
			return err
		}

		// Check slot capacity
		slots, err := s.userRepo.GetUserSlots(txCtx, giverID)
		if err != nil {
			return err
		}
		if outgoingCount >= slots {
			return errorz.ErrAlreadyHasPendingHug
		}
		if pairPending {
			return errorz.ErrPendingHugExists
		}
		if reversePending {
			return errorz.ErrReversePendingHugExists
		}

		// Check shared cooldown (with intimacy-based reduction)
		cooldown, err := s.hugRepo.GetCooldown(txCtx, giverID, receiverID)
		if err != nil {
			return err
		}

		if cooldown != nil {
			// Check decline cooldown first
			if cooldown.DeclineCooldownUntil != nil && cooldown.DeclineCooldownUntil.After(time.Now()) {
				return errorz.ErrDeclineCooldownActive
			}

			// Apply intimacy-based cooldown reduction (no floor — intimacy can push below 5 min)
			effectiveCooldownSeconds := cooldown.CooldownSeconds
			intimacy, _ := s.intimacyRepo.GetPairIntimacy(txCtx, giverID, receiverID)
			if intimacy != nil {
				tier := models.ComputeTier(intimacy.RawScore)
				reduction := float64(effectiveCooldownSeconds) * tier.CooldownReduction
				effectiveCooldownSeconds -= int32(reduction)
				if effectiveCooldownSeconds < 0 {
					effectiveCooldownSeconds = 0
				}
			}

			// Check regular cooldown with effective seconds
			elapsed := time.Since(cooldown.LastHugAt)
			if elapsed < time.Duration(effectiveCooldownSeconds)*time.Second {
				return errorz.ErrHugCooldownActive
			}
		}

		// Insert the pending hug
		h, err = s.hugRepo.InsertHug(txCtx, giverID, receiverID, models.HugStatusPending, hugType, comment)
		return err
	})
	if err != nil {
		return nil, nil, err
	}

	// Fire WebSocket notification asynchronously to avoid blocking the HTTP response.
	if s.onHugSuggestion != nil {
		hugCopy := *h
		go func() {
			giver, _ := s.userRepo.GetByID(context.WithoutCancel(ctx), giverID)
			giverUsername := ""
			var giverGender *string
			if giver != nil {
				giverUsername = giver.Username
				giverGender = giver.Gender
			}
			s.onHugSuggestion(receiverID, &models.PendingHugInboxItem{
				ID:            hugCopy.ID,
				GiverID:       hugCopy.GiverID,
				ReceiverID:    hugCopy.ReceiverID,
				GiverUsername: giverUsername,
				GiverGender:   giverGender,
				HugType:       hugCopy.HugType,
				Comment:       hugCopy.Comment,
				CreatedAt:     hugCopy.CreatedAt,
			}, hugCopy.Comment)
		}()
	}

	return h, receiver, nil
}

// AcceptHug accepts a pending hug suggestion.
func (s *service) AcceptHug(ctx context.Context, hugID, receiverID uuid.UUID) (*models.Hug, error) {
	var acceptedHug *models.Hug
	var earnedBonusCoins int32

	err := s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		// First, look up the hug to get giver/receiver IDs for streak computation
		existing, err := s.hugRepo.GetHugByID(txCtx, hugID)
		if err != nil {
			return err
		}
		if existing == nil {
			return errorz.ErrHugNotFound
		}
		if existing.Status != models.HugStatusPending {
			return errorz.ErrHugNotPending
		}

		// Compute streak tier before accepting (so we can stamp it)
		streakTier := s.computeAndUpdateStreak(txCtx, existing.GiverID, existing.ReceiverID)

		h, err := s.hugRepo.AcceptHug(txCtx, hugID, receiverID, streakTier)
		if err != nil {
			return err
		}
		if h == nil {
			// It was pending but the 24h window passed
			return errorz.ErrHugExpired
		}

		// Increment pair intimacy (before computing bonus coins)
		intimacy, err := s.intimacyRepo.UpsertPairIntimacy(txCtx, h.GiverID, h.ReceiverID)
		if err != nil {
			return err
		}

		// Compute bonus coins from intimacy tier
		bonusCoins := int32(0)
		if intimacy != nil {
			tier := models.ComputeTier(intimacy.RawScore)
			bonusCoins = int32(tier.BonusCoins)
		}
		earnedBonusCoins = bonusCoins

		// +1 base coin + bonus to initiator (giver).
		// If the giver attached a comment, the comment costs the same as the earned amount,
		// so the giver nets 0 coins.
		if h.Comment == nil {
			_, err = s.balanceRepo.AddBalance(txCtx, h.GiverID, 1+bonusCoins)
			if err != nil {
				return err
			}
		}
		// Giver with comment gets nothing — cost equals earnings.

		// +1 base coin + bonus to acceptor (receiver) — always full amount
		_, err = s.balanceRepo.AddBalance(txCtx, h.ReceiverID, 1+bonusCoins)
		if err != nil {
			return err
		}

		// Start/refresh shared cooldown. UpsertCooldown's ON CONFLICT only updates
		// last_hug_at and preserves the existing cooldown_seconds, so the default
		// value is only used for the initial INSERT.
		_, err = s.hugRepo.UpsertCooldown(txCtx, h.GiverID, h.ReceiverID, int32(defaultCooldownSeconds))
		if err != nil {
			return err
		}

		acceptedHug = h
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Invalidate leaderboard cache since a hug was just completed.
	s.leaderboardCache.InvalidateAll()

	// Fire WebSocket broadcast asynchronously to avoid blocking the HTTP response.
	if s.onHugCompleted != nil && acceptedHug != nil {
		hugCopy := *acceptedHug
		bonus := earnedBonusCoins
		go func() {
			bgCtx := context.WithoutCancel(ctx)
			giver, _ := s.userRepo.GetByID(bgCtx, hugCopy.GiverID)
			receiver, _ := s.userRepo.GetByID(bgCtx, hugCopy.ReceiverID)
			giverName := ""
			receiverName := ""
			var giverGender *string
			if giver != nil {
				giverName = giver.Username
				giverGender = giver.Gender
			}
			if receiver != nil {
				receiverName = receiver.Username
			}
			completedAt := hugCopy.CreatedAt
			if hugCopy.AcceptedAt != nil {
				completedAt = *hugCopy.AcceptedAt
			}
			s.onHugCompleted(&models.HugFeedItem{
				ID:               hugCopy.ID,
				GiverID:          hugCopy.GiverID,
				ReceiverID:       hugCopy.ReceiverID,
				GiverUsername:    giverName,
				ReceiverUsername: receiverName,
				GiverGender:      giverGender,
				HugType:          hugCopy.HugType,
				HasComment:       hugCopy.Comment != nil,
				StreakTier:       hugCopy.StreakTier,
				CreatedAt:        completedAt,
			}, bonus, hugCopy.Comment)
		}()
	}

	return acceptedHug, nil
}

// computeAndUpdateStreak handles the streak logic when a hug is accepted.
// It updates the pair streak state and returns the current streak tier key to stamp on the hug.
func (s *service) computeAndUpdateStreak(ctx context.Context, giverID, receiverID uuid.UUID) string {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	streak, err := s.hugRepo.GetPairStreak(ctx, giverID, receiverID)
	if err != nil {
		// Non-critical: if streak lookup fails, just don't stamp a tier
		return ""
	}

	// Determine canonical ordering: user_a_id < user_b_id
	var userAID, userBID uuid.UUID
	if giverID.String() < receiverID.String() {
		userAID = giverID
		userBID = receiverID
	} else {
		userAID = receiverID
		userBID = giverID
	}

	// Is the giver user_a or user_b?
	giverIsA := giverID == userAID

	if streak == nil {
		// First hug for this pair — create initial streak record
		streak = &models.PairStreak{
			UserAID:       userAID,
			UserBID:       userBID,
			CurrentStreak: 0,
			BestStreak:    0,
			AHuggedToday:  false,
			BHuggedToday:  false,
			TodayDate:     today,
		}
	}

	// Day transition: if today_date in record is not today, evaluate and reset daily flags
	if !streak.TodayDate.Equal(today) {
		yesterday := today.AddDate(0, 0, -1)

		// Check if the previous day was completed (both sides hugged)
		prevDayCompleted := streak.AHuggedToday && streak.BHuggedToday

		if prevDayCompleted && streak.TodayDate.Equal(yesterday) {
			// Previous day was completed and it was yesterday — streak already incremented on that day
			// Just reset daily flags for today
		} else if streak.LastStreakDate != nil && streak.LastStreakDate.Equal(yesterday) {
			// Last streak date was yesterday but today's flags weren't both set —
			// the streak is still valid (grace: streak only breaks if a FULL day passes without completion)
		} else if streak.LastStreakDate != nil && streak.LastStreakDate.Equal(today) {
			// Already completed today somehow (shouldn't happen but handle gracefully)
		} else {
			// Streak is broken — more than 1 day gap since last completion
			streak.CurrentStreak = 0
		}

		// Reset daily flags for the new day
		streak.AHuggedToday = false
		streak.BHuggedToday = false
		streak.TodayDate = today
	}

	// Mark the giver's side as hugged today
	if giverIsA {
		streak.AHuggedToday = true
	} else {
		streak.BHuggedToday = true
	}

	// Check if both sides have now hugged today
	if streak.AHuggedToday && streak.BHuggedToday {
		// Check if we already incremented for today
		alreadyCountedToday := streak.LastStreakDate != nil && streak.LastStreakDate.Equal(today)
		if !alreadyCountedToday {
			streak.CurrentStreak++
			streak.LastStreakDate = &today
			if streak.CurrentStreak > streak.BestStreak {
				streak.BestStreak = streak.CurrentStreak
			}
		}
	}

	// Persist updated streak
	_, err = s.hugRepo.UpsertPairStreak(ctx, streak)
	if err != nil {
		// Non-critical failure — log but don't fail the hug
		return ""
	}

	// Return the current tier key
	tier := models.ComputeStreakTier(streak.CurrentStreak)
	return tier.Key
}

// DeclineHug declines a pending hug suggestion.
func (s *service) DeclineHug(ctx context.Context, hugID, receiverID uuid.UUID) error {
	var h *models.Hug

	// Wrap decline + cooldown set in a transaction so the cooldown is always applied
	// when the hug is declined (prevents giver from immediately re-sending).
	err := s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		h, err = s.hugRepo.DeclineHug(txCtx, hugID, receiverID)
		if err != nil {
			return err
		}
		if h == nil {
			// Check why
			existing, lookupErr := s.hugRepo.GetHugByID(txCtx, hugID)
			if lookupErr != nil {
				return lookupErr
			}
			if existing == nil {
				return errorz.ErrHugNotFound
			}
			if existing.Status != models.HugStatusPending {
				return errorz.ErrHugNotPending
			}
			return errorz.ErrHugExpired
		}

		// Set 5-minute decline cooldown on the pair
		declineUntil := time.Now().Add(time.Duration(declineCooldownSeconds) * time.Second)
		return s.hugRepo.SetDeclineCooldown(txCtx, h.GiverID, h.ReceiverID, declineUntil)
	})
	if err != nil {
		return err
	}

	// Fire WebSocket hug_declined to giver (outside tx — fire-and-forget)
	if s.onHugDeclined != nil && h != nil {
		s.onHugDeclined(h.GiverID, hugID, h.ReceiverID)
	}

	return nil
}

// CancelHug cancels the giver's own outgoing pending hug.
func (s *service) CancelHug(ctx context.Context, hugID, giverID uuid.UUID) error {
	h, err := s.hugRepo.CancelHug(ctx, hugID, giverID)
	if err != nil {
		return err
	}
	if h == nil {
		existing, lookupErr := s.hugRepo.GetHugByID(ctx, hugID)
		if lookupErr != nil {
			return lookupErr
		}
		if existing == nil {
			return errorz.ErrHugNotFound
		}
		if existing.Status != models.HugStatusPending {
			return errorz.ErrHugNotPending
		}
		return errorz.ErrHugExpired
	}

	// Fire WebSocket hug_cancelled to receiver
	if s.onHugCancelled != nil {
		s.onHugCancelled(h.ReceiverID, hugID)
	}

	return nil
}

// CooldownInfoResult bundles cooldown data with intimacy reduction info.
type CooldownInfoResult struct {
	Cooldown             *models.HugCooldown
	RemainingSeconds     int32
	CanHug               bool
	DeclineRemaining     int32
	EffectiveCooldown    int32
	IntimacyReductionPct int
}

// GetCooldownInfo returns cooldown details for a pair of users, including intimacy reduction.
func (s *service) GetCooldownInfo(ctx context.Context, userA, userB uuid.UUID) (*CooldownInfoResult, error) {
	cooldown, err := s.hugRepo.GetCooldown(ctx, userA, userB)
	if err != nil {
		return nil, err
	}

	// Get intimacy for reduction computation
	reductionPct := 0
	intimacy, _ := s.intimacyRepo.GetPairIntimacy(ctx, userA, userB)
	if intimacy != nil {
		tier := models.ComputeTier(intimacy.RawScore)
		reductionPct = int(tier.CooldownReduction * 100)
	}

	if cooldown == nil {
		baseCooldown := int32(defaultCooldownSeconds)
		effectiveCooldown := baseCooldown - int32(float64(baseCooldown)*float64(reductionPct)/100.0)
		if effectiveCooldown < 0 {
			effectiveCooldown = 0
		}
		return &CooldownInfoResult{
			Cooldown: &models.HugCooldown{
				UserAID:         userA,
				UserBID:         userB,
				CooldownSeconds: baseCooldown,
			},
			CanHug:               true,
			EffectiveCooldown:    effectiveCooldown,
			IntimacyReductionPct: reductionPct,
		}, nil
	}

	// Compute effective cooldown with intimacy reduction (no floor — intimacy can push below 5 min)
	effectiveCooldown := cooldown.CooldownSeconds
	reduction := float64(effectiveCooldown) * float64(reductionPct) / 100.0
	effectiveCooldown -= int32(reduction)
	if effectiveCooldown < 0 {
		effectiveCooldown = 0
	}

	elapsed := time.Since(cooldown.LastHugAt)
	remaining := time.Duration(effectiveCooldown)*time.Second - elapsed
	if remaining < 0 {
		remaining = 0
	}
	canHug := remaining <= 0

	var declineRemaining int32
	if cooldown.DeclineCooldownUntil != nil {
		dr := time.Until(*cooldown.DeclineCooldownUntil)
		if dr > 0 {
			declineRemaining = int32(dr.Seconds())
			canHug = false
		}
	}

	return &CooldownInfoResult{
		Cooldown:             cooldown,
		RemainingSeconds:     int32(remaining.Seconds()),
		CanHug:               canHug,
		DeclineRemaining:     declineRemaining,
		EffectiveCooldown:    effectiveCooldown,
		IntimacyReductionPct: reductionPct,
	}, nil
}

// UpgradeCooldown allows either user in a pair to pay to reduce the shared cooldown.
func (s *service) UpgradeCooldown(ctx context.Context, payerID, otherUserID uuid.UUID) (*models.HugCooldown, error) {
	var reduced *models.HugCooldown

	// Wrap deduct + cooldown reduction in a transaction so balance is rolled back
	// if the cooldown reduction fails.
	err := s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		// Deduct balance
		b, err := s.balanceRepo.DeductBalance(txCtx, payerID, int32(upgradeCost))
		if err != nil {
			return err
		}
		if b == nil {
			return errorz.ErrInsufficientBalance
		}

		// Ensure cooldown row exists
		cooldown, err := s.hugRepo.GetCooldown(txCtx, payerID, otherUserID)
		if err != nil {
			return err
		}
		if cooldown == nil {
			// Create one with default then reduce
			_, err = s.hugRepo.UpsertCooldown(txCtx, payerID, otherUserID, defaultCooldownSeconds)
			if err != nil {
				return err
			}
		}

		reduced, err = s.hugRepo.ReduceCooldown(txCtx, payerID, otherUserID, cooldownReductionPerUpgrade)
		if err != nil {
			return err
		}
		if reduced == nil {
			return errorz.ErrCooldownNotFound
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return reduced, nil
}
