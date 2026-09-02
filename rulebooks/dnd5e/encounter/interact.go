// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// interact.go is the encounter half of interacting with a placed world NPC
// (rpg-toolkit#1404, design.md's "Interaction Verb"). It answers only what
// this package already knows how to answer for any member — identity,
// adjacency, and visibility — and nothing about what the NPC IS: no ref, no
// capabilities, no policy, no descriptor. That assembly is session's job,
// one layer up, from its own member-ID-keyed NPC store (design.md's Session
// Seam). This file has no NPC content to leak because it never holds any.

// InteractInput names who is reaching for whom, and how far that reach
// extends.
type InteractInput struct {
	// Actor is the player member initiating the interaction.
	Actor MemberID

	// Target is the KindWorld member being interacted with.
	Target MemberID

	// Range is the maximum distance, in cells, Target may stand from Actor.
	// Zero (the default) means adjacent — one cell.
	Range int
}

// InteractOutput confirms who was reached. Session resolves what that
// member IS by looking Target up in its own NPC store — this type carries
// no NPC content because encounter holds none.
type InteractOutput struct {
	// Target echoes the confirmed member ID, so a caller need not trust its
	// own input back.
	Target MemberID

	// Seq is the sequence number of the recorded interaction beat.
	Seq uint64
}

// Interact confirms a player can reach a placed world NPC: both exist, the
// actor is a player, the target is a KindWorld member, they stand within
// range (adjacent by default), and the target is currently visible to the
// actor. It appends a story beat and returns confirmation of who was
// reached — nothing about the NPC's content, nothing about its state, and
// no feature-specific behavior. That is deliberate: this verb has nothing
// else to say.
//
// Validation order (R5): nil input → empty actor/target → closed → actor
// exists and is a player → target exists and is a world NPC → both placed
// → target in range → target visible.
//
// Errors: ErrNilInput, ErrNoMember, ErrClosed, ErrNotMember (actor/target
// missing, or present but the wrong kind), ErrBadPlacement (either member
// has no cell), ErrOutOfRange, ErrNotVisible.
func (e *Encounter) Interact(in *InteractInput) (*InteractOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("interact: %w", ErrNilInput)
	}
	if in.Actor == "" || in.Target == "" {
		return nil, fmt.Errorf("interact: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("interact: %w", ErrClosed)
	}

	actor, ok := e.members[in.Actor]
	if !ok {
		return nil, fmt.Errorf("interact: actor %q: %w", in.Actor, ErrNotMember)
	}
	if actor.Kind != KindPlayer {
		return nil, fmt.Errorf("interact: actor %q is not a player: %w", in.Actor, ErrNotMember)
	}

	target, ok := e.members[in.Target]
	if !ok {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrNotMember)
	}
	if target.Kind != KindWorld {
		return nil, fmt.Errorf("interact: target %q is not a world npc: %w", in.Target, ErrNotMember)
	}

	actorCell, placed := e.canvas.GetEntityPosition(string(in.Actor))
	if !placed {
		return nil, fmt.Errorf("interact: actor %q: %w", in.Actor, ErrBadPlacement)
	}
	targetCell, placed := e.canvas.GetEntityPosition(string(in.Target))
	if !placed {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrBadPlacement)
	}

	maxRange := in.Range
	if maxRange <= 0 {
		maxRange = 1
	}
	if e.Distance(actorCell, targetCell) > float64(maxRange) {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrOutOfRange)
	}

	visible, err := e.currentlyPerceives(in.Actor, in.Target)
	if err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}
	if !visible {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrNotVisible)
	}

	at := uint64(e.clock.ToData().HighWater)
	payload, err := json.Marshal(map[string]interface{}{
		"beat":   "interacted",
		"actor":  string(in.Actor),
		"target": string(in.Target),
	})
	if err != nil {
		return nil, fmt.Errorf("interact: marshal beat: %w", err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		Audience: []MemberID{in.Actor, in.Target},
		Tags:     map[string]string{"tag": "interact"},
		Payload:  payload,
		At:       at,
	})
	if err != nil {
		return nil, fmt.Errorf("interact: append beat: %w", err)
	}

	return &InteractOutput{Target: in.Target, Seq: out.Seq}, nil
}

// currentlyPerceives reports whether observer's CURRENT intel holdings name
// subject — the same "current, not held" rule unawareOfOpposition and
// contactBetween already apply: a subject once seen but not seen now (a
// Held ghost) does not count, the same as one never seen at all.
func (e *Encounter) currentlyPerceives(observer, subject MemberID) (bool, error) {
	holdings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: observer})
	if err != nil {
		return false, fmt.Errorf("held by %q: %w", observer, err)
	}
	for _, h := range holdings {
		if h.Status == intel.Current && MemberID(h.Subject) == subject {
			return true, nil
		}
	}
	return false, nil
}
