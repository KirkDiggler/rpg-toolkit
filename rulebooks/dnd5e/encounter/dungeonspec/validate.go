// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The author-facing vocabularies. Each is closed, and an unrecognised word is
// refused rather than mapped to the nearest one — for [encounter.Void]'s
// reason: a word this build has never heard of is a dialect it does not speak,
// and picking the closest answer would author a dungeon the host did not.
var (
	voids        = map[string]bool{"opaque": true, "transparent": true}
	orientations = map[string]encounter.Orientation{
		"pointy": encounter.HexesArePointyTop(),
		"flat":   encounter.HexesAreFlatTop(),
	}

	// The targeting words are carried, not interpreted — but a TYPO is still
	// worth catching, because "lowest-helth" would otherwise ride all the way
	// through the compiler and be rejected (or worse, ignored) by a rulebook
	// the author never sees.
	targetings = map[string]bool{"closest": true, "lowest-health": true, "lowest-ac": true}

	// facings is the ONE eight-name true-compass vocabulary, the same set
	// under BOTH hex orientations (rpg-project#272 ruling, superseding
	// #261's orientation-scoped six-name sets): compass directions live in
	// world space, so which way the hexes point does not change which words
	// exist. A name outside the set is an ERROR, never a silent snap to the
	// nearest valid one (ideas/dungeon-builder/cardinal-facings.md).
	facings = map[string]bool{
		"n": true, "ne": true, "e": true, "se": true,
		"s": true, "sw": true, "w": true, "nw": true,
	}

	keyShape = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// The ref type segments this compiler can route. A ref's type decides what a
// placement BECOMES, which is why an unknown one is refused: there is no
// default kind of thing to be.
const (
	typeProps    = "props"
	typeMonsters = "monsters"
)

// Validate reports every way a decoded spec is not a dungeon, each at the
// YAML path of the thing that is wrong. An empty list means the spec
// compiles.
//
// # Every defect, not the first
//
// Version 1 stopped at the first defect. This reports all of them because the
// builder draws each one on the canvas where it belongs (rpg-project#256
// design §5): a cell painted twice glows in both regions, a wall between cells
// that do not touch is drawn red, a prop with no `blocks_los` is flagged at
// its cell. An author fixing a dungeon by hand gets the same list.
//
// # Geometry is checked here
//
// Version 1 kept validation "before geometry" because its layout was derived.
// Version 2's file IS the geometry — the floor is the cells the author listed
// — so whether a wall's endpoints touch is a question about the file, asked
// under the orientation it declares, and answered by spatial rather than by a
// parity table. When the orientation itself is not a word this build knows,
// every check that needs it is skipped and only the orientation is reported:
// a pile of adjacency errors under a wrong layout would all be noise.
func Validate(spec *Spec) []FieldError {
	if spec == nil {
		return []FieldError{{Message: "no dungeon spec"}}
	}
	v := &validation{spec: spec}
	v.header()
	v.regions()
	if v.geometryUsable() {
		// SCENERY FIRST inside the geometry block: everything below asks
		// what is floor, and scenery is half the answer (rpg-project#360).
		v.scenery()
		v.walls()
		v.doors()
		v.start()
		// INTEL BEFORE PLACE, because a placement's `holds` names a record
		// and place() asks whether it exists — the same ordering reason
		// scenery runs before the walls that stand on it.
		v.intel()
		v.place()
		v.exits()
		v.scenarios()
		v.concealment()
	}
	return v.errs
}

// validation accumulates defects and the floor they are checked against.
type validation struct {
	spec *Spec
	errs []FieldError

	orientation encounter.Orientation

	// owner is every OWNED cell (absolute axial) to the index of the region
	// that owns it — the same map compileField builds, built here so the
	// file's own defects are reported in the file's own paths. The floor is
	// this and sceneryAt together; [validation.floor] is the pair.
	owner map[spatial.Position]int

	// sceneryAt is every scenery cell (absolute axial) — the other half of
	// the floor, the half nobody stands on. Disjoint from owner: a cell in
	// both is refused.
	sceneryAt map[spatial.Position]bool

	// regionOK is whether the region list was sound enough to check edges
	// and placements against.
	regionOK bool

	// wallCrossings is every crossing some wall blocks, to the index of the
	// first wall that blocks it — DERIVED, never authored (C7). A door
	// standing in one of these opens it; everything else about a crossing is
	// this map's answer.
	wallCrossings map[[2]spatial.Position]int

	// derived is every wall's footprint, crossings and cost — worked out
	// once, by walls(), and read by doors(), start() and place() to say
	// which wall is the reason a cell has nowhere to stand.
	derived wallDerivation

	// authored is the reverse of the cell conversion: every floor cell back
	// to the [col,row] pair the author wrote, so a refusal about a crossing
	// can name it in the file's own coordinates.
	authored map[spatial.Position][2]int

	// doorAt is every validated door crossing to the index of the door
	// standing in it, for the coherence check's question: is this way in a
	// door, and is that door concealed?
	doorAt map[[2]spatial.Position]int

	// placeIDs is every authored placement id to the index that declared it
	// — built by place(), read by scenarios() to answer "does this binding
	// name something that exists". A collision is refused at place() and the
	// FIRST index stays here, so a later binding to a duplicated id is
	// reported once, at the duplicate, rather than twice.
	placeIDs map[string]int

	// exitIDs is every authored exit id to the index that declared it, the
	// other half of what a binding may name.
	exitIDs map[string]int

	// intelIDs is every authored intel record id to the index that declared
	// it — built by intel(), read by place() to answer "does this holder
	// name a record that exists".
	intelIDs map[string]int
}

func (v *validation) fail(path, format string, args ...any) {
	v.errs = append(v.errs, FieldError{Path: path, Message: fmt.Sprintf(format, args...)})
}

func (v *validation) header() {
	s := v.spec
	if s.Version != Version {
		v.fail("version", "dungeon spec version %d, which this build does not speak (it speaks %d)", s.Version, Version)
	}
	if s.Key == "" {
		v.fail("key", "the dungeon has no key")
	} else if !keyShape.MatchString(s.Key) {
		v.fail("key", "key %q is not [a-z0-9-]", s.Key)
	}
	if o, ok := orientations[s.Orientation]; ok {
		v.orientation = o
	} else {
		v.fail("orientation", "the dungeon does not say which way its hexes point "+
			"(%q; pointy or flat, and every [col,row] in the file depends on it)", s.Orientation)
	}
	if !voids[s.Void] {
		v.fail("void", "the dungeon does not say what its void is (%q; opaque or transparent, and there is no default)", s.Void)
	}
}

func (v *validation) geometryUsable() bool { return v.orientation != nil && v.regionOK }

// cell is the absolute axial cell an authored pair names under the declared
// orientation — the same conversion the composition runs.
func (v *validation) cell(at [2]int) spatial.Position {
	return encounter.HexCellAt(v.orientation, at[0], at[1])
}

func (v *validation) regions() {
	s := v.spec
	v.owner = map[spatial.Position]int{}
	v.authored = map[spatial.Position][2]int{}
	v.regionOK = v.orientation != nil
	if len(s.Regions) == 0 {
		v.fail("regions", "the dungeon has no regions, so it has no floor")
		v.regionOK = false
		return
	}

	ids := map[string]int{}
	for i, r := range s.Regions {
		p := fmt.Sprintf("regions[%d]", i)
		if r.ID == "" {
			v.fail(p+".id", "the region has no id")
			v.regionOK = false
		} else if prev, dup := ids[r.ID]; dup {
			v.fail(p+".id", "region %q is already declared at regions[%d]", r.ID, prev)
			v.regionOK = false
		} else {
			ids[r.ID] = i
		}
		if r.Archetype == "" {
			v.fail(p+".archetype", "the region has no archetype, and there is no default: the assets cannot dress it")
		}
		switch {
		case r.Lighting == nil:
			v.fail(p+".lighting", "the region does not say its lighting, and there is no default")
		case r.Lighting.Intensity == nil:
			v.fail(p+".lighting.intensity", "the lighting block does not say its intensity")
		case math.IsNaN(*r.Lighting.Intensity) || *r.Lighting.Intensity < 0 || *r.Lighting.Intensity > 1:
			v.fail(p+".lighting.intensity", "intensity %g is outside [0,1]", *r.Lighting.Intensity)
		}

		count := 0
		for row, cells := range r.Cells {
			for col, at := range cells {
				count++
				if v.orientation == nil {
					continue
				}
				cp := fmt.Sprintf("%s.cells[%d][%d]", p, row, col)
				cell := v.cell(at)
				if prev, taken := v.owner[cell]; taken {
					if prev == i {
						v.fail(cp, "[%d,%d] is painted twice in this region", at[0], at[1])
					} else {
						v.fail(cp, "[%d,%d] is already painted in region %q", at[0], at[1], s.Regions[prev].ID)
					}
					v.regionOK = false
					continue
				}
				v.owner[cell] = i
				v.authored[cell] = at
			}
		}
		if count == 0 {
			v.fail(p+".cells", "the region has no cells, and a region is its cells")
			v.regionOK = false
		}
	}
}

// scenery marks every authored scenery cell as floor nobody owns, refusing a
// cell a region already holds and one listed twice (F1). Cells land in
// `authored` beside the regions' so a refusal about a crossing into scenery
// can name it in the file's own coordinates.
func (v *validation) scenery() {
	v.sceneryAt = map[spatial.Position]bool{}
	for row, cells := range v.spec.Scenery {
		for col, at := range cells {
			p := fmt.Sprintf("scenery[%d][%d]", row, col)
			cell := v.cell(at)
			if prev, taken := v.owner[cell]; taken {
				v.fail(p, "[%d,%d] is already painted in region %q", at[0], at[1], v.spec.Regions[prev].ID)
				continue
			}
			if v.sceneryAt[cell] {
				v.fail(p, "[%d,%d] is painted twice in the scenery", at[0], at[1])
				continue
			}
			v.sceneryAt[cell] = true
			v.authored[cell] = at
		}
	}
}

// floor reports whether a cell is FLOOR — a region's, or scenery's. What a
// wall stands on, what a door opens onto, and what a prop sits on (C2, C3).
func (v *validation) floor(cell spatial.Position) bool {
	if _, owned := v.owner[cell]; owned {
		return true
	}

	return v.sceneryAt[cell]
}

// floorSet is every floor cell as a plain set — what the wall derivations ask
// (deriveWalls), which have no business knowing who owns what.
func (v *validation) floorSet() map[spatial.Position]bool {
	out := make(map[spatial.Position]bool, len(v.owner)+len(v.sceneryAt))
	for c := range v.owner {
		out[c] = true
	}
	for c := range v.sceneryAt {
		out[c] = true
	}

	return out
}

// wallPath is the YAML path of one wall entry.
func wallPath(i int) string { return fmt.Sprintf("walls[%d]", i) }

// doorPath is the YAML path of one door entry.
func doorPath(i int) string { return fmt.Sprintf("doors[%d]", i) }

// position checks one authored position: a representable cell, and an offset
// that is one of the seven this orientation knows (F8). Returns the fractional
// axial point it names.
func (v *validation) position(path, which string, g hexGeom, p PositionSpec) (axialPoint, bool) {
	for _, c := range p.Cell {
		if c > maxAuthoredCoord || c < -maxAuthoredCoord {
			v.fail(path+"."+which, "the %s cell [%d,%d] is outside the map", which, p.Cell[0], p.Cell[1])

			return axialPoint{}, false
		}
	}
	at, ok := g.axialAt(v.cell(p.Cell), p.Offset)
	if !ok {
		v.fail(path+"."+which,
			"the %s offset [%g,%g] is not one of the seven points a wall may stand at "+
				"under %s hexes: the six side midpoints, or the centre [0,0]",
			which, p.Offset[0], p.Offset[1], v.orientation.Kind())

		return axialPoint{}, false
	}

	return at, true
}

// maxAuthoredCoord bounds an authored cell the same way the composition does,
// so a coordinate that would overflow the embedding is refused in the file's
// own path rather than at construction.
const maxAuthoredCoord = 1 << 30

// adjacencyGrid is the calculator every adjacency question here asks: any
// instance of the family measures absolute cells correctly, since Distance
// never reads the grid's own bounds.
var adjacencyGrid = spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})

func normalizedCrossing(a, b spatial.Position) [2]spatial.Position {
	if b.X < a.X || (b.X == a.X && b.Y < a.Y) {
		return [2]spatial.Position{b, a}
	}
	return [2]spatial.Position{a, b}
}

// walls checks every wall is a line this build can read, then derives what it
// does: the cells it passes through, the crossings it blocks, and which of
// those cells it leaves too little of to stand on (C7, C8, C10).
//
// THE FILE HOLDS THE LINE AND NOTHING ELSE (design §1.5). There is no edge to
// check for adjacency and no run to check for contiguity, because there are no
// edges and no runs: two positions and the floor decide all of it.
func (v *validation) walls() {
	g := geometryOf(v.orientation)
	v.wallCrossings = map[[2]spatial.Position]int{}

	skip := map[int]bool{}
	ends := map[[2]axialPoint]int{}
	for i, w := range v.spec.Walls {
		p := wallPath(i)
		if w.Height != nil && (*w.Height < 1 || *w.Height > 3) {
			v.fail(p+".height", "height %g is outside [1,3]: walls raise, they never lower (rpg-project#273)", *w.Height)
		}
		start, okStart := v.position(p, "start", g, w.Start)
		end, okEnd := v.position(p, "end", g, w.End)
		if !okStart || !okEnd {
			skip[i] = true
			continue
		}
		if start == end {
			v.fail(p, "this wall starts and ends at the same point, so it stands nowhere")
			skip[i] = true
			continue
		}
		deg, straight := g.directionOf(start, end)
		if !straight {
			v.fail(p,
				"this wall runs at %.1f°, which is not one of the twelve directions a wall may take: "+
					"they are 30° apart, so move an end to a position that lines up",
				deg)
			skip[i] = true
			continue
		}
		key := [2]axialPoint{start, end}
		if end.Q < start.Q || (end.Q == start.Q && end.R < start.R) {
			key = [2]axialPoint{end, start}
		}
		if prev, dup := ends[key]; dup {
			v.fail(p, "this wall runs exactly where %s already does", wallPath(prev))
			skip[i] = true
			continue
		}
		ends[key] = i
	}

	v.derived = deriveWalls(v.spec, v.orientation, v.floorSet(), skip)

	for _, wg := range v.derived.Walls {
		// C2 IN THE LINE FORM. The envelope is still implied: a wall may run
		// along the outside of the world and cut nothing, but a wall that
		// touches no floor at all stands in a place nobody will ever be.
		if len(wg.Footprint) == 0 {
			v.fail(wallPath(wg.Index),
				"this wall passes through no floor at all: move it so it stands on the map")
			continue
		}
		for _, c := range wg.Crossings {
			if _, taken := v.wallCrossings[c]; !taken {
				v.wallCrossings[c] = wg.Index
			}
		}
	}
}

// doors checks every door is one position on exactly one wall, and works out
// the single crossing it opens (C14, F10, F11).
func (v *validation) doors() {
	g := geometryOf(v.orientation)
	v.doorAt = map[[2]spatial.Position]int{}
	ids := map[string]int{}
	for i, d := range v.spec.Doors {
		p := doorPath(i)
		if d.ID == "" {
			v.fail(p+".id", "the door has no id")
		} else if prev, dup := ids[d.ID]; dup {
			v.fail(p+".id", "door %q is already declared at doors[%d]", d.ID, prev)
		} else {
			ids[d.ID] = i
		}
		if d.Locked != nil {
			v.approaches(p+".locked",
				"this locked door needs at least one way through it — an ability and a DC", d.Locked)
		}
		if d.Concealed != nil {
			v.approaches(p+".concealed",
				"this concealed door needs at least one way to find it — an ability and a DC", d.Concealed)
		}

		at, ok := v.position(p, "at", g, d.At)
		if !ok {
			continue
		}
		// A DOOR IS A WAY THROUGH A SIDE (F11), so the centre is not a place
		// one can stand: there is no crossing under it to open.
		step, side := g.stepAt(d.At.Offset)
		if !side {
			v.fail(p+".at",
				"this door stands at the middle of [%d,%d], where there is no crossing to open: "+
					"put it on one of the six side midpoints",
				d.At.Cell[0], d.At.Cell[1])
			continue
		}

		// F10: EXACTLY ONE WALL. None is a door standing in the open; two is
		// a door in two walls at once, and neither has an answer.
		var through []int
		for _, wg := range v.derived.Walls {
			if wallPasses(wg, at) {
				through = append(through, wg.Index)
			}
		}
		switch len(through) {
		case 0:
			v.fail(p+".at",
				"no wall passes through this point, and a door is an opening in a wall: "+
					"run a wall through it, or move the door onto one")
			continue
		case 1:
		default:
			v.fail(p+".at", "two walls pass through this point (%s and %s), and a door can only open one of them",
				v.wallLabel(through[0]), v.wallLabel(through[1]))
			continue
		}

		here := v.cell(d.At.Cell)
		there := spatial.Position{X: here.X + float64(step[0]), Y: here.Y + float64(step[1])}
		// A DOOR BETWEEN TWO SEALED CELLS IS LEGAL (F11a) — nobody passes it,
		// and open, sight passes the gap: a window. A door into the VOID is
		// not, because there is nothing on the far side to open onto.
		if !v.floor(here) || !v.floor(there) {
			v.fail(p+".at",
				"this door opens between [%d,%d] and the void, and a door needs floor on both sides",
				d.At.Cell[0], d.At.Cell[1])
			continue
		}

		c := normalizedCrossing(here, there)
		if prev, taken := v.doorAt[c]; taken {
			v.fail(p+".at", "door %q already opens this crossing (doors[%d]), and one crossing cannot have two states",
				v.spec.Doors[prev].ID, prev)
			continue
		}
		v.doorAt[c] = i
	}
}

// wallLabel names a wall the way a refusal should: the author's own word when
// the wall carries one, its path when it does not.
func (v *validation) wallLabel(i int) string {
	if name := v.spec.Walls[i].Name; name != "" {
		return name
	}

	return wallPath(i)
}

// approaches validates one authored check: at least one approach, each naming
// the ability it rolls and a DC of at least 1. The empty-check refusal is the
// caller's sentence — worded for the form-filler at the door, since "the
// check has no approaches" means one thing on a lock and another on a
// concealment — and every per-approach refusal names the field that is
// missing at the row that misses it.
func (v *validation) approaches(path, none string, check CheckSpec) {
	if len(check) == 0 {
		v.fail(path, "%s", none)
		return
	}
	for j, a := range check {
		ap := fmt.Sprintf("%s[%d]", path, j)
		if a.Ability == "" {
			v.fail(ap+".ability", "the approach does not say which ability it rolls")
		}
		if a.DC < 1 {
			v.fail(ap+".dc", "an approach with dc %d has nothing to beat", a.DC)
		}
	}
}

func (v *validation) start() {
	s := v.spec
	if s.Start == nil {
		v.fail("start", "the dungeon does not say where the party starts")
		return
	}
	// STANDABLE, NOT MERELY FLOOR (rpg-project#360, F2). Scenery is floor
	// and nobody's feet touch it, so a start painted on the strip is refused
	// with the reason rather than with "not floor", which would now be a lie.
	start := v.cell(*s.Start)
	if v.sceneryAt[start] {
		v.fail("start", "the party starts at [%d,%d], which is scenery: nobody can stand there", s.Start[0], s.Start[1])
		return
	}
	if _, floor := v.owner[start]; !floor {
		v.fail("start", "the party starts at [%d,%d], which is not floor", s.Start[0], s.Start[1])
		return
	}
	// C12: A CELL A WALL HAS TOO LITTLE OF IS NOT A PLACE TO STAND, and the
	// refusal names the wall rather than the cell, because the wall is what
	// the author moves to fix it.
	if wall, sealed := v.derived.Sealed[start]; sealed {
		v.fail("start", "the party starts at [%d,%d], where %s leaves no room to stand",
			s.Start[0], s.Start[1], v.wallLabel(wall))
		return
	}
	for i, pl := range s.Place {
		if pl.At == *s.Start {
			v.fail("start", "the party starts at [%d,%d], where %q (place[%d]) already stands", s.Start[0], s.Start[1], pl.Ref, i)
		}
	}
}

func (v *validation) place() {
	s := v.spec
	occupied := map[[2]int]int{}
	bosses := map[int]int{} // region index -> place index of its boss
	v.placeIDs = map[string]int{}
	for i, pl := range s.Place {
		p := fmt.Sprintf("place[%d]", i)
		kind, err := refKind(pl.Ref)
		if err != nil {
			v.fail(p+".ref", "%v", err)
		}
		// P2: an id is optional, and unique when written. The refusal names
		// BOTH lines because the author has to look at the two of them to
		// decide which one keeps the name.
		if pl.ID != "" {
			if prev, dup := v.placeIDs[pl.ID]; dup {
				v.fail(p+".id", "id %q is already declared by %q (place[%d])", pl.ID, s.Place[prev].Ref, prev)
			} else {
				v.placeIDs[pl.ID] = i
			}
		}
		at := v.cell(pl.At)
		owner, owned := v.owner[at]
		// A PROP MAY SIT ON ANY FLOOR; A MONSTER NEEDS SOMEWHERE TO STAND
		// (rpg-project#360, F2). Rubble in a doorway is exactly what a
		// scenery brush is for; a skeleton on it is a creature standing on
		// something that is not standable, and the refusal says which.
		switch {
		case !owned && v.sceneryAt[at]:
			if kind == typeMonsters {
				v.fail(p+".at", "%q at [%d,%d], which is scenery: nobody can stand there", pl.Ref, pl.At[0], pl.At[1])
			}
		case !owned:
			v.fail(p+".at", "%q at [%d,%d], which is not floor", pl.Ref, pl.At[0], pl.At[1])
		default:
			// C12, the other half: a monster on a cell a wall has cut down to
			// nothing has nowhere to stand. A prop on one is fine — a wall
			// through a cell is exactly where an author puts rubble.
			if wall, sealed := v.derived.Sealed[at]; sealed && kind == typeMonsters {
				v.fail(p+".at", "%q at [%d,%d], where %s leaves no room to stand",
					pl.Ref, pl.At[0], pl.At[1], v.wallLabel(wall))
			}
		}
		// A holder names a record in this file, whatever KIND of thing it is
		// (R6): a monster carries it from spawn, a prop carries it until
		// somebody picks it up. Refused by name when it does not exist — a
		// placement holding nothing the author declared is a secret they
		// think they placed and did not.
		for j, id := range pl.Holds {
			if _, ok := v.intelIDs[id]; !ok {
				v.fail(fmt.Sprintf("%s.holds[%d]", p, j),
					"%q holds intel %q, and no record in this dungeon has that id", pl.Ref, id)
			}
		}

		if prev, taken := occupied[pl.At]; taken {
			v.fail(p+".at", "%q and %q (place[%d]) are on the same cell [%d,%d]", pl.Ref, s.Place[prev].Ref, prev, pl.At[0], pl.At[1])
		} else {
			occupied[pl.At] = i
		}

		switch kind {
		case typeMonsters:
			if pl.Holdable != nil {
				v.fail(p+".holdable", "%q is not a prop and cannot be held", pl.Ref)
			}
			if pl.BlocksMovement != nil {
				v.fail(p+".blocks_movement", "%q is not a prop and cannot declare what it blocks", pl.Ref)
			}
			if pl.BlocksLoS != nil {
				v.fail(p+".blocks_los", "%q is not a prop and cannot declare what it blocks", pl.Ref)
			}
			if pl.Facing != "" {
				v.fail(p+".facing", "%q is not a prop and cannot declare an authored facing", pl.Ref)
			}
			if pl.Offset != nil {
				v.fail(p+".offset", "%q is not a prop and cannot declare an authored offset", pl.Ref)
			}
			if pl.Targeting != nil && !targetings[*pl.Targeting] {
				v.fail(p+".targeting", "%q declares targeting %q, which is not a word this build knows", pl.Ref, *pl.Targeting)
			}
			if pl.Boss && owned {
				if prev, dup := bosses[owner]; dup {
					v.fail(p+".boss", "region %q already names %q (place[%d]) as its boss", s.Regions[owner].ID, s.Place[prev].Ref, prev)
				} else {
					bosses[owner] = i
				}
			}
		case typeProps:
			// A holdable prop must be nameable: the scenario binding names it
			// and so does the `held` beat.
			if pl.Holdable != nil && *pl.Holdable && pl.ID == "" {
				v.fail(p+".id", "%q is holdable and has no id, and a thing that can be picked up has to be nameable", pl.Ref)
			}
			if pl.Targeting != nil {
				v.fail(p+".targeting", "%q is not a monster and cannot have targeting", pl.Ref)
			}
			if pl.Boss {
				v.fail(p+".boss", "%q is not a monster and cannot be the boss", pl.Ref)
			}
			if pl.BlocksMovement == nil {
				v.fail(p+".blocks_movement", "%q does not say whether it blocks movement, and there is no default", pl.Ref)
			}
			if pl.BlocksLoS == nil {
				v.fail(p+".blocks_los", "%q does not say whether it blocks line of sight, and there is no default", pl.Ref)
			}
			if pl.Facing != "" && !facings[pl.Facing] {
				v.fail(p+".facing", "%q is not a compass direction: a facing is one of n|ne|e|se|s|sw|w|nw", pl.Facing)
			}
			if pl.Offset != nil {
				if len(pl.Offset) != 2 && len(pl.Offset) != 3 {
					v.fail(p+".offset", "offset must be [x,y] or [x,y,height], got %d value(s)", len(pl.Offset))
				} else {
					for j, c := range pl.Offset {
						if j < 2 {
							if c < -0.5 || c > 0.5 {
								v.fail(p+".offset", "offset[%d] %g is outside [-0.5,0.5]", j, c)
							}
						} else if c < 0 || c > 3 {
							v.fail(p+".offset", "offset[2] %g is outside [0,3]", c)
						}
					}
				}
			}
		}
	}
}

// exits validates the ways out (design §3.1): each has an id, no two share
// one, and each stands on floor somebody's feet can actually touch.
//
// STANDABLE, NOT MERELY FLOOR, and the refusals are [validation.start]'s
// word for word — an exit is the same kind of authored cell a start is, and
// the two answering differently about the same cell would be the bug. An
// exit nobody can stand on is a run nobody can leave, which is
// [encounter.ErrNoEnding]'s liveness hole reached from the outside.
func (v *validation) exits() {
	s := v.spec
	v.exitIDs = map[string]int{}
	for i, ex := range s.Exits {
		p := fmt.Sprintf("exits[%d]", i)
		if ex.ID == "" {
			v.fail(p+".id", "the exit has no id")
		} else if prev, dup := v.exitIDs[ex.ID]; dup {
			v.fail(p+".id", "exit %q is already declared at exits[%d]", ex.ID, prev)
		} else {
			v.exitIDs[ex.ID] = i
		}

		at := v.cell(ex.At)
		if v.sceneryAt[at] {
			v.fail(p+".at", "exit %q is at [%d,%d], which is scenery: nobody can stand there", ex.ID, ex.At[0], ex.At[1])
			continue
		}
		if _, floor := v.owner[at]; !floor {
			v.fail(p+".at", "exit %q is at [%d,%d], which is not floor", ex.ID, ex.At[0], ex.At[1])
			continue
		}
		if wall, sealed := v.derived.Sealed[at]; sealed {
			v.fail(p+".at", "exit %q is at [%d,%d], where %s leaves no room to stand",
				ex.ID, ex.At[0], ex.At[1], v.wallLabel(wall))
		}
	}
}

// intel validates the authored knowledge records (design §2): each has an
// id, no two share one, and each says exactly one thing it reveals that this
// dungeon actually has.
//
// RUN BEFORE place(), which asks whether a holder names a record that
// exists — the same ordering reason scenery runs before walls.
func (v *validation) intel() {
	s := v.spec
	v.intelIDs = map[string]int{}
	doorIDs := map[string]bool{}
	for _, d := range s.Doors {
		if d.ID != "" {
			doorIDs[d.ID] = true
		}
	}

	for i, rec := range s.Intel {
		p := fmt.Sprintf("intel[%d]", i)
		if rec.ID == "" {
			v.fail(p+".id", "the intel record has no id")
		} else if prev, dup := v.intelIDs[rec.ID]; dup {
			v.fail(p+".id", "intel %q is already declared at intel[%d]", rec.ID, prev)
		} else {
			v.intelIDs[rec.ID] = i
		}

		// EXACTLY ONE TARGET, and a record that reveals nothing is the one
		// an author wrote and did not finish. Nothing is defaulted: there is
		// no "reveals the nearest door".
		switch {
		case rec.Reveals.Door == "":
			v.fail(p+".reveals",
				"intel %q does not say what it reveals — today that is `door: <door id>`", rec.ID)
		case !doorIDs[rec.Reveals.Door]:
			v.fail(p+".reveals.door",
				"intel %q reveals door %q, and no door in this dungeon has that id", rec.ID, rec.Reveals.Door)
		}
	}
}

// scenarios validates the scenario bindings, and validates EXACTLY ONE THING
// about them: that every binding's value names a placement id or an exit id
// that exists in this file (design law C1, ruled 2026-09-01 — "the dungeon
// spec stores {scenario_id, bindings} as pure references").
//
// WHAT IS DELIBERATELY NOT CHECKED HERE, and where it is checked instead:
// whether the scenario id is one that exists, which keys it wants, which are
// required, and whether the thing a key names is the right KIND of thing —
// a prop where a prop is wanted, an exit where an exit is wanted, and
// holdable when the scenario is about carrying something out. Every one of
// those is a fact about a SCENARIO, and a scenario is content this package
// may not resolve. They are the scenario package's own refusals, made at its
// `New(cfg, compiled)` in form-filler words, where the author is looking at
// the form that asked the question.
//
// So the refusal here is about the FILE: a binding that names nothing is a
// dangling reference whatever scenario reads it, and this package is the one
// layer that can see the whole file at once.
//
// Enumeration is sorted by scenario id and then by field key, because a Go
// map range is not, and a refusal list whose order changes between runs is
// one nobody can diff.
func (v *validation) scenarios() {
	s := v.spec
	ids := make([]string, 0, len(s.Scenarios))
	for id := range s.Scenarios {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		bindings := s.Scenarios[id]
		keys := make([]string, 0, len(bindings))
		for k := range bindings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			named := bindings[k]
			p := fmt.Sprintf("scenarios.%s.%s", id, k)
			if named == "" {
				v.fail(p, "scenario %q binds %s to nothing", id, k)
				continue
			}
			if _, ok := v.placeIDs[named]; ok {
				continue
			}
			if _, ok := v.exitIDs[named]; ok {
				continue
			}
			v.fail(p, "scenario %q binds %s to %q, and nothing in this dungeon has that id",
				id, k, named)
		}
	}
}

// axialSteps are the six unit crossings out of an axial hex cell. Fixed and
// orientation-free BY CONSTRUCTION: orientation is spent converting the
// authored [col,row] into axial (encounter.HexCellAt), and in axial space
// every cell's neighbours are these six whichever way the hexes point —
// the same fact adjacencyGrid.Distance == 1 measures, enumerated instead of
// tested.
var axialSteps = [6][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, -1}, {-1, 1}}

// concealment is the authoring-coherence check (rpg-project#351,
// reformulated after #1370's review round caught the first form
// direction-blind). The invariant is boundary-shaped, not entrance-local:
//
//	THE FRONTIER BETWEEN VISIBLE AND HIDDEN SPACE CONSISTS OF CONCEALED
//	DOORS AND NOTHING ELSE, AND VISIBLE SPACE IS CONNECTED FROM THE
//	PARTY START.
//
// A "way" is a WALL-FREE PATH between two regions whose interior cells are
// all scenery: a crossing straight from one room to the other, or an area of
// scenery joining them (rpg-project#360, C4).
//
// IT IS A CONCEALED WAY IFF ANY CROSSING ALONG IT IS A CONCEALED DOOR, which
// is the same answer from either end. The author may put the secret's door on
// the hidden room's edge, on the visible room's edge, or between two scenery
// cells, and a crossing further along the path counts exactly as much as one
// at its end. See [validation.waysFrom] for why the walk is a flood and what
// classifying a way by its ENDS got wrong.
//
// Scenery had to enter this walk rather than be left out of it. Before it,
// every way was one crossing long and "is this room walled off" was a
// question about neighbours; a strip of ownerless floor between a visible
// room and a secret one is a corridor anybody can walk, and a frontier rule
// that could not see down it would have compiled a walk-in secret.
//
// A door's ways count ONCE PER DOOR and neighbouring pair, never once per
// edge — a door is one state over its edges (rpg-toolkit#1123), so a two-edge
// gate is one way in and refuses once. Two refusals fall out, both at the
// region's own concealed field, which is the one the form-filler flips:
//
//   - a way between an unconcealed region and a concealed one that is not a
//     concealed door — a plain door, or a bare gap — is a way anyone can
//     walk into the secret;
//   - an unconcealed region the party cannot reach from the start over
//     unconcealed ways through visible space is a room only a found secret
//     ever opens, which is a secret nobody declared.
//
// Everything wholly inside hidden space is nobody's business — a secret
// suite's interior doors and passages between two concealed rooms — and
// everything wholly inside visible space is free, including a concealed
// shortcut between two plain rooms (the room is no secret, the shortcut
// is). The first, entrance-local draft of this check refused the minimal
// honest dungeon — a visible start room whose only crossing is the one
// concealed closet door — and made secret suites unauthorable; the frontier
// form keeps every true refusal and drops the false ones.
//
// Deliberately NOT refused: the start's own region concealed (presence
// pierces at runtime — the occupants begin knowing, everyone else begins
// blind), and a region with no ways at all, concealed or not (dead content
// is the author's own business). A dungeon that authors no concealment
// anywhere has no frontier and is not walked at all: it compiles exactly as
// it always did.
//
// Enumeration walks the authored cells in file order, so every refusal
// lands deterministically and names the crossing in the author's own
// coordinates.
func (v *validation) concealment() {
	s := v.spec
	authored := false
	for _, r := range s.Regions {
		authored = authored || r.Concealed
	}
	for _, d := range s.Doors {
		authored = authored || d.Concealed != nil
	}
	if !authored {
		return
	}

	// Two walks over the same map, differing in one crossing kind.
	//
	// `open` stops at concealed doors, so every way it finds has no concealed
	// door anywhere along it — which by the rule above is exactly the set of
	// UNCONCEALED ways, and exactly what the frontier refuses and what visible
	// reach may use. `all` crosses them, so it finds every way of any kind,
	// which is what "a region with no ways at all is nobody's business" needs
	// to be able to tell apart from "a region reachable only through a
	// secret".
	var open, all []wayIn
	for i := range s.Regions {
		open = append(open, v.waysFrom(i, false)...)
		all = append(all, v.waysFrom(i, true)...)
	}

	// The frontier: a way between visible and hidden space with no concealed
	// door on it is a hole in the secret. The refusal names the way as the
	// SECRET'S OWN side meets it, which is the crossing its author has to wall
	// or conceal.
	//
	// ONE REFUSAL PER CROSSING NAMED, not per way. Since the walk floods once
	// per departure, two holes in a VISIBLE room leading through one area of
	// scenery to a single crossing on the secret's side are two ways — and
	// they are the same defect said twice, because the sentence the author
	// reads is about the secret's crossing and closing either hole is not what
	// fixes it. Two holes on the SECRET's own side name two crossings and
	// survive as two, which is the point of the enumeration.
	told := map[string]bool{}
	for _, w := range open {
		if s.Regions[w.a].Concealed == s.Regions[w.b].Concealed {
			continue
		}
		hidden, desc := w.a, w.descA
		if s.Regions[w.b].Concealed {
			hidden, desc = w.b, w.descB
		}
		at := fmt.Sprintf("regions[%d].concealed", hidden)
		if told[at+desc] {
			continue
		}
		told[at+desc] = true
		v.fail(at,
			"this room is concealed, but %s is there for anyone — "+
				"a walk-in room cannot be a secret: conceal every way in, or unconceal the room", desc)
	}

	// Visible space is connected from the party start, over unconcealed ways
	// between unconcealed regions only: a concealed door does not extend
	// visible reach (finding it is what would), and hidden space is not a
	// corridor visible space may route through. SCENERY IS such a corridor —
	// it belongs to nobody and hides nothing — so a room reachable only across
	// a strip is reachable, and `open` already says so.
	if s.Start == nil {
		return // start defects are start()'s to report; reach needs an anchor
	}
	startRegion, onFloor := v.owner[v.cell(*s.Start)]
	if !onFloor {
		return
	}
	visible := map[int]bool{startRegion: true}
	for changed := true; changed; {
		changed = false
		for _, w := range open {
			if s.Regions[w.a].Concealed || s.Regions[w.b].Concealed {
				continue
			}
			if visible[w.a] != visible[w.b] {
				visible[w.a], visible[w.b] = true, true
				changed = true
			}
		}
	}
	hasWay := make([]bool, len(s.Regions))
	for _, w := range all {
		hasWay[w.a], hasWay[w.b] = true, true
	}
	for i, r := range s.Regions {
		if r.Concealed || visible[i] || !hasWay[i] {
			continue
		}
		v.fail(fmt.Sprintf("regions[%d].concealed", i),
			"this room can only be entered through a concealed door — "+
				"conceal the room too, or give it another way in")
	}
}

// wayIn is one way between two regions: which two, and how it reads from each
// side. The two descriptions differ only for a way that runs THROUGH scenery,
// which meets each room at its own crossing; a way that IS one crossing reads
// the same from both.
type wayIn struct {
	a, b         int // region indices, a < b by construction (see waysFrom)
	descA, descB string
}

// crossingKey identifies one crossing for deduplication: the DOOR standing in
// it when there is one, so a two-edge gate counts once (rpg-toolkit#1123), and
// otherwise the crossing itself.
type crossingKey struct {
	door     int // -1 when no door stands in the crossing
	crossing [2]spatial.Position
}

// wayKey identifies one way for deduplication: the two regions it joins, the
// crossing it DEPARTS by, and the crossing it ARRIVES by.
//
// BOTH ENDS, because either one is a hole the author has to close, and a key
// that named only one of them enumerated that side's holes while collapsing
// the other's. Which side collapsed was decided by region index order — the
// walk runs from the lower-indexed room — so the same dungeon reported one
// hole or two depending on the order its rooms were written in. Found by
// Copilot on PR #1465, on a comment here that claimed the enumeration this
// key now actually performs.
type wayKey struct {
	a, b           int
	depart, arrive crossingKey
}

// crossingKeyOf collapses a crossing to what deduplication should count: the
// door, when a door stands in it, and the crossing itself otherwise.
func crossingKeyOf(c [2]spatial.Position, door int) crossingKey {
	if door >= 0 {
		return crossingKey{door: door}
	}

	return crossingKey{door: -1, crossing: c}
}

// waysFrom walks outward from one region and returns every way it finds.
//
// A FLOOD OVER CELLS, not a scan of neighbours (rpg-project#360, C4 as
// ruled). It leaves the origin region by any crossing it may take, travels
// only through SCENERY — never through another region's cells, whose floor is
// somebody's and whose crossings are that region's own frontier — and stops
// at the first cell of another region, which is a way between the two.
//
// `pastConcealed` is the one thing that varies. False stops at a concealed
// door as it stops at a wall, so nothing it finds has a concealed door
// anywhere along it: those are the unconcealed ways. True crosses them, so it
// finds every way of either kind.
//
// That is what makes the rule "a way is concealed iff ANY crossing along it is
// a concealed door" fall out rather than be enforced. An earlier draft
// enumerated strips and classified them by their two END crossings, which is
// strip-shaped thinking: a door may stand between two scenery cells (C2), and
// a scenery area may touch three regions, so a way has interior crossings and
// more than two ends. A flood has no ends to be short of.
//
// # Once per DEPARTURE, not once per strip
//
// `visited` is keyed by cell AND the crossing the path departed by, so two
// holes from one room into one area of scenery flood it twice and produce
// their own ways. Keyed by the cell alone, the second hole found the first
// hole's cells already marked and vanished — and the author, having walled the
// hole the refusal named, recompiled to meet its twin.
//
// The cost is a flood per departure rather than per region, which is the
// honest shape of the question: each hole is a separate thing the author has
// to close, so each one has to be walked.
func (v *validation) waysFrom(origin int, pastConcealed bool) []wayIn {
	type reached struct {
		cell   spatial.Position
		depart crossingKey // the crossing this path left the origin region by
		first  string      // and how to say so
	}

	var out []wayIn
	seen := map[wayKey]bool{}
	visited := map[reached]bool{}
	var queue []reached

	// arrive records a way, or extends the flood, for one crossing out of
	// `from` into `n`. A zero `depart.door` marks a step out of the origin
	// region itself, which is where the departure is established.
	arrive := func(from, n spatial.Position, path reached) {
		c := normalizedCrossing(from, n)
		door, concealed, wall := v.claims(c)
		if wall || (concealed && !pastConcealed) {
			return
		}
		desc := v.crossingDesc(from, n, door)
		if path.first == "" {
			path.depart, path.first = crossingKeyOf(c, door), desc
		}
		if there, owned := v.owner[n]; owned {
			// EACH PAIR IS DISCOVERED FROM ITS LOWER-INDEXED SIDE ONLY.
			// Crossings are undirected and both walls and concealed doors
			// stop the flood in either direction, so whatever B reaches
			// through the scenery A reaches too — walking from both ends
			// would report one hole twice. `there == origin` is the same
			// rule's other half: out through the strip and back into the
			// room you left is not a way between two rooms.
			if there <= origin {
				return
			}
			key := wayKey{a: origin, b: there, depart: path.depart, arrive: crossingKeyOf(c, door)}
			if seen[key] {
				return
			}
			seen[key] = true
			out = append(out, wayIn{a: origin, b: there, descA: path.first, descB: desc})
			return
		}
		if !v.sceneryAt[n] {
			return
		}
		next := reached{cell: n, depart: path.depart, first: path.first}
		if visited[next] {
			return
		}
		visited[next] = true
		queue = append(queue, next)
	}

	for _, row := range v.spec.Regions[origin].Cells {
		for _, at := range row {
			cell := v.cell(at)
			for _, step := range axialSteps {
				arrive(cell, spatial.Position{X: cell.X + step[0], Y: cell.Y + step[1]}, reached{})
			}
		}
	}
	for len(queue) > 0 {
		here := queue[0]
		queue = queue[1:]
		for _, step := range axialSteps {
			arrive(here.cell, spatial.Position{X: here.cell.X + step[0], Y: here.cell.Y + step[1]}, here)
		}
	}

	return out
}

// claims reports what an authored thing has put in a crossing: the index of
// the door standing in it (or -1), whether that door is concealed, and whether
// a wall is drawn there.
//
// A door's claim is not a wall's: a door standing in a wall replaces it (see
// doors()), which is why the walk treats that crossing as a way in.
func (v *validation) claims(c [2]spatial.Position) (door int, concealed, wall bool) {
	if d, open := v.doorAt[c]; open {
		return d, v.spec.Doors[d].Concealed != nil, false
	}
	_, blocked := v.wallCrossings[c]

	return -1, false, blocked
}

// crossingDesc says how a crossing reads in the author's own coordinates, for
// the refusal that names it. A crossing into or out of scenery says so, since
// "the open way between two cells" would otherwise send the author looking for
// a room that is not there.
func (v *validation) crossingDesc(from, to spatial.Position, door int) string {
	if door >= 0 {
		return fmt.Sprintf("its door %q (doors[%d])", v.spec.Doors[door].ID, door)
	}
	f, t := v.authored[from], v.authored[to]
	switch {
	case v.sceneryAt[from]:
		return fmt.Sprintf("the open way between [%d,%d] and the scenery at [%d,%d]", t[0], t[1], f[0], f[1])
	case v.sceneryAt[to]:
		return fmt.Sprintf("the open way between [%d,%d] and the scenery at [%d,%d]", f[0], f[1], t[0], t[1])
	default:
		return fmt.Sprintf("the open way between [%d,%d] and [%d,%d]", f[0], f[1], t[0], t[1])
	}
}

// refKind returns a ref's type segment, which is what routes a placement.
//
// Parsed here rather than through the rulebook's own ref parser for the reason
// this package exists: importing one would break design law C1. The check is
// deliberately shallow — three non-empty segments — because "is this a ref that
// resolves to real content" is a question only the layer that owns content can
// answer.
func refKind(ref string) (string, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("ref %q is not module:type:id", ref)
	}
	switch parts[1] {
	case typeProps, typeMonsters:
		return parts[1], nil
	default:
		return "", fmt.Errorf("ref %q names type %q, which this compiler cannot place", ref, parts[1])
	}
}
