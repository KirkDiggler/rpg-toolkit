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
// THE TARGET IS A REGION, NEVER A DOOR: a player cannot target structure
// they do not know exists, so the verb names a place to sweep and the world
// decides what that place holds. Universally attemptable — no prerequisites,
// no class gate; the barbarian may search. In v1 the named region must be
// the one the searcher stands in — presence is the host's truth.
//
// THE ANSWER NEVER LEAKS THE QUESTION. An empty region and a failed check
// return the same bytes: the output carries nothing that varies with what
// the region holds — no found list, no roll, no count, no seq — and a
// failed check writes no fact and no beat, so even the persisted blob is
// identical. A find reaches the searcher the way all world change reaches
// anybody: as a recipient-scoped DOOR_REVEALED beat, audience the searcher
// alone. Finding a door never reveals the region behind it — two knowledge
// moments, deliberately distinct — with one exception that is not an
// exception: a door found standing OPEN is a door perceived open, and
// perceiving a concealed door open is what reveals its region.

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

// Search sweeps a region's concealed declarations: every concealed door
// with an edge in the region that the searcher has not already found rolls
// its find check through the injected [CheckResolver] — the resolver
// applies the searcher's best listed approach — and a success writes the
// location fact with audience = the searcher alone, plus their
// DOOR_REVEALED beat.
//
// A field with no concealment accepts the verb and sweeps nothing: refusing
// it would itself answer the question a search asks.
//
// Search refreshes no sight — nothing moved and no geometry changed — but
// it ends with the same concealment sweep every sight refresh runs, so a
// find whose door stands open grants the region reveal through the one
// mechanism that owns perception rather than a special case here.
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
	for _, doorID := range e.world.concealedDoors {
		d := e.doorsByID[doorID]
		if !e.doorTouchesRegion(d, in.Region) || e.world.knowsDoor(in.Member, d.id) {
			continue
		}

		verdict, err := e.checkResolver.ResolveCheck(&ResolveCheckInput{
			Member:     in.Member,
			Approaches: append([]CheckApproach(nil), d.concealed...),
		})
		if err != nil {
			return nil, fmt.Errorf("search: resolve find for door %q: %w", d.id, err)
		}
		if !verdict.Beaten {
			continue
		}
		if err := e.revealDoorTo(in.Member, d, "found it by search", at); err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
	}

	// The sweep, for the found-open case and anything else present state
	// now forces — same call every sight refresh makes.
	if err := e.sweepConcealment(); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	return &SearchOutput{}, nil
}

// doorTouchesRegion reports whether any edge endpoint of a door stands in
// the region — the membership test a region sweep uses.
func (e *Encounter) doorTouchesRegion(d *doorRecord, region RegionID) bool {
	for _, edge := range d.edges {
		for _, cell := range []spatial.Position{edge.From, edge.To} {
			if r, owned := e.field.regionOf(cell); owned && r == region {
				return true
			}
		}
	}
	return false
}
