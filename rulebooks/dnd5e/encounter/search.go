// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// search.go is THE SEARCH VERB'S RULE HALF (rpg-toolkit#1371; ruled on
// rpg-project#350): the session's Search verb calls this, and everything
// that keeps a secret secret is decided here rather than at the seam.
//
// THE TARGET IS A REGION, NEVER A SECRET: a player cannot target structure
// they do not know exists, so the verb names a place to sweep and the world
// decides what that place holds. Universally attemptable — no prerequisites,
// no class gate; the barbarian may search. In v1 the named region must be
// the one the searcher stands in — presence is the host's truth.
//
// ONE ROLL PER CONCEALMENT, never per hidden thing (rpg-project#490, E2). A
// vault hidden behind a door, a bookcase and three cells is ONE secret and
// one check: rolling per member would give a searcher several chances at the
// same wall and would leak, through the number of rolls, how much was there.
//
// THE ANSWER NEVER LEAKS THE QUESTION. An empty region and a failed check
// return the same bytes: the output carries nothing that varies with what
// the region holds — no found list, no roll, no count, no seq — and a
// failed check writes no fact and no beat, so even the persisted blob is
// identical. A find reaches the searcher the way all world change reaches
// anybody: as a recipient-scoped CONCEALMENT_REVEALED beat, audience the
// searcher alone.

// SearchInput declares a search of a region.
type SearchInput struct {
	// Member is who searches.
	Member MemberID

	// Region is the region to sweep — where the searcher stands, in v1.
	// Any other region is refused (ErrElsewhere), and a region that does
	// not exist refuses IDENTICALLY: the searcher is not standing in it,
	// and that is the whole of what the refusal says. A distinct
	// no-such-region answer would let a guessed ID probe for hidden rooms.
	Region RegionID
}

// SearchOutput acknowledges that the search happened, and deliberately
// nothing more.
//
// Empty is the design, not an economy: any field here would have to be
// constant across "found something", "found nothing", and "there was
// nothing to find", or it would tell a failed searcher whether there had
// been something to find — the one thing a failed search must not learn.
type SearchOutput struct{}

// Search sweeps the concealments TOUCHING a region: every one the searcher
// has not already found rolls its checks through the injected
// [CheckResolver] — the resolver applies the searcher's best listed
// approach — and a success writes the knowledge fact with audience = the
// searcher alone, plus their CONCEALMENT_REVEALED beat.
//
// TOUCHING is [Encounter.concealmentTouchesRegion]: a cell the concealment
// hides that is in the region or next to one of its cells, or a door of it
// with an edge endpoint in the region. Next to, and not only in, because a
// secret's whole point is that its floor is not floor anyone can see they
// are standing beside.
//
// A field that hides nothing accepts the verb and sweeps nothing: refusing
// it would itself answer the question a search asks.
//
// Search refreshes no sight — nothing moved and no geometry changed — but
// it ends with the same concealment sweep every sight refresh runs, so
// anything present state now forces lands through the one mechanism that
// owns perception rather than a special case here.
//
// Validation order (R5): nil input → empty member → closed → not a member →
// not standing in the named region. Errors: ErrNilInput, ErrNoMember,
// ErrClosed, ErrNotMember, ErrElsewhere; a [CheckResolver] error is a
// wiring fault and aborts the verb.
func (e *Encounter) Search(in *SearchInput) (*SearchOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("search: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("search: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("search: %w", ErrClosed)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("search: %w", ErrNotMember)
	}

	cell, placed := e.canvas.GetEntityPosition(string(in.Member))
	if !placed {
		return nil, fmt.Errorf("search: member %q: %w", in.Member, ErrBadPlacement)
	}
	standing, owned := e.field.regionOf(cell)
	if !owned || in.Region == "" || in.Region != standing {
		return nil, fmt.Errorf("search: member %q does not stand in region %q: %w", in.Member, in.Region, ErrElsewhere)
	}

	at := uint64(e.clock.ToData().HighWater)
	for _, id := range e.world.concealments {
		c := e.field.concealmentOf(id)
		if c == nil || e.world.knowsConcealment(in.Member, c.id) || !e.concealmentTouchesRegion(c, in.Region) {
			continue
		}

		verdict, err := e.checkResolver.ResolveCheck(&ResolveCheckInput{
			Member:     in.Member,
			Approaches: append([]CheckApproach(nil), c.checks...),
		})
		if err != nil {
			return nil, fmt.Errorf("search: resolve find for concealment %q: %w", c.id, err)
		}
		if !verdict.Beaten {
			continue
		}
		if err := e.revealConcealmentTo(in.Member, c, "found it by search", at); err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
	}

	// The sweep, for anything present state now forces — same call every
	// sight refresh makes.
	if err := e.sweepConcealment(); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// THE WORLD'S PRICE FOR AN ACTION, paid after the outcome has landed and
	// before anything refreshes sight (design §5, worldtime.go): one round on
	// the world clock for the actor, and the world thinks on it. Nothing at
	// all for a member inside a fight, where the round is what prices time.
	if err := e.spendWorldAction(in.Member); err != nil {
		return nil, err
	}

	return &SearchOutput{}, nil
}

// concealmentTouchesRegion reports whether a sweep of one region reaches a
// concealment: a cell it hides that is IN the region or ADJACENT to one of
// the region's cells, or a door of it with an edge endpoint in the region.
//
// ADJACENCY IS THE POINT, not a convenience. A hidden cell is withheld from
// the searcher's own atlas, so "the region it is in" is a question only the
// engine can answer and a vault beyond the east wall belongs to no room
// anybody is standing in. What a searcher can honestly be said to sweep is
// the floor they are on and what it borders.
//
// A FOOTPRINT DOOR IS REACHED THROUGH ITS CELLS, because it stands in no
// crossing: [Encounter.hiddenCellsOf] already counts the floor a hidden
// rectangle occupies as the concealment's, and this asks the same set rather
// than a second one.
func (e *Encounter) concealmentTouchesRegion(c *concealment, region RegionID) bool {
	for _, cell := range e.hiddenCellsOf(c) {
		if r, owned := e.field.regionOf(cell); owned && r == region {
			return true
		}
		for _, neighbor := range adjacencyGrid.GetNeighbors(cell) {
			if r, owned := e.field.regionOf(neighbor); owned && r == region {
				return true
			}
		}
	}
	for _, id := range c.doors {
		d, ok := e.doorsByID[id]
		if !ok {
			continue
		}
		for _, edge := range d.edges {
			for _, cell := range []spatial.Position{edge.From, edge.To} {
				if r, owned := e.field.regionOf(cell); owned && r == region {
					return true
				}
			}
		}
	}

	return false
}
