// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SearchInput selects the legacy explicit search of a region.
// Deprecated: game hosts supply DiscoveryCheckResolver and use proximity.
type SearchInput struct {
	Member MemberID
	Region RegionID
}

// SearchOutput acknowledges a legacy search without disclosing hidden content.
// Deprecated: automatic discovery reports recipient-scoped check/reveal beats.
type SearchOutput struct{}

// Search preserves the explicit-search contract for existing hosts that do not
// supply automatic discovery. An automatic-discovery host refuses this path so
// it cannot bypass proximity or the attempt budget. The game no longer exposes
// an active Search RPC or button. Automatic hosts return ErrSearchRetired.
func (e *Encounter) Search(in *SearchInput) (*SearchOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("search: %w", ErrNilInput)
	}
	if _, automatic := e.checkResolver.(DiscoveryCheckResolver); automatic {
		return nil, fmt.Errorf("search: %w", ErrSearchRetired)
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
		return nil, fmt.Errorf("search: %w", ErrBadPlacement)
	}
	standing, owned := e.field.regionOf(cell)
	if !owned || in.Region == "" || in.Region != standing {
		return nil, fmt.Errorf("search: %w", ErrElsewhere)
	}
	at := uint64(e.clock.ToData().HighWater)
	for _, id := range e.world.concealments {
		c := e.field.concealmentOf(id)
		if c == nil || e.world.knowsConcealment(in.Member, c.id) || !e.concealmentTouchesRegion(c, in.Region) {
			continue
		}
		verdict, err := e.checkResolver.ResolveCheck(&ResolveCheckInput{Member: in.Member, Approaches: append([]CheckApproach(nil), c.checks...)})
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
		if verdict.Beaten {
			if err = e.revealConcealmentTo(in.Member, c, "found it by search", at); err != nil {
				return nil, err
			}
		}
	}
	if err := e.sweepConcealment(); err != nil {
		return nil, err
	}
	if err := e.spendWorldAction(in.Member); err != nil {
		return nil, err
	}
	return &SearchOutput{}, nil
}

func (e *Encounter) concealmentTouchesRegion(c *concealment, region RegionID) bool {
	inOrBeside := func(cell spatial.Position) bool {
		if r, owned := e.field.regionOf(cell); owned && r == region {
			return true
		}
		for _, neighbor := range adjacencyGrid.GetNeighbors(cell) {
			if r, owned := e.field.regionOf(neighbor); owned && r == region {
				return true
			}
		}
		return false
	}
	for _, cell := range e.hiddenCellsOf(c) {
		if inOrBeside(cell) {
			return true
		}
	}
	for _, cell := range e.memberPropCells(c) {
		if inOrBeside(cell) {
			return true
		}
	}
	for _, id := range c.doors {
		if d := e.doorsByID[id]; d != nil {
			for _, cell := range e.doorFootprintCells(d) {
				if inOrBeside(cell) {
					return true
				}
			}
			for _, edge := range d.edges {
				for _, cell := range []spatial.Position{edge.From, edge.To} {
					if r, owned := e.field.regionOf(cell); owned && r == region {
						return true
					}
				}
			}
		}
	}
	return false
}
