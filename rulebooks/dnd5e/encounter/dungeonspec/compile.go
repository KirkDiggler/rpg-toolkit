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

	field := encounter.FieldInput{
		Canvas:  encounter.CanvasInput{Void: void, Orientation: orientation},
		Regions: regionsOf(spec),
		Props:   propsOf(spec),
		Walls:   wallsOf(spec),
		Doors:   doorsOf(spec, orientation),
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
			Lighting: &encounter.Lighting{Intensity: intensity},
		})
	}
	return out
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
		// Validate has already confirmed len(p.Offset) is 0 or 2 by the time
		// Compile runs; anything else is unreachable here.
		if len(p.Offset) == 2 {
			prop.Offset = [2]float64{p.Offset[0], p.Offset[1]}
		}
		out = append(out, prop)
	}
	return out
}

// wallsOf carries every wall as an edge that blocks movement and sight.
func wallsOf(spec *Spec) []spatial.Boundary {
	out := make([]spatial.Boundary, 0, len(spec.Walls))
	for _, w := range spec.Walls {
		out = append(out, spatial.Boundary{
			From: authored(w[0]), To: authored(w[1]),
			BlocksMovement: true, BlocksLineOfSight: true,
		})
	}
	return out
}

// doorsOf mints each door as `<key>/<id>`, its edges converted to the absolute
// axial cells [encounter.DoorInput] takes, in the state the file gives it.
//
// THE ID IS PREFIXED BY THE DUNGEON'S KEY so two dungeons in one process
// cannot collide, and a door's STATE persists under its ID
// (rpg-toolkit#1123), so the id the author wrote is what the state is keyed
// by — renaming a door in the file is what loses a party's progress through
// it, and nothing else does.
func doorsOf(spec *Spec, o encounter.Orientation) []encounter.DoorInput {
	var out []encounter.DoorInput
	for _, d := range spec.Doors {
		edges := make([]encounter.DoorEdge, 0, len(d.Edges))
		for _, e := range d.Edges {
			edges = append(edges, encounter.DoorEdge{
				From: encounter.HexCellAt(o, e[0][0], e[0][1]),
				To:   encounter.HexCellAt(o, e[1][0], e[1][1]),
			})
		}
		var state encounter.DoorState
		switch {
		case d.Locked != nil:
			state = encounter.DoorIsLocked(encounter.Lock{DC: d.Locked.DC, Ability: d.Locked.Ability, Tool: d.Locked.Tool})
		case d.Closed:
			state = encounter.DoorIsClosed()
		default:
			state = encounter.DoorIsOpen()
		}
		out = append(out, encounter.DoorInput{ID: spec.Key + "/" + d.ID, Edges: edges, State: state})
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

// ownerOf is every floor cell, absolute axial, to the ID of the region that
// owns it — the file's own statement of the floor, read once per compile.
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
