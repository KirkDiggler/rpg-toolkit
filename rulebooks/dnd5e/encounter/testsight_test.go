// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// unlimitedSight is the range these tests hand out when light is not their
// subject: further than the longest sightline any canvas in this package draws,
// which is the reference tomb's 27 cells.
const unlimitedSight = 1_000_000

// everyoneSeesTheWholeMap is the Sight capability these tests install by
// default.
//
// Nobody is ever bounded by distance, which is what every scene written before
// sight had a distance term was already assuming. Installing it explicitly
// rather than letting a nil mean it is the whole point of the capability being
// required: a scene states what it believes about light out loud
// (rpg-toolkit#1033 — capabilities are supplied, never defaulted).
type everyoneSeesTheWholeMap struct{}

func (everyoneSeesTheWholeMap) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	return sameForEveryone(members, unlimitedSight), nil
}

// sameForEveryone answers the same distance for every member asked about, which
// is what a v1 rulebook with no light model does.
func sameForEveryone(members []encounter.MemberID, cells int) map[encounter.MemberID]int {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = cells
	}

	return out
}

// sightList reports a caller-set distance per member, and can be changed
// MID-SCENE — which is how a test puts out a torch, or lights one, without
// rebuilding the encounter. That is the only way to test a pull: if the
// composition cached the answer, changing this would change nothing.
//
// It answers about exactly the members it was ASKED about, because that is what
// a real implementation does. Anybody with no entry in reach gets fallback, so a
// scene names only the members whose light it cares about.
type sightList struct {
	reach    map[encounter.MemberID]int
	fallback int
}

func (s *sightList) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		if cells, ok := s.reach[id]; ok {
			out[id] = cells
			continue
		}
		out[id] = s.fallback
	}

	return out, nil
}

// sightStrangerWhenTold is a rulebook that starts well-behaved and then answers
// about somebody who is not in the encounter at all — the mis-wiring this
// capability could arrive as, held here so the composition's refusal can be
// asserted from a scene that was already running.
type sightStrangerWhenTold struct {
	lying bool
}

func (s *sightStrangerWhenTold) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := sameForEveryone(members, unlimitedSight)
	if s.lying {
		out["a-ghost"] = 4
	}

	return out, nil
}

// sightSkippingWhenTold is a rulebook that stops covering one of the members it
// was asked about. Absence means nothing in a sight answer, so the composition
// has nothing to fall back on and must say so.
type sightSkippingWhenTold struct {
	skip encounter.MemberID
}

func (s *sightSkippingWhenTold) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := sameForEveryone(members, unlimitedSight)
	if s.skip != "" {
		delete(out, s.skip)
	}

	return out, nil
}

// sightBelowZero answers with a number that is not a distance.
type sightBelowZero struct {
	who   encounter.MemberID
	cells int
}

func (s *sightBelowZero) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := sameForEveryone(members, unlimitedSight)
	out[s.who] = s.cells

	return out, nil
}

// errRulebookCannotSee is what a rulebook that cannot answer looks like from the
// composition's side — a content store that is down rather than a creature in
// the dark.
var errRulebookCannotSee = errors.New("the rulebook cannot say how far anyone can see")

// sightBrokenWhenTold is a rulebook that starts well-behaved and then stops
// answering, so the R5 arm can be driven from a scene that was already running.
type sightBrokenWhenTold struct {
	broken bool
}

func (s *sightBrokenWhenTold) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	if s.broken {
		return nil, errRulebookCannotSee
	}

	return sameForEveryone(members, unlimitedSight), nil
}

// countingSight is everyoneSeesTheWholeMap that remembers what it was asked and
// how often, which is how a test tells "the consult did not fire" from "the
// consult found nothing to bound".
type countingSight struct {
	calls int
	asked [][]encounter.MemberID
}

func (c *countingSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	c.calls++
	c.asked = append(c.asked, append([]encounter.MemberID(nil), members...))

	return sameForEveryone(members, unlimitedSight), nil
}
