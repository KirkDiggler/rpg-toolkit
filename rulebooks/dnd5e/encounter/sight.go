// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
)

// Sight reports how far each of the given members can see, in cells. The
// composition asks; the rulebook answers.
//
// Injected rather than held, exactly as [Standing] and [InitiativeRoller] are,
// and for the same reason [Standing]'s doc gives with "hit points" swapped for
// "light": this module's go.mod cannot import the rulebook (law C1), so how far
// a creature can see is a fact it can only be TOLD. Member IDs in, one number
// out per member — nothing here learns what a torch is, or what darkvision is,
// or what a spell that puts out both is, and nothing here has to.
//
// # Per member, which is the whole design
//
// The question is "how far can THIS member see", never "how far can anyone
// see". A flat range on [SetupInput] would have been cheaper and would have
// been a rule 5e does not have: sight is per-creature and per-light-source, so
// the dwarf with darkvision and the human holding her torch answer differently
// while standing on the same cell. A single number could only ever have been
// REPLACED by the real model, never grown into it — which is the difference
// between a slice that pays for itself and one the next slice deletes
// (rpg-toolkit#1111).
//
// # Today's answer is one number, and that is the point
//
// An implementation may return the same number for everybody, and the fixtures
// in this module's own tests do. What is load-bearing is the SHAPE: the light
// model 5e actually states — bright light, dim light, darkness, darkvision,
// blindness, magical darkness — arrives as this capability returning a better
// ANSWER, with nothing else in this module moving. That is the promise
// [Standing] made and kept: rpg-toolkit#977's Undead Fortitude will change what
// "down" means without the composition learning a thing about hit points, and
// the light model will change what "can see" means without it learning a thing
// about light.
//
// # Cells, not feet, and the conversion is yours
//
// The number is a count of CELLS on the map, because cells are the only
// distance this module has: it measures with the grid's own metric, which is
// Chebyshev on squares and cube distance on hexes. Feet are a 5e unit, and "a
// square is five feet" is a 5e RULE — one this module is not allowed to know,
// for the same reason it is not allowed to know what a hit point is. So 60-foot
// darkvision is 12, and the division is the rulebook's, which is the side that
// knows both halves. The conversion is exact rather than approximate:
// Chebyshev is 5e's own "a diagonal costs five feet" measure.
//
// Zero is a legal answer and means blind — nothing but the cell underfoot,
// which nobody is ever asked about. A negative is not a shorter answer; it is
// not an answer, and it is refused (ErrNoSight).
//
// # It is a pull, and it is never remembered
//
// The composition stores no sight range, in memory or in its blob. It asks at
// the choke point where percepts are built and uses the answer it gets, once,
// for that refresh. The alternative — being handed a range at construction and
// keeping it — recreates the dual state [Standing]'s doc describes: light the
// torch and the character sheet says sixty feet while the composition still
// says twenty.
//
// Being a pull is also what makes it COMPLETE. Every route to a changed range —
// a torch lit, a torch dropped, a blindness spell, a light source another
// creature is carrying, a rule nobody has written yet — is noticed at the next
// refresh, without that route knowing this interface exists.
//
// # What an implementation owes
//
// Answer about the members you were asked about, and answer about ALL of them.
// A name in the answer that was not in the question is a defect this module
// refuses (ErrNotMember) rather than ignores, exactly as [Standing] does. A
// member in the QUESTION missing from the answer is refused too (ErrNoSight),
// and that half is stricter than [Standing]'s on purpose: absence from a
// standing answer means "standing", but absence from this one means nothing at
// all, and a member with no supplied range is a member whose sight this module
// would have to invent. Inventing it is the thing rpg-toolkit#1033 forbids.
//
// The answer is asked for again rather than carried between refreshes. Carrying
// it would be a cache, and a cache is the smallest possible version of the dual
// state above.
//
// Errors abort whatever verb was running, atomically (R5), the same as
// [InitiativeRoller]'s and [Standing]'s: a world that cannot find out how far
// somebody can see does not half-build a percept on a guess.
//
// # The asymmetry this creates is real, and it is not stealth
//
// Two members can answer differently, so the dwarf may see the goblin from
// forty feet while the goblin, in the dark, sees nothing back. Sight stays
// symmetric in the GEOMETRIC sense — spatial v0.9.1 pins line of sight as a
// mutual property over fuzzed rooms in every grid family, and this capability
// does not touch that — and what differs is reach.
//
// That asymmetry is not rpg-toolkit#1020 and does not stand in for it. #1020 is
// a Stealth contest against passive Perception, deciding whether an observer's
// percept holds a subject that is in plain view; this decides how far a percept
// extends. They compose on the same seam: #1020's per-observer answer rides
// into [Encounter.rebuildPercepts] beside this one, filtering what survives the
// range and the ray, and neither has to know the other exists.
//
// What the asymmetry DOES do, today, is make 5e surprise producible for the
// first time. See [Encounter.classify]: the spotted and drop arms were written
// against asymmetric sight and had no reachable input while every observer saw
// the same distance. They have one now, and classification did not move.
type Sight interface {
	// Sight reports how far each of the given members can see, in cells. Every
	// member asked about must appear in the answer, and nobody else may.
	Sight(members []MemberID) (map[MemberID]int, error)
}

// sightNow asks the capability how far each member can see, right now, and
// returns the answer keyed by member.
//
// The roster goes over SORTED, for C8's reason and [Encounter.standingNow]'s:
// what a pass concludes must be a function of persisted data rather than of map
// iteration order, and a capability handed its question in a different order
// each time could answer differently each time. An empty roster is not a
// question worth asking, and the capability is not called for one.
//
// The whole roster is asked about rather than only this refresh's observers,
// which is the same choice [Encounter.standingNow] makes and for the same
// reason: the question a rulebook receives should not depend on which verb
// happens to be running. Every caller of [Encounter.rebuildPercepts] passes the
// roster anyway.
//
// Three refusals, and each one exists to make a mis-wired capability LOOK like
// a mis-wired capability rather than like a rule that silently never fires: a
// stranger in the answer (ErrNotMember, exactly as [Standing] does), a member
// the answer skipped (ErrNoSight), and a distance that is not one (ErrNoSight).
// See [Sight] for why the second is stricter here than it is there.
func (e *Encounter) sightNow() (map[MemberID]int, error) {
	if len(e.members) == 0 {
		return nil, nil
	}

	roster := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		roster = append(roster, id)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i] < roster[j] })

	reported, err := e.sight.Sight(roster)
	if err != nil {
		return nil, fmt.Errorf("sight: %w", err)
	}

	for id, cells := range reported {
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("sight: reported a range for %q, who is not a member: %w", id, ErrNotMember)
		}
		if cells < 0 {
			return nil, fmt.Errorf("sight: reported %d cells for %q, which is not a distance: %w", cells, id, ErrNoSight)
		}
	}

	for _, id := range roster {
		if _, ok := reported[id]; !ok {
			return nil, fmt.Errorf("sight: did not say how far %q can see: %w", id, ErrNoSight)
		}
	}

	return reported, nil
}
