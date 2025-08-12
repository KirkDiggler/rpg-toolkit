// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"errors"
	"fmt"
)

// Common errors returned by features.
var (
	// ErrFeatureNotFound indicates the requested feature doesn't exist.
	ErrFeatureNotFound = errors.New("feature not found")

	// ErrAlreadyActive indicates the feature is already active.
	ErrAlreadyActive = errors.New("feature is already active")

	// ErrNotActive indicates the feature is not currently active.
	ErrNotActive = errors.New("feature is not active")

	// ErrNoUsesRemaining indicates the feature has no uses left.
	ErrNoUsesRemaining = errors.New("no uses remaining")

	// ErrTargetRequired indicates the feature requires a target but none was provided.
	ErrTargetRequired = errors.New("target required")

	// ErrInvalidTarget indicates the provided target is not valid for this feature.
	ErrInvalidTarget = errors.New("invalid target")

	// ErrCannotActivate indicates the feature cannot be activated in the current state.
	ErrCannotActivate = errors.New("cannot activate feature")

	// ErrInvalidRef indicates a feature reference is malformed or missing.
	ErrInvalidRef = errors.New("invalid feature reference")

	// ErrMarshalFailed indicates JSON marshaling failed.
	ErrMarshalFailed = errors.New("failed to marshal feature data")

	// ErrUnmarshalFailed indicates JSON unmarshaling failed.
	ErrUnmarshalFailed = errors.New("failed to unmarshal feature data")
)

// ActivationError provides detailed information about why activation failed.
type ActivationError struct {
	Feature string // Feature ref that failed to activate
	Reason  error  // Underlying reason
}

// Error implements the error interface.
func (e *ActivationError) Error() string {
	return fmt.Sprintf("cannot activate %s: %v", e.Feature, e.Reason)
}

// Unwrap allows errors.Is and errors.As to work with the underlying error.
func (e *ActivationError) Unwrap() error {
	return e.Reason
}

// NewActivationError creates a new ActivationError.
func NewActivationError(feature string, reason error) *ActivationError {
	return &ActivationError{
		Feature: feature,
		Reason:  reason,
	}
}

// LoadError provides detailed information about why loading failed.
type LoadError struct {
	Ref    string // Feature ref that failed to load
	Reason error  // Underlying reason
}

// Error implements the error interface.
func (e *LoadError) Error() string {
	return fmt.Sprintf("cannot load feature %s: %v", e.Ref, e.Reason)
}

// Unwrap allows errors.Is and errors.As to work with the underlying error.
func (e *LoadError) Unwrap() error {
	return e.Reason
}

// NewLoadError creates a new LoadError.
func NewLoadError(ref string, reason error) *LoadError {
	return &LoadError{
		Ref:    ref,
		Reason: reason,
	}
}

// IsActivationError checks if an error is an activation error.
func IsActivationError(err error) bool {
	var activationErr *ActivationError
	return errors.As(err, &activationErr)
}

// IsLoadError checks if an error is a load error.
func IsLoadError(err error) bool {
	var loadErr *LoadError
	return errors.As(err, &loadErr)
}

// IsRetryable indicates if an error might succeed if retried.
// For example, ErrAlreadyActive is not retryable, but a temporary
// resource constraint might be.
func IsRetryable(err error) bool {
	// These errors won't change if retried immediately
	if errors.Is(err, ErrAlreadyActive) ||
		errors.Is(err, ErrNotActive) ||
		errors.Is(err, ErrFeatureNotFound) ||
		errors.Is(err, ErrInvalidRef) ||
		errors.Is(err, ErrInvalidTarget) {
		return false
	}

	// ErrNoUsesRemaining might change after a rest
	// ErrCannotActivate might change based on conditions
	return true
}
