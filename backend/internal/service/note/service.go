// Package note hosts the business logic for per-user private notes.
//
// A note is a (author_id, target_id) tuple holding a short string the author
// keeps to themselves. Self-notes (author == target) are intentionally
// allowed — they show up in the UI with an "о самом себе" easter-egg label.
package note

import (
	"context"
	"strings"
	"unicode/utf8"

	"go-service-template/internal/errorz"
	"go-service-template/internal/models"

	"github.com/google/uuid"
)

type noteRepo interface {
	GetNote(ctx context.Context, authorID, targetID uuid.UUID) (*models.UserNote, error)
	UpsertNote(ctx context.Context, authorID, targetID uuid.UUID, content string) (*models.UserNote, error)
	DeleteNote(ctx context.Context, authorID, targetID uuid.UUID) error
	ListNotes(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]*models.UserNote, error)
}

type userRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
}

type Service struct {
	notes noteRepo
	users userRepo
}

func New(notes noteRepo, users userRepo) *Service {
	return &Service{notes: notes, users: users}
}

// ResolveTarget accepts a UUID, a bare username, or "@username" and returns
// the resolved user. Returns errorz.ErrUserNotFound when neither form
// matches.
func (s *Service) ResolveTarget(ctx context.Context, raw string) (*models.User, error) {
	if id, err := uuid.Parse(raw); err == nil {
		return s.users.GetByID(ctx, id)
	}
	username := strings.TrimPrefix(raw, "@")
	if username == "" {
		return nil, errorz.ErrUserNotFound
	}
	return s.users.GetByUsername(ctx, username)
}

// Get returns the caller's note about target, or nil if none.
func (s *Service) Get(ctx context.Context, authorID, targetID uuid.UUID) (*models.UserNote, error) {
	return s.notes.GetNote(ctx, authorID, targetID)
}

// Upsert validates content and writes the note. Returns errorz.ErrNoteInvalid
// if the content is empty or longer than models.MaxUserNoteLength.
func (s *Service) Upsert(ctx context.Context, authorID, targetID uuid.UUID, rawContent string) (*models.UserNote, error) {
	content := strings.TrimSpace(rawContent)
	if content == "" {
		return nil, errorz.ErrNoteInvalid
	}
	if utf8.RuneCountInString(content) > models.MaxUserNoteLength {
		return nil, errorz.ErrNoteInvalid
	}
	return s.notes.UpsertNote(ctx, authorID, targetID, content)
}

// Delete removes the caller's note. No error if it doesn't exist (matches the
// HTTP 204 semantics in the spec).
func (s *Service) Delete(ctx context.Context, authorID, targetID uuid.UUID) error {
	return s.notes.DeleteNote(ctx, authorID, targetID)
}

// List returns the caller's notes ordered by recency.
func (s *Service) List(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]*models.UserNote, error) {
	return s.notes.ListNotes(ctx, authorID, limit, offset)
}
