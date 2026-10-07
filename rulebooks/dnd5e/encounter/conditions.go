// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
)

// ConditionSet is what a member was seen holding.
//
// A nil *ConditionSet means there was nothing to observe — no sheet behind the
// member, or testimony from a build that predates this fact. A non-nil set with
// no Conditions is "seen holding none". Those are two claims, for the reason
// [Equipment] gives for hands: collapsing them would let an unobserved member
// read as a member observed to be clean.
type ConditionSet struct {
	// Conditions are the held conditions, each by its [ConditionKey], in the
	// order the rulebook reported them. No key repeats, and none has an empty
	// ConditionRef. A key carries no condition payload or state: what was held
	// and by whose hand is enough for a rule that answers by reference.
	Conditions []ConditionKey
}

// Conditions reports which conditions each of the given members holds. The
// composition asks; the rulebook answers.
//
// It is the third fact of a sighting after position and hands, and every word of
// [Equipment]'s doc applies to it unchanged: the capability answers TRUTH and is
// not told who is looking; the composition snapshots the answer into each
// observer's [SightTestimony] at sight refresh and never reads it live, so a
// condition an observer saw is testimony that can later be wrong; nothing is
// remembered between refreshes, which is what makes every route to a changed
// condition visible at the next refresh without that route knowing this
// interface exists.
//
// No condition is filtered as imperceptible. A sighting carries every condition
// the member holds; a perceivability filter waits for a condition that must be
// secret (rpg-project#520, ruling R16).
//
// Every member asked about must appear in the answer and nobody else may
// (ErrNoConditions, ErrNotMember). A nil value is the answer "nothing to
// observe" and is accepted; see [ConditionSet] for why that is not an empty set.
//
// Errors abort whatever verb was running, atomically (R5), the same as
// [Equipment]'s.
type Conditions interface {
	// Conditions reports what each given member holds. Every member asked
	// about must appear and nobody else may; a nil value says there is
	// nothing to observe.
	Conditions(members []MemberID) (map[MemberID]*ConditionSet, error)
}

// EquipmentWithConditions is the shape the Equipment field of [SetupInput] and
// [LoadEncounterInput] must have: the existing Equipment capability that also
// answers [Conditions].
//
// It rides the Equipment value rather than adding a field for the reason
// [StandingWithParticipation] rides Standing: both answer a sighting fact from
// the same sheets. The Equipment fields of SetupInput and LoadEncounterInput
// are typed EquipmentWithConditions, so an Equipment without it does not
// compile (rpg-toolkit#1958) — never defaulted, because "nobody holds
// anything" is testimony and not an absence of it.
type EquipmentWithConditions interface {
	Equipment
	Conditions
}

// conditionsNow asks the capability what every member holds, right now, and
// returns the answer keyed by member.
//
// It is [Encounter.equipmentNow] for conditions, with the same sorted roster,
// the same once-per-refresh question and the same two refusals: a stranger in
// the answer (ErrNotMember) and a member the answer skipped (ErrNoConditions).
// It also refuses an entry with no ConditionRef, and a key repeated
// within one member, as ErrInvalidData — a list a rule reads by reference must
// not hold an address it cannot name or one it holds twice.
//
// Each member's list keeps the order reported. Sheet order is persisted and
// deterministic, so sorting here would only discard it.
func (e *Encounter) conditionsNow() (map[MemberID]*ConditionSet, error) {
	if len(e.members) == 0 {
		return nil, nil
	}

	roster := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		roster = append(roster, id)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i] < roster[j] })

	reported, err := e.conditions.Conditions(roster)
	if err != nil {
		return nil, fmt.Errorf("conditions: %w", err)
	}

	for id := range reported {
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("conditions: reported conditions for %q, who is not a member: %w", id, ErrNotMember)
		}
	}

	for _, id := range roster {
		set, ok := reported[id]
		if !ok {
			return nil, fmt.Errorf("conditions: did not say what %q holds: %w", id, ErrNoConditions)
		}
		if err := validateConditionSet(set); err != nil {
			return nil, fmt.Errorf("conditions: %q: %w: %w", id, err, ErrInvalidData)
		}
	}

	return reported, nil
}

// validateConditionSet refuses an entry with no ConditionRef and a repeated
// key. A nil set is valid: nothing to observe.
func validateConditionSet(set *ConditionSet) error {
	if set == nil {
		return nil
	}
	seen := make(map[ConditionKey]struct{}, len(set.Conditions))
	for _, held := range set.Conditions {
		if held.ConditionRef == "" {
			return fmt.Errorf("condition with no ref")
		}
		if _, repeated := seen[held]; repeated {
			return fmt.Errorf("condition %q from %q held twice", held.ConditionRef, held.SourceID)
		}
		seen[held] = struct{}{}
	}
	return nil
}
