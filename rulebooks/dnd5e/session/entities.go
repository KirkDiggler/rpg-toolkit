// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

// Loading an entity, and the cleanup that must not happen.
//
// There is no session process. A verb loads what it needs, attaches it to a bus
// created for that call, acts, writes back, and returns; the whole object graph
// is garbage the moment the response is written. Answer is not the resumption
// of anything living — it is this same load-and-attach performed again from
// persisted data.
//
// That shape is forced rather than chosen. The interrupt spine established that
// no Go stack survives a wait, and a loaded character with live subscriptions
// is exactly that kind of state. So a suspension drops every entity, and
// anything a condition holds that does not survive ToData() is lost across it.
//
// ONE BUS PER CALL, SHARED BY EVERY ENTITY IN THAT CALL. Shared rather than
// per-entity because a condition on one member must be able to observe what
// happens to another — that is the whole reason the bus exists here, and it is
// the prerequisite for reactions. A bus per character would compile, pass any
// test that loaded one character, and quietly make cross-entity observation
// impossible.
//
// CHARACTER.CLEANUP MUST NOT BE CALLED. Its first statement is
// `c.conditions = nil`, and ToData() serializes c.conditions — so cleaning up
// before the save persists a character with ZERO conditions. Raging,
// unconscious, a death save in progress: gone, with no error and no failed
// call. Its other half, unsubscribing, buys nothing when the bus dies with the
// response; Cleanup is built for a long-lived character in a long-lived
// process, which is the architecture we do not have.
//
// Skipping it is safe rather than merely tolerable: conditions intercept on the
// bus rather than mutating character fields, so there is no modification left
// un-reversed when the character is dropped.

// newCallBus returns the event bus for one verb.
//
// A function rather than an inline call so there is exactly one place to look
// when asking what the lifetime is, and so the answer stays "one call" when a
// later wave is tempted to cache one.
func newCallBus() events.EventBus {
	return events.NewEventBus()
}

// loadCharacter reconstitutes a player character and attaches its features and
// conditions to the call's bus.
//
// Returns ErrNoCharacter when the repository does not hold the ID, and
// ErrBadCharacter when it holds bytes that cannot be reconstituted. The two are
// kept apart because they send whoever debugs it to different places: a bad
// request versus corrupt storage.
//
// The returned character is for this call only. It is never stored on the
// manager (S1), never held across a suspension, and never returned to the host
// (S2) — the host named an ID and gets data back, not an object.
func (m *Manager) loadCharacter(
	ctx context.Context, bus events.EventBus, id string,
) (*character.Character, error) {
	data, err := m.characters.GetCharacter(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("character %q: %w", id, ErrNoCharacter)
		}
		return nil, fmt.Errorf("character %q: %w", id, err)
	}
	if data == nil {
		// A repository reporting success with no data has violated its
		// contract. Guessing in either direction — treating it as absent, or
		// carrying a nil into LoadFromData — is worse than saying so.
		return nil, fmt.Errorf("character %q: %w", id, ErrBadRepository)
	}

	ch, err := character.LoadFromData(ctx, data, bus)
	if err != nil {
		return nil, fmt.Errorf("character %q: %w: %w", id, ErrBadCharacter, err)
	}
	return ch, nil
}

// projectCharacter reports the state of a loaded character.
//
// Read through the character's own accessors, never through ToData(). ToData is
// a SERIALISATION, not a getter: it clones several maps, marshals every feature
// and condition to JSON, and stamps UpdatedAt with the current time. Calling it
// to read three integers would put that cost on every join, and would make a
// read path non-deterministic for no reason.
//
// Speed is the field that carries the weight here. It is not stored on the
// character at all — it is derived from race when asked — so it is the one value
// that cannot be produced by echoing bytes, and the one the tests lean on to
// prove reconstitution actually happened. The rest are reported as loaded.
func projectCharacter(ch *character.Character) *CharacterState {
	if ch == nil {
		return nil
	}
	return &CharacterState{
		ID:               ch.GetID(),
		Name:             ch.GetName(),
		Level:            ch.GetLevel(),
		Speed:            ch.GetSpeed(),
		HitPoints:        ch.GetHitPoints(),
		MaxHitPoints:     ch.GetMaxHitPoints(),
		ArmorClass:       ch.AC(),
		ProficiencyBonus: ch.ProficiencyBonus(),
	}
}
