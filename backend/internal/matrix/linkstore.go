package matrix

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/google/uuid"
)

const linkTokenTTL = 10 * time.Minute

// LinkRequest represents a pending link confirmation that the bot has DM'd
// to a user; the user must accept (via reaction or `!accept`) to finalize.
type LinkRequest struct {
	UserID    uuid.UUID
	MatrixID  string // MXID the user wants to link
	RoomID    string // DM room ID created by the bot
	MessageID string // event ID of the confirmation message (for reaction matching)
	Token     string // opaque token used in the deep-link URL
	ExpiresAt time.Time
}

// LinkStore holds pending Matrix link requests, keyed by token AND by (mxid+roomID)
// so the bot can resolve incoming reactions/commands to a pending request.
type LinkStore struct {
	mu      sync.Mutex
	byToken map[string]*LinkRequest
	byRoom  map[string]*LinkRequest // key: roomID
}

// NewLinkStore creates a new in-memory link request store.
func NewLinkStore() *LinkStore {
	return &LinkStore{
		byToken: make(map[string]*LinkRequest),
		byRoom:  make(map[string]*LinkRequest),
	}
}

// Create stores a new pending link request and returns the generated token.
// If a request already exists for the same room, it is replaced.
func (s *LinkStore) Create(userID uuid.UUID, matrixID, roomID, messageID string) (string, error) {
	token, err := generateRandomToken()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	req := &LinkRequest{
		UserID:    userID,
		MatrixID:  matrixID,
		RoomID:    roomID,
		MessageID: messageID,
		Token:     token,
		ExpiresAt: time.Now().Add(linkTokenTTL),
	}
	s.byToken[token] = req
	s.byRoom[roomID] = req
	return token, nil
}

// GetByRoom returns the pending request for the given room (if any, not expired).
func (s *LinkStore) GetByRoom(roomID string) (*LinkRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.byRoom[roomID]
	if !ok {
		return nil, false
	}
	if time.Now().After(req.ExpiresAt) {
		delete(s.byRoom, roomID)
		delete(s.byToken, req.Token)
		return nil, false
	}
	return req, true
}

// Consume removes the request for the given room, returning it if present.
// Used after a successful accept/reject so the same DM doesn't trigger again.
func (s *LinkStore) Consume(roomID string) (*LinkRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.byRoom[roomID]
	if !ok {
		return nil, false
	}
	delete(s.byRoom, roomID)
	delete(s.byToken, req.Token)
	if time.Now().After(req.ExpiresAt) {
		return nil, false
	}
	return req, true
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
