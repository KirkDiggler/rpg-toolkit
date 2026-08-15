// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ClockKind names which kind of time a member is living in.
//
// A string enum owned here rather than the composition's, for the reason every
// other enum in this package is: it maps onto a proto enum, and a host must
// never come to name an inner type (S2).
type ClockKind string

const (
	// ClockWorld is free roam. Everyone acts; their own movement is what
	// advances the clock, and there is no turn order to be next in.
	ClockWorld ClockKind = "world"

	// ClockTurn is a fight. The members in it act in an order, one at a time.
	ClockTurn ClockKind = "turn"
)

// TurnInput asks what one member is waiting on.
//
// MEMBER IS REQUIRED, and that is the whole design rather than a validation
// detail. See [Manager.Turn].
type TurnInput struct {
	// Session is the session to look in.
	Session string

	// Member is who is being asked about. Required.
	Member string
}

// TurnOutput is what that member's clock looks like.
type TurnOutput struct {
	// Clock is which kind of time they are in.
	Clock ClockKind `json:"clock"`

	// Active is whose turn it currently is on that clock. Empty on the world
	// clock, which has no turn order.
	Active string `json:"active,omitempty"`

	// Round is which round that clock is in. Zero on the world clock.
	Round int `json:"round,omitempty"`

	// Order is the initiative order of the fight they are in, first to act
	// first. Empty on the world clock.
	Order []string `json:"order,omitempty"`

	// Yours is whether the member asked about is the one to act. A
	// convenience over Active, because it is the question a client actually
	// has and computing it from Active is the sort of thing every client
	// would get subtly wrong on the world clock, where nobody is active and
	// everybody may act.
	Yours bool `json:"yours"`
}

// Turn reports what one member is waiting on.
//
// ASKED OF A MEMBER, NEVER OF THE SESSION. "Whose turn is it?" has no answer
// here and never will: several clocks can run at once — a fight in the crypt
// while the rest of the party explores the hall — and the question only means
// something relative to somebody. A top-level query would have to pick one
// privileged clock to be THE clock, which is the mode model this stack
// deliberately does not have.
//
// So a caller asks "what is Alice waiting on?" and gets an answer that is true
// for Alice. This is charter clause 6 and it is the one shape that is not
// additive later: a convenience field on some other verb answering "the"
// active actor would create the privileged clock by implication, and every
// client written against it would have to be rewritten when a second fight
// starts. [Manager.Status] answers whether the encounter is open and MUST
// NEVER learn anything per-member, which is pinned rather than asked for.
//
// The composition models this correctly already and says so in its own docs;
// this verb is projection, not policy.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, or ErrNoMember.
func (m *Manager) Turn(ctx context.Context, in *TurnInput) (*TurnOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("turn: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("turn: %w", ErrNoMemberID)
	}

	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("turn: %w", err)
	}

	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("turn: %w", translate(err))
	}

	return projectTurn(clock, in.Member), nil
}

// EndTurnInput ends one member's turn in the fight they are in.
type EndTurnInput struct {
	// Session is the session to act in.
	Session string

	// Member is whose turn is ending. Required — and it must be THEIR turn.
	Member string
}

// EndTurnOutput reports what ending the turn produced.
type EndTurnOutput struct {
	// Next is who acts now.
	Next string `json:"next"`

	// RoundWrapped is whether that was the last turn of the round, so the
	// order has come back around.
	RoundWrapped bool `json:"round_wrapped"`

	// Seq is the story sequence of the recorded beat.
	Seq uint64 `json:"seq"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// EndTurn ends a member's turn and hands the fight to whoever is next.
//
// It is a write verb: the clock moves and the story records it, so the world
// is saved and the beat fans out. A member who is not in a fight, or whose
// turn it is not, is refused — the composition owns both of those rules and
// this verb propagates them rather than re-deciding.
//
// There is deliberately no "end the current turn" form. That would be the
// top-level question again wearing a verb's clothes: it could only mean
// anything if one clock were privileged.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember, ErrNotInFight, ErrClosed, or ErrSaveFailed with
// a populated report. Ending a turn that is not yours is refused by the clock
// itself, and that refusal passes through wrapped rather than being flattened
// into a sentinel this package invented — see translate.
func (m *Manager) EndTurn(ctx context.Context, in *EndTurnInput) (*EndTurnOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("endturn: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("endturn: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("endturn: %w", err)
	}

	ended, err := scope.enc.EndTurn(&encounter.EndTurnInput{
		Member: encounter.MemberID(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("endturn: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("endturn: %w", err)
	}

	return &EndTurnOutput{
		Next:         string(ended.Next),
		RoundWrapped: ended.RoundWrapped,
		Seq:          ended.Seq,
		Saved:        report,
		Delivery:     delivery,
	}, nil
}

// projectTurn turns the composition's clock report into the SDK's own shape.
func projectTurn(in *encounter.ClockOfOutput, member string) *TurnOutput {
	out := &TurnOutput{
		Clock:  ClockKind(in.Kind),
		Active: string(in.Active),
		Round:  in.Round,
		Yours:  in.Active != "" && string(in.Active) == member,
	}
	for _, id := range in.Order {
		out.Order = append(out.Order, string(id))
	}
	return out
}
