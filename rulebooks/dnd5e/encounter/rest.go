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
// THE HOUR IS A JUMP, NOT A DRIVE ([Encounter.elapseWorld]): the reading
// moves by the hour, no round of it is driven, no creature acts during it,
// no beat but the rest's own is written, and nothing is owed to the next
// step. Nothing else in the run moves — no pace, no turn, no area, no
// creature. A rest somebody interrupts is deferred (owner unset).
//
// DURATIONS ARE NOT RUN BY THE JUMP. An effect that lasts minutes, or until
// a rest, is ended by the rulebook's rest entry, not by this clock moving.

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

	// HitPointsRestored is the hit points the rest healed, after the cap at
	// maximum — so it can be less than the calculation's total. Negative is
	// refused (ErrInvalidData). CARRIED, NEVER COMPARED.
	HitPointsRestored int

	// HitPoints is the character's hit points after the rest, so a reader
	// shows the rest landing without re-reading the sheet. Negative is
	// refused (ErrInvalidData).
	HitPoints int

	// HitDiceSpent is how many hit dice the rest spent. Negative is refused
	// (ErrInvalidData).
	HitDiceSpent int

	// HitDiceReturned is how many hit dice the rest gave back — a long
	// rest's, so zero for every rest this module records today. Negative is
	// refused (ErrInvalidData).
	HitDiceReturned int

	// HitDiceRemaining is the hit dice the character has left to spend after
	// the rest. Negative is refused (ErrInvalidData).
	HitDiceRemaining int

	// Calculation is the sourced arithmetic of the hit dice the rest spent:
	// every die with the resting character as its source, and the
	// Constitution modifier each adds. Required when HitDiceSpent is above
	// zero and refused when it is zero — dice that were never thrown are not
	// something anybody saw. Validated as a structure
	// ([ValidateRollCalculation]), never as a rule.
	Calculation *RollCalculation

	// ResourcesRefilled is every resource the rest refilled, by its full ref
	// string ("dnd5e:features:second_wind"), in the rulebook's order.
	// CARRIED, NEVER READ: a resource is listed because its own reset kind
	// said this rest refills it, and that is the rulebook's. An empty entry
	// is refused (ErrInvalidData).
	ResourcesRefilled []string
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

	// IntelDeltas is each observer's intel movement from the refresh after
	// the hour, the same shape [RecordEquipOutput] carries.
	IntelDeltas map[MemberID]*IntelDelta

	// Formed is a fight the refresh after the hour started. Nobody moves
	// during a rest, so none is expected; reported rather than dropped if
	// one ever is.
	Formed *FormedBubble
}

// RecordRest records one rest: a `rested` beat told to the members who see
// the rester, stamped with the reading the rest began at, then the rest's
// duration jumped on the world clock ([Encounter.elapseWorld]) and one sight
// refresh, whose results are returned.
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
// ErrInBubble, ErrBadPlacement, or a clock or sight-refresh failure (drop the
// encounter unsaved — doc.go's caller rule).
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
		"hit_points_restored": in.HitPointsRestored,
		"hit_points":          in.HitPoints,
		"hit_dice_spent":      in.HitDiceSpent,
		"hit_dice_returned":   in.HitDiceReturned,
		"hit_dice_remaining":  in.HitDiceRemaining,
	}
	// ABSENT STAYS ABSENT: no dice thrown is no calculation key, and nothing
	// refilled is no list — never a zero-valued stand-in.
	if in.Calculation != nil {
		body["calculation"] = in.Calculation
	}
	if len(in.ResourcesRefilled) > 0 {
		body["resources_refilled"] = in.ResourcesRefilled
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

	if err := e.elapseWorld(rounds); err != nil {
		return nil, fmt.Errorf("record rest: %w", err)
	}

	// THE ONE REFRESH, returned rather than discarded. Nobody moved and
	// nothing was driven, so nothing is expected to form or change — and if
	// a time-keyed fact ever makes it, the caller is told.
	deltas, formed, err := e.refreshSight(e.rosterIDs())
	if err != nil {
		return nil, fmt.Errorf("record rest: %w", err)
	}

	return &RecordRestOutput{
		Seq:         appended.Seq,
		Audience:    witnesses,
		Clock:       uint64(e.clock.ToData().HighWater),
		IntelDeltas: deltas,
		Formed:      formed,
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
	counts := []struct {
		name string
		n    int
	}{
		{"hit points restored", in.HitPointsRestored},
		{"hit points", in.HitPoints},
		{"hit dice spent", in.HitDiceSpent},
		{"hit dice returned", in.HitDiceReturned},
		{"hit dice remaining", in.HitDiceRemaining},
	}
	for _, c := range counts {
		if c.n < 0 {
			return fmt.Errorf("record rest: %s %d: %w", c.name, c.n, ErrInvalidData)
		}
	}
	if in.HitDiceSpent == 0 && in.Calculation != nil {
		return fmt.Errorf("record rest: a calculation with no hit dice spent: %w", ErrInvalidData)
	}
	if in.HitDiceSpent > 0 {
		if err := ValidateRollCalculation(in.Calculation); err != nil {
			return fmt.Errorf("record rest: calculation: %v: %w", err, ErrInvalidData)
		}
	}
	for i, ref := range in.ResourcesRefilled {
		if ref == "" {
			return fmt.Errorf("record rest: resources refilled[%d] is empty: %w", i, ErrInvalidData)
		}
	}

	return nil
}
