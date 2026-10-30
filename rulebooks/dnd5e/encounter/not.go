// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
)

// not.go is THE ONE NEGATION (rpg-project#719): `{ not: <one predicate> }`,
// legal wherever a predicate is and scoped by what it wraps. A failed theft
// closes the cook's lines and not the keep's because an option can now be
// withheld on a fact the speaker learned: `requires: { not: { fact:
// caught-in-kitchen } }`.
//
// # Judged the same way the inner one is
//
// A `not` has no notice point of its own and no third value. It is asked
// exactly where and when its inner predicate is asked, and answers the
// opposite:
//
//   - READ where somebody asks. An option's `requires` reads the speaker's
//     knowledge and pockets at the moment they speak ([Encounter.requirementHolds]),
//     and a leave ending's `if` reads the world when the last one is out
//     ([Encounter.truthHolds]). A `not` there is the inner read, negated.
//   - WAITED FOR everywhere else — an until, an arrival, an ending's `when` —
//     at the inner form's own site. Most forms only ever become true: a fact
//     is never unlearned, a round never unstarts, a fall stays a fall, the
//     party never comes back in. Their `not` holds from the first frame and
//     can only stop holding, which is a predicate nothing can fire, refused
//     at construction by the liveness law every other dead predicate meets
//     ([notHoldsFromTheStart]). Two forms can genuinely stop being true at a
//     site that notices them: a STANCE, which turns in the fold after a flip
//     ([onStance] fires a `not` when the pair turns away from the stance),
//     and a DEPARTURE, where `when: { not: { left: gate } }` is the last one
//     out by any other door ([Encounter.departureEnding]).
//
// # Scoped by what it wraps
//
// A `not` of `holds` is legal only where `holds` is — an option's `requires`
// — and a `not` of `left` only where `left` is, an ending's `when`; each is
// refused elsewhere in the inner form's own sentence. A `not` of a `not` is
// the predicate itself, and is refused as one.

// TriggerNot is `{ not: <predicate> }`: holds when Inner does not, judged at
// Inner's own site ([not.go]). Inner is REQUIRED and is never itself a
// TriggerNot.
type TriggerNot struct {
	// Inner is the one predicate negated.
	Inner Trigger
}

// isTrigger marks TriggerNot as a Trigger.
func (t TriggerNot) isTrigger() {}

// notOfNot is the sentence a `not` wrapping a `not` is refused with.
const notOfNot = "a not of a not is the predicate itself: write the inner one"

// notHoldsFromTheStart is the liveness sentence a waited-for `not` of a form
// that only ever becomes true is refused with.
const notHoldsFromTheStart = "a not of a fact, a round or a fall holds from the start and can only stop holding, " +
	"so nothing can fire it: a not is read by an option's requires or a leave ending's if, " +
	"and waited for only on a stance a pair can turn away from"

// validateNot refuses a waited-for `not` (an until, an arrival, an ending's
// `when`) that cannot be one: nothing inside, a not inside, the inner form's
// own scope refusal, or a negation that holds from the start. A `not` of a
// `left` is an ending's and is judged by [field.validateLeaveEnding]; here it
// is refused in left's own words, the way a bare left is.
func (f *field) validateNot(what string, t TriggerNot, sentinel error) error {
	what += "'s not"
	switch inner := t.Inner.(type) {
	case nil:
		return fmt.Errorf("%s names nothing to negate: %w", what, sentinel)
	case TriggerNot:
		return fmt.Errorf("%s: %s: %w", what, notOfNot, sentinel)
	case TriggerLeft, TriggerHolds:
		return f.validatePredicate(what, inner, sentinel)
	case TriggerStance:
		return f.validateNotStance(what, inner, sentinel)
	case TriggerRound, TriggerFact, TriggerMemberDown:
		if err := f.validatePredicate(what, inner, sentinel); err != nil {
			return err
		}
		return fmt.Errorf("%s: %s: %w", what, notHoldsFromTheStart, sentinel)
	default:
		return fmt.Errorf("%s negates a %T, which is not a predicate: %w", what, t.Inner, sentinel)
	}
}

// validateNotStance is the stance form's liveness turned over: `{ not: {
// stance: S } }` waits for the pair to turn AWAY from S, so the pair must be
// S from the start (otherwise it holds from the start and nothing fires it)
// and must be able to leave S — its own until turns it, or it is neutral and
// a blow across it turns it hostile (R3).
func (f *field) validateNotStance(what string, p TriggerStance, sentinel error) error {
	for _, id := range p.Between {
		if !f.isFaction(id) {
			return fmt.Errorf("%s waits on faction %q, which this field does not declare: %w", what, id, sentinel)
		}
	}
	if p.Between[0] == p.Between[1] {
		return fmt.Errorf("%s waits on a stance between %q and itself: %w", what, p.Between[0], sentinel)
	}
	if !knownStance(p.Stance) {
		return fmt.Errorf("%s waits for stance %q, which is not hostile, neutral or allied: %w",
			what, p.Stance, sentinel)
	}
	pair := pairOf(p.Between[0], p.Between[1])
	declared, _ := f.declaredStance(pair)
	if declared != p.Stance {
		return fmt.Errorf("%s waits for %s to stop being %s, which they are not from the start, "+
			"so it holds from the start and nothing can fire it: %w", what, pair, p.Stance, sentinel)
	}
	leaves := declared == StanceNeutral // R3: a blow across a neutral pair turns it hostile
	for _, to := range f.untilTurnsTo(pair) {
		leaves = leaves || to != p.Stance
	}
	if !leaves {
		return fmt.Errorf("%s waits for %s to stop being %s, and nothing turns them: %w",
			what, pair, p.Stance, sentinel)
	}
	return nil
}

// validateReadNot refuses a READ `not` — an option's `requires` or a leave
// ending's `if` — that cannot be one: nothing inside, or a not inside. What
// the inner may be is the read site's own question, asked by its caller with
// inner.
func validateReadNot(what string, t TriggerNot) (Trigger, error) {
	switch t.Inner.(type) {
	case nil:
		return nil, fmt.Errorf("%s's not names nothing to negate", what)
	case TriggerNot:
		return nil, fmt.Errorf("%s's not: %s", what, notOfNot)
	}
	return t.Inner, nil
}

// notLeftLiveness refuses `when: { not: { left: exit } }` on a field whose
// only way out is that exit: the last one out can leave by no other door, so
// nothing can fire it.
func (f *field) notLeftLiveness(key string, exit ExitID) error {
	for id := range f.exitCells {
		if id != exit {
			return nil
		}
	}
	return fmt.Errorf("ending %q waits for the party to leave by any way but %q, and it is the only way out: %w",
		key, exit, ErrNoEnding)
}

// turnedAway reports whether pair turned away from s between before and
// after: it was s, and it is not now. The `not` half of [onStance].
func turnedAway(before, after map[factionPair]Stance, pair factionPair, s Stance) bool {
	return before[pair] == s && after[pair] != s
}

// unwrapNot is a predicate's inner form and whether it was negated, so a
// read site asks one question either way.
func unwrapNot(t Trigger) (Trigger, bool) {
	if n, ok := t.(TriggerNot); ok {
		return n.Inner, true
	}
	return t, false
}
