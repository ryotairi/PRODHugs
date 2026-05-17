package models

import (
	"time"

	"github.com/google/uuid"
)

// UserNote is a private per-pair note authored by one user about another.
// Each (AuthorID, TargetID) pair has at most one note; the note is only ever
// readable by its author.
type UserNote struct {
	AuthorID  uuid.UUID
	TargetID  uuid.UUID
	Content   string
	UpdatedAt time.Time

	// Populated only by the list endpoint (joined from users).
	TargetUsername    string
	TargetDisplayName *string
}

// MaxUserNoteLength is the upper bound on note content length. Matches the
// CHECK constraint in migration 00030_user_notes.sql.
const MaxUserNoteLength = 256
