// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// region.go is A ROOM IS A REGION (rpg-toolkit#1108).
//
// S0 made the field one canvas and left the authored chambers with exactly one
// runtime job: saying which of them holds a given cell. This file gives that
// job a name and a surface. A region is A NAMED SET OF CELLS — you ask it which
// region holds a cell, or which members stand in one. It is not a coordinate
// space, not a container, and not a visibility rule: a member's cell is the
// canvas's to know, and their region is derived from it.
//
// ROOMS ARE STILL HOW A DUNGEON IS AUTHORED (Kirk's ruling on rpg-toolkit#1105:
// "we author rooms; that they become one thing is after render"). RoomInput,
// MemberInput.Room and TriggerReachedPosition.Room are construction data and
// stay room-shaped. What changed is the word the RUNTIME answers in: after the
// compile there is one map, and the authored chambers survive on it as named
// regions over its cells.

// RegionID names one region — the authored chamber's own ID (RoomInput.ID),
// carried through the compile unchanged.
//
// An alias rather than a defined type, following MemberID: it exists to say
// which of two strings a signature means, not to make callers convert.
type RegionID = string

// RegionAt reports which region holds a dungeon-absolute cell.
//
// TOTAL OVER FLOOR, AND ONLY OVER FLOOR: a cell is floor if and only if exactly
// one region owns it. The canvas spans the field's whole bounding BOX, so
// "on the map" and "somewhere a member can stand" are different questions, and
// this answers the second one. False means the cell is void — the space between
// or beside the authored chambers — which Step and Join refuse to place anybody
// on.
//
// W2 (rooms never overlap, so regions do not either) plus integral origins make
// ownership unique, so iteration order never matters: at most one region's
// bounds check can pass.
//
// A SQUARE region is fractional-tolerant and a member may legitimately stand
// between its cells (RoomInput.Grid's doc comment, and Atlas's on that
// asymmetry); such a member is in the region whose span contains them, by the
// same rule. A HEX region is not: a fractional axial position is not a cell at
// all, and no region holds it — the rule, and why it lives one layer down, are
// regionAt's.
//
// A DOORWAY DOES NOT GET ITS OWN ANSWER, and that is a decision rather than an
// omission. The old stack made a door's cell belong to no region on purpose —
// its compiler puts a one-cell wall column between chambers and carves a floor
// door cell into it, so there is a real cell between two regions to leave
// unnamed. This composition has no such cell: a connection's two endpoints are
// room-local cells OF THEIR OWN ROOMS (ConnectionInput's doc comment) and W3
// makes them adjacent, so nothing sits between them. And a cell no region owns
// is not floor here, so "belongs to no region" and "you cannot stand there"
// would be the same sentence — an unnamed doorway would be a doorway nobody
// could stand in. A member in a doorway is therefore in the region whose cell
// is under their feet, and the member facing them one cell away is in the
// other. Pinned by TestAMemberInTheDoorwayStandsInTheRegionTheyStandOn.
//
// A host that wants "is this cell an opening" is asking a different question,
// and [Encounter.Atlas] answers it: every doorway reports both endpoint cells
// in absolute space. Standing in one is a fact about the doorway, not about
// which region holds you.
func (e *Encounter) RegionAt(cell spatial.Position) (RegionID, bool) {
	return regionAt(e.fieldInput, e.roomGrids, cell)
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
	if _, ok := e.roomGrids[region]; !ok {
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

// regionAt is THE region lookup — the one place this package turns a
// dungeon-absolute cell into a room-local one to ask who owns it.
//
// A free function rather than a method because the load seam needs the same
// answer before an Encounter exists (R5: a blob is validated in full before
// anything is constructed). That used to be three implementations of one
// question — Encounter.roomAt (gone with this slice), a load-time twin that
// said so in its own doc comment, and an inline third inside outcome
// validation — which is the dual-dispatch defect #1059 spent two PRs deleting
// for movement, grown back around ownership. TestRegionOwnershipIsAskedInOneFunction is what keeps it
// at one: spatial.Position.Subtract appears in exactly one function body in
// this package, and a second lookup cannot exist without subtracting an origin.
//
// Each region is asked with its OWN constructed grid, kept from construction,
// so this answers exactly what the authored room itself would.
//
// THE INTEGRALITY CHECK IS HERE, and it did not used to be. While this was
// Encounter.roomAt the check lived only at the verbs — every way a cell
// reached it had already asked (Step, Join, and Load each name a fractional
// axial cell as itself, which is a better error than "not floor"), so a check
// here was a branch no input could take, which is a branch no test could pin.
//
// Making the question PUBLIC made that branch reachable. A hex grid's
// IsValidPosition bounds-checks and nothing more (isIntegralAxialPosition's doc
// comment says so, and tools/spatial's AxialHexGrid is where it is true), so
// without this, RegionAt answered "yes, the hall holds it" for an axial
// position Join refuses as not a cell — measured, before the fix:
//
//	RegionAt((3.5, -1.5)) on a hex field = ("only", true)
//	Join at the same position: not an integral axial cell: bad placement
//
// Two answers to "is this floor" is exactly what this file exists to prevent,
// so the predicate answers the question it claims to. HEX ONLY: square is
// fractional-tolerant by design, and a square member standing between cells is
// really in the region whose span holds them. (Found by Copilot on PR #1109.)
func regionAt(rooms []RoomInput, grids map[string]spatial.Grid, cell spatial.Position) (RegionID, bool) {
	for _, ri := range rooms {
		grid, ok := grids[ri.ID]
		if !ok {
			continue
		}
		local := cell.Subtract(ri.Origin)
		if !isIntegralAxialPosition(grid, local) {
			continue
		}
		if !grid.IsValidPosition(local) {
			continue
		}
		return ri.ID, true
	}
	return "", false
}
