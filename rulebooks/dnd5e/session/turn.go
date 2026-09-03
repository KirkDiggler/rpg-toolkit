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

	// Participants is the same members as Order, in the same order, each
	// with what a bare id cannot carry: name, kind, standing, and whether
	// they are the active one. Empty on the world clock. Order stays for
	// the readers it already has — a new client reads this and never joins
	// the two. Lands with rpg-toolkit#1137.
	//
	// NOT A ROSTER READ, and the line matters because this seam refuses one
	// on purpose (see Manager.Where). Participants are the members of the
	// fight the asker is IN — two sides that have, by construction, seen
	// each other — so nothing here leaks a cell or a member the asker has
	// not perceived. There is no position on it for that reason: where a
	// participant stands is View's answer, gated by sight.
	Participants []Participant `json:"participants,omitempty"`
}

// Participant is one member of the fight the asker is in, as TurnOutput
// reports them. Lands with rpg-toolkit#1137.
type Participant struct {
	// Member is the member's id, as it appears in Order.
	Member string `json:"member"`

	// Name is the display name. Never empty for a member the server can
	// name — which is every member it can list.
	Name string `json:"name"`

	// Kind categorises the member.
	Kind MemberKind `json:"kind"`

	// Standing is on their feet or downed.
	//
	// A DOWNED PARTICIPANT DOES NOT LINGER HERE. This mirrors Order exactly
	// (Participants is a projection of it, never a second opinion about who
	// belongs), and the composition splices a body out of Order the moment
	// it notices one (encounter.noticeDown, rpg-toolkit#1078: "a body keeps
	// no turn, and the order closes over the gap rather than holding it
	// open"). Standing is still meaningful here, though: a member reported
	// active or seated in an initiative order this call already trusts can
	// still, in principle, be down for one beat before the next sight
	// refresh notices — Where, View and Story all keep answering about a
	// downed member regardless of whether Order still holds them.
	Standing Standing `json:"standing"`

	// Active is whether it is this member's turn. Exactly one true on the
	// turn clock — the same member TurnOutput.Active names — so a caller
	// marks the active row without a lookup.
	Active bool `json:"active"`

	// LifeState is the root rulebook's explicit provider-derived state.
	// Consumers do not infer it from Standing or optional DeathSaves.
	LifeState LifeState `json:"life_state"`

	// DeathSaves is provider-owned progress, present for character life states
	// whose provider reports it and absent for conscious characters/monsters.
	DeathSaves *DeathSaveProgress `json:"death_saves,omitempty"`
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

	data, err := m.loadSessionData(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("turn: %w", err)
	}
	enc, err := m.loadWorld(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("turn: %w", err)
	}

	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("turn: %w", translate(err))
	}

	participants, err := m.participantsFor(ctx, data, enc, clock)
	if err != nil {
		return nil, fmt.Errorf("turn: %w", err)
	}

	out := projectTurn(clock)
	out.Participants = participants
	return out, nil
}

// participantsFor builds the Participant list a TurnOutput reports: the
// same members Order names, each with a display name, kind, standing, and
// whether they are the active one — what a bare id on Order cannot carry
// (rpg-toolkit#1137). Nil on the world clock, where Order is empty too.
func (m *Manager) participantsFor(
	ctx context.Context, data *SessionData, enc *encounter.Encounter, clock *encounter.ClockOfOutput,
) ([]Participant, error) {
	if len(clock.Order) == 0 {
		return nil, nil
	}

	roster, err := enc.Members()
	if err != nil {
		return nil, translate(err)
	}
	names := rosterNames(roster)
	kinds := make(map[string]encounter.MemberKind, len(roster))
	for _, member := range roster {
		kinds[string(member.ID)] = member.Kind
	}

	// One provider-derived snapshot supplies both the compatibility Standing
	// field and the explicit rich state/progress projection.
	participation, err := richParticipationSet(m.standingFor(ctx, data), clock.Order)
	if err != nil {
		return nil, err
	}

	out := make([]Participant, 0, len(clock.Order))
	for _, id := range clock.Order {
		key := string(id)
		st := StandingUp
		if participation.members[key].Down {
			st = StandingDowned
		}
		view := participation.views[key]
		out = append(out, Participant{
			Member: key, Name: names[key], Kind: MemberKind(kinds[key]),
			Standing: st, Active: key == string(clock.Active),
			LifeState: view.LifeState, DeathSaves: view.DeathSaves,
		})
	}
	return out, nil
}

// EndTurnInput ends one member's turn in the fight they are in.
type EndTurnInput struct {
	// Session is the session to act in.
	Session string

	// Member is whose turn is ending. Required — and it must be THEIR turn.
	Member string

	// DeclarationID is the opaque current EndTurn selector returned by Afford.
	// Required. The client echoes it and never parses it.
	DeclarationID string
}

// EndTurnOutput reports what ending the turn produced.
type EndTurnOutput struct {
	// Next is who acts now.
	Next string `json:"next"`

	// RoundWrapped is whether that was the last turn of the round, so the
	// order has come back around.
	RoundWrapped bool `json:"round_wrapped"`

	// Seq is the turn-ended beat's sequence IN THE ACTOR'S OWN delivered
	// numbering (stream.go) — the same number their event for it carries.
	Seq uint64 `json:"seq"`

	// Corrected reports location-belief corrections made by driven turns.
	Corrected []IntelCorrection `json:"corrected,omitempty"`

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
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoDeclarationID,
// ErrNoSession, ErrNoEncounter, ErrNoMember, ErrNotInFight, ErrNotYourTurn,
// ErrStaleDeclaration, ErrClosed, or ErrSaveFailed with a populated report.
// Ending a turn that is not yours is
// refused by the clock itself and translated to ErrNotYourTurn — the same
// sentinel Move's own turn gate produces (rpg-toolkit#1169) — rather than
// left unnamed; see translate.
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

	// EndTurn's real gate is the clock alone. Keep it ahead of selection so a
	// world-clock member and a member whose turn it is not retain the specific
	// refusals this verb has always promised; unlike Attack and Move, no sheet,
	// standing capability, or economy is consulted to compile this selector.
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("endturn: %w", translate(err))
	}
	if ClockKind(clock.Kind) != ClockTurn {
		return nil, fmt.Errorf("endturn: %w", ErrNotInFight)
	}
	if string(clock.Active) != in.Member {
		return nil, fmt.Errorf("endturn: %w", ErrNotYourTurn)
	}
	current, err := m.buildEndTurnOffer(scope.session, in.Member)
	if err != nil {
		return nil, fmt.Errorf("endturn: %w", err)
	}
	if _, err := selectCompiledOffer([]compiledOffer{current}, VerbEndTurn, in.DeclarationID); err != nil {
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
		Seq:          scope.deliveredSeq(in.Member, ended.Seq),
		Corrected:    projectIntelCorrections(ended.IntelDeltas),
		Saved:        report,
		Delivery:     delivery,
	}, nil
}

// projectTurn turns the composition's clock report into the SDK's own shape.
//
// There was a Yours bool here — "is it this member's turn" — and it came out
// because it could not be told apart from the comparison it wrapped. Active is
// empty on the world clock and Member is validated non-empty, so
// `Active == member` gives the same answer everywhere reachable, and the field
// doc's claim that a client would get that subtly wrong was refuted by this
// function.
//
// The question that WOULD have earned a field is a different one — "may I act
// now", which is true in free roam — and it belongs to the action economy
// rather than to a projection. Answering it with a bool here would be
// pre-empting the package that will own it.
func projectTurn(in *encounter.ClockOfOutput) *TurnOutput {
	out := &TurnOutput{
		Clock:  ClockKind(in.Kind),
		Active: string(in.Active),
		Round:  in.Round,
	}
	for _, id := range in.Order {
		out.Order = append(out.Order, string(id))
	}
	return out
}
