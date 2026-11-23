// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// seenfilter.go is A SIGHTING WAKES ONLY A CREATURE WHOSE STEP COULD FIRE FOR
// THE ONE SEEN (rpg-project#1123, slice rpg-project#1126). When someone comes
// into a creature's sight, the engine first asks what the creature's current
// `seen` steps REQUIRE of the one seen; if no step could fire for them, no
// beat lands and the machine is not asked. The same reading the machine's
// own necessaryScope makes of a guard's scope, for three more leaves.
//
// WHETHER A SIGHTING WAKES A CREATURE IS DECIDED BY THE STATE IT IS IN WHEN
// THE SIGHTING HAPPENS, as listening at all already is (landTopicBeat: "a
// state that is not listening does not hear it"). A creature that changes
// state between a sighting and the wake is judged by the state it was in.

// seenFilterOn is whether a sighting is filtered by what the creature's steps
// require. A variable only so a test can run one scene both ways.
var seenFilterOn = true

// seenLeafKind names the three leaves a `seen` step can require of the one
// seen.
type seenLeafKind int

const (
	// seenNotedTopic is `noted:<topic>` asked eq true: a note about them.
	seenNotedTopic seenLeafKind = iota + 1
	// seenNotedAs is `noted.as:<topic>` asked eq a non-empty value.
	seenNotedAs
	// seenStanceIs is `stance` asked eq a value.
	seenStanceIs
)

// seenLeaf is one thing a step requires of the one seen.
type seenLeaf struct {
	kind  seenLeafKind
	topic TopicID
	value string
}

// requiredOfSeen is the leaves g requires of every event it applies to,
// among the three that ask about the one the beat names: its own fact leaf
// or one under a top-level all (an all inside an all counts), never one under
// any or not. A note asked eq false and a value asked as the empty string ask
// for an absence and require nothing. A guard with none requires nothing: it
// is "anyone". Pure.
func requiredOfSeen(g *machine.Guard) []seenLeaf {
	if g == nil {
		return nil
	}
	var out []seenLeaf
	if leaf, ok := seenLeafOf(g.Fact); ok {
		out = append(out, leaf)
	}
	for i := range g.All {
		out = append(out, requiredOfSeen(&g.All[i])...)
	}

	return out
}

// seenLeafOf reads one fact leaf as a requirement, when it is one.
func seenLeafOf(f *machine.FactLeaf) (seenLeaf, bool) {
	if f == nil || f.Op != machine.OpEq {
		return seenLeaf{}, false
	}
	name := string(f.Name)
	switch {
	case strings.HasPrefix(name, notedAsPrefix) && len(name) > len(notedAsPrefix):
		if v, ok := f.Value.(string); ok && v != "" {
			return seenLeaf{kind: seenNotedAs, topic: TopicID(name[len(notedAsPrefix):]), value: v}, true
		}
	case strings.HasPrefix(name, notedPrefix) && len(name) > len(notedPrefix):
		if v, ok := f.Value.(bool); ok && v {
			return seenLeaf{kind: seenNotedTopic, topic: TopicID(name[len(notedPrefix):])}, true
		}
	case f.Name == factStance:
		if v, ok := f.Value.(string); ok {
			return seenLeaf{kind: seenStanceIs, value: v}, true
		}
	}

	return seenLeaf{}, false
}

// seenWants is what a member's current `seen` steps require: one set of
// leaves for each step, all of which must hold for that step.
type seenWants [][]seenLeaf

// seenWantsOf reads the requirement of a recipe state's `seen` steps. It
// reports false when there is no `seen` step or any step requires nothing:
// the member is "anyone" and is never filtered.
func seenWantsOf(transitions []machine.Transition) (seenWants, bool) {
	var wants seenWants
	for _, t := range transitions {
		if t.On != beatSeen {
			continue
		}
		set := requiredOfSeen(t.When)
		if len(set) == 0 {
			return nil, false
		}
		wants = append(wants, set)
	}

	return wants, len(wants) > 0
}

// seenLeafHolds is whether a leaf's leaves all hold for holder about subject, asked
// with the readers the guard's own words use, so the two never disagree. A
// belief that cannot be read does not filter.
func (e *Encounter) seenLeafHolds(holder, subject MemberID, l seenLeaf) bool {
	switch l.kind {
	case seenNotedTopic:
		return e.hasNoted(holder, subject, l.topic)
	case seenNotedAs:
		return e.notedValue(holder, subject, l.topic) == l.value
	case seenStanceIs:
		stance, err := e.BelievedStance(holder, subject)
		if err != nil {
			return true
		}

		return string(stance) == l.value
	}

	return true
}

// couldBeWokenBySeeing is whether any one of holder's `seen` steps could fire
// for subject: some set of required leaves holds entirely. A holder with no
// requirement, or with the filter off, is always woken.
func (e *Encounter) couldBeWokenBySeeing(holder, subject MemberID) bool {
	wants, ok := e.seenWants[holder]
	if !ok || !seenFilterOn {
		return true
	}
	for _, set := range wants {
		all := true
		for _, l := range set {
			if !e.seenLeafHolds(holder, subject, l) {
				all = false

				break
			}
		}
		if all {
			return true
		}
	}

	return false
}
