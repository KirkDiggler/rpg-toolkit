// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// layout.go places every region (README: keepgen.Layout). It works in ODD-R
// OFFSET cells — col runs west to east, row north to south, odd rows shifted
// half a hex east — because a room is a rectangle there; [cell.axial] is the
// one conversion to the file's axial `{q, r}`.
//
// THE SCENE FRAME is pointy-top with circumradius 1 (`hexRadius: 1`): a
// cell's centre is x = sqrt(3)(col + (row&1)/2), z = 1.5 row. A vertical
// seam before column c runs at x = sqrt(3)(c - 1/4) and a horizontal seam
// before row r at z = 1.5r - 3/4 — quarter lines, the reference tomb's and
// bigSite's, that pass between the centres of both row parities and cover
// none.

// maxScale is the most a room is scaled, linear (design One keep, R2).
const maxScale = 1.5

var sqrt3 = math.Sqrt(3)

// cell is an odd-r offset cell.
type cell struct{ col, row int }

// axial is the file's spelling of the cell: q = col - (row - (row&1))/2.
func (c cell) axial() dungeonspec.RoomCell {
	return dungeonspec.RoomCell{Q: c.col - (c.row-(c.row&1))/2, R: c.row}
}

// fromAxial is [cell.axial] backwards.
func fromAxial(q, r int) cell { return cell{col: q + (r-(r&1))/2, row: r} }

// centre is the cell's centre in scene units.
func (c cell) centre() (x, z float64) {
	return sqrt3 * (float64(c.col) + float64(c.row&1)/2), 1.5 * float64(c.row)
}

// axialDirections are the six neighbours of an axial cell.
var axialDirections = [6][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, -1}, {-1, 1}}

// neighbours are the cell's six neighbours.
func (c cell) neighbours() [6]cell {
	a := c.axial()
	var out [6]cell
	for i, d := range axialDirections {
		out[i] = fromAxial(a.Q+d[0], a.R+d[1])
	}
	return out
}

// hexDistance is the number of steps between two cells.
func hexDistance(a, b cell) int {
	p, q := a.axial(), b.axial()
	dq, dr := p.Q-q.Q, p.R-q.R
	return (abs(dq) + abs(dr) + abs(dq+dr)) / 2
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// seamX is the scene x of the vertical seam before column c.
func seamX(c int) float64 { return sqrt3 * (float64(c) - 0.25) }

// seamZ is the scene z of the horizontal seam before row r.
func seamZ(r int) float64 { return 1.5*float64(r) - 0.75 }

// rect is a half-open block of offset cells: cols [c0, c1), rows [r0, r1).
type rect struct{ c0, c1, r0, r1 int }

func (r rect) width() int { return r.c1 - r.c0 }
func (r rect) depth() int { return r.r1 - r.r0 }

// region is one placed region: a rectangle and, for the south grounds only,
// a partial row of `extra` cells under it from its west edge — the remainder
// that makes the floor the requested count exactly.
type region struct {
	id, name string
	kind     string // palette; grounds for the courtyard and both grounds
	side     string // empty for the courtyard and grounds
	lighting float64
	// open marks the courtyard and both grounds: they take the grounds
	// density rather than the room density.
	open  bool
	rect  rect
	extra int
}

// size is the region's cell count.
func (g region) size() int { return g.rect.width()*g.rect.depth() + g.extra }

// cells lists the region's cells, rows north to south, west to east.
func (g region) cells() []cell {
	out := make([]cell, 0, g.size())
	for row := g.rect.r0; row < g.rect.r1; row++ {
		for col := g.rect.c0; col < g.rect.c1; col++ {
			out = append(out, cell{col, row})
		}
	}
	for col := g.rect.c0; col < g.rect.c0+g.extra; col++ {
		out = append(out, cell{col, g.rect.r1})
	}
	return out
}

// layout is every region placed, plus where the party starts and the exits.
type layout struct {
	scale     float64
	planHexes int
	regions   []region // rooms in plan order, courtyard, west grounds, south grounds
	index     map[string]int
	start     cell
	exits     []placedExit
}

type placedExit struct {
	id string
	at cell
}

// planHexes is the plan's floor at scale 1 (plan G1): rooms, courtyard and
// both grounds. It refuses a plan whose sides do not close.
func planHexes(p *Plan) (int, error) {
	k, err := keepOf(p)
	if err != nil {
		return 0, err
	}
	return k.hexes(p), nil
}

// hexes is the keep's floor and both grounds at scale 1.
func (k keep) hexes(p *Plan) int {
	return k.w*k.h + p.Grounds.West.Width*k.h + p.Grounds.South.Depth*(p.Grounds.West.Width+k.w)
}

// keep is the plan's keep at scale 1: its sides, its size and its bands.
type keep struct {
	north, east, south, west []Room
	w, h                     int // the keep's width and height
	dn, ds, ww, we           int // band depth north and south, band width west and east
}

// keepOf sorts the rooms by side and checks that the sides close into a
// keep: each side has a room; north and south rooms share their side's depth
// and span the same width; west and east rooms share their side's width and
// fill the same height; and a courtyard is left in the middle.
func keepOf(p *Plan) (keep, error) {
	var k keep
	for _, r := range p.Rooms {
		switch r.Side {
		case SideNorth:
			k.north = append(k.north, r)
		case SideEast:
			k.east = append(k.east, r)
		case SideSouth:
			k.south = append(k.south, r)
		case SideWest:
			k.west = append(k.west, r)
		}
	}
	for _, s := range []struct {
		name  string
		rooms []Room
	}{{SideNorth, k.north}, {SideEast, k.east}, {SideSouth, k.south}, {SideWest, k.west}} {
		if len(s.rooms) == 0 {
			return keep{}, fmt.Errorf("keepgen: layout: the %s side has no room", s.name)
		}
	}
	shared := func(side string, rooms []Room, of func(Room) int, what string) (int, error) {
		v := of(rooms[0])
		for _, r := range rooms[1:] {
			if of(r) != v {
				return 0, fmt.Errorf("keepgen: layout: %s rooms share one %s: %q is %d, %q is %d",
					side, what, rooms[0].ID, v, r.ID, of(r))
			}
		}
		return v, nil
	}
	sum := func(rooms []Room, of func(Room) int) int {
		t := 0
		for _, r := range rooms {
			t += of(r)
		}
		return t
	}
	width := func(r Room) int { return r.Width }
	depth := func(r Room) int { return r.Depth }
	var err error
	if k.dn, err = shared(SideNorth, k.north, depth, "depth"); err != nil {
		return keep{}, err
	}
	if k.ds, err = shared(SideSouth, k.south, depth, "depth"); err != nil {
		return keep{}, err
	}
	if k.ww, err = shared(SideWest, k.west, width, "width"); err != nil {
		return keep{}, err
	}
	if k.we, err = shared(SideEast, k.east, width, "width"); err != nil {
		return keep{}, err
	}
	k.w = sum(k.north, width)
	if s := sum(k.south, width); s != k.w {
		return keep{}, fmt.Errorf("keepgen: layout: the north side spans %d hexes and the south side %d; both span the keep", k.w, s)
	}
	mid := sum(k.west, depth)
	if e := sum(k.east, depth); e != mid {
		return keep{}, fmt.Errorf("keepgen: layout: the west side fills %d rows and the east side %d; both fill the keep between north and south", mid, e)
	}
	if k.ww+k.we >= k.w {
		return keep{}, fmt.Errorf("keepgen: layout: the west and east sides (%d + %d) leave no courtyard in a keep %d wide", k.ww, k.we, k.w)
	}
	k.h = k.dn + mid + k.ds
	return k, nil
}

// layoutOf places the plan at hexes (design One keep, R2). The scale is
// min(1.5, sqrt(hexes / planHexes)) and every room boundary is the plan's
// boundary scaled and rounded, so the sides still close; the west grounds
// scale with the keep; the south grounds take whatever floor is left as
// full rows across the site and one partial row under them, so the floor is
// exactly hexes.
//
// It refuses fewer hexes than the plan's own floor, a plan whose sides do not
// close, and a size whose south grounds would be left without a row.
func layoutOf(p *Plan, hexes int) (*layout, error) {
	k, err := keepOf(p)
	if err != nil {
		return nil, err
	}
	base := k.hexes(p)
	if hexes < base {
		return nil, fmt.Errorf("keepgen: the plan needs at least %d hexes; %d were asked for", base, hexes)
	}
	s := math.Min(maxScale, math.Sqrt(float64(hexes)/float64(base)))
	sc := func(v int) int { return int(math.Round(s * float64(v))) }

	out := &layout{scale: s, planHexes: base, index: map[string]int{}}
	gw := sc(p.Grounds.West.Width) // the keep starts east of the west grounds
	add := func(g region) {
		out.index[g.id] = len(out.regions)
		out.regions = append(out.regions, g)
	}
	rooms := map[string]rect{}
	place := func(rooms []Room, at func(from, to int) rect, of func(Room) int) map[string]rect {
		got := map[string]rect{}
		pos := 0
		for _, r := range rooms {
			got[r.ID] = at(pos, pos+of(r))
			pos += of(r)
		}
		return got
	}
	width := func(r Room) int { return r.Width }
	depth := func(r Room) int { return r.Depth }
	for id, r := range place(k.north, func(a, b int) rect {
		return rect{gw + sc(a), gw + sc(b), 0, sc(k.dn)}
	}, width) {
		rooms[id] = r
	}
	for id, r := range place(k.south, func(a, b int) rect {
		return rect{gw + sc(a), gw + sc(b), sc(k.dn + (k.h - k.dn - k.ds)), sc(k.h)}
	}, width) {
		rooms[id] = r
	}
	for id, r := range place(k.west, func(a, b int) rect {
		return rect{gw, gw + sc(k.ww), sc(k.dn + a), sc(k.dn + b)}
	}, depth) {
		rooms[id] = r
	}
	for id, r := range place(k.east, func(a, b int) rect {
		return rect{gw + sc(k.w-k.we), gw + sc(k.w), sc(k.dn + a), sc(k.dn + b)}
	}, depth) {
		rooms[id] = r
	}
	for _, r := range p.Rooms {
		add(region{id: r.ID, name: r.Name, kind: r.Kind, side: r.Side, lighting: *r.Lighting, rect: rooms[r.ID]})
	}
	add(region{
		id: p.Courtyard.ID, name: p.Courtyard.Name, kind: KindGrounds, lighting: *p.Courtyard.Lighting, open: true,
		rect: rect{gw + sc(k.ww), gw + sc(k.w-k.we), sc(k.dn), sc(k.h - k.ds)},
	})
	w, h := sc(k.w), sc(k.h)
	add(region{
		id: p.Grounds.West.ID, name: p.Grounds.West.Name, kind: KindGrounds, lighting: *p.Grounds.West.Lighting, open: true,
		rect: rect{0, gw, 0, h},
	})
	across := gw + w
	rest := hexes - across*h
	rows, extra := rest/across, rest%across
	if rows < 1 {
		return nil, fmt.Errorf("keepgen: at %d hexes the south grounds get %d, less than one row of %d", hexes, rest, across)
	}
	add(region{
		id: p.Grounds.South.ID, name: p.Grounds.South.Name, kind: KindGrounds, lighting: *p.Grounds.South.Lighting, open: true,
		rect: rect{0, across, h, h + rows}, extra: extra,
	})

	start := out.regions[out.index[p.PartyStart]].rect
	out.start = cell{(start.c0 + start.c1) / 2, (start.r0 + start.r1) / 2}
	for _, ex := range p.Exits {
		g := out.regions[out.index[ex.Room]]
		r := g.rect
		midCol, midRow := (r.c0+r.c1)/2, (r.r0+r.r1)/2
		at := cell{midCol, r.r0}
		switch g.side {
		case SideSouth:
			at = cell{midCol, r.r1 - 1}
		case SideWest:
			at = cell{r.c0, midRow}
		case SideEast:
			at = cell{r.c1 - 1, midRow}
		}
		out.exits = append(out.exits, placedExit{id: ex.ID, at: at})
	}
	return out, nil
}

// owners maps every floor cell to its region's index.
func (l *layout) owners() map[cell]int {
	out := make(map[cell]int, 1<<14)
	for i, g := range l.regions {
		for _, c := range g.cells() {
			out[c] = i
		}
	}
	return out
}

// floor is the number of floor cells.
func (l *layout) floor() int {
	n := 0
	for _, g := range l.regions {
		n += g.size()
	}
	return n
}
