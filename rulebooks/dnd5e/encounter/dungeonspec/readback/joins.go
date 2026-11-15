// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// joins.go says where the places meet (rpg-project#971, design R30, R31). A
// place is a region; a join is where two regions meet:
//
//   - a DOOR: the door's crossings, each side looked up to its region;
//   - OPEN FLOOR: a crossing between two regions' cells that FEET can make
//     on the fixed geometry, with no door in it. Feet decide a join (design
//     R30: a join is a way from one place to the next); the sentence says
//     whether sight crosses too. Two regions whose cells touch across a wall
//     are NOT joined, and a seam sight crosses and feet do not (a window) is
//     said as a sight seam, never a join;
//   - a PASSAGE that is not physical: a line's `relocate`, and an arrest
//     with the jail that sets the taken down.
//
// The geometry is the engine's ([encounter.FloorTopology]): the same traced
// crossings and the same partition canvas play reads, never re-derived here.
// A join's state (free, locked, held, hidden) is not restated: it is the
// door's, the door's holders' and the secret's, each already said and each
// reached through the join's connections. A door that joins no two regions
// — both sides one region's, or a side off the floor — is said as such, so
// nothing is skipped silently.

// joinGroupTitle is the title of the group the joins are said in, under the
// place.
const joinGroupTitle = "Where the places join"

// joins says every join and every region in none, in one group: doors by
// door id, open floor by region pair, passages in the order the read-back
// says their lines and conversations, then the regions joined to nothing.
// lines is every statement said so far, read for the rows that relocate.
func (b *builder) joins(lines []Statement) Group {
	about := Subject{Kind: "site", ID: b.c.Key}
	g := Group{Title: joinGroupTitle, About: about}
	topo, err := encounter.FloorTopology(encounter.FloorTopologyInput{Field: b.c.Field})
	if err != nil {
		b.unsaid = append(b.unsaid, Unsaid{Path: "room.room.regions", Kind: "joins",
			Why: "the engine could not measure where the regions meet: " + err.Error()})
		return g
	}
	regionOf := map[spatial.Position]string{}
	for _, r := range b.c.Field.Regions {
		for _, c := range r.Cells {
			regionOf[c] = b.unqualify(string(r.ID))
		}
	}
	hiddenBy := map[string]string{} // "door:<id>" or "prop:<id>" → concealment
	for _, c := range b.c.Concealments {
		for _, d := range c.Doors {
			hiddenBy["door:"+b.unqualify(string(d))] = b.unqualify(string(c.ID))
		}
		for _, p := range c.Props {
			hiddenBy["prop:"+b.unqualify(string(p))] = b.unqualify(string(c.ID))
		}
	}
	hidden := func(kind, id string) []Connection {
		if c, ok := hiddenBy[kind+":"+id]; ok {
			return []Connection{{Kind: "concealment", ID: c, Role: "hidden-by"}}
		}
		return nil
	}

	// offFloor is, by region, the doors that lead from it off the floor:
	// said with the region if it joins nothing, so "no door" is never said
	// of a region a reader can see a door in.
	offFloor := map[string][]string{}
	for _, d := range topo.Doors {
		id := b.unqualify(string(d.Door))
		pairs := regionPairs(d.Crossings, regionOf)
		for _, pair := range pairs {
			conns := append([]Connection{
				{Kind: "region", ID: pair[0], Role: "joins"},
				{Kind: "region", ID: pair[1], Role: "joins"},
				{Kind: "door", ID: id, Role: "by"},
			}, hidden("door", id)...)
			s := fmt.Sprintf("%s and %s join at the door %s. Whether a member may cross there is the door's to say: "+
				"how the door starts, its lock and its holders are said with the door %s.",
				b.regionWords(pair[0]), b.regionWords(pair[1]), quote(id), quote(id))
			g.Statements = append(g.Statements, b.stmt(KindJoinDoor,
				Subject{Kind: "join", ID: fmt.Sprintf("door:%s:%s:%s", pair[0], pair[1], id)}, b.doorPath(id), s, conns...))
		}
		if len(pairs) == 0 {
			g.Statements = append(g.Statements, b.doorJoiningNothing(id, d, regionOf, offFloor))
		}
	}

	// Feet decide a join (design R30: a join is where a member can go from
	// one place to the next). A seam sight crosses and feet do not is said
	// as a sight seam, never a join.
	type seamWays struct{ movesSees, movesBlind, seesOnly bool }
	ways := map[[2]string]*seamWays{}
	var order [][2]string
	for _, seam := range topo.Seams {
		pairs := regionPairs([]encounter.FloorCrossing{seam.FloorCrossing}, regionOf)
		if len(pairs) == 0 {
			continue
		}
		pair := pairs[0]
		w := ways[pair]
		if w == nil {
			w = &seamWays{}
			ways[pair] = w
			order = append(order, pair)
		}
		switch {
		case seam.Moves && seam.Sees:
			w.movesSees = true
		case seam.Moves:
			w.movesBlind = true
		default:
			w.seesOnly = true
		}
	}
	sortPairs(order)
	for _, pair := range order {
		w := ways[pair]
		a, z := b.regionWords(pair[0]), b.regionWords(pair[1])
		if !w.movesSees && !w.movesBlind {
			s := fmt.Sprintf("%s and %s see each other where their cells touch, but no member can step from one to the "+
				"other there: what stands between them stops feet and lets sight through. It is not a way between them.", a, z)
			g.Statements = append(g.Statements, b.stmt(KindSeamSight,
				Subject{Kind: "seam", ID: fmt.Sprintf("sight:%s:%s", pair[0], pair[1])}, "room.room.regions", s,
				Connection{Kind: "region", ID: pair[0], Role: "sees"},
				Connection{Kind: "region", ID: pair[1], Role: "sees"}))
			continue
		}
		var s string
		switch {
		case w.movesSees && !w.movesBlind:
			s = fmt.Sprintf("%s and %s join on open floor: where their cells touch, no wall, door or opaque prop stands "+
				"between them, so the two are one room as the walls make it, for sight, hearing and feet alike.", a, z)
		case !w.movesSees:
			s = fmt.Sprintf("%s and %s join on open floor: where their cells touch, a member can step from one to the "+
				"other with no door between them, but sight does not cross there: what stands between them blocks it.", a, z)
		default:
			s = fmt.Sprintf("%s and %s join on open floor: where their cells touch, a member can step from one to the "+
				"other with no door between them; sight crosses at some of those places and is blocked at others.", a, z)
		}
		g.Statements = append(g.Statements, b.stmt(KindJoinOpen,
			Subject{Kind: "join", ID: fmt.Sprintf("open:%s:%s", pair[0], pair[1])}, "room.room.regions", s,
			Connection{Kind: "region", ID: pair[0], Role: "joins"},
			Connection{Kind: "region", ID: pair[1], Role: "joins"}))
	}

	g.Statements = append(g.Statements, b.relocatePassages(lines, topo, regionOf, hidden)...)
	g.Statements = append(g.Statements, b.arrestPassages()...)

	joined := map[string]bool{}
	for _, st := range g.Statements {
		if !joinKind(st.Kind) {
			continue
		}
		for _, c := range st.Connects {
			if c.Kind == "region" {
				joined[c.ID] = true
			}
		}
	}
	for i, r := range b.spec.Room.Gameplay.Regions {
		if len(r.Cells) == 0 || joined[r.ID] {
			continue
		}
		s := fmt.Sprintf("%s joins no other region: no door, open floor or passage leads from %s to another region.",
			b.regionWords(r.ID), b.regionName(r.ID))
		var conns []Connection
		for _, d := range offFloor[r.ID] {
			s += fmt.Sprintf(" The door %s at %s leads off the floor.", quote(d), b.regionName(r.ID))
			conns = append(conns, Connection{Kind: "door", ID: d, Role: "edge"})
		}
		g.Statements = append(g.Statements, b.stmt(KindRegionUnjoined, Subject{Kind: "region", ID: r.ID},
			fmt.Sprintf("room.room.regions[%d]", i), s, conns...))
	}
	return g
}

// joinKind is whether a statement says a join: a door, open floor or a
// passage. A sight seam and a door that joins nothing are not.
func joinKind(k Kind) bool {
	return k == KindJoinDoor || k == KindJoinOpen || k == KindJoinPassage
}

// doorJoiningNothing says a door that stands between no two regions: both
// its sides one region's floor (within), or a side off the floor (edge).
func (b *builder) doorJoiningNothing(id string, d encounter.DoorCrossings, regionOf map[spatial.Position]string,
	offFloor map[string][]string) Statement {
	about := Subject{Kind: "door", ID: id}
	var touched []string
	within := len(d.Crossings) > 0
	for _, c := range d.Crossings {
		a, okA := regionOf[c.From]
		z, okZ := regionOf[c.To]
		if !okA || !okZ {
			within = false
		}
		for _, r := range []string{a, z} {
			if r != "" && !contains(touched, r) {
				touched = append(touched, r)
			}
		}
	}
	for _, cell := range d.Cells {
		if r, ok := regionOf[cell]; ok && !contains(touched, r) {
			touched = append(touched, r)
		}
	}
	sort.Strings(touched)
	var words []string
	for _, r := range touched {
		words = append(words, b.regionWords(r))
	}
	if within && len(touched) == 1 {
		s := fmt.Sprintf("The door %s stands within %s: both its sides are %s's floor, so the door joins %s to no "+
			"other region.", quote(id), words[0], b.regionName(touched[0]), b.regionName(touched[0]))
		return b.stmt(KindDoorWithin, about, b.doorPath(id), s, Connection{Kind: "region", ID: touched[0], Role: "within"})
	}
	var conns []Connection
	for _, r := range touched {
		conns = append(conns, Connection{Kind: "region", ID: r, Role: "edge"})
		offFloor[r] = append(offFloor[r], id)
	}
	s := fmt.Sprintf("The door %s leads off the floor: no region's floor lies on its far side, so the door joins no "+
		"region to another.", quote(id))
	if len(words) > 0 {
		s = fmt.Sprintf("The door %s stands at the edge of %s and leads off the floor: no region's floor lies on its "+
			"far side, so the door joins %s to no other region.", quote(id), joinAnd(words), joinAnd(words))
	}
	return b.stmt(KindDoorEdge, about, b.doorPath(id), s, conns...)
}

// sortPairs orders region pairs by their first id, then their second.
func sortPairs(out [][2]string) {
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
}

// regionPairs is every distinct pair of different regions the crossings
// stand between, each pair in id order, the pairs in order. A crossing with
// a side in no region, or with one region on both sides, joins nothing.
func regionPairs(crossings []encounter.FloorCrossing, regionOf map[spatial.Position]string) [][2]string {
	seen := map[[2]string]bool{}
	var out [][2]string
	for _, c := range crossings {
		a, okA := regionOf[c.From]
		z, okZ := regionOf[c.To]
		if !okA || !okZ || a == z {
			continue
		}
		if z < a {
			a, z = z, a
		}
		pair := [2]string{a, z}
		if !seen[pair] {
			seen[pair] = true
			out = append(out, pair)
		}
	}
	sortPairs(out)
	return out
}

// doorPath is where a door is set: its binding, else the wall opening that
// carries it.
func (b *builder) doorPath(id string) string {
	gp := b.spec.Room.Gameplay
	if _, ok := gp.DoorBindings[id]; ok {
		return "room.room.doorBindings." + id
	}
	for i, w := range gp.Walls {
		for j, o := range w.Openings {
			if o.Door != nil && o.Door.ID == id {
				return fmt.Sprintf("room.room.walls[%d].openings[%d].door", i, j)
			}
		}
	}
	return "room.room.walls"
}

// relocatePassages says one passage per carrier and destination, from every
// row already said that relocates: the line's carrier (a prop, a creature or
// a faction's creatures), where a prop stands, and where the row sends.
func (b *builder) relocatePassages(lines []Statement, topo encounter.FloorTopologyOutput,
	regionOf map[spatial.Position]string, hidden func(kind, id string) []Connection) []Statement {
	placedAt := map[string][]spatial.Position{}
	for _, p := range topo.Placed {
		placedAt[b.unqualify(string(p.Prop))] = p.Cells
	}
	for _, p := range b.c.Field.Props {
		placedAt[b.unqualify(string(p.ID))] = []spatial.Position{p.At}
	}
	var out []Statement
	said := map[string]bool{}
	for _, st := range lines {
		if st.Kind != KindLineGrant {
			continue
		}
		to := ""
		for _, c := range st.Connects {
			if c.Kind == "region" && c.Role == "sends-to" {
				to = c.ID
			}
		}
		if to == "" {
			continue
		}
		carrier := st.About
		var from []string
		var who string
		switch carrier.Kind {
		case "prop":
			for _, cell := range placedAt[carrier.ID] {
				if r, ok := regionOf[cell]; ok && r != to && !contains(from, r) {
					from = append(from, r)
				}
			}
			who = "The prop " + b.propName(carrier.ID)
			if len(from) > 0 {
				var where []string
				for _, r := range from {
					where = append(where, b.regionWords(r))
				}
				who += ", standing in " + joinAnd(where) + ","
			}
		case "creature":
			who = "The creature " + b.creature(carrier.ID)
		default:
			who = fmt.Sprintf("Any creature of the faction %s that has no lines of its own", quote(carrier.ID))
		}
		id := fmt.Sprintf("passage:%s:%s:%s-%s", strings.Join(from, "+"), to, carrier.Kind, carrier.ID)
		if said[id] {
			continue
		}
		said[id] = true
		line := b.optionID(st.Path)
		s := fmt.Sprintf("%s is a passage to %s: the line %s sets the speaker down in %s, without walking the way "+
			"between.", who, b.regionWords(to), quote(line), b.regionName(to))
		conns := []Connection{{Kind: "region", ID: to, Role: "to"}}
		for _, r := range from {
			conns = append(conns, Connection{Kind: "region", ID: r, Role: "from"})
		}
		conns = append(conns, Connection{Kind: carrier.Kind, ID: carrier.ID, Role: "by"})
		conns = append(conns, hidden(carrier.Kind, carrier.ID)...)
		out = append(out, b.stmt(KindJoinPassage, Subject{Kind: "join", ID: id}, st.Path, s, conns...))
	}
	return out
}

// optionID is the id of the line a row's path is under
// (`….options[2].success[0].relocate` → the id written at `….options[2]`).
func (b *builder) optionID(path string) string {
	i := strings.LastIndex(path, ".options[")
	if i < 0 {
		return path
	}
	j := strings.IndexByte(path[i:], ']')
	if j < 0 {
		return path
	}
	if n := b.doc.node(path[:i+j+1]); n != nil {
		for k := 0; k+1 < len(n.Content); k += 2 {
			if n.Content[k].Value == "id" {
				return n.Content[k+1].Value
			}
		}
	}
	return path[:i+j+1]
}

// arrestPassages says one passage per arrest: from the regions of the
// arrest's area to its cell, carried by the conversation, with the states
// whose jail sets the taken down — or that none does.
func (b *builder) arrestPassages() []Statement {
	var out []Statement
	for _, id := range sortedKeys(b.spec.Interactions) {
		outcomes := b.spec.Interactions[id].Outcomes
		var jails []string
		for _, state := range sortedKeys(outcomes) {
			for _, c := range outcomes[state] {
				if c.Jail != nil {
					jails = append(jails, quote(state))
				}
			}
		}
		for _, state := range sortedKeys(outcomes) {
			for k, c := range outcomes[state] {
				if c.Arrest == nil {
					continue
				}
				path := fmt.Sprintf("interactions.%s.outcomes.%s[%d].arrest", id, state, k)
				var from, where []string
				for _, r := range b.arrestAreaRegions(c.Arrest, path) {
					if r != c.Arrest.Cell {
						from = append(from, r)
					}
				}
				conns := []Connection{{Kind: "region", ID: c.Arrest.Cell, Role: "to"}}
				for _, r := range from {
					where = append(where, b.regionWords(r))
					conns = append(conns, Connection{Kind: "region", ID: r, Role: "from"})
				}
				conns = append(conns, Connection{Kind: "conversation", ID: id, Role: "by"})
				s := fmt.Sprintf("The conversation %s is a passage", quote(id))
				if len(where) > 0 {
					s += fmt.Sprintf(" from %s, the arrest's area,", joinAnd(where))
				}
				s += fmt.Sprintf(" to %s: entering %s takes the speaker's party", b.regionWords(c.Arrest.Cell), quote(state))
				if len(jails) > 0 {
					s += fmt.Sprintf(", and entering %s sets the taken down in %s, without walking the way between.",
						joinOr(jails), b.regionWords(c.Arrest.Cell))
				} else {
					s += fmt.Sprintf(" and names %s as where they are held; no jail of the conversation sets them down, "+
						"so nobody passes yet.", b.regionWords(c.Arrest.Cell))
				}
				out = append(out, b.stmt(KindJoinPassage,
					Subject{Kind: "join", ID: fmt.Sprintf("passage:%s:%s:conversation-%s", strings.Join(from, "+"), c.Arrest.Cell, id)},
					path, s, conns...))
			}
		}
	}
	return out
}

// arrestAreaRegions is the regions an arrest's area names, then the regions
// holding its loose cells, each once, in that order.
func (b *builder) arrestAreaRegions(a *dungeonspec.ArrestSpec, path string) []string {
	var out []string
	for _, r := range a.Area.Regions {
		if !contains(out, r) {
			out = append(out, r)
		}
	}
	for _, cell := range dungeonspec.ExpandRows(a.Area.Cells, path+".area.cells", func(string, string) {}) {
		if r := b.specRegionAt(cell.Q, cell.R); r != "" && !contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// specRegionAt is the id of the authored region whose rows hold the axial
// cell, or empty.
func (b *builder) specRegionAt(q, r int) string {
	for _, reg := range b.spec.Room.Gameplay.Regions {
		for _, row := range reg.Cells {
			if row.R != r {
				continue
			}
			for _, span := range row.Q {
				if q >= span[0] && q <= span[1] {
					return reg.ID
				}
			}
		}
	}
	return ""
}
