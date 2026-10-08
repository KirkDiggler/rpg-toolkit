// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// rest.go is THE COMPOSITION'S HALF OF A REST (rpg-project#542, "Rest", R5):
// whether the members resting together may rest where they are, the hour it
// takes on the world clock, and who is told.
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
// THE HOUR IS A JUMP, NOT A DRIVE ([Encounter.elapseWorld]), ONE PER REST
// however many members took it: the reading moves by the hour, no round of
// it is driven, no creature acts during it, no beat but the rest's own is
// written, and nothing is owed to the next
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

// RecordRestInput is one rest the party took together, as the rulebook
// reports it: the kind, and what it did for each member who rested.
//
// A REST IS THE PARTY'S ACT (rpg-project#542). The members who rest together
// share one hour, so one call is one jump of the clock however many rested;
// two calls are two hours.
type RecordRestInput struct {
	// Members is everybody who rested together, each with what the rest did
	// for them. Never empty (ErrNoMember), and no member twice
	// (ErrInvalidData). Every one must be a member (ErrNotMember), placed
	// (ErrBadPlacement) and not in a fight (ErrInBubble) — one refusal
	// refuses the whole rest, and nothing is written.
	Members []RestingMember

	// Kind is the kind of rest. Only [RestShort] is recorded; any other,
	// the zero value included, is refused (ErrInvalidData).
	Kind RestKind
}

// RestingMember is one member who rested, and what the rest did for them.
type RestingMember struct {
	// Member is who rested.
	Member MemberID

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
	// refused, and so is any above zero on a short rest (ErrInvalidData).
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

	// ConcentrationBreaks is every concentration the rest ended on this
	// member — a spell that outlasted nothing an hour long — each in the
	// shape every other break takes ([ConcentrationBreak]), with the
	// conditions it was holding as its Removed. Carried on the member's own
	// rested beat (`concentration_ended`), not as beats of their own, so the
	// story tells what the rest ended with the rest. Each must name this
	// member as its Caster, and none may carry a Save: a rest asks for no
	// check (ErrInvalidData). Otherwise validated exactly as a break
	// recorded with a cast or a strike is.
	ConcentrationBreaks []ConcentrationBreak

	// Ended is every condition or effect the rest took off this member —
	// prone, dodging, blessed — in the rulebook's order, exactly as the
	// rulebook's rest returned them, so nothing is converted on the way to
	// the beat. Carried on the rested beat (`ended`) in the activation-result
	// shape every other removal is told in. Each must be a
	// [ResultConditionRemoved] and is validated as every removal is
	// (ErrInvalidData, ErrNotMember).
	Ended []ActivationResult
}

// RestedMember is where one member's rested beat landed.
type RestedMember struct {
	// Member is who rested.
	Member MemberID

	// Seq is the sequence of their rested beat.
	Seq uint64

	// Audience is every member told their beat: the member and every member
	// whose sight reaches their cell ([Encounter.Witnesses]). Sorted.
	Audience []MemberID
}

// RecordRestOutput reports the beats, the clock and the one refresh.
type RecordRestOutput struct {
	// Rested is one entry per member who rested, in the order given.
	Rested []RestedMember

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

// RecordRest records one rest the party took together: a `rested` beat per
// member, each told to that member's own witnesses and stamped with the
// reading the rest began at, then ONE jump of the rest's duration on the
// world clock ([Encounter.elapseWorld]) and one sight refresh, whose results
// are returned.
//
// THE SHEETS MUST ALREADY CARRY THE REST. This module reads no sheet; it
// records what the caller says the rest did.
//
// EVERY MEMBER IS CHECKED BEFORE ANYTHING IS WRITTEN. A member in a fight
// cannot rest — a fight prices its own time by the round, and an hour inside
// one is not a thing the turn clock can mean — so one member in a fight
// refuses the whole rest with [ErrInBubble], and so does any other refusal.
//
// Validation order (R5): nil input → closed → kind → empty list → per
// member, in order: empty id, repeated, not a member, what the rest restored,
// in a fight, not placed.
//
// Errors: ErrNilInput, ErrClosed, ErrInvalidData, ErrNoMember, ErrNotMember,
// ErrInBubble, ErrBadPlacement, or a clock or sight-refresh failure (drop the
// encounter unsaved — doc.go's caller rule).
func (e *Encounter) RecordRest(in *RecordRestInput) (*RecordRestOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("record rest: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record rest: %w", ErrClosed)
	}
	rounds, err := restRounds(in.Kind)
	if err != nil {
		return nil, err
	}
	if len(in.Members) == 0 {
		return nil, fmt.Errorf("record rest: nobody rested: %w", ErrNoMember)
	}

	type prepared struct {
		member    MemberID
		witnesses []MemberID
		payload   []byte
	}
	beats := make([]prepared, 0, len(in.Members))
	seen := make(map[MemberID]bool, len(in.Members))
	for i := range in.Members {
		m := &in.Members[i]
		if m.Member == "" {
			return nil, fmt.Errorf("record rest: members[%d]: %w", i, ErrNoMember)
		}
		if seen[m.Member] {
			return nil, fmt.Errorf("record rest: member %q rests twice: %w", m.Member, ErrInvalidData)
		}
		seen[m.Member] = true
		if _, ok := e.members[m.Member]; !ok {
			return nil, fmt.Errorf("record rest: member %q: %w", m.Member, ErrNotMember)
		}
		if err := validateRestRestored(in.Kind, m); err != nil {
			return nil, err
		}
		bubble, err := e.bubbleFor(m.Member)
		if err != nil {
			return nil, fmt.Errorf("record rest: %w", err)
		}
		if bubble != nil {
			return nil, fmt.Errorf("record rest: member %q: %w", m.Member, ErrInBubble)
		}
		_, witnesses, err := e.audienceOf(m.Member)
		if err != nil {
			return nil, fmt.Errorf("record rest: %w", err)
		}

		body := map[string]interface{}{
			"beat":                BeatRested,
			"member":              string(m.Member),
			"kind":                string(in.Kind),
			"hit_points_restored": m.HitPointsRestored,
			"hit_points":          m.HitPoints,
			"hit_dice_spent":      m.HitDiceSpent,
			"hit_dice_returned":   m.HitDiceReturned,
			"hit_dice_remaining":  m.HitDiceRemaining,
		}
		// ABSENT STAYS ABSENT: no dice thrown is no calculation key, and
		// nothing refilled is no list — never a zero-valued stand-in.
		if m.Calculation != nil {
			body["calculation"] = m.Calculation
		}
		if len(m.ResourcesRefilled) > 0 {
			body["resources_refilled"] = m.ResourcesRefilled
		}
		// WHAT THE REST ENDED, on the same beat and omitted when nothing did.
		ended, err := e.restEnded(m)
		if err != nil {
			return nil, err
		}
		if len(ended.concentration) > 0 {
			body["concentration_ended"] = ended.concentration
		}
		if len(ended.ended) > 0 {
			body["ended"] = ended.ended
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("record rest: marshal beat: %w", err)
		}
		beats = append(beats, prepared{member: m.Member, witnesses: witnesses, payload: payload})
	}

	// STAMPED WITH THE READING THE REST BEGAN AT — the stamp is the cause's
	// ([Encounter.landDeedAt]) — and the hour passes after they are told.
	at := uint64(e.clock.ToData().HighWater)
	rested := make([]RestedMember, 0, len(beats))
	for _, b := range beats {
		appended, err := e.appendBeat(&record.AppendInput{
			At:       at,
			Audience: b.witnesses,
			Tags:     map[string]string{"tag": "rest"},
			Payload:  b.payload,
		})
		if err != nil {
			return nil, fmt.Errorf("record rest: append beat: %w", err)
		}
		rested = append(rested, RestedMember{Member: b.member, Seq: appended.Seq, Audience: b.witnesses})
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
		Rested:      rested,
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
func validateRestRestored(kind RestKind, in *RestingMember) error {
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
			return fmt.Errorf("record rest: member %q: %s %d: %w", in.Member, c.name, c.n, ErrInvalidData)
		}
	}
	// A SHORT REST RETURNS NO HIT DICE — returning them is a long rest's —
	// so a short rest reporting some is a rest that cannot have happened as
	// told.
	if kind == RestShort && in.HitDiceReturned > 0 {
		return fmt.Errorf("record rest: member %q: a short rest returned %d hit dice: %w", in.Member, in.HitDiceReturned, ErrInvalidData)
	}
	if in.HitDiceSpent == 0 && in.Calculation != nil {
		return fmt.Errorf("record rest: member %q: a calculation with no hit dice spent: %w", in.Member, ErrInvalidData)
	}
	if in.HitDiceSpent > 0 {
		if err := ValidateRollCalculation(in.Calculation); err != nil {
			return fmt.Errorf("record rest: member %q: calculation: %v: %w", in.Member, err, ErrInvalidData)
		}
	}
	for i, ref := range in.ResourcesRefilled {
		if ref == "" {
			return fmt.Errorf("record rest: member %q: resources refilled[%d] is empty: %w", in.Member, i, ErrInvalidData)
		}
	}

	return nil
}

// restEndedPayload is what one member's rest ended, ready for the rested
// beat: each concentration with the conditions it was holding, and every
// other condition or effect that came off, in the activation-result shape.
type restEndedPayload struct {
	concentration []restConcentrationPayload
	ended         []interface{}
}

// restConcentrationPayload is one concentration a rest ended, on the rested
// beat: the caster, the spell, why, and the conditions it took off the board
// in the activation-result shape every other removal is told in.
type restConcentrationPayload struct {
	Caster  MemberID             `json:"caster"`
	Spell   spellIdentityPayload `json:"spell"`
	Reason  string               `json:"reason"`
	Removed []interface{}        `json:"removed,omitempty"`
}

// restEnded validates and shapes what a rest ended on one member. A break is
// held to every rule [Encounter.prepareConcentrationBreaks] makes, plus the
// two only a rest has: the caster is the resting member, and there is no
// save.
func (e *Encounter) restEnded(m *RestingMember) (restEndedPayload, error) {
	var out restEndedPayload
	for i, broken := range m.ConcentrationBreaks {
		if broken.Caster != m.Member {
			return out, fmt.Errorf("record rest: member %q: concentration break %d names caster %q: %w",
				m.Member, i, broken.Caster, ErrInvalidData)
		}
		if broken.Save != nil {
			return out, fmt.Errorf("record rest: member %q: concentration break %d carries a save, and a rest asks for none: %w",
				m.Member, i, ErrInvalidData)
		}
	}
	if _, err := e.prepareConcentrationBreaks("record rest", m.Member, m.ConcentrationBreaks); err != nil {
		return out, err
	}
	for i, broken := range m.ConcentrationBreaks {
		c := restConcentrationPayload{
			Caster: broken.Caster,
			Spell:  spellIdentityPayload{Ref: broken.Spell.Ref, Name: broken.Spell.Name},
			Reason: broken.Reason,
		}
		for j, removed := range broken.Removed {
			payload, err := e.prepareActivationResult(fmt.Sprintf("record rest: concentration break %d", i), j, removed)
			if err != nil {
				return out, err
			}
			c.Removed = append(c.Removed, payload)
		}
		out.concentration = append(out.concentration, c)
	}
	for i, removed := range m.Ended {
		if removed.Kind != ResultConditionRemoved {
			return out, fmt.Errorf("record rest: member %q: ended[%d] kind %q: %w", m.Member, i, removed.Kind, ErrInvalidData)
		}
		payload, err := e.prepareActivationResult(fmt.Sprintf("record rest: member %q: ended", m.Member), i, removed)
		if err != nil {
			return out, err
		}
		out.ended = append(out.ended, payload)
	}

	return out, nil
}
