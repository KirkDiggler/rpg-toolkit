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

	// All-answered: NextID persists, Windows stays nil — symmetric with
	// the zero-value shape a caller writes as LedgerData{NextID: n}.
	if len(l.windows) == 0 {
		return LedgerData{NextID: l.nextID}
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
// R9 validations, in the implementation's deterministic order:
// - A stored NextID of exactly 1 (fresh is 0; the first Pose advances to 2)
// - Then per window, in slice order:
//   - Window ID of 0 (IDs start from 1)
//   - IDs not strictly ascending (covers duplicates and descending)
//   - ID >= NextID — with NextID 0 every window trips this, so it also
//     subsumes NextID-0-with-windows, wraparound forgery included
//   - Empty audience
//   - Nil or empty options slice (liveness guard)
//   - Any empty option token, then any duplicate option token
//
// Nil payload is LEGAL — Pose accepts nil payloads (reachable by design),
// and payload is omitempty, so rejecting would make LoadLedger refuse the module's own snapshots.
// At is never validated (opaque provenance).
func LoadLedger(data LedgerData) (*Ledger, error) {
	// R5: validate everything first, then construct

	// R9: a stored NextID of 1 is unreachable (the record precedent:
	// 0 = fresh, first Pose advances to 2; ToData never emits 1).
	if data.NextID == 1 {
		return nil, fmt.Errorf("load ledger: next_id 1 is unreachable: %w", ErrInvalidData)
	}

	// R9: per-window validations
	var prevID WindowID
	for i, wd := range data.Windows {
		// R9: window ID of 0
		if wd.ID == 0 {
			return nil, fmt.Errorf("load ledger: window[%d]: id 0: %w", i, ErrInvalidData)
		}

		// R9: strictly ascending IDs (covers duplicates and descending)
		if i > 0 && wd.ID <= prevID {
			return nil, fmt.Errorf("load ledger: window[%d]: id not ascending: %w", i, ErrInvalidData)
		}

		// R9: ID >= NextID. With NextID 0 every window ID trips this, so
		// it also subsumes NextID-0-with-windows — including the
		// math.MaxUint64 wraparound forgery — with no dedicated branch.
		if uint64(wd.ID) >= data.NextID {
			return nil, fmt.Errorf("load ledger: window[%d]: id >= next_id: %w", i, ErrInvalidData)
		}

		// R9: empty audience
		if wd.Audience == "" {
			return nil, fmt.Errorf("load ledger: window[%d]: empty audience: %w", i, ErrInvalidData)
		}

		// R9: nil or empty options
		if len(wd.Options) == 0 {
			return nil, fmt.Errorf("load ledger: window[%d]: no options: %w", i, ErrInvalidData)
		}

		// R9: per-option validations
		seen := make(map[Option]struct{})
		for _, opt := range wd.Options {
			// R9: empty option token
			if opt == "" {
				return nil, fmt.Errorf("load ledger: window[%d]: empty option: %w", i, ErrInvalidData)
			}

			// R9: duplicate option
			if _, exists := seen[opt]; exists {
				return nil, fmt.Errorf("load ledger: window[%d]: duplicate option: %w", i, ErrInvalidData)
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
