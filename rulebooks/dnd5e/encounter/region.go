// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// region.go is A REGION IS A NAMED SET OF CELLS (rpg-toolkit#1108,
// rpg-project#256).
//
// S0 made the field one canvas and left the authored chambers with exactly one
// runtime job: saying which of them holds a given cell. #256 made that the
// whole of what a region IS at authoring time too: a region lists its cells,
// the floor is their union, and this file answers which region holds a cell
// and which members stand in one. It is not a coordinate space, not a
// container, and not a visibility rule: a member's cell is the canvas's to
// know, and their region is derived from it.

// RegionID names one region — the authored region's own ID (RegionInput.ID),
// carried through the compile unchanged.
//
// An alias rather than a defined type, following MemberID: it exists to say
// which of two strings a signature means, not to make callers convert.
type RegionID = string

// RegionAt reports which region holds a dungeon-absolute cell.
//
// TOTAL OVER OWNED FLOOR, AND ONLY OVER IT: a cell has a region if and only if
// exactly one region owns it. The canvas spans the field's whole bounding BOX,
// so "on the map" and "somewhere a member can stand" are different questions,
// and this answers the second one.
//
// FALSE NO LONGER MEANS VOID (rpg-project#360). Since scenery, an unowned cell
// is void OR floor nobody stands on, and this read cannot tell them apart —
// which is right, because it is the OWNERSHIP question and scenery's answer to
// it is genuinely "nobody". A caller that wants to know whether there is
// ground on a cell is asking a different question, and [Encounter.Atlas]
// answers it: a cell in Cells and in no region's cells is scenery. What has
// not changed is what this read is FOR: both a void cell and a scenery cell
// are cells Step and Join refuse to place anybody on.
//
// W2 (regions never overlap) makes ownership unique, and it is enforced at
// construction rather than checked here: the owner map is built by
// compileField, which refuses a cell listed twice, so the lookup is a lookup.
//
// A fractional axial position is not a cell at all, and no region holds it:
// the map is keyed by whole cells and a fractional key simply misses.
//
// A DOORWAY DOES NOT GET ITS OWN ANSWER, and that is a decision rather than an
// omission. The old top-level encounter module made a door's cell belong to no
// region on purpose, because its compiler put a one-cell wall column between
// chambers and carved a floor cell into it. This composition has no such cell:
// a door stands on the EDGE between two adjacent floor cells (rpg-toolkit#1123),
// and a cell no region owns is not floor here, so "belongs to no region" and
// "you cannot stand there" would be the same sentence. A member in a doorway
// is therefore in the region whose cell is under their feet, and the member
// facing them one cell away is in the other. Pinned by
// TestAMemberInTheDoorwayStandsInTheRegionTheyStandOn.
//
// A host that wants "is this cell an opening" is asking a different question,
// and [Encounter.Atlas] answers it: every doorway reports both cells in
// absolute space.
func (e *Encounter) RegionAt(cell spatial.Position) (RegionID, bool) {
	return e.field.regionOf(cell)
}

// Region reports one region as authored: its name, its cells in absolute
// space (sorted), its archetype and its lighting.
//
// Returns ErrNoRegion for a region the field does not have, for
// [Encounter.MembersIn]'s reason. Copy-out: the cell slice is freshly
// allocated per call.
func (e *Encounter) Region(id RegionID) (AtlasRegion, error) {
	for _, r := range e.field.regions {
		if r.ID != id {
			continue
		}

		return AtlasRegion{
			ID:        r.ID,
			Name:      r.Name,
			Cells:     append([]spatial.Position(nil), e.field.regionCells[r.ID]...),
			Archetype: r.Archetype,
			Lighting:  *r.Lighting,
			Concealed: r.Concealed,
		}, nil
	}

	return AtlasRegion{}, fmt.Errorf("region %q: %w", id, ErrNoRegion)
}

// MembersIn reports who is standing in a region, in the same stable order (and
// with the same placement) [Encounter.Members] reports them.
//
// A ROSTER READ, filtered — deliberately built from the same projection every
// other member read uses (placementOf), because Members and MembersIn
// disagreeing about where somebody stands is the dual-state defect this
// composition has paid for before.
//
// Returns ErrNoRegion for a region the field does not have. An EMPTY region is
// an ordinary answer — nobody has reached the tomb yet is a fact worth
// reporting — so a mistyped name must not be able to say it. Returns ErrNoField
// if a member's cell cannot be resolved, for [Encounter.Members]' reason.
func (e *Encounter) MembersIn(region RegionID) ([]Member, error) {
	if _, ok := e.field.regionCells[region]; !ok {
		return nil, fmt.Errorf("members in %q: %w", region, ErrNoRegion)
	}

	all, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("members in %q: %w", region, err)
	}

	in := make([]Member, 0, len(all))
	for _, m := range all {
		if m.Region == region {
			in = append(in, m)
		}
	}
	return in, nil
}
