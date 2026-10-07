// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// PartitionRegionInput names the complete static field and the declared region
// whose floor is to be partitioned. No live encounter, roster or knowledge is
// needed. Door geometry is treated as closed regardless of its initial state.
type PartitionRegionInput struct {
	Field  FieldInput
	Region RegionID
}

// PartitionRegionOutput contains floor components with unambiguous opaque
// footing assigned to its adjoining component, in the INPUT'S
// authored cell frame, ready for RegionInput.Cells. Components and their cells
// are ordered by canonical axial cell; returned slices do not alias the input.
type PartitionRegionOutput struct {
	Components [][]spatial.Position
	// Footing is permanently occupied opaque floor that cannot be assigned
	// to one adjoining component. A compiler carries it as scenery, not as
	// an undiscoverable room. It excludes doors and movable/reserved props.
	Footing []spatial.Position
}

// PartitionRegion groups one region's cells by clear adjacent sight crossings
// through the same compiled canvas geometry used in play. Search belongs to
// spatial.Field; this query does not replace SightLanes or learn any knowledge.
// Doors are topology boundaries even when authored open, so opening or closing
// one cannot change a derived room's identity. Input geometry/state is untouched.
// Invalid field/door declarations are refused by the normal construction gates;
// an absent region returns ErrNoRegion. Any failure returns a zero output.
func PartitionRegion(in PartitionRegionInput) (PartitionRegionOutput, error) {
	f, err := compileField(in.Field)
	if err != nil {
		return PartitionRegionOutput{}, err
	}
	if err := validateDoorInputs(f, in.Field.Doors); err != nil {
		return PartitionRegionOutput{}, err
	}
	cells, exists := f.regionCells[in.Region]
	if !exists {
		return PartitionRegionOutput{}, fmt.Errorf("partition region %q: %w", in.Region, ErrNoRegion)
	}
	// Mutable props do not define fixed room topology. These are validated,
	// owned construction copies, not edits to the caller's field.
	fixedPlaced := f.placed[:0]
	for _, p := range f.placed {
		if !p.mutable() {
			fixedPlaced = append(fixedPlaced, p)
		}
	}
	f.placed = fixedPlaced
	fixedProps := f.props[:0]
	for _, p := range f.props {
		if !p.Holdable && p.Arrives == nil {
			fixedProps = append(fixedProps, p)
		}
	}
	f.props = fixedProps
	doors, _ := doorRecordsFrom(in.Field.Doors)
	for _, door := range doors {
		door.state = DoorIsClosed()
	}
	canvas, err := f.compileCanvas(doors, nil)
	if err != nil {
		return PartitionRegionOutput{}, err
	}
	authored := make(map[spatial.Position]spatial.Position, len(cells))
	for _, region := range in.Field.Regions {
		if region.ID != in.Region {
			continue
		}
		for _, cell := range region.Cells {
			authored[f.cellAt(cell)] = cell
		}
	}
	visited := make(map[spatial.Position]bool, len(cells))
	groups := make([][]spatial.Position, 0)
	for _, seed := range cells {
		if visited[seed] {
			continue
		}
		component, err := spatial.Field(canvas.GetGrid(), spatial.FieldInput{
			Sources: []spatial.Position{seed},
			Passable: func(from, to spatial.Position) bool {
				_, within := authored[to]
				return within && !canvas.IsLineOfSightBlocked(from, to)
			},
		})
		if err != nil {
			return PartitionRegionOutput{}, err
		}
		group := make([]spatial.Position, 0, len(component.Dist))
		for _, cell := range cells {
			if _, included := component.Dist[cell]; !included {
				continue
			}
			group = append(group, cell)
			visited[cell] = true
		}
		groups = append(groups, group)
	}
	groups, footing, err := joinOpaqueFooting(canvas, groups)
	if err != nil {
		return PartitionRegionOutput{}, err
	}
	out := PartitionRegionOutput{Components: make([][]spatial.Position, 0, len(groups))}
	for _, cell := range footing {
		out.Footing = append(out.Footing, authored[cell])
	}
	for _, group := range groups {
		cells := make([]spatial.Position, 0, len(group))
		for _, cell := range group {
			cells = append(cells, authored[cell])
		}
		out.Components = append(out.Components, cells)
	}
	return out, nil
}

// permanentOpaqueFootingAt uses the same centre-contact standing rule as the
// field, restricted to immutable opaque contributors. Door states are excluded:
// their floor must remain owned so it can be stood on after opening.
func (f *field) permanentOpaqueFootingAt(cell spatial.Position) (bool, error) {
	for _, p := range f.props {
		if !p.Holdable && p.Arrives == nil && p.BlocksMovement != nil && *p.BlocksMovement &&
			p.BlocksLineOfSight != nil && *p.BlocksLineOfSight && f.cellAt(p.At) == cell {
			return true, nil
		}
	}
	centre := f.plane.CellCentre(cell)
	for _, p := range f.placed {
		if p.mutable() || !p.blocksMovement || !p.blocksLineOfSight {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: p.placement, From: centre, To: centre})
		if err != nil {
			return false, err
		}
		if contact.Contact {
			return true, nil
		}
	}
	return false, nil
}

// An opaque pillar's own cell is not another room. Attach completely opaque
// footing only when it borders ONE clear component; never join two rooms through
// a wall/closed door. Permanently occupied ambiguous footing is scenery, never
// a phantom discovery region. Doors retain owned floor because they can open.
func joinOpaqueFooting(canvas *canvasRoom, groups [][]spatial.Position) ([][]spatial.Position, []spatial.Position, error) {
	owner := make(map[spatial.Position]int)
	opaque := make([]bool, len(groups))
	obstructions := canvasSightObstructions{canvas: canvas}
	for i, cells := range groups {
		opaque[i] = true
		for _, cell := range cells {
			owner[cell] = i
			at, err := obstructions.At(spatial.SightCellInput{At: cell})
			if err != nil {
				return nil, nil, err
			}
			if !at.Blocked {
				opaque[i] = false
			}
		}
	}
	joined := make([][]spatial.Position, len(groups))
	footing := make([]spatial.Position, 0)
	for i, cells := range groups {
		target, ambiguous := -1, false
		if opaque[i] {
			for _, cell := range cells {
				for _, neighbor := range canvas.GetGrid().GetNeighbors(cell) {
					other, owned := owner[neighbor]
					if !owned || opaque[other] {
						continue
					}
					if target >= 0 && target != other {
						ambiguous = true
					}
					target = other
				}
			}
		}
		if target < 0 || ambiguous {
			permanent := opaque[i]
			for _, cell := range cells {
				fixed, err := canvas.field.permanentOpaqueFootingAt(cell)
				if err != nil {
					return nil, nil, err
				}
				permanent = permanent && fixed
			}
			if permanent {
				footing = append(footing, cells...)
				continue
			}
			target = i
		}
		joined[target] = append(joined[target], cells...)
	}
	out := make([][]spatial.Position, 0, len(joined))
	for _, cells := range joined {
		if len(cells) > 0 {
			sortCells(cells)
			out = append(out, cells)
		}
	}
	sort.Slice(out, func(i, j int) bool { return cellBefore(out[i][0], out[j][0]) })
	sortCells(footing)
	return out, footing, nil
}
