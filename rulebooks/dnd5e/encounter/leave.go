// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
)

// leave.go is THE RUN ENDS AT THE DOOR (rpg-project#716, ruled 2026-10-29):
// "walk out the front door to end. king taken care of? no? failure, yes?
// success." A [TriggerLeft] ending fires when the last standing player leaves
// by its exit ([Encounter.departureEnding] asks, in declaration order), its [EndingInput.If] says which of the endings waiting on
// that door it is, and its [EndingInput.Result] says what it meant.

// validateLeaveEnding refuses the leave half of one declared ending: a
// `left` naming an exit this field does not declare, a `left` with no
// result, a result that is neither success nor failure, an `if` beside
// anything but a `left`, and an `if` that is not a down, a fact or a stance.
// ErrNoEnding, wrapped in a sentence naming the ending.
func (f *field) validateLeaveEnding(ei EndingInput) error {
	switch ei.Result {
	case "", ResultSuccess, ResultFailure:
	default:
		return fmt.Errorf("ending %q says its outcome is %q, and an outcome is success or failure: %w",
			ei.Key, ei.Result, ErrNoEnding)
	}

	left, isLeft := ei.Trigger.(TriggerLeft)
	if !isLeft {
		if ei.If != nil {
			return fmt.Errorf("ending %q has an `if`, and only an ending that waits for the party to leave has one: %w",
				ei.Key, ErrNoEnding)
		}

		return nil
	}
	if _, declared := f.exitCells[left.Exit]; !declared {
		return fmt.Errorf("ending %q waits for the party to leave by %q, which this field does not declare: %w",
			ei.Key, left.Exit, ErrNoEnding)
	}
	if ei.Result == "" {
		return fmt.Errorf("ending %q waits for the party to leave and does not say whether that is success or failure: %w",
			ei.Key, ErrNoEnding)
	}

	what := fmt.Sprintf("ending %q's if", ei.Key)
	switch c := ei.If.(type) {
	case nil:
	case TriggerMemberDown:
		if c.Member == "" {
			return fmt.Errorf("%s names no member: %w", what, ErrNoEnding)
		}
	case TriggerFact:
		if c.Fact == "" {
			return fmt.Errorf("%s names no fact: %w", what, ErrNoEnding)
		}
	case TriggerStance:
		// A STATE, NOT AN EVENT: an `if` asks how the pair stands when the
		// party leaves, so a stance they hold from the start is a fine
		// answer here, unlike an until or an ending that waits for a turn.
		for _, id := range c.Between {
			if !f.isFaction(id) {
				return fmt.Errorf("%s asks about faction %q, which this field does not declare: %w", what, id, ErrNoEnding)
			}
		}
		if !knownStance(c.Stance) {
			return fmt.Errorf("%s asks for stance %q, which is not hostile, neutral or allied: %w", what, c.Stance, ErrNoEnding)
		}
	default:
		return fmt.Errorf("%s is a %T, and an if asks whether somebody is down, a fact is known, or a pair stands so: %w",
			what, ei.If, ErrNoEnding)
	}

	return nil
}

// noStandingPlayer reports whether no player member still standing remains
// in the run: the last one out has gone. DOWN MEMBERS DO NOT KEEP THE RUN
// OPEN ([TriggerLeft]): a body left behind is not somebody still playing.
func (e *Encounter) noStandingPlayer(down map[MemberID]bool) bool {
	for id, m := range e.members {
		if m.Kind == KindPlayer && !down[id] {
			return false
		}
	}

	return true
}

// truthHolds answers an ending's `if` as world truth at this moment: the
// member is down now, anybody in the run has learned the fact, or the pair
// stands so now. Nil holds. Any other kind holds never, the fail-closed
// answer for a value that could only arrive by hand: the door refuses one.
func (e *Encounter) truthHolds(cond Trigger, down map[MemberID]bool) (bool, error) {
	switch c := cond.(type) {
	case nil:
		return true, nil
	case TriggerMemberDown:
		return down[c.Member], nil
	case TriggerFact:
		return e.world.anyoneKnowsFact(c.Fact), nil
	case TriggerStance:
		now, err := e.Stance(c.Between[0], c.Between[1])
		if err != nil {
			return false, fmt.Errorf("if: %w", err)
		}

		return now == c.Stance, nil
	default:
		return false, nil
	}
}

// resultOf is what the declared ending named key means for the party, or
// empty for one that declares nothing or a key the composition owns
// ("abandoned", party defeat).
func (e *Encounter) resultOf(key string) EndingResult {
	for i := range e.endings {
		if e.endings[i].key == key {
			return e.endings[i].result
		}
	}

	return ""
}
