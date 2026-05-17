package errorz

import "errors"

var (
	ErrNoteNotFound = errors.New("note not found")
	ErrNoteInvalid  = errors.New("note content is empty or longer than 256 characters")
)
