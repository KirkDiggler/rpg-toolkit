// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"errors"
	"fmt"
)

// Common condition errors
var (
	// ErrAlreadyActive indicates the condition is already applied
	ErrAlreadyActive = errors.New("condition already active")

	// ErrNotActive indicates the condition is not currently active
	ErrNotActive = errors.New("condition not active")

	// ErrNoTarget indicates no target was provided for the condition
	ErrNoTarget = errors.New("no target provided")

	// ErrInvalidTarget indicates the target is not valid for this condition
	ErrInvalidTarget = errors.New("invalid target for condition")

	// ErrInvalidRef indicates the condition reference is invalid
	ErrInvalidRef = errors.New("invalid condition reference")

	// ErrConditionImmune indicates the target is immune to this condition
	ErrConditionImmune = errors.New("target immune to condition")

	// ErrConditionSuppressed indicates this condition is suppressed by a stronger one
	ErrConditionSuppressed = errors.New("condition suppressed by stronger effect")

	// ErrInvalidLevel indicates an invalid condition level (e.g., exhaustion > 6)
	ErrInvalidLevel = errors.New("invalid condition level")

	// ErrUnknownCondition indicates the condition type is not registered
	ErrUnknownCondition = errors.New("unknown condition type")
)

// ApplyError wraps an error that occurred during condition application
type ApplyError struct {
	ConditionRef string
	Target       string
	Err          error
}

// Error implements the error interface
func (e *ApplyError) Error() string {
	return fmt.Sprintf("failed to apply condition %s to %s: %v",
		e.ConditionRef, e.Target, e.Err)
}

// Unwrap returns the underlying error
func (e *ApplyError) Unwrap() error {
	return e.Err
}

// RemoveError wraps an error that occurred during condition removal
type RemoveError struct {
	ConditionRef string
	Target       string
	Err          error
}

// Error implements the error interface
func (e *RemoveError) Error() string {
	return fmt.Sprintf("failed to remove condition %s from %s: %v",
		e.ConditionRef, e.Target, e.Err)
}

// Unwrap returns the underlying error
func (e *RemoveError) Unwrap() error {
	return e.Err
}

// LoadError wraps an error that occurred during condition loading
type LoadError struct {
	Data []byte
	Err  error
}

// Error implements the error interface
func (e *LoadError) Error() string {
	return fmt.Sprintf("failed to load condition: %v", e.Err)
}

// Unwrap returns the underlying error
func (e *LoadError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error condition might succeed on retry
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// These errors might succeed if conditions change
	switch {
	case errors.Is(err, ErrConditionSuppressed):
		return true // Might work if stronger condition is removed
	case errors.Is(err, ErrConditionImmune):
		return true // Might work if immunity is removed
	default:
		return false
	}
}
