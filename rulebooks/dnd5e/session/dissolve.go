// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// DissolveKind names why a fight ended, for the wire form.
type DissolveKind string

// DissolveByDecision is a fight the members chose to leave.
const DissolveByDecision DissolveKind = "decision"

// DissolveByDefeat is a fight one side stopped standing in.
const DissolveByDefeat DissolveKind = "defeat"

// DissolveCause is why a fight ended: a closed set, sealed the way
// [saves.DCSource] is and for the same reason.
//
// A fight can end for reasons of two different kinds, and the difference is
// the whole design. A party DECIDING to disengage is a choice somebody makes,
// so it reaches the toolkit as a verb. Being DEFEATED is a fact the world
// notices, so when the composition can see it, it will end fights the way
// sight already starts them — automatically, with no caller involved
// (rpg-toolkit#964's mirror).
//
// Those are different events, which is why they are not two mechanisms. When
// defeat becomes visible it arrives as another CALLER of this shape, not as a
// second way to end a fight — and its case is earned then, by the caller that
// forces it, rather than declared now against a future nobody has built.
//
// The unexported method is what makes that structural rather than aspirational:
// a second case cannot be declared outside this package, so adding one means
// editing this file, and editing this file means having the caller in hand.
// A bare string enum would have let any caller invent `DissolveCause("defeat")`
// today and pretend the world noticed something it cannot yet see.
//
// [saves.DCSource]: https://pkg.go.dev/github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves#DCSource
type DissolveCause interface {
	// Kind names which cause this is.
	Kind() DissolveKind

	// isDissolveCause seals the set. See the type's godoc.
	isDissolveCause()
}

// ByDecision is a fight its members chose to walk away from: the party breaks
// off to watch the goblin rather than trade blows with it.
//
// The only cause today, and the only one that is a DECISION. It is a function
// rather than a package-level variable so nothing can reassign what it means
// at runtime, matching how the save gate's sources read at a call site.
func ByDecision() DissolveCause { return byDecision{} }

type byDecision struct{}

func (byDecision) Kind() DissolveKind { return DissolveByDecision }
func (byDecision) isDissolveCause()   {}

// ByDefeat is a fight that ended because a side stopped standing: the last
// skeleton drops and there is nothing left to fight.
//
// THE CASE THIS TYPE WAS WAITING FOR, and it arrived the way its own doc said
// it would — as another CALLER of this shape rather than as a second mechanism.
// Nobody declares it. The composition consults the rulebook about who is
// standing at every sight refresh, and a bubble left with nobody upright on one
// side of it dissolves itself in that pass, with this cause (ruled fork (c) on
// rpg-toolkit#959, built as rpg-toolkit#1078).
//
// It is the translation of the composition's own [encounter.ByDefeat]. Two
// sealed sets rather than one shared type is unavoidable — the composition
// cannot import this package, because this package imports it — so each is
// extended at the layer its caller lives in, and causeOf is the seam between
// them.
//
// Only the BUBBLE ends. The encounter stays open, the bodies stay on the map
// and in the roster, and Exit still carries them out. A defeat is not an
// ending; it is a fight running out of a side.
func ByDefeat() DissolveCause { return byDefeat{} }

type byDefeat struct{}

func (byDefeat) Kind() DissolveKind { return DissolveByDefeat }
func (byDefeat) isDissolveCause()   {}

// DissolveInput ends the fight a member is in.
type DissolveInput struct {
	// Session is the session to act in.
	Session string

	// Member is anyone in the fight. Required.
	//
	// A fight is reached THROUGH a member rather than by name, because it has
	// no name: the composition models a bubble as something you find by asking
	// who is in it, which R6 ("an entity belongs to at most one clock") makes
	// a total question.
	Member string

	// Cause is why it is ending. Required.
	//
	// [ByDecision] is the only cause a caller can honestly declare, because
	// this verb IS the decision. Defeat is something the world notices, and a
	// caller who hands it in here is not believed: the answer reports what the
	// world actually did, and a fight a caller ended is a fight the party chose
	// to leave. Handing in the wrong cause does not fail the call and does not
	// change the outcome — it simply does not survive contact with it.
	Cause DissolveCause
}

// DissolveOutput reports what ending the fight produced.
type DissolveOutput struct {
	// Members are everyone who was in it, now back on the world clock.
	Members []string `json:"members"`

	// Cause is why it ended, read off what the world did rather than off what
	// the caller asked for. See [DissolveInput.Cause].
	Cause DissolveKind `json:"cause"`

	// Seq is the story sequence of the recorded beat.
	Seq uint64 `json:"seq"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// Dissolve ends the fight a member is in and returns everyone to free roam.
//
// Fights start on their own — sight starts them, and nothing asks the caller
// first (rpg-toolkit#964) — and until now nothing could end one. Members were
// refused every free-roam verb with ErrInBubble, correctly and forever
// (rpg-toolkit#1024). This is the half of that a verb can fix: the party
// deciding to disengage.
//
// It is NOT the other half. Defeat ending a fight is a fact the world notices,
// not a decision anyone makes, and it belongs where sight already lives — in
// the composition, automatically. That cannot be built yet because nothing in
// the composition can see defeat: it has no hit points, no damage and no death.
// When it can, it arrives as another caller of [DissolveCause], never as a
// second mechanism.
//
// Time spent fighting is spent, not banked: everyone re-homes to the world
// clock at budget zero. That is the composition's rule and this verb does not
// soften it.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoCause, ErrNoSession,
// ErrNoEncounter, ErrNoMember, ErrNotInFight, ErrClosed, or ErrSaveFailed with
// a populated report.
func (m *Manager) Dissolve(ctx context.Context, in *DissolveInput) (*DissolveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("dissolve: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("dissolve: %w", ErrNoMemberID)
	}
	if in.Cause == nil {
		return nil, fmt.Errorf("dissolve: %w", ErrNoCause)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("dissolve: %w", err)
	}

	dissolved, err := scope.enc.Dissolve(&encounter.DissolveInput{
		Member: encounter.MemberID(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("dissolve: %w", translate(err))
	}

	cause, err := causeOf(dissolved.Cause)
	if err != nil {
		return nil, fmt.Errorf("dissolve: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("dissolve: %w", err)
	}

	members := make([]string, 0, len(dissolved.Members))
	for _, id := range dissolved.Members {
		members = append(members, string(id))
	}

	return &DissolveOutput{
		Members:  members,
		Cause:    cause.Kind(),
		Seq:      dissolved.Seq,
		Saved:    report,
		Delivery: delivery,
	}, nil
}
