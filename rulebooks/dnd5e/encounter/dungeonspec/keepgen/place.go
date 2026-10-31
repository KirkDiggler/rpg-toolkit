// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"sort"
)

// place.go places every thing (README: keepgen.Place): the walls, one per
// shared edge of two regions; one opening and one door per plan door; props
// and monsters from the palettes at the requested densities, seeded.

const (
	// doorWidth is every opening's width, scene units: the castle's.
	doorWidth = 2.0
	// minDoorWall is the shortest wall a door is cut into.
	minDoorWall = 2 * doorWidth
	// thresholdRadius is how far from an opening's centre, scene units, a
	// cell centre is kept clear of props and monsters: both cells of every
	// crossing the opening spans, with room to step through.
	thresholdRadius = 2.6
	// startRadius is how many steps around the party start are kept clear,
	// so the start region always seats a party of four together.
	startRadius = 2
	// exitRadius is how many steps around an exit cell are kept clear.
	exitRadius = 1

	// wallHeight, wallThickness and blockerDepth are the castle's wall.
	// The blocker is thinner than a quarter line's distance to the nearest
	// centre on either side (sqrt(3)/4 across a vertical seam, 3/4 across a
	// horizontal one), so no wall covers a cell centre.
	wallHeight    = 3.0
	wallThickness = 0.3
	blockerDepth  = 0.2
)

// wall is one wall line between two regions.
type wall struct {
	id, label string
	a, b      int // region indices, a < b
	sx, sz    float64
	ex, ez    float64
	length    float64
	door      *door
}

// door is a plan door: an opening in its wall with an attached door.
type door struct {
	id, openingID string
	position      float64 // along the wall from its start
	cx, cz        float64 // the opening's centre
}

// placedProp is one scene prop.
type placedProp struct {
	id       string
	region   int
	entry    PropEntry
	at       cell
	rotation float64 // 0 or pi/2
	covered  []cell
	blocking bool // carries a propDeclaration
}

// placedMonster is one declared monster.
type placedMonster struct {
	id, ref string
	region  int
	at      cell
}

// site is everything placed.
type site struct {
	walls    []wall
	props    []placedProp
	monsters []placedMonster
	reserved map[cell]bool
}

// placeAll runs the placement. A plan door between two regions that share
// no wall, or share one too short to cut a door into, is refused by name.
func placeAll(p *Plan, l *layout, o Options) (*site, error) {
	owner := l.owners()
	s := &site{reserved: map[cell]bool{}}
	if err := s.placeWalls(p, l); err != nil {
		return nil, err
	}
	s.reserve(l, owner)
	occupied := map[cell]bool{}
	for i, g := range l.regions {
		density := o.PropDensity
		if g.open {
			density = o.GroundsDensity
		}
		s.placeProps(p, l, owner, occupied, i, count(g.size(), density), o.Seed)
	}
	taken := map[cell]bool{}
	for i, g := range l.regions {
		s.placeMonsters(p, l, owner, occupied, taken, i, count(g.size(), o.MonsterDensity), o.Seed)
	}
	return s, nil
}

// count is how many things a region of size cells takes at density per 100.
func count(size int, density float64) int {
	return int(math.Round(float64(size) * density / 100))
}

// placeWalls draws one wall per pair of regions that share an edge — except
// two open regions (the grounds), which meet without one — and cuts each
// plan door into its pair's wall at the wall's middle.
func (s *site) placeWalls(p *Plan, l *layout) error {
	pairs := map[[2]int]int{}
	for i := range l.regions {
		for j := i + 1; j < len(l.regions); j++ {
			a, b := l.regions[i], l.regions[j]
			if a.open && b.open {
				continue
			}
			w, ok := sharedEdge(a.rect, b.rect)
			if !ok {
				continue
			}
			w.a, w.b = i, j
			w.id = fmt.Sprintf("wall-%s-%s", a.id, b.id)
			w.label = a.name + " – " + b.name
			pairs[[2]int{i, j}] = len(s.walls)
			s.walls = append(s.walls, w)
		}
	}
	for _, d := range p.Doors {
		i, j := l.index[d[0]], l.index[d[1]]
		key := [2]int{min(i, j), max(i, j)}
		k, ok := pairs[key]
		if !ok {
			return fmt.Errorf("keepgen: door %s–%s: the two share no wall", d[0], d[1])
		}
		w := &s.walls[k]
		if w.length < minDoorWall {
			return fmt.Errorf("keepgen: door %s–%s: their wall is %.2f long, shorter than %.0f", d[0], d[1], w.length, minDoorWall)
		}
		id := fmt.Sprintf("door-%s-%s", d[0], d[1])
		pos := w.length / 2
		w.door = &door{
			id: id, openingID: id + "-gap", position: pos,
			cx: w.sx + (w.ex-w.sx)*pos/w.length,
			cz: w.sz + (w.ez-w.sz)*pos/w.length,
		}
	}
	return nil
}

// sharedEdge is the wall two rectangles share, if they share an edge of at
// least one cell: a vertical seam where one's east edge is the other's west
// edge over common rows, or a horizontal seam where one's south edge is the
// other's north edge over common columns. The line runs west to east or
// north to south.
func sharedEdge(a, b rect) (wall, bool) {
	if a.c1 == b.c0 || b.c1 == a.c0 {
		c := a.c1
		if b.c1 == a.c0 {
			c = a.c0
		}
		lo, hi := max(a.r0, b.r0), min(a.r1, b.r1)
		if lo < hi {
			x := seamX(c)
			return wall{
				sx: x, sz: seamZ(lo), ex: x, ez: seamZ(hi), length: seamZ(hi) - seamZ(lo),
			}, true
		}
	}
	if a.r1 == b.r0 || b.r1 == a.r0 {
		r := a.r1
		if b.r1 == a.r0 {
			r = a.r0
		}
		lo, hi := max(a.c0, b.c0), min(a.c1, b.c1)
		if lo < hi {
			z := seamZ(r)
			return wall{
				sx: seamX(lo), sz: z, ex: seamX(hi), ez: z, length: seamX(hi) - seamX(lo),
			}, true
		}
	}
	return wall{}, false
}

// reserve keeps clear every door threshold, the cells around the party
// start, and the cells around each exit.
func (s *site) reserve(l *layout, owner map[cell]int) {
	for _, w := range s.walls {
		if w.door == nil {
			continue
		}
		for _, ri := range []int{w.a, w.b} {
			for _, c := range l.regions[ri].cells() {
				x, z := c.centre()
				if math.Hypot(x-w.door.cx, z-w.door.cz) <= thresholdRadius {
					s.reserved[c] = true
				}
			}
		}
	}
	around := func(at cell, radius int) {
		for _, c := range l.regions[owner[at]].cells() {
			if hexDistance(c, at) <= radius {
				s.reserved[c] = true
			}
		}
	}
	around(l.start, startRadius)
	for _, ex := range l.exits {
		around(ex.at, exitRadius)
	}
}

// stream is a region's own seeded source, so one region's draws never shift
// another's.
func stream(seed int64, regionID, what string) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(regionID + "/" + what))
	return rand.New(rand.NewPCG(uint64(seed), h.Sum64()))
}

// placeProps places up to n props in region ri from its kind's palette.
//
// TWO IN THREE BLOCK (design Defaults): prop k is a blocker unless k%3 == 2,
// and draws from the palette's blockers or its dressing accordingly (from
// either, if the palette has only one). Each prop is anchored at a cell
// centre with rotation 0 or a quarter turn. It is placed only where every
// cell its footprint covers is INTERIOR to its region (all six neighbours in
// it), clear of the reservations, and not touching another prop's cells —
// so its footprint stays inside its room (plan G2), no prop seals a pocket,
// and every free cell stays connected. A prop that fits nowhere is skipped.
func (s *site) placeProps(p *Plan, l *layout, owner map[cell]int, occupied map[cell]bool, ri, n int, seed int64) {
	g := l.regions[ri]
	pal := p.Palettes[g.kind]
	if n == 0 || len(pal.Props) == 0 {
		return
	}
	var blockers, dressing []PropEntry
	for _, e := range pal.Props {
		if *e.BlocksMovement {
			blockers = append(blockers, e)
		} else {
			dressing = append(dressing, e)
		}
	}
	interior := func(c cell) bool {
		if o, ok := owner[c]; !ok || o != ri {
			return false
		}
		for _, nb := range c.neighbours() {
			if o, ok := owner[nb]; !ok || o != ri {
				return false
			}
		}
		return true
	}
	var candidates []cell
	for _, c := range g.cells() {
		if interior(c) && !s.reserved[c] {
			candidates = append(candidates, c)
		}
	}
	rng := stream(seed, g.id, "props")
	rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	used := make([]bool, len(candidates))
	minX, maxX := seamX(g.rect.c0), seamX(g.rect.c1)
	minZ, maxZ := seamZ(g.rect.r0), seamZ(g.rect.r1)
	placed := 0
	for k := 0; k < n; k++ {
		pool := blockers
		if k%3 == 2 {
			pool = dressing
		}
		if len(pool) == 0 {
			pool = pal.Props
		}
		entry := pool[rng.IntN(len(pool))]
		rotation := 0.0
		if rng.IntN(2) == 1 {
			rotation = math.Pi / 2
		}
		hx, hz := entry.Width/2, entry.Depth/2
		if rotation != 0 {
			hx, hz = hz, hx
		}
	scan:
		for ci, at := range candidates {
			if used[ci] {
				continue
			}
			cx, cz := at.centre()
			if cx-hx < minX || cx+hx > maxX || cz-hz < minZ || cz+hz > maxZ {
				continue
			}
			covered := footprintCells(at, hx, hz)
			for _, c := range covered {
				if !interior(c) || s.reserved[c] || occupied[c] {
					continue scan
				}
				for _, nb := range c.neighbours() {
					if occupied[nb] {
						continue scan
					}
				}
			}
			used[ci] = true
			for _, c := range covered {
				occupied[c] = true
			}
			s.props = append(s.props, placedProp{
				id: fmt.Sprintf("%s-prop-%02d", g.id, placed), region: ri, entry: entry, at: at,
				rotation: rotation, covered: covered, blocking: *entry.BlocksMovement,
			})
			placed++
			break
		}
	}
}

// footprintCells are the cells whose centre lies inside (or on) the
// axis-aligned rectangle of half extents hx, hz centred on at's centre —
// the cells a centre-covered footprint closes — rows north to south, west
// to east. It always includes at.
func footprintCells(at cell, hx, hz float64) []cell {
	const touch = 1e-6
	cx, cz := at.centre()
	dr := int(math.Ceil(hz/1.5)) + 1
	dc := int(math.Ceil(hx/sqrt3)) + 1
	var out []cell
	for row := at.row - dr; row <= at.row+dr; row++ {
		for col := at.col - dc; col <= at.col+dc; col++ {
			c := cell{col, row}
			x, z := c.centre()
			if math.Abs(x-cx) <= hx+touch && math.Abs(z-cz) <= hz+touch {
				out = append(out, c)
			}
		}
	}
	return out
}

// placeMonsters declares up to n monsters in region ri from its kind's
// palette, each on its own free cell: not reserved, not under a prop.
func (s *site) placeMonsters(p *Plan, l *layout, owner map[cell]int, occupied, taken map[cell]bool, ri, n int, seed int64) {
	g := l.regions[ri]
	refs := p.Palettes[g.kind].Monsters
	if n == 0 || len(refs) == 0 {
		return
	}
	var free []cell
	for _, c := range g.cells() {
		if owner[c] == ri && !s.reserved[c] && !occupied[c] && !taken[c] {
			free = append(free, c)
		}
	}
	rng := stream(seed, g.id, "monsters")
	rng.Shuffle(len(free), func(i, j int) { free[i], free[j] = free[j], free[i] })
	if n > len(free) {
		n = len(free)
	}
	chosen := free[:n]
	// Declared in reading order, so the file reads north to south.
	sort.Slice(chosen, func(i, j int) bool {
		if chosen[i].row != chosen[j].row {
			return chosen[i].row < chosen[j].row
		}
		return chosen[i].col < chosen[j].col
	})
	for k, at := range chosen {
		taken[at] = true
		s.monsters = append(s.monsters, placedMonster{
			id: fmt.Sprintf("%s-monster-%02d", g.id, k), ref: refs[rng.IntN(len(refs))], region: ri, at: at,
		})
	}
}
