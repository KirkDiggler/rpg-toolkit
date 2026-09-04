// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// loot.go is the Loot verb at the seam (rpg-toolkit#1496; ruled on
// rpg-project#368, design §4.2): looter and body in, the composition's Loot
// does the rest. It is search.go's shape, and for search.go's reason —
// everything that keeps a secret secret lives at the rule half, and this file
// adds only what the seam adds to every verb.
//
// # The answer never leaks the question (design P3)
//
// Loot is offered on EVERY downed member. A body with nothing to give
// transfers nothing, and this verb's response is IDENTICAL either way: no
// found list, no transferred count, no flag. What the looter gained reaches
// them the way all world change reaches anybody — as their own
// recipient-scoped EventDoorRevealed, the same beat a successful search
// produces (design P4) — and everyone present hears EventLooted, which names
// looter and body and nothing of what moved.
//
// That is why there is no Seq here and no Found. LootOutput is
// [SearchOutput]'s two host-facing reports and nothing else.

// LootInput declares looting one downed member.
type LootInput struct {
	// Session is the session to act in.
	Session string

	// Member is who loots.
	//
	// THE HOST MUST BIND Member TO THE AUTHENTICATED CALLER — the acting-as
	// gate every verb carries: this package takes IDs, not identities, so a
	// host that wires a client-supplied member through unchecked lets one
	// player loot as another.
	Member string

	// Target is the body: a downed member. Every downed member is a legal
	// target and the refusal for a standing one is ordinary (ErrNotDown) —
	// there is no secret in whether somebody is on the floor.
	Target string

	// Range is the maximum distance, in cells, Target may lie from Member.
	// Zero (the default) means adjacent — one cell, as InteractInput.Range
	// does. RANGE IS THE HOST'S TRUTH: it is supplied here and forwarded
	// untouched, and the negative-range refusal lives at the rule half so
	// this seam holds no second copy of it to drift from the first.
	Range int
}

// LootOutput acknowledges that the loot happened, and deliberately nothing
// about what moved.
//
// Two host-facing reports, exactly [SearchOutput]'s: the composition's own
// LootOutput is ack-only for symmetry with Search (design Q1, closed at wave
// 0), so there is nothing else here to carry and nothing this seam could add
// without inventing a fact. A half-failed save is still an error with a
// report — Saved says what landed — but a body that carried the run's only
// secret and one that carried nothing answer with the same bytes.
type LootOutput struct {
	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// Loot moves everything a downed member holds to the looter, for free.
//
// Load-act-save like every verb, and the act is one call: the composition
// appends the `looted` beat to everyone present, then moves each holding by
// what its kind means — intel is COPIED, through the reveal path search
// already owns, with a DOOR_REVEALED to the looter alone; a prop is MOVED, so
// the looter now has it and the body does not.
//
// There is deliberately no downed gate on the LOOTER and no check on the
// loot. R1 ruled the transfer free, and a rule the seam invented for Loot
// alone would be a rule, which this package does not own. What the
// composition does gate is the turn clock: in a fight, a member loots on
// their own turn and not otherwise (design §4.4), which arrives here as
// ErrNotYourTurn.
//
// Returns ErrNilInput, ErrNoMemberID (empty member or target), ErrNoSessionID,
// ErrNoSession, ErrNoEncounter, ErrClosed, ErrNoMember (looter or body not on
// the roster), ErrNotDown, ErrNotYourTurn, ErrOutOfRange, ErrBadPosition
// (a roster member with no cell — an internal inconsistency rather than an
// ordinary caller mistake), or ErrSaveFailed with a populated report.
func (m *Manager) Loot(ctx context.Context, in *LootInput) (*LootOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("loot: %w", ErrNilInput)
	}
	if in.Member == "" || in.Target == "" {
		return nil, fmt.Errorf("loot: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("loot: %w", err)
	}

	if _, err := scope.enc.Loot(&encounter.LootInput{
		Member: encounter.MemberID(in.Member),
		Target: encounter.MemberID(in.Target),
		Range:  in.Range,
	}); err != nil {
		return nil, fmt.Errorf("loot: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("loot: %w", err)
	}

	return &LootOutput{Saved: report, Delivery: delivery}, nil
}
