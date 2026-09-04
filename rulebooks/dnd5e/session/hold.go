// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// hold.go is the Hold verb at the seam (rpg-toolkit#1496; ruled on
// rpg-project#368, design §4.3 and R10): member and prop id in, the
// composition's Hold does the rest. loot.go's shape, one file over, sharing
// its reach rule and its turn rule.
//
// # Hold, never Take (R10)
//
// "Hold means something — it is a fact." A holding is run-scoped state about a
// pair of hands and writes nothing to a character sheet. TAKE IS RESERVED for
// the act that lands a thing in inventory — buying from a merchant, or
// carrying the artifact home — so no verb, no beat and no sentinel in this
// package says "took" for a thing that is only held.
//
// # The probe law reaches the seam intact
//
// For a prop standing in space the member is not shown, the composition
// answers EVERY refusal with a bare ErrNoProp — no such prop, not holdable,
// already held, out of range — so a guessed id cannot map a room nobody has
// found. That survives translate for a structural reason rather than a
// careful one: a translated error carries OUR sentinel alone, with no inner
// text and no wrapped chain, so all four arrive at a caller as the same
// bytes. A prop the member can see refuses by name; there is no secret in a
// pillar.

// HoldInput declares picking up one holdable prop.
type HoldInput struct {
	// Session is the session to act in.
	Session string

	// Member is who picks it up.
	//
	// THE HOST MUST BIND Member TO THE AUTHENTICATED CALLER, as every verb's
	// acting-as gate requires.
	Member string

	// Target is the prop's placement id — the dungeon file's `place[].id`,
	// the same id [AtlasProp.ID] carries.
	//
	// NOT A REF: a dungeon may place two reliquaries, and a verb that named
	// the ref could not say which one. A prop with no authored id cannot be
	// held, which is the author's decision rather than this seam's.
	Target string

	// Range is the maximum distance, in cells, the prop may stand from
	// Member. Zero (the default) means adjacent, as LootInput.Range does, and
	// it is measured from where the prop IS — a dropped prop is picked up
	// from where it now lies, not from where the author drew it.
	Range int
}

// HoldOutput acknowledges that the hold happened, and deliberately nothing
// else — [LootOutput]'s shape and for its reason.
type HoldOutput struct {
	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// Hold picks up a holdable prop: it leaves the map for everyone, and the
// holder has it.
//
// The prop's disappearance is not a mutation of the field. Where a thing
// physically is folds on the TRUTH GRAIN — one answer, the same for every
// member, unlike knowledge, which is audience-scoped — so the composition
// drops it from the atlas itself and every member's Atlas read inherits that.
// A client patches its cached map from the EventHeld beat, which goes to
// everyone present, and a refetch agrees with the patch.
//
// In a fight the composition gates this on the turn clock (design §4.4): a
// member holds on their own turn and not otherwise, which arrives here as
// ErrNotYourTurn. Out of combat it is free.
//
// Returns ErrNilInput, ErrNoMemberID, ErrNoProp (an id this dungeon does not
// have — or ANY refusal about a prop the member cannot see; see the probe law
// at the top of this file), ErrNoSessionID, ErrNoSession, ErrNoEncounter,
// ErrClosed, ErrNoMember, ErrNotHoldable, ErrAlreadyHeld, ErrNotYourTurn,
// ErrOutOfRange, ErrBadPosition, or ErrSaveFailed with a populated report.
func (m *Manager) Hold(ctx context.Context, in *HoldInput) (*HoldOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("hold: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("hold: %w", ErrNoMemberID)
	}
	// An EMPTY target is refused as ErrNoProp rather than as a member-id
	// mistake: the two arguments name different kinds of thing, and the
	// composition answers an empty prop id the same way, so the two layers
	// agree about what "" is.
	if in.Target == "" {
		return nil, fmt.Errorf("hold: %w", ErrNoProp)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("hold: %w", err)
	}

	if _, err := scope.enc.Hold(&encounter.HoldInput{
		Member: encounter.MemberID(in.Member),
		Target: encounter.PropID(in.Target),
		Range:  in.Range,
	}); err != nil {
		return nil, fmt.Errorf("hold: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("hold: %w", err)
	}

	return &HoldOutput{Saved: report, Delivery: delivery}, nil
}
