// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// rest.go is THE COMPOSITION'S HALF OF A REST (rpg-project#542, "Rest", R5):
// whether the member may rest where they are, the hour it takes on the world
// clock, and who is told.
//
// # The rest itself is the rulebook's
//
// Hit dice, hit points and which resources refill are facts on a sheet this
// module cannot read (C1). The session rolls and saves the rest and then
// records it here; the beat CARRIES what it restored and reads none of it.
//
// # The hour is this module's
//
// The world clock is this composition's, so the time a rest takes is
// advanced here and nowhere else: the session asks for a rest by kind and
// never counts rounds (R5). A short rest is [RoundsPerHour] on the clock.
//
// THE RESTER IS THE DRIVER, as every advance in worldtime.go names its
// member. The clock accrues by driver as MAX, so a party resting together —
// each member recording their own rest from the same reading — is one hour
// on the clock, not one hour per member. A member behind the front runner
// raises the reading only by what their own hour passes it.
//
// AND THE WORLD THINKS ON IT, as it does on every raise
// ([Encounter.worldThinks]): a creature that carries orders is owed every
// round the hour granted it, and is given them now rather than handed them
// all at once on the next step somebody takes. Nothing else moves — the
// rester's pace remainder, every fight's turn and every sight area are as
// they were.

// RestKind names the kind of rest a member took.
type RestKind string

const (
	// RestShort is a short rest: an hour on the world clock (PHB p. 186).
	//
	// THE ONLY KIND A RUN RECORDS. The long rest is the first-admission rest
	// a member takes before they are placed, so it is never told here, and a
	// long rest inside a run is deferred until a use case asks for it
	// (rpg-project#542 R13).
	RestShort RestKind = "short"
)

// RecordRestInput is one rest a member took, as the rulebook reports it.
type RecordRestInput struct {
	// Member is who rested. Must be a member (ErrNotMember) and placed
	// (ErrBadPlacement), and not in a fight (ErrInBubble).
	Member MemberID

	// Kind is the kind of rest. Only [RestShort] is recorded; any other,
	// the zero value included, is refused (ErrInvalidData).
	Kind RestKind

	// HitDiceSpent is how many hit dice the rest spent. Negative is refused
	// (ErrInvalidData).
	HitDiceSpent int

	// HitPointsRestored is the hit points the rest restored, after every
	// floor and cap the rulebook applied — so it need not equal the dice's
	// total. Negative is refused (ErrInvalidData). CARRIED, NEVER COMPARED.
	HitPointsRestored int

	// Calculation is the sourced arithmetic of the hit dice the rest spent:
	// every die with the resting character as its source. Required when
	// HitDiceSpent is above zero and refused when it is zero — dice that
	// were never thrown are not something anybody saw. Validated as a
	// structure ([ValidateRollCalculation]), never as a rule.
	Calculation *RollCalculation

	// Refilled is every resource the rest refilled, by its canonical ref
	// string, in the rulebook's order. CARRIED, NEVER READ: what a resource
	// is and why it reset is the rulebook's. An empty entry is refused
	// (ErrInvalidData).
	Refilled []string
}

// RecordRestOutput reports the beat and the clock.
type RecordRestOutput struct {
	// Seq is the sequence of the rested beat.
	Seq uint64

	// Audience is every member told the beat: the resting member and every
	// member whose sight reaches their cell ([Encounter.Witnesses]). Sorted.
	Audience []MemberID

	// Clock is the world clock's reading after the rest.
	Clock uint64
}

// RecordRest records one rest: a `rested` beat told to the members who see
// the rester, stamped with the reading the rest began at, then the rest's
// duration advanced on the world clock with the rester as its driver.
//
// THE SHEET MUST ALREADY CARRY THE REST. This module reads no sheet; it
// records what the caller says the rest did.
//
// A MEMBER IN A FIGHT CANNOT REST, and is refused with [ErrInBubble] before
// anything is written: a fight prices its own time by the round, and an hour
// inside one is not a thing the turn clock can mean.
//
// Validation order (R5): nil input → empty member → closed → not a member →
// kind, counts, calculation or refills that cannot be recorded → in a fight →
// not placed.
//
// Errors: ErrNilInput, ErrNoMember, ErrClosed, ErrNotMember, ErrInvalidData,
// ErrInBubble, ErrBadPlacement, or a failure of the world thinking the hour
// (drop the encounter unsaved — doc.go's caller rule).
func (e *Encounter) RecordRest(in *RecordRestInput) (*RecordRestOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("record rest: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("record rest: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record rest: %w", ErrClosed)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("record rest: member %q: %w", in.Member, ErrNotMember)
	}
	rounds, err := restRounds(in.Kind)
	if err != nil {
		return nil, err
	}
	if err := validateRestRestored(in); err != nil {
		return nil, err
	}

	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("record rest: %w", err)
	}
	if bubble != nil {
		return nil, fmt.Errorf("record rest: member %q: %w", in.Member, ErrInBubble)
	}

	_, witnesses, err := e.audienceOf(in.Member)
	if err != nil {
		return nil, fmt.Errorf("record rest: %w", err)
	}

	body := map[string]interface{}{
		"beat":                BeatRested,
		"member":              string(in.Member),
		"kind":                string(in.Kind),
		"hit_dice_spent":      in.HitDiceSpent,
		"hit_points_restored": in.HitPointsRestored,
	}
	if in.Calculation != nil {
		body["calculation"] = in.Calculation
	}
	if len(in.Refilled) > 0 {
		body["refilled"] = in.Refilled
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("record rest: marshal beat: %w", err)
	}

	// STAMPED WITH THE READING THE REST BEGAN AT — the stamp is the cause's
	// ([Encounter.landDeedAt]) — and the hour passes after it is told.
	appended, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: witnesses,
		Tags:     map[string]string{"tag": "rest"},
		Payload:  payload,
	})
	if err != nil {
		return nil, fmt.Errorf("record rest: append beat: %w", err)
	}

	raised, err := e.advanceWorld(in.Member, rounds)
	if err != nil {
		return nil, fmt.Errorf("record rest: %w", err)
	}
	if raised {
		if err := e.worldThinks(); err != nil {
			return nil, fmt.Errorf("record rest: %w", err)
		}
	}

	return &RecordRestOutput{
		Seq:      appended.Seq,
		Audience: witnesses,
		Clock:    uint64(e.clock.ToData().HighWater),
	}, nil
}

// restRounds is how long a kind of rest takes on the world clock, in rounds.
// Every kind this module records is named here, and anything else is
// refused rather than given a duration nobody ruled.
func restRounds(kind RestKind) (int, error) {
	switch kind {
	case RestShort:
		return RoundsPerHour, nil
	default:
		return 0, fmt.Errorf("record rest: kind %q: %w", kind, ErrInvalidData)
	}
}

// validateRestRestored refuses a report of what a rest restored that cannot
// have happened as told: negative counts, dice spent with no arithmetic or
// arithmetic with no dice spent, arithmetic that does not add up, and a
// refilled resource with no name.
func validateRestRestored(in *RecordRestInput) error {
	if in.HitDiceSpent < 0 {
		return fmt.Errorf("record rest: hit dice spent %d: %w", in.HitDiceSpent, ErrInvalidData)
	}
	if in.HitPointsRestored < 0 {
		return fmt.Errorf("record rest: hit points restored %d: %w", in.HitPointsRestored, ErrInvalidData)
	}
	if in.HitDiceSpent == 0 && in.Calculation != nil {
		return fmt.Errorf("record rest: a calculation with no hit dice spent: %w", ErrInvalidData)
	}
	if in.HitDiceSpent > 0 {
		if err := ValidateRollCalculation(in.Calculation); err != nil {
			return fmt.Errorf("record rest: calculation: %v: %w", err, ErrInvalidData)
		}
	}
	for i, ref := range in.Refilled {
		if ref == "" {
			return fmt.Errorf("record rest: refilled[%d] is empty: %w", i, ErrInvalidData)
		}
	}

	return nil
}
