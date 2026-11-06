// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ApproachesInput asks where a mover's OWN WALK would stop to have each of
// the named members within a reach and in sight (rpg-project#872, D4).
type ApproachesInput struct {
	// Mover is the member who would walk.
	Mover MemberID

	// Targets are the members to approach, answered in this order.
	Targets []MemberID

	// RangeFeet is the reach the walk must end within, in feet: a weapon's
	// reach, converted to cells by [CellsFromFeet] exactly as every range
	// check above this module converts it. Negative is [ErrBadReach].
	RangeFeet int

	// Budget is how many steps the walk may take: a fight's remaining
	// movement in cells. Zero is a walk that goes nowhere, so only a target
	// already in reach is reached. Negative is [ErrBadReach]. Ignored when
	// Unbounded.
	Budget int

	// Unbounded is a walk with no budget, the world clock's (free roam has
	// no movement to spend). The zero value is bounded, so a caller that
	// forgets to say which gets the narrower answer, never the whole floor.
	Unbounded bool
}

// Approach is where one walk ends for one target.
type Approach struct {
	// Target is the member approached.
	Target MemberID

	// Reached is whether the walk ends within reach and in sight of the
	// target. False is "no walk within the budget does", and the fields below
	// are empty.
	Reached bool

	// Cell is the cell the walk ends on: the mover's own cell when the target
	// is already in reach and in sight.
	Cell spatial.Position

	// Path is the walk, EXCLUDING the mover's cell and INCLUDING Cell, each
	// cell adjacent to the last. Empty when no walk is needed. Its length is
	// the steps the walk costs.
	Path []spatial.Position
}

// ApproachesOutput answers [Encounter.Approaches], one row per target, in
// [ApproachesInput.Targets] order.
type ApproachesOutput struct {
	Approaches []Approach
}

// approachSearches is how many targets a walk with no budget searches for
// one at a time before one unbounded flood is the cheaper read.
const approachSearches = 4

// Approaches says, for each target, the cell a mover's own walk would end on
// to have that target within RangeFeet and in sight, within the budget — the
// cheapest such cell, ties broken by [beforeInScanOrder] (C8).
//
// THE PLAYER'S WALK, NOT A PUSH. [Encounter.Route] answers a walk nobody
// chose; this answers the one the mover would take by [Encounter.Step], so
// it reads the same fold that walk meets: the same flood a creature's own
// route reads ([cellQuery.floodFrom]) gated by the mover's own picture (a
// secret they have not found is a wall), closed by a held crossing, and
// closed by a SHUT DOOR, because a player's step refuses a shut door rather
// than opening it. It may cross a cell it may not stop on; the cell it ends
// on is Standable, so a Move whose path is a shortest walk to that cell is
// never refused for its goal or its length.
//
// IN SIGHT AS WELL AS IN REACH, by the one sight test ([sightReach.Reaches],
// as [Encounter.SeesWithin] asks it) from the cell the walk ends on: a cell
// across a wall from the target is within reach and sees nothing, and an
// attack names only a member its attacker sees.
//
// A BUDGET IS ONE FLOOD, bounded by it, however many targets are asked:
// each target then reads only the cells within reach of it. NO BUDGET IS AT
// MOST ONE FLOOR. Up to [approachSearches] targets are searched for one at a
// time ([cellQuery.searchToward]), each settling the cells near its path and
// returning the flood's own cell and path (rpg-project#762, R10); the first
// search that finds nothing has settled every cell the walk reaches, and the
// targets after it read that field. More targets than that take the one
// unbounded flood (~48 ms on the 10k keep). Neither runs when every target
// is already in reach.
//
// READ LIVE, as the step reads it. A body the mover has not seen still
// stops the walk, and still shapes this answer; that is the step's own
// truth, the one a walk planned by any other fold would meet and stop at.
// That is a stated cost: the answer routes round an unseen body and never
// names a cell one stands on, which tells the mover something is there,
// accepted pre-playtest, where everyone sees everything.
//
// THE END PASSES [Encounter.ValidateWalkEnd] WITHOUT ASKING IT: that law
// refuses a cell the mover sees a pass-through body on, a current sighting's
// cell is that body's live cell, and the live fold calls that cell
// PassThrough, never Standable.
//
// IT CHANGES NOTHING. A target that is not a member is [ErrNotMember]; one
// not placed on the map is never reached.
func (e *Encounter) Approaches(in ApproachesInput) (ApproachesOutput, error) {
	if in.Mover == "" {
		return ApproachesOutput{}, fmt.Errorf("approaches: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return ApproachesOutput{}, fmt.Errorf("approaches: %w", ErrClosed)
	}
	m, ok := e.members[in.Mover]
	if !ok {
		return ApproachesOutput{}, fmt.Errorf("approaches %q: %w", in.Mover, ErrNotMember)
	}
	if in.RangeFeet < 0 {
		return ApproachesOutput{}, fmt.Errorf("approaches %q: range %d ft: %w", in.Mover, in.RangeFeet, ErrBadReach)
	}
	if !in.Unbounded && in.Budget < 0 {
		return ApproachesOutput{}, fmt.Errorf("approaches %q: budget %d cells: %w", in.Mover, in.Budget, ErrBadReach)
	}
	for _, target := range in.Targets {
		if _, ok := e.members[target]; !ok {
			return ApproachesOutput{}, fmt.Errorf("approaches %q: target %q: %w", in.Mover, target, ErrNotMember)
		}
	}
	from, err := e.cellOf(m)
	if err != nil {
		return ApproachesOutput{}, fmt.Errorf("approaches %q: %w", in.Mover, err)
	}
	ranges, err := e.sightOf([]MemberID{in.Mover})
	if err != nil {
		return ApproachesOutput{}, fmt.Errorf("approaches %q: %w", in.Mover, err)
	}
	reach := CellsFromFeet(in.RangeFeet)

	// sees is the one sight test, asked as if the mover stood on cell.
	sees := func(cell spatial.Position, target MemberID, at spatial.Position) bool {
		positions := map[MemberID]spatial.Position{in.Mover: cell, target: at}
		return e.newSightReach(positions, ranges).Reaches("sight", in.Mover, target)
	}

	out := ApproachesOutput{Approaches: make([]Approach, len(in.Targets))}
	type pending struct {
		row int
		at  spatial.Position
	}
	var todo []pending
	for i, target := range in.Targets {
		out.Approaches[i] = Approach{Target: target}
		at, placed := e.canvas.GetEntityPosition(string(target))
		if !placed {
			continue
		}
		if e.Distance(from, at) <= float64(reach) && sees(from, target, at) {
			out.Approaches[i] = Approach{Target: target, Reached: true, Cell: from}
			continue
		}
		todo = append(todo, pending{row: i, at: at})
	}
	if len(todo) == 0 {
		return out, nil
	}
	q, err := e.cellQuery()
	if err != nil {
		return ApproachesOutput{}, fmt.Errorf("approaches: %w", err)
	}
	// ends is the law for one target's cell past the walk's own edges: the
	// mover may stop there, and sees the target from it.
	ends := func(cell spatial.Position, target MemberID, at spatial.Position) (bool, error) {
		fact, err := q.cellAt(CellAtInput{Cell: cell, Mover: in.Mover})
		if err != nil {
			return false, err
		}
		return fact.Passage == PassageStandable && sees(cell, target, at), nil
	}
	// fromField answers the targets left from a field that holds every cell
	// the walk can take: those within reach of each target, cheapest first.
	fromField := func(field spatial.FieldOutput, rest []pending) error {
		for _, p := range rest {
			target := in.Targets[p.row]
			var cells []spatial.Position
			for _, cell := range e.cellsWithin(p.at, reach) {
				if _, reached := field.Dist[cell]; reached && cell != from {
					cells = append(cells, cell)
				}
			}
			sort.Slice(cells, func(i, j int) bool {
				di, dj := field.Dist[cells[i]], field.Dist[cells[j]]
				if di != dj {
					return di < dj
				}
				return beforeInScanOrder(cells[i], cells[j])
			})
			for _, cell := range cells {
				ok, err := ends(cell, target, p.at)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				if path, found := field.PathTo(cell); found {
					out.Approaches[p.row] = Approach{Target: target, Reached: true, Cell: cell, Path: path}
					break
				}
			}
		}
		return nil
	}

	if in.Unbounded && q.searchable() && len(todo) <= approachSearches {
		for i, p := range todo {
			target := in.Targets[p.row]
			path, found, field, err := q.searchTowardField(in.Mover, in.Mover, from, p.at, float64(reach), nil, false,
				func(cell spatial.Position) (bool, error) {
					if cell == from || e.Distance(cell, p.at) > float64(reach) {
						return false, nil
					}
					return ends(cell, target, p.at)
				})
			if err != nil {
				return ApproachesOutput{}, fmt.Errorf("approaches %q: %w", in.Mover, err)
			}
			if found {
				out.Approaches[p.row] = Approach{Target: target, Reached: true, Cell: path[len(path)-1], Path: path}
				continue
			}
			// A SEARCH THAT FOUND NOTHING SETTLED THE FLOOR: its field is
			// the unbounded flood, so the rest read it and nothing floods
			// again.
			if err := fromField(field, todo[i+1:]); err != nil {
				return ApproachesOutput{}, fmt.Errorf("approaches %q: %w", in.Mover, err)
			}
			break
		}
		return out, nil
	}

	// LIMIT ZERO IS THE WHOLE FLOOR ([cellQuery.floodFrom]), so a real
	// budget of zero says so before it asks.
	limit := 0
	if !in.Unbounded {
		if in.Budget == 0 {
			return out, nil
		}
		limit = in.Budget
	}
	field, err := q.floodFrom(in.Mover, in.Mover, from, nil, limit, false)
	if err != nil {
		return ApproachesOutput{}, fmt.Errorf("approaches %q: %w", in.Mover, err)
	}
	if err := fromField(field, todo); err != nil {
		return ApproachesOutput{}, fmt.Errorf("approaches %q: %w", in.Mover, err)
	}
	return out, nil
}
