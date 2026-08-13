// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "fmt"

// Config carries what a Manager needs from the host. Construct a Manager with
// NewManager; the zero Manager is unusable.
//
// Required fields are those without which no verb can run. Optional ones are
// capabilities: absent, the corresponding behaviour simply does not happen and
// nothing else degrades.
//
// Fields are added over time as waves need them (a character repository arrives
// with entities, for example). That direction is deliberate and safe: adding a
// field here is a compatible change a host adopts when it wants the capability,
// whereas removing one it has already implemented is not. They are therefore
// introduced when something calls them, never in anticipation.
type Config struct {
	// Sessions persists session state. Required.
	Sessions SessionRepository

	// Encounters persists the world. Required.
	Encounters EncounterRepository

	// Events is the live channel to connected clients. Required.
	//
	// Required because a verb's response tells the CALLER about the caller's
	// own action, and that is a small fraction of what a client must render.
	// Monsters acting, the clock advancing, another player crossing a doorway —
	// none of it appears in anyone's return value. A host without a stream is
	// reduced to polling Story, which is a recovery path rather than a design.
	//
	// This is not about multiplayer. A single-player game needs it for exactly
	// the same reason: things happen that the player did not ask for.
	//
	// Use DiscardEvents for tests, headless simulation, or any run that
	// genuinely wants no delivery — an explicit opt-out that reads as a
	// decision, rather than a nil that reads as an oversight.
	Events EventStream
}

// Manager is the host's single point of contact with the toolkit.
//
// It holds the host's repositories and stream, and nothing else (S1). Every
// verb loads what it needs, acts, saves, and drops everything, so a Manager is safe to construct once at
// process start, share across goroutines that do not share a verb call, and
// keep for the life of the process. Nothing about a session is cached between
// calls, which is what allows several servers to serve the same session with
// no coordination.
type Manager struct {
	sessions   SessionRepository
	encounters EncounterRepository
	events     EventStream
}

// NewManager returns a Manager wired to what the host supplied.
//
// Construction is total (S8): every required field is checked here, and a
// missing one is named in the error. The alternative — discovering a nil
// dependency at call time — turns a wiring mistake into a panic in the middle
// of a player's turn, in production, instead of a startup failure a deployment
// can catch.
//
// Returns ErrNilConfig for a nil config, and ErrIncompleteConfig naming the
// first absent required field.
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("newmanager: %w", ErrNilConfig)
	}

	// Checked in a fixed order so the reported name is deterministic: a host
	// wiring several at once fixes them one predictable step at a time rather
	// than watching the message change between runs.
	required := []struct {
		name    string
		present bool
	}{
		{"Sessions", cfg.Sessions != nil},
		{"Encounters", cfg.Encounters != nil},
		{"Events", cfg.Events != nil},
	}
	for _, dep := range required {
		if !dep.present {
			return nil, fmt.Errorf("newmanager: %s: %w", dep.name, ErrIncompleteConfig)
		}
	}

	return &Manager{
		sessions:   cfg.Sessions,
		encounters: cfg.Encounters,
		events:     cfg.Events,
	}, nil
}
