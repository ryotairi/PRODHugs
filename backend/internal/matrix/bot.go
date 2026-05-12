package matrix

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"go-service-template/internal/models"

	"github.com/google/uuid"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// Reaction keys used by the bot for confirmation / hug acceptance flows.
const (
	ReactionAccept = "✅"
	ReactionReject = "❌"
)

// botUserRepo is the minimal interface the bot needs for account linking.
type botUserRepo interface {
	SetMatrixID(ctx context.Context, userID uuid.UUID, matrixID, roomID string) (*models.User, error)
	IsMatrixIDTaken(ctx context.Context, matrixID string, excludeUserID uuid.UUID) (bool, error)
	GetByMatrixID(ctx context.Context, matrixID string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	ClearMatrixID(ctx context.Context, userID uuid.UUID) (*models.User, error)
}

// hugAcceptor is the interface for accepting/declining hugs from Matrix.
type hugAcceptor interface {
	AcceptHug(ctx context.Context, hugID, receiverID uuid.UUID) (*models.Hug, error)
	DeclineHug(ctx context.Context, hugID, receiverID uuid.UUID) error
}

// matrixLoginService handles auth/registration logic for Matrix signup.
type matrixLoginService interface {
	LoginViaMatrix(ctx context.Context, info *MatrixUserInfo) (*models.User, error)
}

// pendingHug tracks a hug suggestion message the bot has posted,
// so reactions to it can be resolved back to the hug ID + receiver.
type pendingHug struct {
	HugID      uuid.UUID
	ReceiverID uuid.UUID
}

// Bot is a Matrix bot that handles account-link DMs, signup via `!signup`,
// hug suggestions (with reactions for accept/decline), and the !unlink command.
//
// When MATRIX_ACCESS_TOKEN is empty the bot is disabled and Run() returns immediately.
type Bot struct {
	client      *Client
	linkStore   *LinkStore
	loginStore  *LoginStore
	loginSvc    matrixLoginService
	userRepo    botUserRepo
	hugSvc      hugAcceptor
	logger      *slog.Logger
	displayName string // Русскоязычное имя бота для команды !signup (например "prodhugsbot")

	mu sync.Mutex
	// roomID -> messageID -> pending hug context (for reaction routing)
	pendingHugs map[id.RoomID]map[id.EventID]pendingHug
}

// NewBot creates a Matrix bot. If the client is disabled (no token) the bot is a no-op.
func NewBot(client *Client, linkStore *LinkStore, userRepo botUserRepo, hugSvc hugAcceptor, logger *slog.Logger) *Bot {
	return &Bot{
		client:      client,
		linkStore:   linkStore,
		userRepo:    userRepo,
		hugSvc:      hugSvc,
		logger:      logger,
		pendingHugs: make(map[id.RoomID]map[id.EventID]pendingHug),
	}
}

// SetLoginStore configures the login store and service for signup.
func (b *Bot) SetLoginStore(store *LoginStore, svc matrixLoginService) {
	b.loginStore = store
	b.loginSvc = svc
}

// Enabled reports whether the bot was created with a valid access token.
func (b *Bot) Enabled() bool {
	return b.client != nil && b.client.Enabled()
}

// Run starts the Matrix sync loop. Blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	if !b.Enabled() {
		b.logger.Info("matrix bot disabled (no access token)")
		return
	}

	syncer, ok := b.client.Client.Syncer.(*mautrix.DefaultSyncer)
	if !ok {
		b.logger.Error("matrix bot: unexpected syncer type")
		return
	}

	// Auto-join rooms the bot is invited to (so DMs initiated by the bot get joined
	// automatically on the user side is irrelevant, but we also accept user invites).
	syncer.OnEventType(event.StateMember, func(ctx context.Context, evt *event.Event) {
		if evt.GetStateKey() != b.client.UserID().String() {
			return
		}
		member := evt.Content.AsMember()
		if member.Membership == event.MembershipInvite {
			if _, err := b.client.Client.JoinRoomByID(ctx, evt.RoomID); err != nil {
				b.logger.Error("matrix bot: failed to join on invite", "room_id", evt.RoomID, "error", err)
			}
		}
	})

	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		b.handleMessage(ctx, evt)
	})

	syncer.OnEventType(event.EventReaction, func(ctx context.Context, evt *event.Event) {
		b.handleReaction(ctx, evt)
	})

	b.logger.Info("matrix bot started (sync)", "user_id", b.client.UserID())
	if err := b.client.Client.SyncWithContext(ctx); err != nil && ctx.Err() == nil {
		b.logger.Error("matrix bot: sync terminated with error", "error", err)
	}
	b.logger.Info("matrix bot stopped")
}

// InitiateLink opens a DM with the target MXID and posts a confirmation request.
// Returns the new room ID and event ID on success. Called from the link-token HTTP
// handler (via the service layer).
func (b *Bot) InitiateLink(ctx context.Context, userID uuid.UUID, matrixID string) (roomID, eventID string, err error) {
	if !b.Enabled() {
		return "", "", fmt.Errorf("matrix bot disabled")
	}

	// Find the user whose account we're linking (for the nice name in the message).
	u, err := b.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("lookup user: %w", err)
	}

	// Check MXID is not already taken.
	taken, err := b.userRepo.IsMatrixIDTaken(ctx, matrixID, userID)
	if err != nil {
		return "", "", fmt.Errorf("check matrix id: %w", err)
	}
	if taken {
		return "", "", fmt.Errorf("matrix id taken")
	}

	// Create an unencrypted direct chat with the user.
	room, err := b.client.Client.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Preset:   "trusted_private_chat",
		IsDirect: true,
		Invite:   []id.UserID{id.UserID(matrixID)},
	})
	if err != nil {
		return "", "", fmt.Errorf("create room: %w", err)
	}

	name := u.Username
	if u.DisplayName != nil && *u.DisplayName != "" {
		name = *u.DisplayName
	}

	text := fmt.Sprintf(
		"Привет! Кто-то (аккаунт «%s» в PRODнимашках) просит привязать этот Matrix аккаунт.\n\n"+
			"Если это ты — ответь реакцией %s или командой !accept.\n"+
			"Если нет — реакцией %s или командой !reject.\n\n"+
			"Позже этот чат можно отвязать командой !unlink или через настройки профиля.",
		name, ReactionAccept, ReactionReject,
	)

	resp, err := b.client.Client.SendText(ctx, room.RoomID, text)
	if err != nil {
		return "", "", fmt.Errorf("send confirmation: %w", err)
	}

	// Add reactions so the user can tap them directly.
	_, _ = b.client.Client.SendReaction(ctx, room.RoomID, resp.EventID, ReactionAccept)
	_, _ = b.client.Client.SendReaction(ctx, room.RoomID, resp.EventID, ReactionReject)

	// Store in the link-request store (keyed by roomID).
	if _, err := b.linkStore.Create(userID, matrixID, room.RoomID.String(), resp.EventID.String()); err != nil {
		return "", "", fmt.Errorf("store link request: %w", err)
	}

	return room.RoomID.String(), resp.EventID.String(), nil
}

// SendHugSuggestion sends a hug suggestion message to the receiver's DM with
// accept/decline reactions. Falls back silently if the bot is disabled or the
// receiver has no linked Matrix account.
func (b *Bot) SendHugSuggestion(ctx context.Context, receiverID uuid.UUID, hugID uuid.UUID, giverName string, phrase string, comment *string) {
	if !b.Enabled() {
		return
	}

	receiver, err := b.userRepo.GetByID(ctx, receiverID)
	if err != nil || receiver.MatrixRoomID == nil || *receiver.MatrixRoomID == "" {
		return
	}
	roomID := id.RoomID(*receiver.MatrixRoomID)

	text := fmt.Sprintf("🤗 %s %s!", giverName, phrase)
	if comment != nil && *comment != "" {
		text += "\n\n💬 " + *comment
	}
	text += fmt.Sprintf("\n\nРеакции: %s принять, %s отклонить. Команды: !accept %s / !decline %s",
		ReactionAccept, ReactionReject, hugID.String(), hugID.String())

	resp, err := b.client.Client.SendText(ctx, roomID, text)
	if err != nil {
		b.logger.Error("matrix bot: failed to send hug suggestion", "receiver_id", receiverID, "error", err)
		return
	}

	_, _ = b.client.Client.SendReaction(ctx, roomID, resp.EventID, ReactionAccept)
	_, _ = b.client.Client.SendReaction(ctx, roomID, resp.EventID, ReactionReject)

	// Remember the mapping so reactions on this message route to the right hug.
	b.mu.Lock()
	m, ok := b.pendingHugs[roomID]
	if !ok {
		m = make(map[id.EventID]pendingHug)
		b.pendingHugs[roomID] = m
	}
	m[resp.EventID] = pendingHug{HugID: hugID, ReceiverID: receiverID}
	b.mu.Unlock()
}

// SendPlainToUser sends a plain-text message to the receiver's DM (if linked).
// Used for non-interactive notifications (hug completed, declined, cancelled).
func (b *Bot) SendPlainToUser(ctx context.Context, userID uuid.UUID, text string) {
	if !b.Enabled() {
		return
	}
	u, err := b.userRepo.GetByID(ctx, userID)
	if err != nil || u.MatrixRoomID == nil || *u.MatrixRoomID == "" {
		return
	}
	if _, err := b.client.Client.SendText(ctx, id.RoomID(*u.MatrixRoomID), text); err != nil {
		b.logger.Error("matrix bot: failed to send message", "user_id", userID, "error", err)
	}
}

// ─── Incoming handlers ─────────────────────────────────────────────────────

func (b *Bot) handleMessage(ctx context.Context, evt *event.Event) {
	// Ignore our own messages.
	if evt.Sender == b.client.UserID() {
		return
	}

	msg := evt.Content.AsMessage()
	if msg == nil {
		return
	}
	body := strings.TrimSpace(msg.Body)
	if body == "" || !strings.HasPrefix(body, "!") {
		return
	}

	parts := strings.Fields(body)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "!accept":
		b.handleAcceptCommand(ctx, evt, parts)
	case "!reject", "!decline":
		b.handleRejectCommand(ctx, evt, parts)
	case "!unlink":
		b.handleUnlinkCommand(ctx, evt)
	case "!signup":
		b.handleSignupCommand(ctx, evt, parts)
	case "!help":
		b.reply(ctx, evt.RoomID, "Команды: !accept, !reject, !unlink, !signup <token>")
	}
}

func (b *Bot) handleReaction(ctx context.Context, evt *event.Event) {
	if evt.Sender == b.client.UserID() {
		return
	}
	rc := evt.Content.AsReaction()
	if rc == nil {
		return
	}
	key := rc.RelatesTo.Key
	targetEventID := rc.RelatesTo.EventID

	switch key {
	case ReactionAccept:
		// Could be link confirmation OR hug suggestion — resolve by roomID/eventID.
		if b.tryLinkReaction(ctx, evt, targetEventID, true) {
			return
		}
		b.tryHugReaction(ctx, evt, targetEventID, true)
	case ReactionReject:
		if b.tryLinkReaction(ctx, evt, targetEventID, false) {
			return
		}
		b.tryHugReaction(ctx, evt, targetEventID, false)
	}
}

// tryLinkReaction processes an accept/reject reaction as a link confirmation.
// Returns true if the event matched a pending link request.
func (b *Bot) tryLinkReaction(ctx context.Context, evt *event.Event, targetEventID id.EventID, accept bool) bool {
	req, ok := b.linkStore.GetByRoom(evt.RoomID.String())
	if !ok {
		return false
	}
	if req.MessageID != targetEventID.String() {
		return false
	}
	// Make sure it's the invited user reacting (not the bot itself or a stranger).
	if evt.Sender.String() != req.MatrixID {
		return false
	}
	if accept {
		b.finalizeLink(ctx, evt.RoomID, req)
	} else {
		b.rejectLink(ctx, evt.RoomID, req)
	}
	return true
}

func (b *Bot) tryHugReaction(ctx context.Context, evt *event.Event, targetEventID id.EventID, accept bool) {
	b.mu.Lock()
	rooms := b.pendingHugs[evt.RoomID]
	ph, ok := rooms[targetEventID]
	if ok {
		delete(rooms, targetEventID)
	}
	b.mu.Unlock()
	if !ok {
		return
	}

	// Ensure the reaction comes from the linked user for this room.
	user, err := b.userRepo.GetByID(ctx, ph.ReceiverID)
	if err != nil || user.MatrixID == nil || evt.Sender.String() != *user.MatrixID {
		return
	}

	if accept {
		if _, err := b.hugSvc.AcceptHug(ctx, ph.HugID, ph.ReceiverID); err != nil {
			b.reply(ctx, evt.RoomID, "Не удалось принять объятие: "+friendlyHugError(err))
			return
		}
		b.reply(ctx, evt.RoomID, "✅ Объятие принято! 🤗")
	} else {
		if err := b.hugSvc.DeclineHug(ctx, ph.HugID, ph.ReceiverID); err != nil {
			b.reply(ctx, evt.RoomID, "Не удалось отклонить: "+friendlyHugError(err))
			return
		}
		b.reply(ctx, evt.RoomID, "❌ Объятие отклонено")
	}
}

func (b *Bot) handleAcceptCommand(ctx context.Context, evt *event.Event, parts []string) {
	// First, handle link confirmation.
	if req, ok := b.linkStore.GetByRoom(evt.RoomID.String()); ok {
		if evt.Sender.String() == req.MatrixID {
			b.finalizeLink(ctx, evt.RoomID, req)
			return
		}
	}

	// Otherwise: `!accept <hug_id>` (or just `!accept` targeting the latest pending).
	var targetHug *pendingHug
	b.mu.Lock()
	rooms := b.pendingHugs[evt.RoomID]
	if len(parts) >= 2 {
		hugID, err := uuid.Parse(parts[1])
		if err == nil {
			for eid, ph := range rooms {
				if ph.HugID == hugID {
					delete(rooms, eid)
					targetHug = &ph
					break
				}
			}
		}
	} else {
		// No arg — pick any pending hug in this room.
		for eid, ph := range rooms {
			delete(rooms, eid)
			targetHug = &ph
			break
		}
	}
	b.mu.Unlock()

	if targetHug == nil {
		b.reply(ctx, evt.RoomID, "Нет ожидающих объятий. Если нужно привязать аккаунт — команду !accept шлёт только сам пользователь, указанный в привязке.")
		return
	}

	user, err := b.userRepo.GetByID(ctx, targetHug.ReceiverID)
	if err != nil || user.MatrixID == nil || evt.Sender.String() != *user.MatrixID {
		b.reply(ctx, evt.RoomID, "Вы не можете принимать эти объятия.")
		return
	}

	if _, err := b.hugSvc.AcceptHug(ctx, targetHug.HugID, targetHug.ReceiverID); err != nil {
		b.reply(ctx, evt.RoomID, "Не удалось принять: "+friendlyHugError(err))
		return
	}
	b.reply(ctx, evt.RoomID, "✅ Объятие принято! 🤗")
}

func (b *Bot) handleRejectCommand(ctx context.Context, evt *event.Event, parts []string) {
	// Link rejection path.
	if req, ok := b.linkStore.GetByRoom(evt.RoomID.String()); ok {
		if evt.Sender.String() == req.MatrixID {
			b.rejectLink(ctx, evt.RoomID, req)
			return
		}
	}

	// Hug decline path.
	var targetHug *pendingHug
	b.mu.Lock()
	rooms := b.pendingHugs[evt.RoomID]
	if len(parts) >= 2 {
		hugID, err := uuid.Parse(parts[1])
		if err == nil {
			for eid, ph := range rooms {
				if ph.HugID == hugID {
					delete(rooms, eid)
					targetHug = &ph
					break
				}
			}
		}
	} else {
		for eid, ph := range rooms {
			delete(rooms, eid)
			targetHug = &ph
			break
		}
	}
	b.mu.Unlock()

	if targetHug == nil {
		b.reply(ctx, evt.RoomID, "Нет активных запросов для отклонения.")
		return
	}

	user, err := b.userRepo.GetByID(ctx, targetHug.ReceiverID)
	if err != nil || user.MatrixID == nil || evt.Sender.String() != *user.MatrixID {
		b.reply(ctx, evt.RoomID, "Вы не можете отклонять эти объятия.")
		return
	}

	if err := b.hugSvc.DeclineHug(ctx, targetHug.HugID, targetHug.ReceiverID); err != nil {
		b.reply(ctx, evt.RoomID, "Не удалось отклонить: "+friendlyHugError(err))
		return
	}
	b.reply(ctx, evt.RoomID, "❌ Объятие отклонено")
}

func (b *Bot) handleUnlinkCommand(ctx context.Context, evt *event.Event) {
	u, err := b.userRepo.GetByMatrixID(ctx, evt.Sender.String())
	if err != nil {
		b.reply(ctx, evt.RoomID, "Этот Matrix аккаунт не привязан ни к одному профилю.")
		return
	}
	if _, err := b.userRepo.ClearMatrixID(ctx, u.ID); err != nil {
		b.logger.Error("matrix bot: failed to unlink", "user_id", u.ID, "error", err)
		b.reply(ctx, evt.RoomID, "Произошла ошибка при отвязке. Попробуй позже.")
		return
	}
	b.reply(ctx, evt.RoomID, "✅ Matrix отвязан. Если передумаешь — можно привязать снова через настройки профиля.")
	// Leave the DM so we don't keep a stale room around.
	go func(roomID id.RoomID) {
		_, _ = b.client.Client.LeaveRoom(context.Background(), roomID)
	}(evt.RoomID)
}

func (b *Bot) handleSignupCommand(ctx context.Context, evt *event.Event, parts []string) {
	if b.loginStore == nil || b.loginSvc == nil {
		b.reply(ctx, evt.RoomID, "Регистрация через Matrix сейчас недоступна.")
		return
	}
	if len(parts) < 2 {
		b.reply(ctx, evt.RoomID, "Использование: !signup <токен со страницы регистрации>")
		return
	}

	botToken := strings.TrimSpace(parts[1])
	pollToken, ok := b.loginStore.ConsumeBotToken(botToken)
	if !ok {
		b.reply(ctx, evt.RoomID, "Токен недействителен или истёк. Обнови страницу регистрации и попробуй снова.")
		return
	}

	mxid := evt.Sender.String()

	// Gather display name from the sender's profile (best effort).
	var displayName string
	if prof, err := b.client.Client.GetProfile(ctx, evt.Sender); err == nil && prof != nil {
		displayName = prof.DisplayName
	}

	info := &MatrixUserInfo{
		MatrixID:    mxid,
		RoomID:      evt.RoomID.String(),
		DisplayName: displayName,
	}
	b.loginStore.SetSessionUserInfo(pollToken, info)

	user, err := b.loginSvc.LoginViaMatrix(ctx, info)
	if err != nil {
		b.logger.Error("matrix bot: signup failed", "mxid", mxid, "error", err)
		b.loginStore.FailSession(pollToken, err.Error())
		b.reply(ctx, evt.RoomID, "Не удалось войти: "+friendlyLoginError(err))
		return
	}

	b.loginStore.AuthenticateSession(pollToken, user.ID)
	b.logger.Info("matrix bot: signup successful", "user_id", user.ID, "mxid", mxid)
	b.reply(ctx, evt.RoomID, "✅ Вход выполнен! Можешь вернуться в приложение. Этот чат теперь будет присылать уведомления об объятиях.")
}

func (b *Bot) finalizeLink(ctx context.Context, roomID id.RoomID, req *LinkRequest) {
	taken, err := b.userRepo.IsMatrixIDTaken(ctx, req.MatrixID, req.UserID)
	if err != nil {
		b.logger.Error("matrix bot: taken check failed", "error", err)
		b.reply(ctx, roomID, "Произошла ошибка. Попробуй позже.")
		return
	}
	if taken {
		b.linkStore.Consume(roomID.String())
		b.reply(ctx, roomID, "Этот Matrix аккаунт уже привязан к другому пользователю.")
		return
	}

	if _, err := b.userRepo.SetMatrixID(ctx, req.UserID, req.MatrixID, roomID.String()); err != nil {
		b.logger.Error("matrix bot: failed to set matrix id", "user_id", req.UserID, "error", err)
		b.reply(ctx, roomID, "Произошла ошибка при привязке. Попробуй позже.")
		return
	}
	b.linkStore.Consume(roomID.String())
	b.logger.Info("matrix bot: account linked", "user_id", req.UserID, "mxid", req.MatrixID)
	b.reply(ctx, roomID, "✅ Аккаунт привязан! Теперь ты не пропустишь обнимашки от любимых продовцев.")
}

func (b *Bot) rejectLink(ctx context.Context, roomID id.RoomID, req *LinkRequest) {
	b.linkStore.Consume(roomID.String())
	b.reply(ctx, roomID, "❌ Привязка отклонена. Если передумаешь — можно запросить её снова из настроек профиля.")
	// Leave the DM; we don't need it.
	go func(r id.RoomID) {
		_, _ = b.client.Client.LeaveRoom(context.Background(), r)
	}(roomID)
}

func (b *Bot) reply(ctx context.Context, roomID id.RoomID, text string) {
	if _, err := b.client.Client.SendText(ctx, roomID, text); err != nil {
		b.logger.Error("matrix bot: failed to reply", "room_id", roomID, "error", err)
	}
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

func friendlyHugError(err error) string {
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

// MatrixToURL returns a matrix.to link for chatting directly with the given MXID.
func MatrixToURL(userID string) string {
	return "https://matrix.to/#/" + userID
}
