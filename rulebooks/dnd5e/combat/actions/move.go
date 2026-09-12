// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import "fmt"

// MovePolicy is HOW a creature moved against its will is moved: the shape of
// the route, stated as a rule rather than as cells.
//
// Content never names cells. It says "straight away from me, two", and the
// layer that owns the map works out which cells those are under the fold — so
// a push that would cross a wall stops at the wall without any spell knowing a
// wall exists.
//
// EVERY VALUE HERE IS ONE SOMETHING CAN WALK. A policy arrives with the spell
// that brings its executor and not one line before: "line" came with
// Thunderwave, "away" with Dissonant Whispers, and "toward" with Command,
// whose Approach is the first thing in the catalogue that walks a creature at
// somebody rather than away from them. A policy named here ahead of its
// executor would validate clean in content and then fail at the moment of the
// shove, which moves the refusal from the author to the table.
type MovePolicy string

const (
	// MoveLine continues the line from the anchor through the mover, past the
	// mover, for the budget. A shove: it does not search for anywhere better.
	MoveLine MovePolicy = "line"

	// MoveAway sends the mover to the reached standable cell FARTHEST from the
	// anchor by the ruler, within the budget. Not a direction and not a line:
	// a creature in a dead-end corridor that runs its whole speed and ends
	// nearer the anchor as the crow flies has not run away, so it does not
	// run. The executor is encounter's Route, which floods from the mover and
	// measures every reached cell against the anchor.
	MoveAway MovePolicy = "away"

	// MoveToward sends the mover along the shortest walking route to a cell
	// ADJACENT to the anchor, and stops it there: Command's Approach is
	// "moves toward you by the shortest and most direct route", and what it
	// wants is a creature standing in front of the caster, not one standing
	// on them. When no such cell is reachable within the budget the mover
	// ends on the reached cell nearest the anchor by the ruler, which is the
	// blocked-corridor case: it closed as far as the walls let it. The
	// executor is encounter's Route, which owns the map.
	MoveToward MovePolicy = "toward"
)

// MovePays is what the moved creature spends to be moved.
//
// The zero value is [PaysNothing], and that is deliberate: a directive that
// debited an economy nobody wrote into it would spend a reaction the creature
// was saving, in a rule no spell states.
type MovePays string

const (
	// PaysNothing is the zero value and the push: the creature slides and is
	// charged for none of it.
	PaysNothing MovePays = ""

	// PaysReaction spends the moved creature's reaction, and the move does not
	// happen at all when there is none left.
	PaysReaction MovePays = "reaction"
)

// CastMove declares that a cast MOVES the creature it lands on, and says what
// that move costs them.
//
// A DECLARATION, NOT A ROUTE. Nothing here is geometry: no cells, no
// directions, no map. Resolution turns this into an imposed effect on a failed
// save the way it turns [CastProfile.Effects] into an imposed condition, and
// encounter finds the actual cells under its own fold and walks them. A spell
// holding a path search of its own is the thing this shape exists to prevent.
//
// A POINTER ON [CastProfile] for [CastConcentration]'s reason: a policy and a
// budget sitting beside a cast that does not move anybody are zero values that
// lie. Nil is "this cast moves nobody"; non-nil is the whole answer.
type CastMove struct {
	// Policy is how the mover is moved.
	Policy MovePolicy `json:"policy"`

	// Cells is a fixed budget in cells. Exactly one of Cells and Speed is set:
	// Thunderwave shoves ten feet whoever it catches, and Dissonant Whispers
	// sends its target running as far as its own legs carry it.
	Cells int `json:"cells,omitempty"`

	// Speed says the budget is the MOVER's speed rather than a number content
	// could know. Content cannot know it: the same spell moves a dwarf and a
	// horse different distances.
	Speed bool `json:"speed,omitempty"`

	// Turn says the budget is the mover's REMAINING movement on its own turn,
	// which is a different number from its speed the moment it has already
	// walked. Only meaningful when the move IS the mover's turn — resolution's
	// Obey produces one and no cast does — so it is a third budget rather than
	// a flag on Speed: the two answer different questions and a spell that
	// asked for both would have no rule for picking between them.
	Turn bool `json:"turn,omitempty"`

	// Pays is what the move costs the creature being moved. Zero is nothing.
	Pays MovePays `json:"pays,omitempty"`

	// Provokes says whether the move offers opportunity attacks as it goes.
	// Zero is false, which is the push: being thrown across a room is not the
	// same as walking out of a reach.
	Provokes bool `json:"provokes,omitempty"`
}

// Validate reports whether the move declares a known policy, exactly one of
// the three budgets, and a known price.
func (m CastMove) Validate() error {
	switch m.Policy {
	case MoveLine, MoveAway, MoveToward:
	default:
		return fmt.Errorf("unknown move policy %q", m.Policy)
	}

	// A negative count is named before the count is weighed, so it cannot hide
	// inside "no budget declared" and be reported as an omission when it is a
	// typo.
	if m.Cells < 0 {
		return fmt.Errorf("move budget in cells must not be negative, got %d", m.Cells)
	}
	// Every combination but one is refused for the same reason and in one
	// sentence: no budget at all would move a creature zero cells and record
	// that it moved, and any two of the three are two answers to one question
	// with no rule for picking between them. Counted rather than compared,
	// because a boolean expression over three budgets stops reading as the
	// rule it enforces.
	budgets := 0
	if m.Cells > 0 {
		budgets++
	}
	if m.Speed {
		budgets++
	}
	if m.Turn {
		budgets++
	}
	if budgets != 1 {
		return fmt.Errorf(
			"move must declare exactly one budget, a positive cell count or the mover's speed or its turn")
	}

	switch m.Pays {
	case PaysNothing, PaysReaction:
	default:
		return fmt.Errorf("unknown move price %q", m.Pays)
	}

	return nil
}
