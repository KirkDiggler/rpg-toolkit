// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "fmt"

// Config carries the ports a Manager needs. Construct a Manager with
// NewManager; the zero Manager is unusable.
//
// Required ports are those without which no verb can run. Optional ports are
// capabilities: absent, the corresponding behaviour simply does not happen,
// and nothing else degrades.
//
// Ports are added over time as waves need them (a character repository arrives
// with entities, for example). That direction is deliberate and safe: adding a
// field to this struct is a compatible change a host adopts when it wants the
// capability, whereas removing one it has already implemented is not. Ports
// are therefore introduced when something calls them, never in anticipation.
type Config struct {
	// Sessions persists session state. Required.
	Sessions SessionRepository

	// Encounters persists the world. Required.
	Encounters EncounterRepository

	// Events delivers per-recipient events for multiplayer fan-out. Optional:
	// without it, no events are published and every verb behaves identically
	// in every other respect.
	Events EventStream
}

// Manager is the host's single point of contact with the toolkit.
//
// It holds ports and nothing else (S1). Every verb loads what it needs, acts,
// saves, and drops everything, so a Manager is safe to construct once at
// process start, share across goroutines that do not share a verb call, and
// keep for the life of the process. Nothing about a session is cached between
// calls, which is what allows several servers to serve the same session with
// no coordination.
type Manager struct {
	sessions   SessionRepository
	encounters EncounterRepository
	events     EventStream
}

// NewManager returns a Manager wired to the given ports.
//
// Construction is total (S8): every required port is checked here, and a
// missing one is named in the error. The alternative — discovering a nil port
// at call time — turns a wiring mistake into a panic in the middle of a
// player's turn, in production, instead of a startup failure a deployment can
// catch.
//
// Returns ErrNilConfig for a nil config, and ErrMissingPort naming the first
// absent required port.
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("newmanager: %w", ErrNilConfig)
	}

	// Checked in a fixed order so the reported name is deterministic: a host
	// wiring several ports at once fixes them one predictable step at a time
	// rather than watching the message change between runs.
	required := []struct {
		name    string
		present bool
	}{
		{"Sessions", cfg.Sessions != nil},
		{"Encounters", cfg.Encounters != nil},
	}
	for _, port := range required {
		if !port.present {
			return nil, fmt.Errorf("newmanager: %s: %w", port.name, ErrMissingPort)
		}
	}

	return &Manager{
		sessions:   cfg.Sessions,
		encounters: cfg.Encounters,
		events:     cfg.Events,
	}, nil
}
