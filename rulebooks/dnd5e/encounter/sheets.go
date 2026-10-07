// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
)

// SheetFacts is what one member's sheet says about how it moves and what it
// can do on its own turn: its speed, its actions with their reach, and its
// target-selection strategy. The composition carries every value and
// interprets none (C1).
//
// ONE TYPE FOR BOTH KINDS OF SHEET. A session answers a character from the
// character sheet — walking speed from its race, the equipped weapon's swing
// (or the unarmed strike when the main hand is empty) — and a monster from its
// stat block — SpeedData.Walk, its authored actions, and the targeting word
// the author's placement wrote onto it at spawn. Nothing here asks which kind
// of sheet answered.
//
// THE ZERO VALUE IS A CLAIM, NOT A GAP: a SpeedFeet of 0 is a true speed of
// zero — a creature that cannot move on its own — and the walks it takes on
// the world clock pace nothing. Nothing on the sheet is optional to answer.
// A player is answered from its own sheet exactly as a monster is: its
// walking speed paces its walks (rpg-project#538), and an empty Actions or
// Targeting is what that sheet actually says, not what a host happened to
// have to hand. A host that cannot read a member's sheet refuses the ask
// (see [Sheets]); it never answers zero in its place.
//
// Sight is NOT here. How far a member sees is asked through [Sight], at the
// percept refresh, with light applied; it is a different question asked at a
// different moment.
type SheetFacts struct {
	// SpeedFeet is how far this member moves on its own turn, in FEET — a
	// character's walking speed or a monster's SpeedData.Walk. It paces a walk
	// on the world clock (one round every CellsFromFeet(SpeedFeet) cells) and
	// is a driven turn's movement budget. Zero is legal: this member paces
	// nothing and never moves on its own turn. Negative is refused
	// (ErrInvalidData).
	SpeedFeet int

	// Actions are what this member can attack with on its own turn, each with
	// its reach in feet ([ActionView.RangeFeet]) — a character's equipped
	// weapon's swing, or a monster's authored action definitions. Read for a
	// driven member's [MonsterView.Actions], its [SeenMember.InReach] and the
	// `enemy: reach` band of its [Facts]. A negative range is refused
	// (ErrInvalidData).
	Actions []ActionView

	// Targeting is a monster's target-selection strategy in the rulebook's own
	// words — "closest", "lowest-health", "lowest-ac" — handed to its driver as
	// [MonsterView.Targeting]. Empty for a member with no strategy, which is
	// every player today. Opaque here (C1).
	Targeting string
}

// Sheets reports each given member's [SheetFacts]. The composition asks; the
// rulebook answers from the sheets it holds.
//
// Injected rather than held, exactly as [Equipment] and [Sight] are: this
// module's go.mod cannot import the rulebook (law C1), so how fast a creature
// is and what it can hit with are facts it can only be TOLD.
//
// # It is a pull, and it is never remembered
//
// The composition stores no speed, action or targeting, in memory or in its
// blob (rpg-project#538). It asks at the moment it uses one: pacing a walk on
// the world clock, budgeting a driven turn, building a driver's view, and
// testing whether an enemy is in reach. Being a pull is what makes it
// complete: a weapon swapped, a level gained, a speed changed by a rule nobody
// has written yet — each is read at the next use without that route knowing
// this interface exists. A copy taken at Join would be a wrong answer nobody
// refreshes. A budget already handed to a turn in progress is that turn's
// value and is not re-asked mid-turn.
//
// # What an implementation owes
//
// Answer about every member asked, and nobody else. A member missing from the
// answer is refused (ErrNoSheets): its speed and reach would have to be
// invented, which rpg-toolkit#1033 forbids. A stranger in the answer is
// refused (ErrNotMember). A host that holds no sheet for a member it placed
// should fail here, by name, rather than answer zero — zero is a real answer
// (see [SheetFacts]) and a missing sheet is not.
//
// The roster is asked SORTED and WHOLE, for [Encounter.equipmentNow]'s
// reasons: the question a rulebook receives does not depend on which verb is
// running or which member it happens to care about. Errors abort whatever
// verb was running, atomically (R5).
type Sheets interface {
	// Sheets reports each given member's facts. Every member asked about
	// must appear in the answer and nobody else may.
	Sheets(members []MemberID) (map[MemberID]SheetFacts, error)
}

// sheetsNow asks the capability for every member's sheet facts, right now,
// and returns the answer keyed by member — [Encounter.equipmentNow]'s shape,
// with its two refusals: a stranger in the answer (ErrNotMember) and a member
// the answer skipped (ErrNoSheets). It also refuses a negative speed or
// action range (ErrInvalidData): [CellsFromFeet] divides those, and a
// negative one is not a shorter distance but a defect that would produce a
// nonsense budget or reach.
func (e *Encounter) sheetsNow() (map[MemberID]SheetFacts, error) {
	if len(e.members) == 0 {
		return nil, nil
	}

	roster := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		roster = append(roster, id)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i] < roster[j] })

	reported, err := e.sheets.Sheets(roster)
	if err != nil {
		return nil, fmt.Errorf("sheets: %w", err)
	}

	for id := range reported {
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("sheets: reported a sheet for %q, who is not a member: %w", id, ErrNotMember)
		}
	}

	for _, id := range roster {
		facts, ok := reported[id]
		if !ok {
			return nil, fmt.Errorf("sheets: did not answer for %q: %w", id, ErrNoSheets)
		}
		if err := validateSheetFacts(facts); err != nil {
			return nil, fmt.Errorf("sheets: %q: %w: %w", id, err, ErrInvalidData)
		}
	}

	return reported, nil
}

// sheetOf is [Encounter.sheetsNow] read for one member — the whole roster is
// still asked, so a mis-wired capability fails the same way whichever member
// the verb cared about. An id outside the roster is refused (ErrNotMember)
// rather than answered with a zero sheet nobody gave.
func (e *Encounter) sheetOf(id MemberID) (SheetFacts, error) {
	all, err := e.sheetsNow()
	if err != nil {
		return SheetFacts{}, err
	}
	facts, ok := all[id]
	if !ok {
		return SheetFacts{}, fmt.Errorf("sheets: %q is not a member: %w", id, ErrNotMember)
	}

	return facts, nil
}

// validateSheetFacts refuses a negative speed and a negative action range.
func validateSheetFacts(facts SheetFacts) error {
	if facts.SpeedFeet < 0 {
		return fmt.Errorf("speed %d feet is negative", facts.SpeedFeet)
	}
	for _, a := range facts.Actions {
		if a.RangeFeet < 0 {
			return fmt.Errorf("action %q range %d feet is negative", a.Ref, a.RangeFeet)
		}
	}

	return nil
}
