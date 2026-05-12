package matrix

import (
	"context"
	"fmt"
	"log/slog"

	"go-service-template/internal/models"

	"github.com/google/uuid"
)

// userRepo is the interface for looking up user data needed by the notifier.
type userRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// Notifier sends Matrix notifications for hug events. Methods are fire-and-forget:
// errors are logged but never propagated. Disabled when the bot is disabled.
type Notifier struct {
	bot    *Bot
	repo   userRepo
	logger *slog.Logger
}

// NewNotifier creates a new Notifier. Pass a non-nil bot to enable.
func NewNotifier(bot *Bot, repo userRepo, logger *slog.Logger) *Notifier {
	return &Notifier{
		bot:    bot,
		repo:   repo,
		logger: logger,
	}
}

// Enabled reports whether Matrix notifications can be delivered.
func (n *Notifier) Enabled() bool {
	return n.bot != nil && n.bot.Enabled()
}

// NotifyHugSuggestion delivers a hug suggestion (with reactions for accept/decline).
func (n *Notifier) NotifyHugSuggestion(ctx context.Context, receiverID uuid.UUID, hugID uuid.UUID, giverID uuid.UUID, hugType string, comment *string) {
	if !n.Enabled() {
		return
	}
	giver, err := n.repo.GetByID(ctx, giverID)
	if err != nil {
		n.logger.Error("matrix: failed to look up giver", "giver_id", giverID, "error", err)
		return
	}
	name := displayName(giver)
	phrase := hugTypeSuggestionPhrase(hugType)
	n.bot.SendHugSuggestion(ctx, receiverID, hugID, name, phrase, comment)
}

// NotifyHugCompleted notifies both giver and receiver that the hug went through.
func (n *Notifier) NotifyHugCompleted(ctx context.Context, giverID, receiverID uuid.UUID, hugType string, bonusCoins int32, comment *string) {
	if !n.Enabled() {
		return
	}
	giver, err := n.repo.GetByID(ctx, giverID)
	if err != nil {
		n.logger.Error("matrix: failed to look up giver", "giver_id", giverID, "error", err)
		return
	}
	receiver, err := n.repo.GetByID(ctx, receiverID)
	if err != nil {
		n.logger.Error("matrix: failed to look up receiver", "receiver_id", receiverID, "error", err)
		return
	}

	totalCoins := 1 + bonusCoins
	coinText := fmt.Sprintf("+%d", totalCoins)
	if bonusCoins > 0 {
		coinText = fmt.Sprintf("+%d (бонус +%d)", totalCoins, bonusCoins)
	}
	hugWord := hugTypeCompletedNoun(hugType)

	giverCoinText := coinText
	if comment != nil {
		giverCoinText = "0 (оплата комментария)"
	}

	receiverVerb := genderVerb(receiver.Gender, "принял", "приняла", "принял(а)")
	giverMsg := fmt.Sprintf("🎉 %s %s %s! %s монет", displayName(receiver), receiverVerb, hugWord, giverCoinText)
	receiverMsg := fmt.Sprintf("🎉 Вы обнялись с %s! %s монет", displayName(giver), coinText)

	if comment != nil && *comment != "" {
		receiverMsg += "\n\n💬 " + *comment
	}

	n.bot.SendPlainToUser(ctx, giverID, giverMsg)
	n.bot.SendPlainToUser(ctx, receiverID, receiverMsg)
}

// NotifyHugDeclined notifies the giver that their hug was declined.
func (n *Notifier) NotifyHugDeclined(ctx context.Context, giverID, receiverID uuid.UUID) {
	if !n.Enabled() {
		return
	}
	receiver, err := n.repo.GetByID(ctx, receiverID)
	if err != nil {
		n.logger.Error("matrix: failed to look up receiver", "receiver_id", receiverID, "error", err)
		return
	}
	verb := genderVerb(receiver.Gender, "отклонил", "отклонила", "отклонил(а)")
	n.bot.SendPlainToUser(ctx, giverID, fmt.Sprintf("😔 %s %s объятие", displayName(receiver), verb))
}

// NotifyHugCancelled notifies the receiver that the giver took back the hug request.
func (n *Notifier) NotifyHugCancelled(ctx context.Context, receiverID, giverID uuid.UUID) {
	if !n.Enabled() {
		return
	}
	giver, err := n.repo.GetByID(ctx, giverID)
	if err != nil {
		n.logger.Error("matrix: failed to look up giver", "giver_id", giverID, "error", err)
		return
	}
	verb := genderVerb(giver.Gender, "отменил", "отменила", "отменил(а)")
	n.bot.SendPlainToUser(ctx, receiverID, fmt.Sprintf("❌ %s %s запрос на объятие", displayName(giver), verb))
}

// ── helpers (kept here so the notifier doesn't pull telegram package) ──

func displayName(u *models.User) string {
	if u.DisplayName != nil && *u.DisplayName != "" {
		return *u.DisplayName
	}
	return u.Username
}

func hugTypeSuggestionPhrase(hugType string) string {
	switch hugType {
	case "bear":
		return "хочет обнять тебя по-медвежьи"
	case "group":
		return "хочет обнять тебя вместе со всеми"
	case "warm":
		return "хочет тепло тебя обнять"
	case "soul":
		return "хочет обнять тебя по-душевному"
	default:
		return "хочет тебя обнять"
	}
}

func hugTypeCompletedNoun(hugType string) string {
	switch hugType {
	case "bear":
		return "медвежьи обнимашки"
	case "group":
		return "групповые обнимашки"
	case "warm":
		return "тёплые обнимашки"
	case "soul":
		return "душевные обнимашки"
	default:
		return "обнимашки"
	}
}

func genderVerb(gender *string, male, female, fallback string) string {
	if gender == nil {
		return fallback
	}
	switch *gender {
	case "male":
		return male
	case "female":
		return female
	default:
		return fallback
	}
}
