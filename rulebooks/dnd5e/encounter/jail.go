// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// jail.go is THE CUT TO THE CELLS (rpg-project#925, ruled 2026-11-14: "I do
// like the cut away while walking off and appearing in the cell"): an
// interaction that arrested may, on entering a later state, move its taken
// into the arrest's cell.
//
// # Who goes, and where
//
// The taken of the SAME interaction's arrest ([ArrestRecord.Taken]) who are
// present and still held ([CustodyHeld]). Each is set down on a free cell of
// the arrest's cell region ([Encounter.FreeCellsIn], the region's own order,
// so a replay lands the same cells), through the engine's own custody path —
// [Encounter.relocate] with custody set, which no request can reach — with a
// cause whose words and rounds are the author's ([Jail]). The record's
// custody becomes [CustodyJailed], which holds nothing by itself
// ([CustodyPhase.holds]): the cell's walls and its locked door do.
//
// A taken member who has left the run is not moved. They stay taken in the
// record, are recorded wanted, and the guards who hold `arrested` about them
// come to hold [TopicWanted] about them too, told to those guards alone. A
// member who rejoins later is not placed in the cell (the lead's call on
// #925).
//
// Too few free cells, or nobody to take — no arrest, custody already broken
// or jailed, every taken member gone — is refused loudly: nobody moves and
// nothing is recorded.
//
// # What the guards hold
//
// Every arresting guard who is up comes to hold [TopicJailed] about each
// member moved, told to that guard alone — the guards' knowledge, as
// `wanted` is.
//
// # The escape
//
// When a [Jail] names an Escaped fact, every sight refresh asks: does an
// arresting guard who holds `arrested` about a jailed member see them outside
// the cell region? If so, the guard's faction's mind learns the fact
// ([Encounter.learnFact]), and the content's disposition turns on it the
// ordinary way — the stance flips and the fight forms on sight. A mind that
// is down or gone learns nothing, as for any fact.

// TopicJailed is the topic the arresting guards hold about each member they
// moved into the cell (rpg-project#925).
const TopicJailed TopicID = "jailed"

// jailCause is the cause a jail's relocation names: told to every observer of
// either end, so it names the arrest and never the cell.
var jailCause = core.Ref{Module: "dnd5e", Type: "consequences", ID: "arrest"}

// Jail is the consequence "the taken are moved into the cell"
// (rpg-project#925). It hangs only on an interaction's outcome, on an
// interaction that arrests.
type Jail struct {
	// Words is what each member moved is told ("The guards march you to the
	// cells."). REQUIRED.
	Words string `json:"words"`

	// Rounds is how many world rounds pass with the move (the castle's walk
	// off-screen takes 5). Zero or more.
	Rounds int `json:"rounds,omitempty"`

	// Escaped is the fact the guards' faction's mind learns when an arresting
	// guard sees a jailed member outside the cell region; empty for none.
	Escaped FactID `json:"escaped,omitempty"`
}

// validateJailShape refuses a jail nobody could author on purpose: no words,
// negative rounds.
func validateJailShape(what string, j Jail) error {
	switch {
	case j.Words == "":
		return fmt.Errorf("%s jails and tells the jailed nothing: name its words", what)
	case j.Rounds < 0:
		return fmt.Errorf("%s jails over %d rounds", what, j.Rounds)
	}
	return nil
}

// arrests reports whether any of an interaction's outcomes arrests.
func arrests(outcomes map[string][]Consequence) bool {
	for _, cs := range outcomes {
		for _, c := range cs {
			if c.Arrest != nil {
				return true
			}
		}
	}
	return false
}

// jailPlan is who a jail would take and where, or its loud refusal (this
// file's doc): asked whole before anything changes, by the line that would
// fire it ([Encounter.refuseJailAhead]) and by the jail itself.
func (e *Encounter) jailPlan(made *ArrestRecord) (present, gone []MemberID, cells []spatial.Position, err error) {
	switch {
	case made == nil:
		return nil, nil, nil, fmt.Errorf("jail: the interaction has arrested nobody: %w", ErrBadPlacement)
	case made.Custody != CustodyHeld:
		return nil, nil, nil, fmt.Errorf("jail: custody is %q, so nobody is held to take: %w", made.Custody, ErrBadPlacement)
	}
	for _, t := range made.Taken {
		if _, ok := e.members[t.ID]; ok {
			present = append(present, t.ID)
		} else {
			gone = append(gone, t.ID)
		}
	}
	if len(present) == 0 {
		return nil, nil, nil, fmt.Errorf("jail: every taken member has left the run: %w", ErrBadPlacement)
	}
	cells, err = e.FreeCellsIn(made.Cell, len(present))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("jail: %w", err)
	}
	return present, gone, cells, nil
}

// refuseJailAhead asks, before a line writes anything, whether the state its
// verdict would step its interaction into jails, and if so whether that jail
// would refuse: the line is then refused whole. The step is tried on a copy
// of the interaction's progress, so nothing moves.
func (e *Encounter) refuseJailAhead(option Option, beaten bool) error {
	a := option.Advance
	if a == nil {
		return nil
	}
	beat := a.Failure
	if beaten {
		beat = a.Success
	}
	inst, ok := e.interactions[a.Interaction]
	if beat == "" || !ok {
		return nil
	}
	blob := inst.blob
	blob.Bag = inst.blob.Bag.Clone()
	at := uint64(e.clock.ToData().HighWater)
	stepped, err := inst.program.Step(&blob, beat, at, nil, at)
	if err != nil || !stepped.Fired || stepped.To == "" {
		return nil
	}
	for _, c := range inst.input.Outcomes[stepped.To] {
		if c.Jail != nil {
			if _, _, _, err := e.jailPlan(inst.arrest); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyJail moves the interaction's taken into its arrest's cell (this
// file's doc).
func (e *Encounter) applyJail(r *interactionRecord, j Jail, line recognitionLine) error {
	made := r.arrest
	present, gone, cells, err := e.jailPlan(made)
	if err != nil {
		return err
	}
	moves := make([]Relocation, len(present))
	for i, id := range present {
		moves[i] = Relocation{Member: id, To: cells[i]}
	}
	if _, err := e.relocate(&RelocateInput{
		Moves: moves, Cause: RelocationCause{Ref: jailCause, Words: j.Words, Rounds: j.Rounds}, Reach: line.reach,
	}, true); err != nil {
		return fmt.Errorf("jail: %w", err)
	}

	made.Custody, made.Escape = CustodyJailed, j.Escaped
	made.Jailed = slices.Clone(present)

	reach, err := e.placedSightReach()
	if err != nil {
		return err
	}
	guards, err := e.arrestingGuards(line.interaction)
	if err != nil {
		return err
	}
	told := line
	told.holdersOnly = true
	for _, g := range guards {
		if err := e.holdTopic(reach, g, present, TopicJailed, ChannelJudgement, told); err != nil {
			return err
		}
	}
	// THE GONE STAY WANTED: whoever holds `arrested` about them holds
	// `wanted` too, told to that guard alone.
	for _, id := range gone {
		var by []MemberID
		for _, g := range guards {
			if _, held := e.intelLog.AboutOn(sightMember(g), sightMember(id), perception.Topic(TopicArrested)); held == nil {
				by = append(by, g)
			}
		}
		if len(by) == 0 {
			continue
		}
		made.Wanted = append(made.Wanted, WantedMember{ID: id, RememberedBy: by})
		for _, g := range by {
			if err := e.holdTopic(reach, g, []MemberID{id}, TopicWanted, ChannelJudgement, told); err != nil {
				return err
			}
		}
	}
	return nil
}

// sweepEscapes is the escape rule (this file's doc), asked on every sight
// refresh: an arresting guard who holds `arrested` about a jailed member and
// sees them outside the cell region teaches the jail's Escaped fact to his
// faction's mind. Idempotent: a mind that knows it learns nothing again.
func (e *Encounter) sweepEscapes() error {
	var reach sightReach
	asked := false
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		made := e.interactions[id].arrest
		if made == nil || made.Custody != CustodyJailed || made.Escape == "" {
			continue
		}
		if !asked {
			var err error
			if reach, err = e.placedSightReach(); err != nil {
				return err
			}
			asked = true
		}
		guards, err := e.arrestingGuards(id)
		if err != nil {
			return err
		}
		for _, m := range made.Jailed {
			if _, ok := e.members[m]; !ok {
				continue
			}
			cell, placed := e.canvas.GetEntityPosition(string(m))
			if !placed {
				continue
			}
			if region, ok := e.field.regionOf(cell); ok && region == made.Cell {
				continue
			}
			for _, g := range guards {
				if !reach.Reaches(perception.Sight, g, m) {
					continue
				}
				if _, held := e.intelLog.AboutOn(sightMember(g), sightMember(m), perception.Topic(TopicArrested)); held != nil {
					continue
				}
				mind := e.field.mindOf(factionOf(e.members[g]), e.rosterIDs())
				if mind == "" {
					continue
				}
				if err := e.learnFact(mind, made.Escape, "escaped", uint64(e.clock.ToData().HighWater)); err != nil {
					return fmt.Errorf("escape: %w", err)
				}
			}
		}
	}
	return nil
}
