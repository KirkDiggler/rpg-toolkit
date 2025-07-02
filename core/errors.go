package core

import (
	"errors"
	"fmt"
)

// Common errors that can occur throughout the RPG toolkit.
var (
	// ErrEntityNotFound is returned when an entity cannot be found.
	ErrEntityNotFound = errors.New("entity not found")

	// ErrInvalidEntity is returned when an entity is invalid or malformed.
	ErrInvalidEntity = errors.New("invalid entity")

	// ErrDuplicateEntity is returned when attempting to create an entity with an ID that already exists.
	ErrDuplicateEntity = errors.New("duplicate entity")

	// ErrNilEntity is returned when a nil entity is provided where a valid entity is expected.
	ErrNilEntity = errors.New("nil entity")

	// ErrEmptyID is returned when an entity has an empty or invalid ID.
	ErrEmptyID = errors.New("empty entity ID")

	// ErrInvalidType is returned when an entity has an invalid or unrecognized type.
	ErrInvalidType = errors.New("invalid entity type")
)

// EntityError represents an error related to a specific entity.
type EntityError struct {
	EntityID   string
	EntityType string
	Op         string // Operation that caused the error
	Err        error  // Underlying error
}

// Error implements the error interface for EntityError.
func (e *EntityError) Error() string {
	if e.EntityID != "" && e.EntityType != "" {
		return fmt.Sprintf("%s %s %s: %v", e.Op, e.EntityType, e.EntityID, e.Err)
	} else if e.EntityType != "" {
		return fmt.Sprintf("%s %s: %v", e.Op, e.EntityType, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *EntityError) Unwrap() error {
	return e.Err
}

// NewEntityError creates a new EntityError.
func NewEntityError(op, entityType, entityID string, err error) *EntityError {
	return &EntityError{
		EntityID:   entityID,
		EntityType: entityType,
		Op:         op,
		Err:        err,
	}
}