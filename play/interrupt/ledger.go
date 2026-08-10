// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package interrupt

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// WindowID is a unique identifier for an open window, assigned monotonically from 1.
type WindowID uint64

// Option is an opaque token representing one offered choice.
// The module validates only non-emptiness and uniqueness within a window.
type Option string

// Window is the read-side value representing an open interrupt window.
// The Audience is the entity authorized to answer; Options are the offered choices;
// Payload is opaque frozen resolution data; At is caller-supplied provenance.
type Window struct {
	ID       WindowID
	Audience core.EntityID
	Options  []Option
	Payload  []byte
	At       uint64
}

// Ledger is the container for all open windows in an encounter.
// It tracks interrupt windows and assigns monotonically increasing IDs.
// Not safe for concurrent use. Construct via NewLedger.
type Ledger struct {
	nextID  uint64
	windows []Window
}

// NewLedger creates a new empty Ledger.
func NewLedger() (*Ledger, error) {
	return &Ledger{
		nextID:  1,
		windows: make([]Window, 0),
	}, nil
}

// PoseInput carries the parameters for opening a new interrupt window.
type PoseInput struct {
	Audience core.EntityID
	Options  []Option
	Payload  []byte
	At       uint64
}

// PoseOutput reports the window that was opened.
type PoseOutput struct {
	Window Window
}

// Pose opens a new interrupt window. Validates in order: nil input, audience,
// options (non-empty), then per-option in slice order: empty token, duplicate.
// On any validation error, no state changes (R5 atomicity). On success, assigns
// a monotonically increasing ID, deep-copies the options and payload, appends
// to the ledger, and returns the stored window (deep-copied).
func (l *Ledger) Pose(in *PoseInput) (*PoseOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("pose: %w", ErrNilInput)
	}

	if in.Audience == "" {
		return nil, fmt.Errorf("pose: %w", ErrNoAudience)
	}

	if len(in.Options) == 0 {
		return nil, fmt.Errorf("pose: %w", ErrNoOptions)
	}

	// Validate options in order: empty first, then duplicates
	seen := make(map[Option]struct{})
	for _, opt := range in.Options {
		if opt == "" {
			return nil, fmt.Errorf("pose: %w", ErrNoOption)
		}
		if _, exists := seen[opt]; exists {
			return nil, fmt.Errorf("pose: %w", ErrDuplicateOption)
		}
		seen[opt] = struct{}{}
	}

	// All validation passed; now assign ID and store
	id := WindowID(l.nextID)
	l.nextID++

	// Deep-copy options and payload
	optionsCopy := make([]Option, len(in.Options))
	copy(optionsCopy, in.Options)

	var payloadCopy []byte
	if in.Payload != nil {
		payloadCopy = make([]byte, len(in.Payload))
		copy(payloadCopy, in.Payload)
	}

	// Create and append the window
	window := Window{
		ID:       id,
		Audience: in.Audience,
		Options:  optionsCopy,
		Payload:  payloadCopy,
		At:       in.At,
	}
	l.windows = append(l.windows, window)

	// Return a deep copy of the stored window
	return &PoseOutput{
		Window: l.copyWindow(window),
	}, nil
}

// PendingForInput carries the query for open windows an audience may answer.
type PendingForInput struct {
	Audience core.EntityID
}

// PendingFor returns all open windows for a given audience in pose order,
// with deep-copied options and payload. Unknown audience returns empty slice, nil error.
// Validates: nil input, audience. Returns ErrNilInput or ErrNoAudience on defective input.
func (l *Ledger) PendingFor(in *PendingForInput) ([]Window, error) {
	if in == nil {
		return nil, fmt.Errorf("pending-for: %w", ErrNilInput)
	}

	if in.Audience == "" {
		return nil, fmt.Errorf("pending-for: %w", ErrNoAudience)
	}

	result := make([]Window, 0, len(l.windows))
	for _, w := range l.windows {
		if w.Audience == in.Audience {
			result = append(result, l.copyWindow(w))
		}
	}
	return result, nil
}

// Open returns all open windows in pose order, with deep-copied options and payload.
func (l *Ledger) Open() ([]Window, error) {
	result := make([]Window, 0, len(l.windows))
	for _, w := range l.windows {
		result = append(result, l.copyWindow(w))
	}
	return result, nil
}

// copyWindow returns a deep copy of a window with independent options and payload.
func (l *Ledger) copyWindow(w Window) Window {
	optionsCopy := make([]Option, len(w.Options))
	copy(optionsCopy, w.Options)

	var payloadCopy []byte
	if w.Payload != nil {
		payloadCopy = make([]byte, len(w.Payload))
		copy(payloadCopy, w.Payload)
	}

	return Window{
		ID:       w.ID,
		Audience: w.Audience,
		Options:  optionsCopy,
		Payload:  payloadCopy,
		At:       w.At,
	}
}
