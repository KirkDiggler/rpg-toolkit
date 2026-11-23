// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ownhail.go is A CREATURE HAILS FOR ITSELF, FROM ITS OWN MACHINE
// (rpg-project#1139, slice rpg-project#1140; road step 5 of the machine story
// rpg-project#989): a hail is the guard speaking when someone comes near, and
// each guard hails for himself. Nothing stands afterwards.
//
//	- on: approached
//	  when: { all: [ { scope: { within: 2 } }, ... ] }
//	  do: [ { pick: hail } ]
//
// # Who hails
//
// At the end of a step, after the conversation's own pass ([addressAfterStep]),
// each other member is a hailer of the mover when ALL of these hold: his
// current state has at least one `approached` step and EVERY one requires a
// `within` ([Encounter.ownHailWithin]: the machine's own reading, the larger
// when two steps name different ones); he does not carry an interaction that
// declares an approach (such a creature hails by the conversation's rule
// only); he and the mover are on the map and in no fight, and he is up; he
// sees the mover now; and the mover's step ENDED within his distance and
// STARTED beyond it — crossing in, not standing near. Nearest first, then by
// id, each is woken with `approached` at once, in the step, exactly as a
// carrier is ([Encounter.hail]).
//
// # What it writes
//
// No address, no [BeatAddressed], nothing swept later: the creature's answer
// is the whole of it. The step reports each creature whose `approached` step
// FIRED ([StepOutput.Hailed]): a pick ran, or a state changed. A creature the
// beat woke whose guard refused it (its `when` was false for this mover) has
// not hailed and is not named.

// ownHailOutput is what a step's own-hail pass did and what the hailing
// creatures' answers made.
type ownHailOutput struct {
	hailed []MemberID
	deltas map[MemberID]*IntelDelta
	formed *FormedBubble
}

// ownHailWithin is the distance a member hails at, from his own machine: the
// larger `within` of the `approached` steps of every attached instance's
// current state, and false when there is no such step or any of them requires
// no `within` (an unscoped step, or scopes that measure differently, which
// the machine leaves unscoped).
func ownHailWithin(m *memberRecord) (int, bool) {
	if m.behaviors == nil {
		return 0, false
	}
	status, err := m.behaviors.composer.Inspect(&behavior.InspectComposerInput{})
	if err != nil {
		return 0, false
	}
	within, listens := 0, false
	for _, instance := range status.Instances {
		for _, listen := range instance.Listens {
			if listen.Beat != beatApproached {
				continue
			}
			if listen.Scope == nil || listen.Scope.Within < 1 {
				return 0, false
			}
			listens = true
			within = max(within, listen.Scope.Within)
		}
	}

	return within, listens
}

// carriesAnApproach reports whether a member carries an interaction that
// declares an approach: the creature the conversation's hail rule is for.
func (e *Encounter) carriesAnApproach(m *memberRecord) bool {
	for _, r := range e.interactions {
		if r.input.Approach != nil && carries(m.Options, r.input.ID) {
			return true
		}
	}

	return false
}

// behaviorStatesOf is every attached instance's current state, as one value
// to compare before and after a wake.
func behaviorStatesOf(m *memberRecord) string {
	if m.behaviors == nil {
		return ""
	}
	status, err := m.behaviors.composer.Inspect(&behavior.InspectComposerInput{})
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, instance := range status.Instances {
		b.WriteString(instance.ID)
		b.WriteString("=")
		b.WriteString(instance.State)
		b.WriteString(";")
	}

	return b.String()
}

// ownHailers is who hails the mover for a step from one cell to another:
// every member the doc's conditions admit, nearest first, then by id.
func (e *Encounter) ownHailers(mover MemberID, from, to spatial.Position) ([]MemberID, error) {
	moverFights, err := e.inFight(mover)
	if err != nil || moverFights {
		return nil, err
	}
	type candidate struct {
		id       MemberID
		distance float64
	}
	var near []candidate
	for id, m := range e.members {
		if id == mover {
			continue
		}
		within, ok := ownHailWithin(m)
		if !ok || e.carriesAnApproach(m) {
			continue
		}
		at, placed := e.canvas.GetEntityPosition(string(id))
		if !placed {
			continue
		}
		after := e.Distance(to, at)
		if after > float64(within) || e.Distance(from, at) <= float64(within) {
			continue
		}
		near = append(near, candidate{id: id, distance: after})
	}
	slices.SortFunc(near, func(a, b candidate) int {
		if a.distance != b.distance {
			if a.distance < b.distance {
				return -1
			}
			return 1
		}
		return strings.Compare(string(a.id), string(b.id))
	})
	var out []MemberID
	for _, c := range near {
		down, err := e.isDown(c.id)
		if err != nil {
			return nil, err
		}
		fighting, err := e.inFight(c.id)
		if err != nil {
			return nil, err
		}
		if down || fighting {
			continue
		}
		seen, err := e.sees(c.id, mover)
		if err != nil {
			return nil, err
		}
		if seen {
			out = append(out, c.id)
		}
	}

	return out, nil
}

// ownHailAfterStep is the step's own-hail pass (this file's doc): the mover's
// step went from one cell to another, and each creature that hails for itself
// is woken with `approached`.
//
// Errors: the predicate's capabilities, a creature's machine or pick, the
// refresh after his answer.
func (e *Encounter) ownHailAfterStep(mover MemberID, from, to spatial.Position) (ownHailOutput, error) {
	var out ownHailOutput
	hailers, err := e.ownHailers(mover, from, to)
	if err != nil {
		return out, err
	}
	for _, id := range hailers {
		hail, err := e.wakeOwnHail(mover, id)
		if err != nil {
			return out, err
		}
		if hail.fired {
			out.hailed = append(out.hailed, id)
		}
		out.deltas = mergeIntelDeltas(out.deltas, hail.deltas)
		if hail.formed != nil {
			out.formed = hail.formed
		}
		if e.outcome != nil || out.formed != nil {
			return out, nil
		}
	}

	return out, nil
}

// ownHailWake is one creature's answer to a hail.
type ownHailWake struct {
	fired  bool
	deltas map[MemberID]*IntelDelta
	formed *FormedBubble
}

// wakeOwnHail wakes one creature with `approached` at once, in the step:
// [Encounter.hail]'s wake without an address. His answer is in the step's act,
// and the sight is refreshed only when his answer did something a look can
// see. It reports whether his step fired.
func (e *Encounter) wakeOwnHail(mover, id MemberID) (ownHailWake, error) {
	var out ownHailWake
	m := e.members[id]
	before := behaviorStatesOf(m)
	at := uint64(e.clock.ToData().HighWater)

	e.reactionPicked, e.blockingTurned, e.stanceFormed = false, false, nil
	prevAct := e.reactAct
	e.reactAct = e.CurrentAct()
	defer func() { e.reactAct = prevAct }()
	cell, _ := e.canvas.GetEntityPosition(string(mover))
	if err := e.matchOne(wakeBeat{Beat: beatApproached, Cell: &cell, Subject: mover, Member: id, At: at}); err != nil {
		return ownHailWake{}, fmt.Errorf("approached: %w", err)
	}
	out.fired = e.reactionPicked || e.blockingTurned || behaviorStatesOf(m) != before
	out.formed, e.stanceFormed = e.stanceFormed, nil
	if e.outcome != nil || (!e.reactionPicked && !e.blockingTurned) {
		return out, nil
	}
	deltas, formed, err := e.refreshCostingReach(e.cellsOf(mover, id), mover)
	if err != nil {
		return ownHailWake{}, fmt.Errorf("approached refresh sight: %w", err)
	}
	out.deltas = deltas
	if formed != nil {
		out.formed = formed
	}

	return out, nil
}
