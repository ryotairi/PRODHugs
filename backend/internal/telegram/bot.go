package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go-service-template/internal/models"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/google/uuid"
)

// botUserRepo is the minimal interface the bot needs for account linking and lookups.
type botUserRepo interface {
	SetTelegramID(ctx context.Context, userID uuid.UUID, telegramID int64) (*models.User, error)
	IsTelegramIDTaken(ctx context.Context, telegramID int64, excludeUserID uuid.UUID) (bool, error)
	GetTelegramID(ctx context.Context, userID uuid.UUID) (*int64, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// hugAcceptor is the interface for accepting/declining hugs from Telegram.
type hugAcceptor interface {
	AcceptHug(ctx context.Context, hugID, receiverID uuid.UUID) (*models.Hug, error)
	DeclineHug(ctx context.Context, hugID, receiverID uuid.UUID) error
}

// telegramLoginService handles the auth/registration logic for Telegram login.
type telegramLoginService interface {
	LoginViaTelegram(ctx context.Context, info *TelegramUserInfo) (*models.User, error)
}

// Bot is a long-polling Telegram bot that handles /start deep-link commands
// and inline keyboard callbacks for hug actions.
type Bot struct {
	tg         *tgbot.Bot
	client     *Client
	linkStore  *LinkStore
	loginStore *LoginStore
	loginSvc   telegramLoginService
	userRepo   botUserRepo
	hugSvc     hugAcceptor
	logger     *slog.Logger
	enabled    bool
}

// NewBot creates a new Telegram bot. If the client is disabled (no token),
// Run() will be a no-op and SendHugSuggestion will fall back to plain messages.
func NewBot(client *Client, linkStore *LinkStore, userRepo botUserRepo, hugSvc hugAcceptor, logger *slog.Logger) *Bot {
	b := &Bot{
		client:    client,
		linkStore: linkStore,
		userRepo:  userRepo,
		hugSvc:    hugSvc,
		logger:    logger,
	}

	if !client.Enabled() {
		return b
	}

	// The default getMe init timeout (5s) is too aggressive on cold-start /
	// flaky networks — bump it and don't block app startup on a slow Telegram
	// response. If getMe still fails the bot stays disabled but the rest of
	// the service starts.
	tg, err := tgbot.New(
		client.token,
		tgbot.WithDefaultHandler(b.handleUpdate),
		tgbot.WithCheckInitTimeout(30*time.Second),
	)
	if err != nil {
		logger.Error("telegram bot: failed to create", "error", err)
		return b
	}

	b.tg = tg
	b.enabled = true
	return b
}

// SetLoginStore configures the login store and service for Telegram login.
// Called after construction to break circular dependencies.
func (b *Bot) SetLoginStore(store *LoginStore, svc telegramLoginService) {
	b.loginStore = store
	b.loginSvc = svc
}

// Run starts the long-polling bot. Blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	if !b.enabled {
		b.logger.Info("telegram bot disabled (no token)")
		return
	}

	b.logger.Info("telegram bot started (long-polling)")
	b.tg.Start(ctx)
	b.logger.Info("telegram bot stopped")
}

// SendHugSuggestion sends a hug suggestion notification with Accept/Decline buttons.
func (b *Bot) SendHugSuggestion(ctx context.Context, receiverID uuid.UUID, hugID uuid.UUID, giverName string, phrase string, comment *string) {
	telegramID, err := b.userRepo.GetTelegramID(ctx, receiverID)
	if err != nil || telegramID == nil {
		return
	}

	text := fmt.Sprintf("🤗 <b>%s</b> %s!", tgbot.EscapeMarkdownUnescaped(giverName), phrase)
	if comment != nil && *comment != "" {
		text += fmt.Sprintf("\n\n💬 <i>%s</i>", *comment)
	}

	if !b.enabled {
		// Fallback to plain message via raw client
		_ = b.client.SendMessage(*telegramID, text)
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Обнять 🤗", CallbackData: "accept:" + hugID.String()},
				{Text: "Отклонить", CallbackData: "decline:" + hugID.String()},
			},
		},
	}

	_, err = b.tg.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      *telegramID,
		Text:        text,
		ParseMode:   tgmodels.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		b.logger.Error("telegram bot: failed to send hug suggestion", "receiver_id", receiverID, "error", err)
	}
}

func (b *Bot) handleUpdate(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.CallbackQuery != nil {
		b.handleCallback(ctx, bot, update.CallbackQuery)
		return
	}

	if update.Message != nil && strings.HasPrefix(update.Message.Text, "/start") {
		b.handleStart(ctx, bot, update.Message)
	}
}

func (b *Bot) handleStart(ctx context.Context, bot *tgbot.Bot, msg *tgmodels.Message) {
	chatID := msg.Chat.ID

	parts := strings.SplitN(msg.Text, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		b.reply(ctx, bot, chatID, "Чтобы привязать аккаунт, используй настройки на сайте")
		return
	}

	token := strings.TrimSpace(parts[1])

	// Handle login_ prefixed tokens for Telegram login flow
	if strings.HasPrefix(token, "login_") {
		b.handleLoginStart(ctx, bot, msg, strings.TrimPrefix(token, "login_"))
		return
	}

	// Original account-linking flow
	userID, ok := b.linkStore.ConsumeToken(token)
	if !ok {
		b.reply(ctx, bot, chatID, "Ссылка недействительна или истекла. Попробуй снова через настройки приложения")
		return
	}

	taken, err := b.userRepo.IsTelegramIDTaken(ctx, chatID, userID)
	if err != nil {
		b.logger.Error("telegram bot: failed to check telegram_id", "error", err)
		b.reply(ctx, bot, chatID, "Произошла ошибка. Попробуй позже :(")
		return
	}
	if taken {
		b.reply(ctx, bot, chatID, "Этот Telegram аккаунт уже привязан к другому пользователю :(")
		return
	}

	_, err = b.userRepo.SetTelegramID(ctx, userID, chatID)
	if err != nil {
		b.logger.Error("telegram bot: failed to set telegram_id", "user_id", userID, "chat_id", chatID, "error", err)
		b.reply(ctx, bot, chatID, "Произошла ошибка при привязке. Попробуй позже :(")
		return
	}

	b.logger.Info("telegram bot: account linked", "user_id", userID, "chat_id", chatID)
	b.reply(ctx, bot, chatID, "✅ Аккаунт привязан! Теперь ты не пропустишь обнимашки от любимых продовцев")
}

func (b *Bot) handleLoginStart(ctx context.Context, bot *tgbot.Bot, msg *tgmodels.Message, botToken string) {
	chatID := msg.Chat.ID

	if b.loginStore == nil || b.loginSvc == nil {
		b.reply(ctx, bot, chatID, "Вход через Telegram временно недоступен")
		return
	}

	pollToken, ok := b.loginStore.ConsumeBotToken(botToken)
	if !ok {
		b.reply(ctx, bot, chatID, "Ссылка недействительна или истекла. Попробуй снова")
		return
	}

	// Build Telegram user info from the message sender
	info := &TelegramUserInfo{
		TelegramID: chatID,
		FirstName:  msg.From.FirstName,
		LastName:   msg.From.LastName,
	}
	if msg.From.Username != "" {
		info.Username = msg.From.Username
	}

	// Store user info on the session
	b.loginStore.SetSessionUserInfo(pollToken, info)

	// Attempt login/registration via the service
	user, err := b.loginSvc.LoginViaTelegram(ctx, info)
	if err != nil {
		b.logger.Error("telegram bot: login failed", "chat_id", chatID, "error", err)
		b.loginStore.FailSession(pollToken, err.Error())
		b.reply(ctx, bot, chatID, "Не удалось войти: "+friendlyLoginError(err))
		return
	}

	// Mark session as authenticated
	b.loginStore.AuthenticateSession(pollToken, user.ID)
	b.logger.Info("telegram bot: login successful", "user_id", user.ID, "chat_id", chatID)
	b.reply(ctx, bot, chatID, "✅ Вход выполнен! Можешь вернуться в приложение")
}

func friendlyLoginError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "banned"):
		return "ваш аккаунт заблокирован"
	default:
		return "попробуйте позже"
	}
}

func (b *Bot) handleCallback(ctx context.Context, bot *tgbot.Bot, cb *tgmodels.CallbackQuery) {
	data := cb.Data
	chatID := cb.Message.Message.Chat.ID

	var action, hugIDStr string
	if strings.HasPrefix(data, "accept:") {
		action = "accept"
		hugIDStr = strings.TrimPrefix(data, "accept:")
	} else if strings.HasPrefix(data, "decline:") {
		action = "decline"
		hugIDStr = strings.TrimPrefix(data, "decline:")
	} else {
		return
	}

	hugID, err := uuid.Parse(hugIDStr)
	if err != nil {
		b.answerCallback(ctx, bot, cb.ID, "Ошибка: некорректные данные")
		return
	}

	// Look up the user by their Telegram chat ID
	user, err := b.userRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		b.answerCallback(ctx, bot, cb.ID, "Ваш Telegram не привязан к аккаунту")
		return
	}

	msgID := cb.Message.Message.ID
	originalText := cb.Message.Message.Text

	switch action {
	case "accept":
		_, err = b.hugSvc.AcceptHug(ctx, hugID, user.ID)
		if err != nil {
			b.logger.Error("telegram bot: failed to accept hug", "hug_id", hugID, "error", err)
			b.answerCallback(ctx, bot, cb.ID, "Не удалось принять объятие: "+friendlyError(err))
			return
		}
		b.answerCallback(ctx, bot, cb.ID, "Объятие принято! 🤗")
		b.editMessageText(ctx, bot, chatID, msgID, originalText+"\n\n✅ <b>Принято!</b>")

	case "decline":
		err = b.hugSvc.DeclineHug(ctx, hugID, user.ID)
		if err != nil {
			b.logger.Error("telegram bot: failed to decline hug", "hug_id", hugID, "error", err)
			b.answerCallback(ctx, bot, cb.ID, "Не удалось отклонить: "+friendlyError(err))
			return
		}
		b.answerCallback(ctx, bot, cb.ID, "Объятие отклонено")
		b.editMessageText(ctx, bot, chatID, msgID, originalText+"\n\n❌ <b>Отклонено</b>")
	}
}

func (b *Bot) reply(ctx context.Context, bot *tgbot.Bot, chatID int64, text string) {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		b.logger.Error("telegram bot: failed to send message", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) answerCallback(ctx context.Context, bot *tgbot.Bot, callbackID string, text string) {
	_, err := bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       false,
	})
	if err != nil {
		b.logger.Error("telegram bot: failed to answer callback", "error", err)
	}
}

func (b *Bot) editMessageText(ctx context.Context, bot *tgbot.Bot, chatID int64, messageID int, newText string) {
	_, err := bot.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      newText,
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		b.logger.Error("telegram bot: failed to edit message", "error", err)
	}
}

func friendlyError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		return "объятие не найдено"
	case strings.Contains(msg, "not pending"):
		return "объятие уже обработано"
	case strings.Contains(msg, "expired"):
		return "объятие истекло"
	default:
		return "попробуйте позже"
	}
}

// DeepLinkURL returns the t.me deep-link URL for a given token and bot username.
func DeepLinkURL(botUsername, token string) string {
	return fmt.Sprintf("https://t.me/%s?start=%s", botUsername, token)
}
