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
// resolves to. This package may not know that (design law C1: the composition
// may not import a rulebook, which is why Sight, Standing and Decider are
// injected at all), so the refs come out the far end as the same strings that
// went in.
//
// The split is the honest shape of the seam rather than an unfinished job. A
// caller one layer up imports both this and a rulebook, resolves each ref, and
// builds the roster. What it never has to do is geometry.
type Compiled struct {
	// Field is the compiled world: chambers as regions on one canvas, the
	// walls between them, the openings through those walls, and the doors
	// standing in the locked ones. Ready for [encounter.NewEncounter].
	Field encounter.FieldInput

	// PartyStart is where the party comes in, best seat first.
	//
	// A LIST, because a party is more than one person and the author declares
	// one cell. The first seat is the cell they wrote; the rest are that
	// chamber's other free cells, nearest first, so a caller with four
	// players takes the first four and gets them standing together at the
	// way in. Never empty for a spec that compiled.
	PartyStart []Seat

	// Monsters is every authored monster, in chamber order and then in the
	// order the author wrote them. The half that still needs a sheet.
	Monsters []MonsterPlacement
}

// Seat is one room-local cell somebody can be placed in, named the way
// [encounter.MemberInput] wants it.
type Seat struct {
	// Region is the chamber's ID — [encounter.MemberInput.Room].
	Region string

	// At is the cell within that chamber, in the authored frame:
	// [encounter.MemberInput.Position].
	At spatial.Position
}

// MonsterPlacement is one authored monster: where it stands, and every word
// about it this package is not allowed to interpret.
type MonsterPlacement struct {
	// Ref is content's identifier, exactly as authored.
	Ref string

	// Region is the chamber it stands in.
	Region string

	// At is its cell within that chamber, in the authored frame.
	At spatial.Position

	// Targeting is the author's word for how it picks a target, or empty.
	// CARRIED, NEVER INTERPRETED — what "lowest-health" means is a rule, and
	// this package holds none. It lands on the monster's SHEET one layer up
	// (encounter.Data's own Targeting field, with ParseTargetingStrategy
	// beside it), never on a decider: a decider cannot attack, since Intent
	// is MoveTo or Hold.
	Targeting string

	// Boss is whether this is the monster whose death ends things.
	Boss bool
}

// Load decodes, validates and compiles a dungeon in one call.
//
// The three steps are separately available ([Decode], [Validate], [Compile])
// because their errors are about different things, and a tool that wants to
// lint a file without building a world should not have to build one. Most
// callers want all three, and this is that call.
func Load(raw []byte) (Compiled, error) {
	spec, err := Decode(raw)
	if err != nil {
		return Compiled{}, err
	}
	if err := Validate(spec); err != nil {
		return Compiled{}, err
	}

	return Compile(spec)
}

// Compile turns a VALIDATED spec into a world.
//
// It assumes [Validate] has already passed and does not re-run it: the checks
// below would be a second, weaker copy of rules that already exist, and two
// copies of a rule are two rules waiting to disagree. Handing it an unvalidated
// spec is a programming error, and the failure it produces is a compiled
// dungeon the composition then refuses — which is the right place for it to
// stop, but a worse message than [Validate] would have given.
func Compile(spec *Spec) (Compiled, error) {
	//
	// # The layout rule, stated once
	//
	// Chambers sit in a row, in declaration order, each one immediately east of the
	// last: chamber i's anchor is the sum of the widths before it, and every
	// chamber is the dungeon's full height. That is the entire layout language, and
	// it is deliberately small — [encounter.RoomInput.Origin] can anchor a chamber
	// anywhere, so a richer dialect is a change to this file and nothing else.
	//
	// # Everything here is authored columns and rows
	//
	// A chamber's anchor, a prop's cell, a wall's endpoints and a connector's
	// endpoints are all in the frame the author wrote (rpg-toolkit#1127), and the
	// composition converts them once at construction. The two places this package
	// needs an ABSOLUTE cell — a door's edges, which [encounter.DoorInput] takes
	// absolute because a door spans two chambers and belongs to neither — go
	// through [encounter.HexCellAt], which is the same conversion the composition
	// runs.
	orientation, err := orientationOf(spec)
	if err != nil {
		return Compiled{}, err
	}

	anchors := anchorsOf(spec)
	rooms := make([]encounter.RoomInput, len(spec.Rooms))
	for i, r := range spec.Rooms {
		rooms[i] = encounter.RoomInput{
			ID:     r.ID,
			Grid:   spatial.GridShapeHex,
			Width:  r.Width,
			Height: spec.Height,
			Origin: spatial.Position{X: float64(anchors[i]), Y: 0},
			Props:  propsOf(r),
		}
	}

	connections, doors := openingsOf(spec, orientation, anchors)
	for i := range rooms[:max(len(rooms)-1, 0)] {
		rooms[i].Boundaries = seamWall(spec, orientation, i)
	}

	void, err := voidOf(spec)
	if err != nil {
		return Compiled{}, err
	}

	field := encounter.FieldInput{
		Canvas:      encounter.CanvasInput{Void: void, Orientation: orientation},
		Rooms:       rooms,
		Connections: connections,
		Doors:       doors,
	}

	start, err := seatsOf(spec, orientation, anchors)
	if err != nil {
		return Compiled{}, err
	}

	return Compiled{Field: field, PartyStart: start, Monsters: monstersOf(spec)}, nil
}

// anchorsOf is each chamber's starting column: the sum of the widths before it.
func anchorsOf(spec *Spec) []int {
	out := make([]int, len(spec.Rooms))
	col := 0
	for i, r := range spec.Rooms {
		out[i] = col
		col += r.Width
	}

	return out
}

// voidOf turns the author's word into the declaration the canvas requires.
//
// The default case is unreachable — [Validate] has already refused any word not
// in the vocabulary — and it returns an error rather than panicking because an
// unreachable branch that can only crash is worse than one that can only
// explain itself.
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

// orientationOf turns the author's word into the declaration every authored
// cell in the file is counted in. Unreachable default, for voidOf's reason.
func orientationOf(spec *Spec) (encounter.Orientation, error) {
	switch spec.Orientation {
	case "pointy":
		return encounter.HexesArePointyTop(), nil
	case "flat":
		return encounter.HexesAreFlatTop(), nil
	default:
		return nil, fmt.Errorf("orientation %q reached the compiler: %w", spec.Orientation, ErrBadSpec)
	}
}

// propsOf is one chamber's props, with both blocking answers copied rather than
// aliased.
//
// Copied because [encounter.PropInput] holds them as POINTERS, so a compiled
// field that pointed at the spec's own bools would change behaviour under an
// author who edited the spec afterwards — the aliasing defect rpg-toolkit#1128
// found one indirection down, and the same one is available here.
func propsOf(room RoomSpec) []encounter.PropInput {
	var out []encounter.PropInput
	for _, p := range room.Place {
		if kind, _ := refKind(p.Ref); kind != typeProps {
			continue
		}
		blocksMovement, blocksLoS := *p.BlocksMovement, *p.BlocksLoS
		out = append(out, encounter.PropInput{
			Ref:               p.Ref,
			At:                spatial.Position{X: float64(p.At[0]), Y: float64(p.At[1])},
			BlocksMovement:    &blocksMovement,
			BlocksLineOfSight: &blocksLoS,
		})
	}

	return out
}

// monstersOf is every authored monster, in chamber order and then authored
// order — the order [Compiled.Monsters] promises, and the one a caller's own
// member IDs will be derived from.
func monstersOf(spec *Spec) []MonsterPlacement {
	var out []MonsterPlacement
	for _, r := range spec.Rooms {
		for _, p := range r.Place {
			if kind, _ := refKind(p.Ref); kind != typeMonsters {
				continue
			}
			targeting := ""
			if p.Targeting != nil {
				targeting = *p.Targeting
			}
			out = append(out, MonsterPlacement{
				Ref:       p.Ref,
				Region:    r.ID,
				At:        spatial.Position{X: float64(p.At[0]), Y: float64(p.At[1])},
				Targeting: targeting,
				Boss:      p.Boss,
			})
		}
	}

	return out
}

// seamWall is the wall between chamber i and the one east of it: every crossing
// the two chambers share, minus the one a connector opens.
//
// # A wall is edges, and nothing is carved
//
// The dialect this replaces reserved a column of floor for a doorway, because
// over there a wall was a thing that occupied cells. Here a wall is a set of
// EDGES between cells (rpg-toolkit#1106 put both endpoints on one canvas so a
// seam could be walled at all), so the chambers stay full width and the opening
// is one edge left out.
//
// # It asks which crossings exist rather than knowing
//
// In axial space a cell's neighbours across the +Q edge are a fixed pair. In the
// authored offset frame they STAGGER with the column's parity, and differently
// again under each orientation — so the honest way to draw a wall is to ask
// spatial which of the candidate pairs are actually one step apart. Cheaper to
// write than the parity table, and correct for both layouts.
//
// Endpoints beyond the dungeon's own rows are left out entirely: a crossing
// into the void is a crossing nobody can make, and the void already answers for
// it (see [encounter.Void]).
func seamWall(spec *Spec, o encounter.Orientation, i int) []spatial.Boundary {
	west := spec.Rooms[i]
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
		SpanWidth: seamProbeSpan, SpanHeight: seamProbeSpan,
	})

	// The opening, if this seam has one — in the west chamber's local columns,
	// which is the frame the boundary is authored in.
	open := -1
	if joins(spec, i, i+1) {
		open = doorwayRow(spec.Height)
	}

	out := make([]spatial.Boundary, 0, spec.Height*2)
	for row := 0; row < spec.Height; row++ {
		near := encounter.HexCellAt(o, west.Width-1, row)
		for _, dr := range []int{-1, 0, 1} {
			to := row + dr
			if to < 0 || to >= spec.Height {
				continue
			}
			if dr == 0 && row == open {
				continue // the opening itself
			}
			if grid.Distance(near, encounter.HexCellAt(o, west.Width, to)) != 1 {
				continue // not a crossing on this grid
			}
			out = append(out, spatial.Boundary{
				From:              spatial.Position{X: float64(west.Width - 1), Y: float64(row)},
				To:                spatial.Position{X: float64(west.Width), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}

	return out
}

// seamProbeSpan is the span of the throwaway grid [seamWall] and [seatsOf] ask
// for distance.
//
// THE NUMBER DOES NOT MATTER, and saying so is the point of naming it. Both
// callers only ever ask Distance, and AxialHexGrid.Distance converts its two
// arguments to cube coordinates and subtracts — it never looks at the grid's
// own bounds, so a cell far outside the span still measures correctly. Any
// instance of the family is a valid calculator over absolute cells, which is
// the same standing arrangement continuity_test.go's own throwaway grid runs
// on. A large span is chosen only so that a future caller reaching for
// IsValidPosition on it does not get a surprise; if one ever does, it should
// take the canvas's grid instead of widening this.
const seamProbeSpan = 1e6

// joins reports whether a connector opens the seam between two chambers, by
// their positions in the room list.
func joins(spec *Spec, west, east int) bool {
	if east >= len(spec.Rooms) {
		return false
	}
	for _, c := range spec.Connectors {
		if (c.From == spec.Rooms[west].ID && c.To == spec.Rooms[east].ID) ||
			(c.To == spec.Rooms[west].ID && c.From == spec.Rooms[east].ID) {
			return true
		}
	}

	return false
}

// openingsOf turns the connectors into the openings through the seam walls, and
// the doors standing in the locked ones.
//
// # A connector is a name and, sometimes, a door
//
// An open connector compiles to a [encounter.ConnectionInput] alone: crossing it
// is an ordinary step, and the connection exists so [encounter.StepOutput] can
// NAME the doorway a step went through. A locked one compiles to the same
// connection plus a [encounter.DoorInput] standing in that one crossing — the
// door is what refuses, and the connection is still just a name.
//
// The door's edge is ABSOLUTE, which is the one place this package converts:
// [encounter.DoorInput] takes absolute cells because a door spans two chambers
// and belongs to neither, unlike a wall, which a chamber draws in its own frame.
func openingsOf(spec *Spec, o encounter.Orientation, anchors []int) (
	[]encounter.ConnectionInput, []encounter.DoorInput,
) {
	var connections []encounter.ConnectionInput
	var doors []encounter.DoorInput

	row := doorwayRow(spec.Height)
	index := make(map[string]int, len(spec.Rooms))
	for i, r := range spec.Rooms {
		index[r.ID] = i
	}

	for _, c := range spec.Connectors {
		from, to := index[c.From], index[c.To]

		// The endpoint each side contributes depends on which of them is
		// west, and the author may have written the pair either way round.
		fromLocal, toLocal := 0, 0
		west, east := c.From, c.To
		if from < to {
			fromLocal = spec.Rooms[from].Width - 1
		} else {
			toLocal = spec.Rooms[to].Width - 1
			west, east = c.To, c.From
		}

		// THE ID IS CANONICAL, not authored. A connector is UNDIRECTED —
		// Validate accepts either order and de-duplicates seams ignoring it —
		// so `{from: hall, to: entrance}` is the same opening as
		// `{from: entrance, to: hall}`, and if the ID followed the author's
		// order the two would mint different doors for it. A door's STATE
		// persists under its ID (rpg-toolkit#1123), so that is not cosmetic:
		// reordering a line in the yaml would lose which doors a party had
		// opened. Named west-to-east, which the room list already decides.
		id := spec.Key + ":" + west + "-" + east

		connections = append(connections, encounter.ConnectionInput{
			ID: id, From: c.From, To: c.To,
			FromPosition: spatial.Position{X: float64(fromLocal), Y: float64(row)},
			ToPosition:   spatial.Position{X: float64(toLocal), Y: float64(row)},
		})

		if c.Locked == nil {
			continue
		}
		doors = append(doors, encounter.DoorInput{
			ID: id,
			Edges: []encounter.DoorEdge{{
				From: encounter.HexCellAt(o, anchors[from]+fromLocal, row),
				To:   encounter.HexCellAt(o, anchors[to]+toLocal, row),
			}},
			State: encounter.DoorIsLocked(encounter.Lock{
				DC: c.Locked.DC, Ability: c.Locked.Ability, Tool: c.Locked.Tool,
			}),
		})
	}

	return connections, doors
}

// seatsOf resolves the authored start cell to a chamber and orders that
// chamber's free cells around it, nearest first.
//
// # The author declares one cell and a party is several people
//
// So this hands back a list rather than a cell, and the order is the whole
// value of it: seat 0 is the cell they wrote, and the rest are ordered by how
// far they are from it, so four players take the first four and arrive standing
// together. Ties break on column then row, which is arbitrary and deterministic
// — and determinism is the point, since a roster that reshuffled between two
// loads of the same dungeon would put people in different places for no reason.
//
// Cells with something already standing on them are left out: a party member
// cannot share a cell with a pillar, and a seat that fails at placement is not
// a seat.
func seatsOf(spec *Spec, o encounter.Orientation, anchors []int) ([]Seat, error) {
	col, row := spec.Start[0], spec.Start[1]

	at := -1
	for i, r := range spec.Rooms {
		if col >= anchors[i] && col < anchors[i]+r.Width {
			at = i
			break
		}
	}
	if at < 0 {
		return nil, fmt.Errorf(
			"the party starts at [%d,%d], which is in no chamber: %w", col, row, ErrBadSpec)
	}

	room := spec.Rooms[at]
	taken := make(map[[2]int]bool, len(room.Place))
	for _, p := range room.Place {
		taken[p.At] = true
	}

	local := [2]int{col - anchors[at], row}
	if taken[local] {
		return nil, fmt.Errorf(
			"the party starts at [%d,%d], where something already stands: %w", col, row, ErrBadSpec)
	}

	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
		SpanWidth: seamProbeSpan, SpanHeight: seamProbeSpan,
	})
	from := encounter.HexCellAt(o, col, row)

	seats := make([]Seat, 0, room.Width*spec.Height-len(taken))
	dist := make(map[[2]int]float64, cap(seats))
	for c := 0; c < room.Width; c++ {
		for r := 0; r < spec.Height; r++ {
			cell := [2]int{c, r}
			if taken[cell] {
				continue
			}
			dist[cell] = grid.Distance(from, encounter.HexCellAt(o, anchors[at]+c, r))
			seats = append(seats, Seat{
				Region: room.ID,
				At:     spatial.Position{X: float64(c), Y: float64(r)},
			})
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

	return seats, nil
}
