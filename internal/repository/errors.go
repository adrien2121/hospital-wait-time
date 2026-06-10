package repository

import "errors"

// ErrNotFound is returned when a requested entity does not exist.
// Callers wrap it with entity name and ID for human-readable messages:
//
//	fmt.Errorf("hospital %q: %w", id, repository.ErrNotFound)
var ErrNotFound = errors.New("not found")
