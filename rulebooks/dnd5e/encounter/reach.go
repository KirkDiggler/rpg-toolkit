// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// reach.go is INTERACTION BY REACH (rpg-project#753, R1 amended by R4): who
// and what stands near enough to a member, and is seen by them now, for the
// session to read an offer from. Nothing farther is asked about, so the read
// costs the cells within reach rather than the field.

// WithinReach is every creature and every prop within reach cells of a
// member that the member sees NOW — the universe an offer within reach is
// drawn from.
//
// THE UNIVERSE, NAMED: the cells within reach of the member's own cell by
// [Encounter.Distance], the measure [Encounter.Speak]'s, Hold's and
// Interact's reach rule refuses by, so an offer drawn from this list and the
// door it is said through cannot disagree.
//
//   - Members: every other member standing within reach whom the member's
//     own sightings hold on sight right now — watched, not remembered. An
//     unsighted creature within reach is ABSENT, not refused: proximity
//     reveals nothing a map does not show.
//   - Props: every cell prop and placed footprint standing on the floor (not
//     waiting in reserve, not in somebody's hands, not a secret the member
//     has not found) with a cell within reach that the member sees now — the
//     seen-now test [Encounter.VisibleCellsFor] asks, asked of the cells
//     within reach only. A known prop behind a wall is absent: remembering a
//     thing is not seeing it. A footprint counts when any one cell it stands
//     on qualifies, as Hold measures a footprint.
//
// Both sorted by id; the member is never in their own list. Reach zero means
// 1, adjacent, as every reach does; a negative reach holds nothing.
//
// NO PROJECTION. The props come from the field's placement index, a dropped
// one from its placement record at the moment of asking.
//
// Errors: ErrNotMember (no such member), ErrBadPlacement (not placed), and
// the Sight capability's own refusal for this member.
func (e *Encounter) WithinReach(member MemberID, reach int) ([]MemberID, []PropID, error) {
	if _, ok := e.members[member]; !ok {
		return nil, nil, fmt.Errorf("within reach: %q: %w", member, ErrNotMember)
	}
	from, placed := e.canvas.GetEntityPosition(string(member))
	if !placed {
		return nil, nil, fmt.Errorf("within reach: %q: %w", member, ErrBadPlacement)
	}
	if reach == 0 {
		reach = 1
	}
	members := []MemberID{}
	props := []PropID{}
	if reach < 0 {
		return members, props, nil
	}
	within := func(cell spatial.Position) bool { return e.Distance(from, cell) <= float64(reach) }

	sighted, err := e.creaturesSightedBy(member)
	if err != nil {
		return nil, nil, fmt.Errorf("within reach: %w", err)
	}
	for id := range sighted {
		if id == member {
			continue
		}
		if _, ok := e.members[id]; !ok {
			continue
		}
		if at, placed := e.canvas.GetEntityPosition(string(id)); placed && within(at) {
			members = append(members, id)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i] < members[j] })

	seesNow, err := e.seenNow(member, from)
	if err != nil {
		return nil, nil, fmt.Errorf("within reach: %w", err)
	}
	seen := make(map[spatial.Position]bool)
	for _, cell := range e.cellsWithin(from, reach) {
		if e.field.isFloor(cell) && seesNow(cell) {
			seen[cell] = true
		}
	}
	found := make(map[PropID]bool)
	offer := func(id PropID, cells []spatial.Position) {
		if found[id] || e.hiddenPropTo(member, id) {
			return
		}
		for _, cell := range cells {
			if seen[cell] {
				found[id] = true
				return
			}
		}
	}

	// WHAT THE AUTHOR STOOD THERE AND CANNOT HAVE MOVED, from the index by
	// cell. A thing that can move is asked about below, from its record.
	for cell := range seen {
		for _, ref := range e.field.index.on(cell) {
			switch ref.kind {
			case refPlaced:
				if !e.field.placed[ref.at].mutable() {
					offer(ref.id, []spatial.Position{cell})
				}
			case refCellProp:
				if !cellPropMutable(&e.field.props[ref.at]) {
					offer(ref.id, []spatial.Position{cell})
				}
			case refWall:
				// A wall is not a thing a line is said to.
			}
		}
	}

	// WHAT CAN MOVE, where its record says it stands right now — the
	// holdable and the reserved, the same fold every other read asks.
	placements := e.holdings.propPlacements()
	for i := range e.field.props {
		p := &e.field.props[i]
		if p.ID == "" || !cellPropMutable(p) {
			continue
		}
		at := e.field.cellAt(p.At)
		state, moved := placements[p.ID]
		if p.Arrives != nil && !state.arrived {
			continue
		}
		if moved && state.gone {
			continue
		}
		if moved {
			at = state.at
		}
		offer(p.ID, []spatial.Position{at})
	}
	now := e.field.placedNow()
	for i := range e.field.placed {
		p := &e.field.placed[i]
		if !p.mutable() {
			continue
		}
		placement, standing := now.stands(p)
		if !standing {
			continue
		}
		offer(p.id, e.field.placedCellsOf(p.id, placement))
	}

	for id := range found {
		props = append(props, id)
	}
	sort.Slice(props, func(i, j int) bool { return props[i] < props[j] })

	return members, props, nil
}

// PropWithinReach answers whether a member may address a prop by reach —
// THE ENCOUNTER'S OWN DOOR, exported so a host's pre-charge check and
// [Encounter.Speak] cannot disagree (rpg-project#753, S3): nil when the
// member is shown the prop and any cell it stands on now is within reach
// cells by [Encounter.Distance] (zero means 1, as every reach does), the
// footprint measured as Hold measures one; a prop the member carries is
// always within reach.
//
// SHOWN, NOT SEEN NOW. This is the door's question, wider than
// [Encounter.WithinReach]'s offer: a prop the member remembers behind them
// is not offered, and saying a line to it is still admitted. Every prop
// WithinReach lists, this admits.
//
// Errors, in Speak's order: ErrNotMember, ErrBadPlacement (not placed),
// ErrNoProp (no such prop, or one the member is not shown — a hidden thing
// far off is "no such thing", never "out of range"), then ErrOutOfRange.
func (e *Encounter) PropWithinReach(member MemberID, prop PropID, reach int) error {
	if _, ok := e.members[member]; !ok {
		return fmt.Errorf("prop within reach: %q: %w", member, ErrNotMember)
	}
	from, placed := e.canvas.GetEntityPosition(string(member))
	if !placed {
		return fmt.Errorf("prop within reach: %q: %w", member, ErrBadPlacement)
	}
	_, cells, err := e.speakableProp(member, prop)
	if err != nil {
		return fmt.Errorf("prop within reach: %w", err)
	}

	return e.refusePropOutOfReach("prop within reach", from, prop, cells, reach)
}

// cellPropMutable reports whether a cell prop can stand anywhere but its
// authored cell, or be off the floor: a holdable one can be taken and
// dropped, a reserved one waits to arrive.
func cellPropMutable(p *PropInput) bool { return p.Holdable || p.Arrives != nil }

// creaturesSightedBy is every member the observer's own sightings hold on
// sight right now — watched, not remembered ([Encounter.currentlySees]'s
// question, asked of the whole view in one read).
func (e *Encounter) creaturesSightedBy(observer MemberID) (map[MemberID]bool, error) {
	holdings, err := e.memberIntel(observer)
	if err != nil {
		return nil, err
	}
	out := make(map[MemberID]bool, len(holdings))
	for _, h := range holdings {
		if h.CurrentOn(perception.Sight) {
			out[h.Subject] = true
		}
	}

	return out, nil
}

// cellsWithin is every grid cell within reach of from by [Encounter.Distance]:
// the hex disk, floor or not, walked ring by ring from the centre. The grid's
// own neighbours bound the walk, so it visits the disk and its rim only.
func (e *Encounter) cellsWithin(from spatial.Position, reach int) []spatial.Position {
	seen := map[spatial.Position]bool{from: true}
	out := []spatial.Position{from}
	frontier := []spatial.Position{from}
	for step := 0; step < reach; step++ {
		var next []spatial.Position
		for _, cell := range frontier {
			for _, n := range adjacencyGrid.GetNeighbors(cell) {
				if seen[n] {
					continue
				}
				seen[n] = true
				if e.Distance(from, n) <= float64(reach) {
					out = append(out, n)
					next = append(next, n)
				}
			}
		}
		frontier = next
	}

	return out
}
