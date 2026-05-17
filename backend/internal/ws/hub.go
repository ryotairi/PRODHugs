package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"go-service-template/internal/jwt"
	"go-service-template/internal/metrics"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

const (
	// How often the server sends a keepalive ping to clients.
	pingInterval = 30 * time.Second
	// How long to wait for any data from the client before considering it dead.
	readTimeout = 90 * time.Second
)

// WSMessage is the typed envelope for all outgoing WebSocket messages.
type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type wsAuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// Hub manages WebSocket clients and broadcasts messages
type Hub struct {
	mu         sync.RWMutex
	clients    map[*client]struct{}
	userIndex  map[uuid.UUID]map[*client]struct{} // user -> clients
	jwtManager *jwt.Manager
}

type client struct {
	userID uuid.UUID
	role   string
	ws     *websocket.Conn
	send   chan []byte
}

func NewHub(jwtManager *jwt.Manager) *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		userIndex:  make(map[uuid.UUID]map[*client]struct{}),
		jwtManager: jwtManager,
	}
}

// HandleWS is the Echo handler for WebSocket connections.
// Authenticates via the first client message: {"type":"auth","token":"..."}.
func (h *Hub) HandleWS(c echo.Context) error {
	websocket.Handler(func(ws *websocket.Conn) {
		defer func() { _ = ws.Close() }()

		// Require auth as the very first client message to avoid leaking JWT in URL.
		_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
		authBuf := make([]byte, 4096)
		n, err := ws.Read(authBuf)
		if err != nil {
			return
		}

		var authMsg wsAuthMessage
		if err := json.Unmarshal(authBuf[:n], &authMsg); err != nil || authMsg.Type != "auth" || authMsg.Token == "" {
			return
		}

		userID, role, tokenType, _, _, err := h.jwtManager.ParseToken(authMsg.Token)
		if err != nil || tokenType != "access" {
			return
		}

		_ = ws.SetReadDeadline(time.Time{})

		cl := &client{
			userID: userID,
			role:   role,
			ws:     ws,
			send:   make(chan []byte, 256),
		}

		h.register(cl)

		// Send current online state to newly connected clients.
		h.sendOnlineUsers(cl)
		if role == "admin" {
			h.sendOnlineCount(cl)
		}

		// Write pump: sends queued messages and periodic keepalive pings.
		done := make(chan struct{})
		go func() {
			defer close(done)
			pingTicker := time.NewTicker(pingInterval)
			defer pingTicker.Stop()
			pingPayload := []byte(`{"type":"ping"}`)

			for {
				select {
				case msg, ok := <-cl.send:
					if !ok {
						return // channel closed
					}
					if _, err := ws.Write(msg); err != nil {
						return
					}
				case <-pingTicker.C:
					if _, err := ws.Write(pingPayload); err != nil {
						return
					}
				}
			}
		}()

		// Read pump: keeps the connection alive and detects dead clients via deadline.
		buf := make([]byte, 512)
		for {
			_ = ws.SetReadDeadline(time.Now().Add(readTimeout))
			if _, err := ws.Read(buf); err != nil {
				break
			}
		}

		// Unregister BEFORE closing the channel to prevent Broadcast/SendToUser
		// from sending on a closed channel (which would panic).
		h.unregister(cl)
		close(cl.send)
		<-done
	}).ServeHTTP(c.Response(), c.Request())

	return nil
}

func (h *Hub) register(cl *client) {
	h.mu.Lock()
	h.clients[cl] = struct{}{}
	if h.userIndex[cl.userID] == nil {
		h.userIndex[cl.userID] = make(map[*client]struct{})
	}
	h.userIndex[cl.userID][cl] = struct{}{}
	count := len(h.userIndex)
	h.mu.Unlock()

	metrics.SetWSUniqueUserCount(count)
	h.broadcastOnlineCount()
	h.broadcastOnlineUsers()
}

func (h *Hub) unregister(cl *client) {
	h.mu.Lock()
	delete(h.clients, cl)
	if uClients, ok := h.userIndex[cl.userID]; ok {
		delete(uClients, cl)
		if len(uClients) == 0 {
			delete(h.userIndex, cl.userID)
		}
	}
	count := len(h.userIndex)
	h.mu.Unlock()

	metrics.SetWSUniqueUserCount(count)
	h.broadcastOnlineCount()
	h.broadcastOnlineUsers()
}

// Broadcast sends a typed message to all connected clients.
func (h *Hub) Broadcast(msgType string, data any) {
	msg := WSMessage{Type: msgType, Data: data}
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal ws message", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for cl := range h.clients {
		select {
		case cl.send <- payload:
		default:
			// Client buffer full, skip
		}
	}
}

// SendToUser sends a typed message to all connections of a specific user.
func (h *Hub) SendToUser(userID uuid.UUID, msgType string, data any) {
	msg := WSMessage{Type: msgType, Data: data}
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal ws message", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	uClients, ok := h.userIndex[userID]
	if !ok {
		return
	}

	for cl := range uClients {
		select {
		case cl.send <- payload:
		default:
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// UniqueUserCount returns the number of distinct authenticated users.
func (h *Hub) UniqueUserCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.userIndex)
}

// sendOnlineCount sends the current unique user count to a single client.
func (h *Hub) sendOnlineCount(cl *client) {
	h.mu.RLock()
	count := len(h.userIndex)
	h.mu.RUnlock()

	payload, err := json.Marshal(WSMessage{
		Type: "online_count",
		Data: map[string]int{"count": count},
	})
	if err != nil {
		return
	}

	select {
	case cl.send <- payload:
	default:
	}
}

// OnlineUserIDs returns a snapshot of all unique online user IDs.
func (h *Hub) OnlineUserIDs() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uuid.UUID, 0, len(h.userIndex))
	for id := range h.userIndex {
		ids = append(ids, id)
	}
	return ids
}

// onlineUserIDStrings returns online user IDs as strings (must be called under at least RLock).
func (h *Hub) onlineUserIDStrings() []string {
	ids := make([]string, 0, len(h.userIndex))
	for id := range h.userIndex {
		ids = append(ids, id.String())
	}
	return ids
}

// sendOnlineUsers sends the current set of online user IDs to a single client.
func (h *Hub) sendOnlineUsers(cl *client) {
	h.mu.RLock()
	ids := h.onlineUserIDStrings()
	h.mu.RUnlock()

	payload, err := json.Marshal(WSMessage{
		Type: "online_users",
		Data: map[string][]string{"user_ids": ids},
	})
	if err != nil {
		return
	}

	select {
	case cl.send <- payload:
	default:
	}
}

// broadcastOnlineUsers pushes the current set of online user IDs to all connected clients.
func (h *Hub) broadcastOnlineUsers() {
	h.mu.RLock()
	ids := h.onlineUserIDStrings()

	payload, err := json.Marshal(WSMessage{
		Type: "online_users",
		Data: map[string][]string{"user_ids": ids},
	})
	if err != nil {
		h.mu.RUnlock()
		return
	}

	for cl := range h.clients {
		select {
		case cl.send <- payload:
		default:
		}
	}
	h.mu.RUnlock()
}

// broadcastOnlineCount pushes the current unique user count to all admin clients.
func (h *Hub) broadcastOnlineCount() {
	h.mu.RLock()
	count := len(h.userIndex)

	payload, err := json.Marshal(WSMessage{
		Type: "online_count",
		Data: map[string]int{"count": count},
	})
	if err != nil {
		h.mu.RUnlock()
		return
	}

	for cl := range h.clients {
		if cl.role == "admin" {
			select {
			case cl.send <- payload:
			default:
			}
		}
	}
	h.mu.RUnlock()
}
