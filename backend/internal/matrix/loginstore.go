package matrix

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

const loginTokenTTL = 10 * time.Minute

// LoginSessionStatus represents the state of a Matrix login/signup session.
type LoginSessionStatus string

const (
	LoginSessionPending       LoginSessionStatus = "pending"
	LoginSessionAuthenticated LoginSessionStatus = "authenticated"
	LoginSessionFailed        LoginSessionStatus = "failed"
)

// MatrixUserInfo holds data gathered about the Matrix user during signup.
type MatrixUserInfo struct {
	MatrixID    string
	RoomID      string
	DisplayName string
}

// loginSession represents a pending or completed Matrix login session.
//
// The frontend receives pollToken; the bot receives botToken (embedded in the
// command the user sends, e.g. `!signup <botToken>`). The two are paired here.
type loginSession struct {
	BotToken   string
	PollToken  string
	Status     LoginSessionStatus
	UserID     uuid.UUID
	UserInfo   *MatrixUserInfo
	FailReason string
	ExpiresAt  time.Time
}

// LoginStore manages Matrix login/signup sessions. Thread-safe.
type LoginStore struct {
	mu       sync.Mutex
	sessions map[string]*loginSession // keyed by pollToken
	botIndex map[string]string        // botToken -> pollToken
}

// NewLoginStore creates a new in-memory Matrix login session store.
func NewLoginStore() *LoginStore {
	return &LoginStore{
		sessions: make(map[string]*loginSession),
		botIndex: make(map[string]string),
	}
}

// CreateSession starts a new login session and returns (botToken, pollToken).
// The botToken is what the user sends via `!signup <botToken>`.
// The pollToken is returned to the frontend for polling.
func (s *LoginStore) CreateSession() (botToken, pollToken string, err error) {
	botToken, err = generateRandomToken()
	if err != nil {
		return "", "", err
	}
	pollToken, err = generateRandomToken()
	if err != nil {
		return "", "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[pollToken] = &loginSession{
		BotToken:  botToken,
		PollToken: pollToken,
		Status:    LoginSessionPending,
		ExpiresAt: time.Now().Add(loginTokenTTL),
	}
	s.botIndex[botToken] = pollToken

	return botToken, pollToken, nil
}

// ConsumeBotToken is called by the bot when it receives `!signup <botToken>`.
// Returns the pollToken if valid and not expired. One-shot: removes from botIndex.
func (s *LoginStore) ConsumeBotToken(botToken string) (pollToken string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pollToken, exists := s.botIndex[botToken]
	if !exists {
		return "", false
	}
	delete(s.botIndex, botToken)

	session, exists := s.sessions[pollToken]
	if !exists || time.Now().After(session.ExpiresAt) {
		return "", false
	}
	return pollToken, true
}

// SetSessionUserInfo stores the Matrix user info on the session.
func (s *LoginStore) SetSessionUserInfo(pollToken string, info *MatrixUserInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[pollToken]
	if !exists {
		return
	}
	session.UserInfo = info
}

// AuthenticateSession marks a session as successfully authenticated.
func (s *LoginStore) AuthenticateSession(pollToken string, userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[pollToken]
	if !exists {
		return
	}
	session.Status = LoginSessionAuthenticated
	session.UserID = userID
}

// FailSession marks a session as failed with a reason.
func (s *LoginStore) FailSession(pollToken string, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[pollToken]
	if !exists {
		return
	}
	session.Status = LoginSessionFailed
	session.FailReason = reason
}

// PollResult holds the outcome of polling a session.
type PollResult struct {
	Status     LoginSessionStatus
	UserID     uuid.UUID
	FailReason string
}

// PollSession returns the current state of a session. Terminal sessions
// (authenticated/failed) are deleted after being read.
func (s *LoginStore) PollSession(pollToken string) (PollResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[pollToken]
	if !exists {
		return PollResult{}, false
	}
	if time.Now().After(session.ExpiresAt) {
		delete(s.sessions, pollToken)
		if session.BotToken != "" {
			delete(s.botIndex, session.BotToken)
		}
		return PollResult{}, false
	}

	result := PollResult{
		Status:     session.Status,
		UserID:     session.UserID,
		FailReason: session.FailReason,
	}

	if session.Status == LoginSessionAuthenticated || session.Status == LoginSessionFailed {
		delete(s.sessions, pollToken)
		if session.BotToken != "" {
			delete(s.botIndex, session.BotToken)
		}
	}
	return result, true
}
