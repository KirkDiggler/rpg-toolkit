// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package interrupt

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// LedgerData is the persistent representation of a Ledger.
// It marshals to empty JSON {} when idle (R8 convention).
// NextID is the next ID to assign; Windows is the ordered slice of open windows.
type LedgerData struct {
	NextID  uint64       `json:"next_id,omitempty"`
	Windows []WindowData `json:"windows,omitempty"`
}

// WindowData is the persistent representation of an open Window.
// ID, Audience, and Options are never empty in reachable states (no omitempty).
// Payload is omitempty because nil payloads are legal (reachable by design via Pose).
// At is opaque provenance and omitempty to save space in common idle cases.
type WindowData struct {
	ID       WindowID      `json:"id"`
	Audience core.EntityID `json:"audience"`
	Options  []Option      `json:"options"`
	Payload  []byte        `json:"payload,omitempty"`
	At       uint64        `json:"at,omitempty"`
}

// ToData returns a persistent snapshot of this Ledger.
// Returns zero LedgerData (nil Windows slice, NextID normalized to 0) if Ledger is idle.
// Idle = no windows ever posed (NextID == 1, the initial state).
// All-answered = windows were posed but now closed (NextID > 1, Windows empty);
// NextID persists so IDs are never reused within an encounter.
// All options and payload bytes are deep-copied; mutating the returned LedgerData
// will not affect this Ledger (R8 snapshot immunity).
func (l *Ledger) ToData() LedgerData {
	if len(l.windows) == 0 && l.nextID == 1 {
		// Idle convention: return zero value, marshals to {}
		return LedgerData{}
	}

	// Deep-copy each window's options and payload
	windowsCopy := make([]WindowData, len(l.windows))
	for i, w := range l.windows {
		optionsCopy := make([]Option, len(w.Options))
		copy(optionsCopy, w.Options)

		var payloadCopy []byte
		if w.Payload != nil {
			payloadCopy = make([]byte, len(w.Payload))
			copy(payloadCopy, w.Payload)
		}

		windowsCopy[i] = WindowData{
			ID:       w.ID,
			Audience: w.Audience,
			Options:  optionsCopy,
			Payload:  payloadCopy,
			At:       w.At,
		}
	}

	return LedgerData{
		NextID:  l.nextID,
		Windows: windowsCopy,
	}
}

// LoadLedger reconstructs a Ledger from persistent data.
// Validates every field against R9 in a fixed order before constructing anything (R5).
// Returns (*Ledger, error). On error, returns nil and error wrapping ErrInvalidData.
// Deep-copies all data: mutating the caller's LedgerData after LoadLedger will not
// affect the loaded Ledger (R4).
//
// R9 validations (deterministic order):
// - NextID == 0 with any windows present (IDs are assigned from 1; violation covers wraparound forgery)
// - Any window ID of 0 (IDs start from 1)
// - Window IDs not strictly ascending in slice order (covers duplicates and descending)
// - Any window ID >= NextID (ID must be less than the next ID to assign)
// - Empty audience (required for every window)
// - Nil or empty options slice (required for every window — liveness guard)
// - Any empty option token (required for every option)
// - Any duplicate option token within a window (options are distinct choices)
//
// Nil payload is LEGAL — Pose accepts nil payloads (reachable by design),
// and payload is omitempty, so rejecting would make LoadLedger refuse the module's own snapshots.
// At is never validated (opaque provenance).
func LoadLedger(data LedgerData) (*Ledger, error) {
	// R5: validate everything first, then construct

	// R9: NextID == 0 with windows present
	if data.NextID == 0 && len(data.Windows) > 0 {
		return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
	}

	// R9: per-window validations
	var prevID WindowID
	for i, wd := range data.Windows {
		// R9: window ID of 0
		if wd.ID == 0 {
			return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
		}

		// R9: strictly ascending IDs (covers duplicates and descending)
		if i > 0 && wd.ID <= prevID {
			return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
		}

		// R9: ID >= NextID
		if uint64(wd.ID) >= data.NextID {
			return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
		}

		// R9: empty audience
		if wd.Audience == "" {
			return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
		}

		// R9: nil or empty options
		if len(wd.Options) == 0 {
			return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
		}

		// R9: per-option validations
		seen := make(map[Option]struct{})
		for _, opt := range wd.Options {
			// R9: empty option token
			if opt == "" {
				return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
			}

			// R9: duplicate option
			if _, exists := seen[opt]; exists {
				return nil, fmt.Errorf("load ledger: %w", ErrInvalidData)
			}
			seen[opt] = struct{}{}
		}

		prevID = wd.ID
	}

	// All validated; now reconstruct the Ledger with deep-copied data
	windowsCopy := make([]Window, len(data.Windows))
	for i, wd := range data.Windows {
		optionsCopy := make([]Option, len(wd.Options))
		copy(optionsCopy, wd.Options)

		var payloadCopy []byte
		if wd.Payload != nil {
			payloadCopy = make([]byte, len(wd.Payload))
			copy(payloadCopy, wd.Payload)
		}

		windowsCopy[i] = Window{
			ID:       wd.ID,
			Audience: wd.Audience,
			Options:  optionsCopy,
			Payload:  payloadCopy,
			At:       wd.At,
		}
	}

	// Normalize NextID: idle case (NextID == 0 from marshaled {}) starts at 1
	nextID := data.NextID
	if nextID == 0 {
		nextID = 1
	}

	return &Ledger{
		nextID:  nextID,
		windows: windowsCopy,
	}, nil
}
