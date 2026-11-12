// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// place says the building: its name, the summarised geometry, the way in
// and out, the endings, the doors, the props, the records, the stat blocks
// and the bundles.
func (b *builder) place() []Group {
	about := Subject{Kind: "site", ID: b.c.Key}
	g := Group{Title: fmt.Sprintf("%s (%s)", b.spec.Room.Name, b.c.Key), About: about}
	add := func(s Statement) { g.Statements = append(g.Statements, s) }

	add(b.stmt(KindPlace, about, "key", fmt.Sprintf(
		"The site %s is “%s”, a version %d site document; its void is %s, its lighting %s and its standing rule %s.",
		quote(b.c.Key), b.spec.Room.Name, b.spec.Version, b.spec.Play.Void, b.spec.Play.Lighting, b.spec.Play.Standing)))

	gp := b.spec.Room.Gameplay
	var regionNames []string
	for _, r := range gp.Regions {
		if len(r.Cells) == 0 {
			continue
		}
		regionNames = append(regionNames, fmt.Sprintf("%s (%s)", r.Name, r.ID))
	}
	add(b.stmt(KindRegions, about, "room.room.regions", fmt.Sprintf(
		"The floor is %s with cells: %s. Summarised: their cells, lighting and ground are not said one by one.",
		count(len(regionNames), "region", "regions"), joinAnd(regionNames))))

	doors := 0
	for _, w := range gp.Walls {
		for _, o := range w.Openings {
			if o.Door != nil {
				doors++
			}
		}
	}
	items := 0
	if n := b.doc.node("room.scene.items"); n != nil {
		items = len(n.Content)
	}
	add(b.stmt(KindGeometry, about, "room.room.walls", fmt.Sprintf(
		"Summarised: the coordinate frame, %s with %s in their openings, %s, %s and %s. "+
			"Their positions and shapes are not said one by one.",
		count(len(gp.Walls), "wall line", "wall lines"), count(doors, "door", "doors"),
		count(items, "scene item", "scene items"), count(len(gp.PropDeclarations), "prop footprint", "prop footprints"),
		count(len(gp.ArrangementDeclarations), "arrangement", "arrangements"))))

	if gp.PartyStart != nil {
		at := fmt.Sprintf("The party starts at %s, in %s", cellWords(gp.PartyStart.Q, gp.PartyStart.R),
			b.regionAt(gp.PartyStart.Q, gp.PartyStart.R))
		if b.c.StartFacing != "" {
			at += ", facing " + b.c.StartFacing
		}
		add(b.stmt(KindStart, about, "room.room.partyStart", at+"."))
	}
	for i, e := range b.spec.Exits {
		add(b.stmt(KindExit, Subject{Kind: "exit", ID: e.ID}, fmt.Sprintf("exits[%d]", i), fmt.Sprintf(
			"The exit %s is the cell %s, in %s.", quote(e.ID), cellWords(e.Cell.Q, e.Cell.R), b.regionAt(e.Cell.Q, e.Cell.R))))
	}
	for i, e := range b.c.Endings {
		when, conns := b.triggerWords(e.Trigger, lineContext{})
		s := fmt.Sprintf("The run ends %s, a %s, when %s", quote(e.Key), e.Result, when)
		if e.If != nil {
			cond, more := b.triggerWords(e.If, lineContext{})
			s += ", if " + cond
			conns = append(conns, more...)
		}
		if len(b.c.Endings) > 1 {
			s += fmt.Sprintf(". It is ending %d of %d; the first in the list whose condition holds is the one the run ends on.", i+1, len(b.c.Endings))
		} else {
			s += "."
		}
		f := foldInfo{site: "the ending " + quote(e.Key), state: when, text: fmt.Sprintf("%s (a %s)", quote(e.Key), e.Result)}
		if e.If != nil {
			cond, _ := b.triggerWords(e.If, lineContext{})
			f.text += " if " + cond
		} else {
			f.text += " otherwise"
		}
		add(b.fstmt(KindEnding, Subject{Kind: "ending", ID: e.Key}, fmt.Sprintf("endings[%d]", i), s, f, conns...))
	}
	for _, id := range sortedKeys(gp.DoorBindings) {
		add(b.doorStatement(id))
	}
	for _, id := range sortedKeys(b.doorHolders) {
		holders := b.doorHolders[id]
		if len(holders) < 2 {
			continue
		}
		path := "room.room.doorBindings." + id
		if _, bound := gp.DoorBindings[id]; !bound {
			path = "room.room.monsterBindings." + holders[0] + ".posts"
		}
		var conns []Connection
		for _, h := range holders {
			conns = append(conns, Connection{Kind: "creature", ID: h, Role: "holds"})
		}
		s := fmt.Sprintf("The door %s is held by the posts of %s. Each holds it only while posted: in a state its post "+
			"names, up, and standing on one of that post's cells. The door stays barred to a walker while any one posted "+
			"holder not allied with the walker does not let that walker through; one holder letting them through is not "+
			"enough while another posted holder bars them.", quote(id), b.creatures(holders))
		add(b.fstmt(KindDoorHold, Subject{Kind: "door", ID: id}, path, s, foldInfo{text: s}, conns...))
	}
	for _, id := range sortedKeys(gp.PropBindings) {
		g.Statements = append(g.Statements, b.propStatements(id)...)
	}
	for i, in := range b.c.Intel {
		id := b.unqualify(string(in.ID))
		s := fmt.Sprintf("The record %s: whoever comes to hold it learns ", quote(id))
		var conns []Connection
		switch {
		case in.Reveals.Fact != "":
			s += "the fact " + quote(string(in.Reveals.Fact)) + "."
			conns = append(conns, Connection{Kind: "fact", ID: string(in.Reveals.Fact), Role: "teaches"})
		case in.Reveals.Concealment != "":
			s += "the secret " + quote(b.unqualify(string(in.Reveals.Concealment))) + "."
		default:
			s += "nothing this build can say."
		}
		add(b.fstmt(KindIntel, Subject{Kind: "record", ID: id}, fmt.Sprintf("intel[%d]", i), s,
			foldInfo{site: "holding the record " + quote(id)}, conns...))
	}
	for _, id := range sortedKeys(b.spec.Templates) {
		t := b.spec.Templates[id]
		parts := []string{fmt.Sprintf("The stat block %s (%s) is built on %s", quote(id), t.Name, t.Base)}
		if t.HitDice != "" {
			parts = append(parts, "hit dice "+t.HitDice)
		}
		if t.Armor != "" {
			parts = append(parts, "armour "+t.Armor)
		}
		if len(t.Abilities) > 0 {
			var ab []string
			for _, k := range sortedKeys(t.Abilities) {
				ab = append(ab, fmt.Sprintf("%s %d", k, t.Abilities[k]))
			}
			parts = append(parts, "abilities "+strings.Join(ab, ", "))
		}
		if t.Proficiency > 0 {
			parts = append(parts, fmt.Sprintf("proficiency %d", t.Proficiency))
		}
		if len(t.Skills) > 0 {
			parts = append(parts, "skills "+strings.Join(t.Skills, ", "))
		}
		if len(t.Actions) > 0 {
			parts = append(parts, "arms "+strings.Join(t.Actions, ", "))
		}
		add(b.stmt(KindTemplate, Subject{Kind: "template", ID: id}, "templates."+id, strings.Join(parts, "; ")+"."))
	}
	for _, id := range sortedKeys(b.spec.Bundles) {
		add(b.stmt(KindBundle, Subject{Kind: "bundle", ID: id}, "bundles."+id, fmt.Sprintf(
			"The bundle %s is a group of rows used as one row of a routine's table; each use is said in full where it is used.",
			quote(id))))
	}
	return []Group{g}
}

// doorStatement says one door: how it starts, its lock and what opening it
// teaches.
func (b *builder) doorStatement(id string) Statement {
	about := Subject{Kind: "door", ID: id}
	var door *encounter.DoorInput
	for i := range b.c.Field.Doors {
		if b.unqualify(string(b.c.Field.Doors[i].ID)) == id {
			door = &b.c.Field.Doors[i]
		}
	}
	path := "room.room.doorBindings." + id
	if door == nil || door.State == nil {
		return b.stmt(KindDoor, about, path, fmt.Sprintf("The door %s is bound and starts open.", quote(id)))
	}
	s := fmt.Sprintf("The door %s starts %s", quote(id), door.State.Kind())
	if lock, ok := door.State.Lock(); ok {
		s += "; opening it takes " + checkWords(lock.Approaches)
	}
	var conns []Connection
	if door.Opened != "" {
		s += fmt.Sprintf("; when it opens, whoever opened it and every creature who can see the door learn the fact %s",
			quote(string(door.Opened)))
		conns = append(conns, Connection{Kind: "fact", ID: string(door.Opened), Role: "teaches"})
	}
	return b.fstmt(KindDoor, about, path, s+".", foldInfo{site: "opening the door " + quote(id)}, conns...)
}

// propName is a scene item's label with its id.
func (b *builder) propName(id string) string {
	if n := b.doc.node("room.scene.items"); n != nil {
		for _, item := range n.Content {
			var itemID, label string
			for k := 0; k+1 < len(item.Content); k += 2 {
				switch item.Content[k].Value {
				case "id":
					itemID = item.Content[k+1].Value
				case "label":
					label = item.Content[k+1].Value
				}
			}
			if itemID == id && label != "" {
				return fmt.Sprintf("%s (%s)", label, id)
			}
		}
	}
	return quote(id)
}

// propStatements says one bound prop: whether it can be carried, what it
// holds, when it arrives and its lines.
func (b *builder) propStatements(id string) []Statement {
	about := Subject{Kind: "prop", ID: id}
	path := "room.room.propBindings." + id
	bind := b.spec.Room.Gameplay.PropBindings[id]
	// A prop is compiled either as a cell prop or as a placed footprint; the
	// read-back says what either carries.
	type propView struct {
		Holds   []encounter.IntelID
		Arrives encounter.Trigger
		Options []encounter.Option
	}
	var prop *propView
	for _, p := range b.c.Field.Props {
		if b.unqualify(string(p.ID)) == id {
			prop = &propView{Holds: p.Holds, Arrives: p.Arrives, Options: p.Options}
		}
	}
	for _, p := range b.c.Field.Placed {
		if b.unqualify(string(p.ID)) == id {
			prop = &propView{Holds: p.Holds, Arrives: p.Arrives, Options: p.Options}
		}
	}
	name := b.propName(id)
	s := "The prop " + name
	if bind.Holdable {
		s += " can be picked up and carried"
	} else {
		s += " cannot be carried"
	}
	var conns []Connection
	if prop != nil && len(prop.Holds) > 0 {
		var recs []string
		for _, h := range prop.Holds {
			recs = append(recs, quote(b.unqualify(string(h))))
			conns = append(conns, Connection{Kind: "record", ID: b.unqualify(string(h)), Role: "holds"})
		}
		s += "; it holds the records " + joinAnd(recs) + ", so whoever takes it comes to hold them"
	}
	if prop != nil && prop.Arrives != nil {
		w, more := b.triggerWords(prop.Arrives, lineContext{})
		s += "; it is not there at first and arrives when " + w
		conns = append(conns, more...)
	}
	out := []Statement{b.fstmt(KindProp, about, path, s+".", foldInfo{site: "the prop " + quote(id) + " arriving"}, conns...)}
	if prop != nil && len(prop.Options) > 0 {
		out = append(out, b.lineStatements(prop.Options, path+".options", lineTarget{name: "the prop " + name, short: "the prop", about: about, prop: true})...)
	}
	return out
}

// sides says the factions and how they stand to each other.
func (b *builder) sides() []Group {
	var groups []Group
	for i, f := range b.spec.Factions {
		about := Subject{Kind: "faction", ID: f.ID}
		g := Group{Title: "Faction " + quote(f.ID), About: about}
		var members []string
		for _, d := range b.spec.Room.Gameplay.MonsterDeclarations {
			if b.spec.Room.Gameplay.MonsterBindings[d.ID].Faction == f.ID {
				members = append(members, d.ID)
			}
		}
		s := fmt.Sprintf("The faction %s", quote(f.ID))
		if len(members) > 0 {
			s += " is " + b.creatures(members)
		} else {
			s += " has no creature placed in it"
		}
		var mind string
		for _, cf := range b.c.Factions {
			if string(cf.ID) == f.ID {
				mind = string(cf.Mind)
			}
		}
		if mind != "" {
			s += "; its mind (whose knowledge decides how the side stands) is " + b.creature(mind)
		}
		switch {
		case f.Temper.Word != "":
			s += "; its temper is " + f.Temper.Word
		case len(f.Temper.Mix) > 0:
			var mix []string
			for _, w := range sortedKeys(f.Temper.Mix) {
				mix = append(mix, fmt.Sprintf("%s %d", w, f.Temper.Mix[w]))
			}
			s += "; each member's temper is dealt from the mix " + strings.Join(mix, ", ")
		}
		g.Statements = append(g.Statements, b.stmt(KindFaction, about, fmt.Sprintf("factions[%d]", i), s+"."))
		if len(f.Options) > 0 {
			opts := b.factionLines(f.ID)
			if opts == nil {
				b.unsaid = append(b.unsaid, Unsaid{Path: fmt.Sprintf("factions[%d].options", i), Kind: "faction lines",
					Why: "no creature of the faction speaks them, so they are not in the compiled encounter to be said"})
			}
			g.Statements = append(g.Statements, b.lineStatements(opts, fmt.Sprintf("factions[%d].options", i),
				lineTarget{name: fmt.Sprintf("any creature of the faction %s that has no lines of its own", quote(f.ID)),
					short: "the creature spoken to", about: about})...)
		}
		groups = append(groups, g)
	}
	if len(b.c.Dispositions) > 0 {
		g := Group{Title: "How the sides stand", About: Subject{Kind: "site", ID: b.c.Key}}
		for i, d := range b.c.Dispositions {
			about := Subject{Kind: "disposition", ID: fmt.Sprintf("%s-%s", d.Between[0], d.Between[1])}
			s := fmt.Sprintf("The factions %s and %s start %s", quote(string(d.Between[0])), quote(string(d.Between[1])), d.Stance)
			var conns []Connection
			var minds []string
			for _, side := range d.Between {
				for _, cf := range b.c.Factions {
					if cf.ID == side && cf.Mind != "" {
						minds = append(minds, b.creature(string(cf.Mind)))
					}
				}
			}
			ctx := lineContext{}
			if len(minds) > 0 {
				ctx.me = "the pair's mind, " + joinOr(minds) + ","
			}
			for j, step := range d.Until {
				w, more := b.triggerWords(step.When, ctx)
				then := step.Then
				if then == "" {
					then = encounter.StanceHostile
					if d.Stance == encounter.StanceHostile {
						then = encounter.StanceNeutral
					}
				}
				conns = append(conns, more...)
				lead := "; they become"
				if j > 0 {
					lead = "; else they become"
				}
				s += fmt.Sprintf("%s %s once %s", lead, then, w)
			}
			if len(d.Until) > 1 {
				s += " (the first step that holds wins)"
			}
			g.Statements = append(g.Statements, b.fstmt(KindDisposition, about, fmt.Sprintf("dispositions[%d]", i), s+".",
				foldInfo{site: fmt.Sprintf("how %s and %s stand", quote(string(d.Between[0])), quote(string(d.Between[1])))}, conns...))
		}
		groups = append(groups, g)
	}
	return groups
}

// factionLines is a faction's lines as compiled: those of the first member
// of the faction that speaks them (a member with no lines of its own).
func (b *builder) factionLines(faction string) []encounter.Option {
	for _, d := range b.spec.Room.Gameplay.MonsterDeclarations {
		bind := b.spec.Room.Gameplay.MonsterBindings[d.ID]
		if bind.Faction == faction && len(bind.Options) == 0 {
			if m := b.monsters[d.ID]; m != nil && len(m.Options) > 0 {
				return m.Options
			}
		}
	}
	return nil
}
