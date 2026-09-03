// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Compiled is an authored dungeon turned into a world, in two halves.
//
// # Why two halves and not one
//
// [Compiled.Field] is finished: hand it to [encounter.NewEncounter] as it
// stands. The rest is NOT finished, and cannot be here — a monster needs a
// sheet, and a sheet needs somebody who knows what "dnd5e:monsters:skeleton"
// resolves to. This package may not know that (design law C1), so the refs
// come out the far end as the same strings that went in.
type Compiled struct {
	// Field is the compiled world: the regions as authored, the walls and
	// props at their absolute cells, and the doors standing in their
	// crossings. Ready for [encounter.NewEncounter].
	Field encounter.FieldInput

	// PartyStart is where the party comes in, best seat first.
	//
	// A LIST, because a party is more than one person and the author declares
	// one cell. The first seat is the cell they wrote; the rest are the start
	// region's other free cells, nearest first, so a caller with four players
	// takes the first four and gets them standing together at the way in.
	// Never empty for a spec that compiled.
	PartyStart []Seat

	// Monsters is every authored monster, in the order the author wrote
	// them. The half that still needs a sheet.
	Monsters []MonsterPlacement
}

// Seat is one cell somebody can be placed in, named the way
// [encounter.MemberInput] wants it.
type Seat struct {
	// Region is the ID of the region that owns the cell — derived from the
	// floor, never authored, and carried for a caller that wants to say
	// "she starts in the entrance".
	Region string

	// At is the ABSOLUTE authored cell, offset [col,row]:
	// [encounter.MemberInput.Position].
	At spatial.Position
}

// MonsterPlacement is one authored monster: where it stands, and every word
// about it this package is not allowed to interpret.
type MonsterPlacement struct {
	// Ref is content's identifier, exactly as authored.
	Ref string

	// Region is the ID of the region whose floor it stands on — derived.
	Region string

	// At is its ABSOLUTE authored cell, offset [col,row].
	At spatial.Position

	// Targeting is the author's word for how it picks a target, or empty.
	// CARRIED, NEVER INTERPRETED.
	Targeting string

	// Boss is whether this is the monster whose death ends things.
	Boss bool
}

// Load decodes, validates and compiles a dungeon in one call.
//
// The three steps are separately available ([Decode], [Validate], [Compile])
// because their errors are about different things, and a tool that wants to
// lint a file without building a world should not have to build one. A
// validation failure is returned as a [*ValidationError] carrying every
// defect, and is an [ErrBadSpec].
func Load(raw []byte) (Compiled, error) {
	spec, err := Decode(raw)
	if err != nil {
		return Compiled{}, err
	}
	if errs := Validate(spec); len(errs) > 0 {
		return Compiled{}, &ValidationError{Errors: errs}
	}
	return Compile(spec)
}

// Compile turns a VALIDATED spec into a world.
//
// It assumes [Validate] has already passed and does not re-run it: the checks
// the composition makes at construction are the same rules, and a spec that
// skipped Validate is refused there instead — with the composition's words
// rather than the file's paths.
//
// # Nothing is generated
//
// Version 1 derived a layout (chambers in a row), a seam wall per chamber
// pair, and a doorway row. Version 2 derives nothing: every region, wall,
// door and prop is carried to [encounter.FieldInput] as the author wrote it,
// and the ONE place this package converts a coordinate — a door's edges,
// which [encounter.DoorInput] takes absolute axial — goes through
// [encounter.HexCellAt], the same conversion the composition runs.
func Compile(spec *Spec) (Compiled, error) {
	orientation, ok := orientations[spec.Orientation]
	if !ok {
		return Compiled{}, fmt.Errorf("orientation %q reached the compiler: %w", spec.Orientation, ErrBadSpec)
	}
	void, err := voidOf(spec)
	if err != nil {
		return Compiled{}, err
	}

	// THE GEOMETRY IS RUN ONCE, HERE, and everything downstream is handed the
	// answers (design C9, plan §0). The runtime never embeds a hex: it is
	// given the crossings as pairs, the sealed cells as a list, and the walls
	// as segments it draws and never measures.
	names := authoredOf(spec)
	derived := deriveWalls(spec, orientation, floorOf(spec, orientation), nil)
	doorEdges := doorCrossings(spec, orientation)

	field := encounter.FieldInput{
		Canvas:   encounter.CanvasInput{Void: void, Orientation: orientation},
		Regions:  regionsOf(spec),
		Scenery:  sceneryOf(spec),
		Props:    propsOf(spec),
		Walls:    wallsOf(derived, names, doorEdges),
		Segments: segmentsOf(derived, names, doorEdges),
		Sealed:   sealedOf(spec, orientation, derived),
		Doors:    doorsOf(spec, orientation),
	}

	start, err := seatsOf(spec, orientation)
	if err != nil {
		return Compiled{}, err
	}

	return Compiled{Field: field, PartyStart: start, Monsters: monstersOf(spec, orientation)}, nil
}

// voidOf turns the author's word into the declaration the canvas requires.
// The default case is unreachable after Validate and returns an error rather
// than panicking, because an unreachable branch that can only crash is worse
// than one that can only explain itself.
func voidOf(spec *Spec) (encounter.Void, error) {
	switch spec.Void {
	case "opaque":
		return encounter.VoidIsOpaque(), nil
	case "transparent":
		return encounter.VoidIsTransparent(), nil
	default:
		return nil, fmt.Errorf("void %q reached the compiler: %w", spec.Void, ErrBadSpec)
	}
}

// regionsOf carries the regions verbatim, flattening each one's rows.
func regionsOf(spec *Spec) []encounter.RegionInput {
	out := make([]encounter.RegionInput, 0, len(spec.Regions))
	for _, r := range spec.Regions {
		var cells []spatial.Position
		for _, row := range r.Cells {
			for _, at := range row {
				cells = append(cells, authored(at))
			}
		}
		// Copied, not aliased: a compiled field that pointed at the spec's
		// own float would change under an author who edited the spec
		// afterwards.
		intensity := *r.Lighting.Intensity
		out = append(out, encounter.RegionInput{
			ID: r.ID, Name: r.Name, Cells: cells, Archetype: r.Archetype,
			Lighting:  &encounter.Lighting{Intensity: intensity},
			Concealed: r.Concealed,
		})
	}
	return out
}

// sceneryOf flattens the authored scenery rows into the flat cell list
// [encounter.FieldInput.Scenery] takes, in the authored frame the regions use.
// Nil for a dungeon that authors none, which is the same fact the absent key
// is.
func sceneryOf(spec *Spec) []spatial.Position {
	var cells []spatial.Position
	for _, row := range spec.Scenery {
		for _, at := range row {
			cells = append(cells, authored(at))
		}
	}

	return cells
}

// propsOf is every prop, with both blocking answers copied rather than
// aliased ([encounter.PropInput] holds them as pointers), and Facing/Offset
// carried through exactly as authored — this package validates the word and
// the bounds; it does not reinterpret them (rpg-project#261).
func propsOf(spec *Spec) []encounter.PropInput {
	var out []encounter.PropInput
	for _, p := range spec.Place {
		if kind, _ := refKind(p.Ref); kind != typeProps {
			continue
		}
		blocksMovement, blocksLoS := *p.BlocksMovement, *p.BlocksLoS
		prop := encounter.PropInput{
			Ref: p.Ref, At: authored(p.At),
			BlocksMovement: &blocksMovement, BlocksLineOfSight: &blocksLoS,
			Facing: p.Facing,
		}
		// Validate has already confirmed len(p.Offset) is 0, 2 or 3 by the
		// time Compile runs; anything else is unreachable here. A missing
		// third component is height 0 — on the floor — by design.
		if len(p.Offset) >= 2 {
			prop.Offset = [3]float64{p.Offset[0], p.Offset[1], 0}
			if len(p.Offset) == 3 {
				prop.Offset[2] = p.Offset[2]
			}
		}
		out = append(out, prop)
	}
	return out
}

// wallsOf is THE MECHANICAL TRUTH the runtime is handed: every crossing the
// authored lines block, as the pairs [encounter.WallInput] has always taken,
// minus the ones a door opens.
//
// A DOOR STANDS IN A WALL and the door's crossing is subtracted here, exactly
// as it was under the pair form: the engine still sees walls and doors
// disjoint, and a wall drawn straight through a doorway compiles to the hole
// its door makes. Two walls that block the same crossing — the corner case,
// literally — emit it once, at the first one's height.
func wallsOf(
	derived wallDerivation, names map[spatial.Position][2]int, doors map[[2]spatial.Position]encounter.DoorID,
) []encounter.WallInput {
	var out []encounter.WallInput
	seen := map[[2]spatial.Position]bool{}
	for _, w := range derived.Walls {
		for _, c := range w.Crossings {
			if seen[c] || doors[c] != "" {
				continue
			}
			seen[c] = true
			wall := encounter.WallInput{Boundary: spatial.Boundary{
				From: authored(names[c[0]]), To: authored(names[c[1]]),
				BlocksMovement: true, BlocksLineOfSight: true,
			}}
			if w.Height != nil {
				wall.Height = *w.Height
			}
			out = append(out, wall)
		}
	}

	return out
}

// segmentsOf is WHAT A CLIENT DRAWS: each authored wall as the line it is, in
// fractional axial, with the floor it stands on and the doors that open in it.
//
// PRESENTATION, not mechanics — the crossings above are the mechanics, and the
// two are derived from the same line so they cannot disagree. Before this, a
// client had to guess a wall's shape by chaining the crossings back into runs,
// with a straightness tolerance that regrouped walls the author had drawn; the
// line was in the file all along, and now it reaches the far end intact.
func segmentsOf(
	derived wallDerivation,
	names map[spatial.Position][2]int, doors map[[2]spatial.Position]encounter.DoorID,
) []encounter.SegmentInput {
	var out []encounter.SegmentInput
	for _, w := range derived.Walls {
		seg := encounter.SegmentInput{
			Name: w.Name,
			From: encounter.AxialPointF{Q: w.Start.Q, R: w.Start.R},
			To:   encounter.AxialPointF{Q: w.End.Q, R: w.End.R},
		}
		if w.Height != nil {
			seg.Height = *w.Height
		}
		for _, c := range w.Footprint {
			seg.Footprint = append(seg.Footprint, authored(names[c]))
		}
		for _, c := range w.Crossings {
			if id := doors[c]; id != "" {
				seg.DoorIDs = append(seg.DoorIDs, id)
			}
		}
		out = append(out, seg)
	}

	return out
}

// sealedOf is every OWNED cell a wall leaves too little of to stand on
// (C10) — the cells that keep their region and lose their feet, which is why
// the runtime cannot work them out from region membership any more.
//
// Scenery a wall cuts is deliberately absent: it was never standable, and
// saying so twice would give the runtime two lists to disagree about.
func sealedOf(spec *Spec, o encounter.Orientation, derived wallDerivation) []spatial.Position {
	owner := ownerOf(spec, o)
	var out []spatial.Position
	for cell := range derived.Sealed {
		if _, owned := owner[cell]; !owned {
			continue
		}
		out = append(out, cell)
	}
	names := authoredOf(spec)
	for i, c := range out {
		out[i] = authored(names[c])
	}
	sort.Slice(out, func(i, j int) bool { return cellBefore(out[i], out[j]) })

	return out
}

// doorCrossings is every crossing a door opens, to the compiled door's id.
// Absolute axial, which is the frame the derivations answer in.
func doorCrossings(spec *Spec, o encounter.Orientation) map[[2]spatial.Position]encounter.DoorID {
	g := geometryOf(o)
	out := map[[2]spatial.Position]encounter.DoorID{}
	for _, d := range spec.Doors {
		step, ok := g.stepAt(d.At.Offset)
		if !ok {
			continue
		}
		here := encounter.HexCellAt(o, d.At.Cell[0], d.At.Cell[1])
		there := spatial.Position{X: here.X + float64(step[0]), Y: here.Y + float64(step[1])}
		out[normalizedCrossing(here, there)] = encounter.DoorID(spec.Key + "/" + d.ID)
	}

	return out
}

// floorOf is every floor cell — a region's or scenery's — absolute axial. What
// the wall derivations cut against (C2, C8).
func floorOf(spec *Spec, o encounter.Orientation) map[spatial.Position]bool {
	out := map[spatial.Position]bool{}
	for _, r := range spec.Regions {
		for _, row := range r.Cells {
			for _, at := range row {
				out[encounter.HexCellAt(o, at[0], at[1])] = true
			}
		}
	}
	for _, row := range spec.Scenery {
		for _, at := range row {
			out[encounter.HexCellAt(o, at[0], at[1])] = true
		}
	}

	return out
}

// authoredOf is the reverse of the cell conversion: every floor cell back to
// the [col,row] pair the author wrote, so a derived crossing or footprint can
// be handed on in the frame [encounter.FieldInput] speaks.
func authoredOf(spec *Spec) map[spatial.Position][2]int {
	o := orientations[spec.Orientation]
	out := map[spatial.Position][2]int{}
	for _, r := range spec.Regions {
		for _, row := range r.Cells {
			for _, at := range row {
				out[encounter.HexCellAt(o, at[0], at[1])] = at
			}
		}
	}
	for _, row := range spec.Scenery {
		for _, at := range row {
			out[encounter.HexCellAt(o, at[0], at[1])] = at
		}
	}

	return out
}

// doorsOf mints each door as `<key>/<id>`, standing in THE ONE CROSSING its
// position is the midpoint of (F11), in the state the file gives it.
//
// ONE DOOR, ONE CROSSING. The pair form let a door list any number of edges,
// which made a two-cell gate and a mistake look identical; a wider doorway is
// now two doors, and the compiler has one less thing to be wrong about.
//
// THE ID IS PREFIXED BY THE DUNGEON'S KEY so two dungeons in one process
// cannot collide, and a door's STATE persists under its ID
// (rpg-toolkit#1123), so the id the author wrote is what the state is keyed
// by — renaming a door in the file is what loses a party's progress through
// it, and nothing else does.
func doorsOf(spec *Spec, o encounter.Orientation) []encounter.DoorInput {
	g := geometryOf(o)
	var out []encounter.DoorInput
	for _, d := range spec.Doors {
		step, ok := g.stepAt(d.At.Offset)
		if !ok {
			continue
		}
		here := encounter.HexCellAt(o, d.At.Cell[0], d.At.Cell[1])
		there := spatial.Position{X: here.X + float64(step[0]), Y: here.Y + float64(step[1])}
		// NORMALIZED, because the SAME door can be written from either of the
		// two cells it stands between — [-0.5,0] of one is [0.5,0] of the
		// other — and two files describing one dungeon must compile to one
		// door, not to two orderings of it.
		crossing := normalizedCrossing(here, there)
		var state encounter.DoorState
		switch {
		case d.Locked != nil:
			state = encounter.DoorIsLocked(encounter.Lock{Approaches: approachesOf(d.Locked)})
		case d.Closed:
			state = encounter.DoorIsClosed()
		default:
			state = encounter.DoorIsOpen()
		}
		out = append(out, encounter.DoorInput{
			ID:        encounter.DoorID(spec.Key + "/" + d.ID),
			Edges:     []encounter.DoorEdge{{From: crossing[0], To: crossing[1]}},
			State:     state,
			Concealed: approachesOf(d.Concealed),
		})
	}

	return out
}

// approachesOf carries an authored check's approaches to the composition's
// shape — copied, not aliased, for regionsOf's reason — with nil staying nil:
// a door that was never concealed carries nothing, which is the zero value
// telling the truth. Uninterpreted, every field.
func approachesOf(check CheckSpec) []encounter.CheckApproach {
	if len(check) == 0 {
		return nil
	}
	out := make([]encounter.CheckApproach, 0, len(check))
	for _, a := range check {
		out = append(out, encounter.CheckApproach{Ability: a.Ability, Tool: a.Tool, DC: a.DC})
	}
	return out
}

// monstersOf is every authored monster, in authored order, each naming the
// region whose floor it stands on.
func monstersOf(spec *Spec, o encounter.Orientation) []MonsterPlacement {
	owner := ownerOf(spec, o)
	var out []MonsterPlacement
	for _, p := range spec.Place {
		if kind, _ := refKind(p.Ref); kind != typeMonsters {
			continue
		}
		targeting := ""
		if p.Targeting != nil {
			targeting = *p.Targeting
		}
		out = append(out, MonsterPlacement{
			Ref: p.Ref, Region: owner[encounter.HexCellAt(o, p.At[0], p.At[1])],
			At: authored(p.At), Targeting: targeting, Boss: p.Boss,
		})
	}
	return out
}

// seatsOf resolves the authored start cell to its region and orders that
// region's free cells around it, nearest first.
//
// Seat 0 is the cell they wrote, and the rest are ordered by how far they are
// from it, so four players take the first four and arrive standing together.
// Ties break on column then row, which is arbitrary and deterministic — and
// determinism is the point. Cells with something already standing on them are
// left out: a party member cannot share a cell with a pillar.
func seatsOf(spec *Spec, o encounter.Orientation) ([]Seat, error) {
	owner := ownerOf(spec, o)
	from := encounter.HexCellAt(o, spec.Start[0], spec.Start[1])
	region, ok := owner[from]
	if !ok {
		return nil, fmt.Errorf("the party starts at [%d,%d], which is not floor: %w", spec.Start[0], spec.Start[1], ErrBadSpec)
	}

	taken := make(map[[2]int]bool, len(spec.Place))
	for _, p := range spec.Place {
		taken[p.At] = true
	}

	var seats []Seat
	dist := map[[2]int]float64{}
	for _, r := range spec.Regions {
		if r.ID != region {
			continue
		}
		for _, row := range r.Cells {
			for _, at := range row {
				if taken[at] {
					continue
				}
				dist[at] = adjacencyGrid.Distance(from, encounter.HexCellAt(o, at[0], at[1]))
				seats = append(seats, Seat{Region: region, At: authored(at)})
			}
		}
	}
	sort.Slice(seats, func(i, j int) bool {
		ci := [2]int{int(seats[i].At.X), int(seats[i].At.Y)}
		cj := [2]int{int(seats[j].At.X), int(seats[j].At.Y)}
		if dist[ci] != dist[cj] {
			return dist[ci] < dist[cj]
		}
		if ci[0] != cj[0] {
			return ci[0] < cj[0]
		}
		return ci[1] < cj[1]
	})
	if len(seats) == 0 {
		return nil, fmt.Errorf("the party starts at [%d,%d], where something already stands: %w", spec.Start[0], spec.Start[1], ErrBadSpec)
	}
	return seats, nil
}

// ownerOf is every OWNED cell, absolute axial, to the ID of the region that
// owns it — read once per compile. Scenery is floor too and is deliberately
// absent here: this map exists to answer which region a seat or a monster
// stands in, and scenery's answer to that is that nobody stands on it.
func ownerOf(spec *Spec, o encounter.Orientation) map[spatial.Position]string {
	owner := map[spatial.Position]string{}
	for _, r := range spec.Regions {
		for _, row := range r.Cells {
			for _, at := range row {
				owner[encounter.HexCellAt(o, at[0], at[1])] = r.ID
			}
		}
	}
	return owner
}

// authored is a [col,row] pair as the composition's authored-frame Position.
func authored(at [2]int) spatial.Position {
	return spatial.Position{X: float64(at[0]), Y: float64(at[1])}
}
