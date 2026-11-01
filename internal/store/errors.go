package store

import "errors"

var (
	// ErrNotFound is returned when a requested row does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrDuplicate is returned when an event hash is already stored.
	ErrDuplicate = errors.New("store: duplicate event hash")
)
