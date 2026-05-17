package errorz

import "errors"

var (
	ErrHugCooldownActive         = errors.New("hug cooldown is still active")
	ErrCannotHugSelf             = errors.New("cannot hug yourself")
	ErrInsufficientBalance       = errors.New("insufficient balance")
	ErrDailyRewardAlreadyClaimed = errors.New("daily reward already claimed today")
	ErrCooldownNotFound          = errors.New("cooldown not found for this pair")
	ErrAlreadyHasPendingHug      = errors.New("already has a pending hug")
	ErrPendingHugExists          = errors.New("pending hug already exists for this pair")
	ErrReversePendingHugExists   = errors.New("user has already suggested a hug to you")
	ErrHugNotFound               = errors.New("hug not found")
	ErrHugNotPending             = errors.New("hug is not in pending state")
	ErrHugExpired                = errors.New("hug suggestion has expired")
	ErrDeclineCooldownActive     = errors.New("decline cooldown is active")
	ErrMaxSlotsReached           = errors.New("maximum hug slots reached")
	ErrHugTypeLocked             = errors.New("hug type not unlocked for this pair")
	ErrCaptchaRequired           = errors.New("captcha required")
	ErrCaptchaFailed             = errors.New("captcha failed")
)
