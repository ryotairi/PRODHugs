package models

import (
	"time"

	"github.com/google/uuid"
)

type Announcement struct {
	ID        uuid.UUID
	Message   string
	CreatedAt time.Time
	CreatedBy uuid.UUID
	Active    bool
}
