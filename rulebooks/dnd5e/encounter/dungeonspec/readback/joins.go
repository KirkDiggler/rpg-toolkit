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
//   - OPEN FLOOR: a crossing between two regions' cells that nothing fixed
//     closes — no wall, no door, no opaque prop — so the two are one wall
//     partition of the floor. Two regions whose cells touch across a wall
//     are NOT joined;
//   - a PASSAGE that is not physical: a line's `relocate`, and an arrest
//     with the jail that sets the taken down.
//
// The geometry is the engine's ([encounter.FloorTopology]): the same traced
// crossings and the same partition canvas play reads, never re-derived here.
// A join's state (free, locked, held, hidden) is not restated: it is the
// door's, the door's holders' and the secret's, each already said and each
// reached through the join's connections.

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

	for _, d := range topo.Doors {
		id := b.unqualify(string(d.Door))
		for _, pair := range regionPairs(d.Crossings, regionOf) {
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
	}

	for _, pair := range regionPairs(topo.Open, regionOf) {
		s := fmt.Sprintf("%s and %s join on open floor: where their cells touch, no wall, door or opaque prop stands "+
			"between them, so the two are one room as the walls make it, for sight, hearing and feet alike.",
			b.regionWords(pair[0]), b.regionWords(pair[1]))
		g.Statements = append(g.Statements, b.stmt(KindJoinOpen,
			Subject{Kind: "join", ID: fmt.Sprintf("open:%s:%s", pair[0], pair[1])}, "room.room.regions", s,
			Connection{Kind: "region", ID: pair[0], Role: "joins"},
			Connection{Kind: "region", ID: pair[1], Role: "joins"}))
	}

	g.Statements = append(g.Statements, b.relocatePassages(lines, topo, regionOf, hidden)...)
	g.Statements = append(g.Statements, b.arrestPassages()...)

	joined := map[string]bool{}
	for _, st := range g.Statements {
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
		s := fmt.Sprintf("%s joins no other region: no door, open floor or passage reaches %s.", b.regionWords(r.ID), b.regionName(r.ID))
		g.Statements = append(g.Statements, b.stmt(KindRegionUnjoined, Subject{Kind: "region", ID: r.ID},
			fmt.Sprintf("room.room.regions[%d]", i), s))
	}
	return g
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
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
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
