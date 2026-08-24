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

	// facingsByOrientation is the SIX names each hex orientation actually
	// has, and the two sets are DIFFERENT (rpg-project#261 ruling): a
	// flat-top hex has flat top and bottom edges, so its facings are
	// n|s|ne|nw|se|sw; a pointy-top hex has pointed top and bottom instead,
	// so its facings are e|w|ne|nw|se|sw. A name from the wrong set is an
	// ERROR, never a silent snap to the nearest valid one — the vocabulary
	// and the rendered yaw both derive from the SAME declared orientation
	// (ideas/dungeon-builder/prop-facing-offset.md).
	facingsByOrientation = map[encounter.OrientationKind]map[string]bool{
		encounter.OrientationFlatTop:   {"n": true, "s": true, "ne": true, "nw": true, "se": true, "sw": true},
		encounter.OrientationPointyTop: {"e": true, "w": true, "ne": true, "nw": true, "se": true, "sw": true},
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
		c, ok := v.edge(p, w)
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
		}
		if d.Locked != nil {
			if d.Locked.DC < 1 {
				v.fail(p+".locked.dc", "a lock with dc %d has nothing to beat", d.Locked.DC)
			}
			if d.Locked.Ability == "" {
				v.fail(p+".locked.ability", "a lock must say which ability beats it")
			}
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
			if pl.Facing != "" && !facingsByOrientation[v.orientation.Kind()][pl.Facing] {
				v.fail(p+".facing", "%s-top has no facing %q", v.orientation.Kind(), pl.Facing)
			}
			if pl.Offset != nil {
				if len(pl.Offset) != 2 {
					v.fail(p+".offset", "offset must be [x,y], got %d value(s)", len(pl.Offset))
				} else {
					for j, c := range pl.Offset {
						if c < -0.5 || c > 0.5 {
							v.fail(p+".offset", "offset[%d] %g is outside [-0.5,0.5]", j, c)
						}
					}
				}
			}
		}
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
