// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

// instantiate builds catalog content into a new member's sheet.
//
// This is the other half of how an entity enters a session, and the split is
// between where the data comes from rather than between players and monsters.
// A character is LOADED: it already exists, the host owns it, and only the
// host's repository can produce it — which is exactly why a character has no
// ref. A monster is INSTANTIATED: it exists in code, and a ref names the code
// that builds it.
//
// That distinction is the one that survives. "Player or monster" does not: a
// durable NPC would be a monster you load, and homebrew content can be either.
//
// The ref routes on (Module, Type), which is what a ref is for — it says which
// package can produce this data. Today one route exists. A build that wants
// homebrew:monsters registers a loader for it; until then, saying "no loader"
// is honest, where guessing would not be.
//
// The ID is separate from the ref because a template cannot carry identity:
// one skeleton entry makes five skeletons, and each needs its own name in the
// encounter.
func instantiate(id string, ref string) (*monster.Data, error) {
	if ref == "" {
		return nil, ErrNoRef
	}
	parsed, err := core.ParseString(ref)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", ref, ErrBadRef)
	}
	if parsed.Module != refs.Module || parsed.Type != refs.TypeMonsters {
		return nil, fmt.Errorf("%q: %w", ref, ErrNoLoader)
	}

	// Look up by the PARSED ref's canonical string rather than the caller's
	// bytes, so formatting differences cannot turn a known monster into an
	// unknown one.
	build, ok := monsters.ByRef(parsed.String())
	if !ok {
		return nil, fmt.Errorf("%q: %w", ref, ErrUnknownContent)
	}

	built := build(id)
	if built == nil {
		return nil, fmt.Errorf("%q: %w", ref, ErrUnknownContent)
	}
	return built.ToData(), nil
}

// projectMonster reports the state of an instantiated NPC.
//
// Read from the data rather than from a live monster, because Spawn already
// holds the data — it is what gets stored. That is the opposite of
// projectCharacter's rule, and for the opposite reason: there, serialising to
// read was the expensive path; here the serialisation has already happened and
// re-hydrating a monster to ask it questions would be the wasteful one.
//
// Only the walking speed is reported. Fly, swim, climb and burrow exist on the
// stored sheet, and a client that needs them is asking a movement question this
// projection does not answer — pretending otherwise by summing or maxing them
// would invent a number the rules do not have.
func projectMonster(data *monster.Data) *MonsterState {
	if data == nil {
		return nil
	}
	state := &MonsterState{
		ID:               data.ID,
		Name:             data.Name,
		HitPoints:        data.HitPoints,
		MaxHitPoints:     data.MaxHitPoints,
		ArmorClass:       data.ArmorClass,
		Speed:            data.Speed.Walk,
		ProficiencyBonus: data.ProficiencyBonus,
	}
	if data.Ref != nil {
		state.Ref = data.Ref.String()
	}
	return state
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
