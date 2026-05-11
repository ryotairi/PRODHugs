package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"go-service-template/internal/models"

	"github.com/google/uuid"
)

// userRepo is the interface for looking up user data needed by the notifier.
type userRepo interface {
	GetTelegramID(ctx context.Context, userID uuid.UUID) (*int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// Notifier sends Telegram notifications for hug events.
// All methods are fire-and-forget: errors are logged but never propagated.
type Notifier struct {
	client *Client
	bot    *Bot
	repo   userRepo
	logger *slog.Logger
}

// NewNotifier creates a new Notifier. If client is nil or disabled, notifications are skipped.
// The bot parameter is used for sending hug suggestions with inline buttons;
// pass nil to send plain-text suggestions instead.
func NewNotifier(client *Client, bot *Bot, repo userRepo, logger *slog.Logger) *Notifier {
	return &Notifier{
		client: client,
		bot:    bot,
		repo:   repo,
		logger: logger,
	}
}

// Enabled returns true if Telegram notifications are configured.
func (n *Notifier) Enabled() bool {
	return n.client != nil && n.client.Enabled()
}

// NotifyHugSuggestion notifies the receiver with Accept/Decline buttons.
func (n *Notifier) NotifyHugSuggestion(ctx context.Context, receiverID uuid.UUID, hugID uuid.UUID, giverID uuid.UUID, hugType string, comment *string) {
	giver, err := n.repo.GetByID(ctx, giverID)
	if err != nil {
		n.logger.Error("telegram: failed to look up giver", "giver_id", giverID, "error", err)
		return
	}
	name := displayName(giver)
	phrase := hugTypeSuggestionPhrase(hugType)

	if n.bot != nil {
		n.bot.SendHugSuggestion(ctx, receiverID, hugID, name, phrase, comment)
		return
	}
	// Fallback: plain text without buttons
	text := fmt.Sprintf("🤗 <b>%s</b> %s!", name, phrase)
	if comment != nil && *comment != "" {
		text += fmt.Sprintf("\n\n💬 <i>%s</i>", *comment)
	}
	n.sendToUser(ctx, receiverID, text)
}

// NotifyHugCompleted notifies both participants that the hug was accepted.
func (n *Notifier) NotifyHugCompleted(ctx context.Context, giverID, receiverID uuid.UUID, hugType string, bonusCoins int32, comment *string) {
	giver, err := n.repo.GetByID(ctx, giverID)
	if err != nil {
		n.logger.Error("telegram: failed to look up giver", "giver_id", giverID, "error", err)
		return
	}
	receiver, err := n.repo.GetByID(ctx, receiverID)
	if err != nil {
		n.logger.Error("telegram: failed to look up receiver", "receiver_id", receiverID, "error", err)
		return
	}

	totalCoins := 1 + bonusCoins
	coinText := fmt.Sprintf("+%d", totalCoins)
	if bonusCoins > 0 {
		coinText = fmt.Sprintf("+%d (бонус +%d)", totalCoins, bonusCoins)
	}

	hugWord := hugTypeCompletedNoun(hugType)

	// Message to giver: if comment was attached, they pay for it (net 0 coins)
	giverCoinText := coinText
	if comment != nil {
		giverCoinText = "0 (оплата комментария)"
	}

	receiverVerb := genderVerb(receiver.Gender, "принял", "приняла", "принял(а)")
	giverMsg := fmt.Sprintf("🎉 <b>%s</b> %s %s! %s %s", displayName(receiver), receiverVerb, hugWord, giverCoinText, pluralObnimani(int(totalCoins)))
	if comment != nil {
		giverMsg = fmt.Sprintf("🎉 <b>%s</b> %s %s! %s", displayName(receiver), receiverVerb, hugWord, giverCoinText)
	}
	receiverMsg := fmt.Sprintf("🎉 Вы обнялись с <b>%s</b>! %s %s", displayName(giver), coinText, pluralObnimani(int(totalCoins)))

	// Append the comment to the receiver's notification
	if comment != nil && *comment != "" {
		receiverMsg += fmt.Sprintf("\n\n💬 <i>%s</i>", *comment)
	}

	n.sendToUser(ctx, giverID, giverMsg)
	n.sendToUser(ctx, receiverID, receiverMsg)
}

func pluralObnimani(n int) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	mod10 := abs % 10
	mod100 := abs % 100
	if mod10 == 1 && mod100 != 11 {
		return "обниманя"
	}
	if mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) {
		return "обнимани"
	}
	return "обнимань"
}

// NotifyHugDeclined notifies the giver that their hug was declined.
func (n *Notifier) NotifyHugDeclined(ctx context.Context, giverID, receiverID uuid.UUID) {
	receiver, err := n.repo.GetByID(ctx, receiverID)
	if err != nil {
		n.logger.Error("telegram: failed to look up receiver", "receiver_id", receiverID, "error", err)
		return
	}

	verb := genderVerb(receiver.Gender, "отклонил", "отклонила", "отклонил(а)")
	n.sendToUser(ctx, giverID, fmt.Sprintf("😔 <b>%s</b> %s объятие", displayName(receiver), verb))
}

// NotifyHugCancelled notifies the receiver that the hug request was cancelled.
func (n *Notifier) NotifyHugCancelled(ctx context.Context, receiverID, giverID uuid.UUID) {
	giver, err := n.repo.GetByID(ctx, giverID)
	if err != nil {
		n.logger.Error("telegram: failed to look up giver", "giver_id", giverID, "error", err)
		return
	}

	verb := genderVerb(giver.Gender, "отменил", "отменила", "отменил(а)")
	n.sendToUser(ctx, receiverID, fmt.Sprintf("❌ <b>%s</b> %s запрос на объятие", displayName(giver), verb))
}

func (n *Notifier) sendToUser(ctx context.Context, userID uuid.UUID, text string) {
	if !n.Enabled() {
		return
	}

	telegramID, err := n.repo.GetTelegramID(ctx, userID)
	if err != nil {
		n.logger.Error("telegram: failed to get user telegram_id", "user_id", userID, "error", err)
		return
	}
	if telegramID == nil {
		return // user hasn't configured Telegram notifications
	}

	if err := n.client.SendMessage(*telegramID, text); err != nil {
		n.logger.Error("telegram: failed to send message", "user_id", userID, "telegram_id", *telegramID, "error", err)
	}
}

// displayName returns the user's display name, falling back to username.
func displayName(u *models.User) string {
	if u.DisplayName != nil && *u.DisplayName != "" {
		return *u.DisplayName
	}
	return u.Username
}

// hugTypeSuggestionPhrase returns the full "хочет ... обнять" phrase for hug suggestions.
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

// hugTypeCompletedPhrase returns a phrase for completed hug messages to the giver.
// e.g. "принял(а) медвежьи обнимашки"
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

// genderVerb returns the appropriate Russian verb form based on user gender.
// male/female get distinct forms; nil/unknown gets the "(а)" fallback.
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
