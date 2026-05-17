package models

import (
	"time"

	"github.com/google/uuid"
)

// PairStreak represents the streak state for a pair of users.
type PairStreak struct {
	UserAID        uuid.UUID
	UserBID        uuid.UUID
	CurrentStreak  int32
	BestStreak     int32
	LastStreakDate *time.Time // DATE only, stored as time.Time
	AHuggedToday   bool
	BHuggedToday   bool
	TodayDate      time.Time // DATE only
}

// StreakTier defines a streak league with its visual properties.
type StreakTier struct {
	Level   int
	Name    string // Russian name
	Key     string // machine-readable key for frontend color mapping
	MinDays int
}

// StreakTiers ordered from highest to lowest for lookup convenience.
var StreakTiers = []StreakTier{
	{Level: 6, Name: "Легендарная", Key: "legendary", MinDays: 90},
	{Level: 5, Name: "Обсидиановая", Key: "obsidian", MinDays: 60},
	{Level: 4, Name: "Алмазная", Key: "diamond", MinDays: 30},
	{Level: 3, Name: "Сапфировая", Key: "sapphire", MinDays: 21},
	{Level: 2, Name: "Рубиновая", Key: "ruby", MinDays: 14},
	{Level: 1, Name: "Изумрудная", Key: "emerald", MinDays: 7},
	{Level: 0, Name: "", Key: "", MinDays: 0},
}

// ComputeStreakTier returns the tier for a given streak day count.
func ComputeStreakTier(streakDays int32) StreakTier {
	for _, t := range StreakTiers {
		if int(streakDays) >= t.MinDays {
			return t
		}
	}
	return StreakTiers[len(StreakTiers)-1]
}

// StreakInfo is the computed streak data returned to clients.
type StreakInfo struct {
	CurrentStreak int32  `json:"current_streak"`
	BestStreak    int32  `json:"best_streak"`
	TierLevel     int    `json:"tier_level"`
	TierName      string `json:"tier_name"`
	TierKey       string `json:"tier_key"`
	NextTierAt    *int   `json:"next_tier_at,omitempty"`
	AHuggedToday  bool   `json:"a_hugged_today"`
	BHuggedToday  bool   `json:"b_hugged_today"`
}

// ComputeStreakInfo builds the full StreakInfo from streak days.
func ComputeStreakInfo(streak *PairStreak) StreakInfo {
	if streak == nil {
		return StreakInfo{}
	}
	tier := ComputeStreakTier(streak.CurrentStreak)
	info := StreakInfo{
		CurrentStreak: streak.CurrentStreak,
		BestStreak:    streak.BestStreak,
		TierLevel:     tier.Level,
		TierName:      tier.Name,
		TierKey:       tier.Key,
		AHuggedToday:  streak.AHuggedToday,
		BHuggedToday:  streak.BHuggedToday,
	}

	// Find next tier threshold
	for i := len(StreakTiers) - 1; i >= 0; i-- {
		if StreakTiers[i].MinDays > int(streak.CurrentStreak) {
			nextAt := StreakTiers[i].MinDays
			info.NextTierAt = &nextAt
			break
		}
	}

	return info
}

// TopStreakEntry represents a user's streak with another user (for the "top streaks" list).
type TopStreakEntry struct {
	UserID        uuid.UUID
	Username      string
	DisplayName   *string
	Gender        *string
	CurrentStreak int32
	BestStreak    int32
	TierLevel     int
	TierName      string
	TierKey       string
}

// StreakCalendarDay represents a single day's hug activity for a pair.
type StreakCalendarDay struct {
	Date      time.Time // DATE only
	HugCount  int64
	Completed bool // both directions had at least 1 hug
}
