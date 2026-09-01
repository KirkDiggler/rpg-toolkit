// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"math"
	"regexp"
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
		v.walls()
		v.doors()
		v.start()
		v.place()
		v.concealment()
	}
	return v.errs
}

// validation accumulates defects and the floor they are checked against.
type validation struct {
	spec *Spec
	errs []FieldError

	orientation encounter.Orientation

	// owner is every floor cell (absolute axial) to the index of the region
	// that owns it — the same map compileField builds, built here so the
	// file's own defects are reported in the file's own paths.
	owner map[spatial.Position]int

	// regionOK is whether the region list was sound enough to check edges
	// and placements against.
	regionOK bool

	// crossings is every wall's and door's normalized crossing to the path
	// that claimed it, so an edge listed twice — or as both — is refused.
	crossings map[[2]spatial.Position]string

	// authored is the reverse of the cell conversion: every floor cell back
	// to the [col,row] pair the author wrote, so a refusal about a crossing
	// can name it in the file's own coordinates.
	authored map[spatial.Position][2]int

	// doorAt is every validated door crossing to the index of the door
	// standing in it, for the coherence check's question: is this way in a
	// door, and is that door concealed?
	doorAt map[[2]spatial.Position]int
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

// edge checks one authored crossing: both endpoints floor, and adjacent under
// the orientation. Returns the normalized absolute crossing and whether it
// passed.
func (v *validation) edge(path string, e EdgeSpec) ([2]spatial.Position, bool) {
	a, b := v.cell(e[0]), v.cell(e[1])
	ok := true
	for i, end := range []spatial.Position{a, b} {
		if _, floor := v.owner[end]; !floor {
			v.fail(path, "endpoint [%d,%d] is not floor: the envelope is implied, never written", e[i][0], e[i][1])
			ok = false
		}
	}
	if a == b {
		v.fail(path, "[%d,%d] is both ends of the edge", e[0][0], e[0][1])
		return [2]spatial.Position{}, false
	}
	if ok && adjacencyGrid.Distance(a, b) != 1 {
		v.fail(path, "[%d,%d] and [%d,%d] are not adjacent under %s", e[0][0], e[0][1], e[1][0], e[1][1], v.orientation.Kind())
		ok = false
	}
	return normalizedCrossing(a, b), ok
}

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

func (v *validation) walls() {
	v.crossings = map[[2]spatial.Position]string{}
	for i, w := range v.spec.Walls {
		p := fmt.Sprintf("walls[%d]", i)
		if w.Height != nil && (*w.Height < 1 || *w.Height > 3) {
			v.fail(p+".height", "height %g is outside [1,3]: walls raise, they never lower (rpg-project#273)", *w.Height)
		}
		c, ok := v.edge(p, w.Between)
		if !ok {
			continue
		}
		if prev, dup := v.crossings[c]; dup {
			v.fail(p, "the same edge is already listed at %s", prev)
			continue
		}
		v.crossings[c] = p
	}
}

func (v *validation) doors() {
	v.doorAt = map[[2]spatial.Position]int{}
	ids := map[string]int{}
	for i, d := range v.spec.Doors {
		p := fmt.Sprintf("doors[%d]", i)
		if d.ID == "" {
			v.fail(p+".id", "the door has no id")
		} else if prev, dup := ids[d.ID]; dup {
			v.fail(p+".id", "door %q is already declared at doors[%d]", d.ID, prev)
		} else {
			ids[d.ID] = i
		}
		if len(d.Edges) == 0 {
			v.fail(p+".edges", "the door stands in no edges")
		}
		for j, e := range d.Edges {
			ep := fmt.Sprintf("%s.edges[%d]", p, j)
			c, ok := v.edge(ep, e)
			if !ok {
				continue
			}
			if prev, taken := v.crossings[c]; taken {
				if strings.HasPrefix(prev, "walls[") {
					v.fail(ep, "this edge is also a wall (%s), and a door cannot stand in a wall", prev)
				} else {
					v.fail(ep, "this edge is already a door's (%s), and one edge cannot have two states", prev)
				}
				continue
			}
			v.crossings[c] = ep
			v.doorAt[c] = i
		}
		if d.Locked != nil {
			v.approaches(p+".locked",
				"this locked door needs at least one way through it — an ability and a DC", d.Locked)
		}
		if d.Concealed != nil {
			v.approaches(p+".concealed",
				"this concealed door needs at least one way to find it — an ability and a DC", d.Concealed)
		}
	}
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
	if _, floor := v.owner[v.cell(*s.Start)]; !floor {
		v.fail("start", "the party starts at [%d,%d], which is not floor", s.Start[0], s.Start[1])
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
	for i, pl := range s.Place {
		p := fmt.Sprintf("place[%d]", i)
		kind, err := refKind(pl.Ref)
		if err != nil {
			v.fail(p+".ref", "%v", err)
		}
		owner, floor := v.owner[v.cell(pl.At)]
		if !floor {
			v.fail(p+".at", "%q at [%d,%d], which is not floor", pl.Ref, pl.At[0], pl.At[1])
		}
		if prev, taken := occupied[pl.At]; taken {
			v.fail(p+".at", "%q and %q (place[%d]) are on the same cell [%d,%d]", pl.Ref, s.Place[prev].Ref, prev, pl.At[0], pl.At[1])
		} else {
			occupied[pl.At] = i
		}

		switch kind {
		case typeMonsters:
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
			if pl.Boss && floor {
				if prev, dup := bosses[owner]; dup {
					v.fail(p+".boss", "region %q already names %q (place[%d]) as its boss", s.Regions[owner].ID, s.Place[prev].Ref, prev)
				} else {
					bosses[owner] = i
				}
			}
		case typeProps:
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
// A "way" is a crossing between two regions that is not a wall: a door's
// edge, or a bare doorless gap. A door's ways count ONCE PER DOOR and
// neighbouring pair, never once per edge — a door is one state over its
// edges (rpg-toolkit#1123), so a two-edge gate is one way in and refuses
// once. Two refusals fall out, both at the region's own concealed field,
// which is the one the form-filler flips:
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

	// The ways: every non-wall crossing between two regions, door ways
	// deduped per door and pair.
	type wayIn struct {
		a, b      int // region indices, a <= b
		concealed bool
		desc      string
	}
	var ways []wayIn
	seenCrossing := map[[2]spatial.Position]bool{}
	seenDoor := map[[3]int]bool{}
	for _, r := range s.Regions {
		for _, row := range r.Cells {
			for _, at := range row {
				cell := v.cell(at)
				here := v.owner[cell]
				for _, step := range axialSteps {
					n := spatial.Position{X: cell.X + step[0], Y: cell.Y + step[1]}
					there, floor := v.owner[n]
					if !floor || there == here {
						continue
					}
					c := normalizedCrossing(cell, n)
					if seenCrossing[c] {
						continue
					}
					seenCrossing[c] = true
					path, claimed := v.crossings[c]
					if claimed && strings.HasPrefix(path, "walls[") {
						continue // a wall is not a way in
					}
					a, b := here, there
					if b < a {
						a, b = b, a
					}
					w := wayIn{a: a, b: b}
					if claimed {
						d := v.doorAt[c]
						if seenDoor[[3]int{d, a, b}] {
							continue // one door, one way (rpg-toolkit#1123)
						}
						seenDoor[[3]int{d, a, b}] = true
						w.concealed = s.Doors[d].Concealed != nil
						w.desc = fmt.Sprintf("its door %q (doors[%d])", s.Doors[d].ID, d)
					} else {
						na := v.authored[n]
						w.desc = fmt.Sprintf("the open way between [%d,%d] and [%d,%d]", at[0], at[1], na[0], na[1])
					}
					ways = append(ways, w)
				}
			}
		}
	}

	// The frontier: a way between visible and hidden space that is not a
	// concealed door is a hole in the secret.
	for _, w := range ways {
		if s.Regions[w.a].Concealed == s.Regions[w.b].Concealed || w.concealed {
			continue
		}
		hidden := w.a
		if s.Regions[w.b].Concealed {
			hidden = w.b
		}
		v.fail(fmt.Sprintf("regions[%d].concealed", hidden),
			"this room is concealed, but %s is there for anyone — "+
				"a walk-in room cannot be a secret: conceal every way in, or unconceal the room", w.desc)
	}

	// Visible space is connected from the party start. Walked over
	// unconcealed ways between unconcealed regions only: a concealed door
	// does not extend visible reach (finding it is what would), and hidden
	// space is not a corridor visible space may route through.
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
		for _, w := range ways {
			if w.concealed || s.Regions[w.a].Concealed || s.Regions[w.b].Concealed {
				continue
			}
			if visible[w.a] != visible[w.b] {
				visible[w.a], visible[w.b] = true, true
				changed = true
			}
		}
	}
	hasWay := make([]bool, len(s.Regions))
	for _, w := range ways {
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
