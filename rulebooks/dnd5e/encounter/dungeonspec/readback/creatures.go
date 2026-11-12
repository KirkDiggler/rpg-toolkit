// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// creatureGroups says every placed creature, in the order the file places
// them: where it stands, what it holds, when it arrives, the routine it
// runs, the posts it holds and the lines it can be spoken to with.
func (b *builder) creatureGroups() []Group {
	var groups []Group
	for i, d := range b.spec.Room.Gameplay.MonsterDeclarations {
		m := b.monsters[d.ID]
		if m == nil {
			continue
		}
		name := b.creature(d.ID)
		about := Subject{Kind: "creature", ID: d.ID}
		bindPath := "room.room.monsterBindings." + d.ID
		bind := b.spec.Room.Gameplay.MonsterBindings[d.ID]
		g := Group{Title: name, About: about}
		add := func(s Statement) { g.Statements = append(g.Statements, s) }

		loc := d.StartingCell.Location
		s := fmt.Sprintf("%s is a %s", name, d.Ref)
		lead := len(s)
		var conns []Connection
		if m.Faction != "" {
			s += " of the faction " + quote(m.Faction)
			conns = append(conns, Connection{Kind: "faction", ID: m.Faction, Role: "member-of"})
		} else {
			s += " of no faction (the monsters)"
		}
		s += fmt.Sprintf(", standing at %s in %s", cellWords(loc.Q, loc.R), b.regionWords(m.Region))
		if d.StartingCell.Facing != "" {
			s += ", facing " + d.StartingCell.Facing
		}
		if m.AppearanceRef != "" {
			s += "; it looks like " + m.AppearanceRef
		}
		if m.DefaultActivityRef != "" {
			s += "; at rest it plays " + m.DefaultActivityRef
		}
		if bind.Temper != "" {
			s += "; its temper is " + bind.Temper
		}
		if m.Boss {
			s += "; its death ends the run"
		}
		short := "a " + d.Ref + s[lead:]
		short = strings.Replace(short, " of the faction ", " of ", 1)
		short = strings.Replace(short, ", standing at ", " at ", 1)
		short = strings.Replace(short, " in "+b.regionWords(m.Region), " in "+b.regionName(m.Region), 1)
		short = strings.Replace(short, "; it looks like ", "; looks like ", 1)
		add(b.fstmt(KindCreature, about, fmt.Sprintf("room.room.monsterDeclarations[%d]", i), s+".",
			foldInfo{text: short}, conns...))

		if len(m.Holds) > 0 {
			var recs []string
			var hc []Connection
			for _, h := range m.Holds {
				id := b.unqualify(h)
				recs = append(recs, quote(id))
				hc = append(hc, Connection{Kind: "record", ID: id, Role: "holds"})
			}
			add(b.fstmt(KindHolds, about, bindPath+".holds", fmt.Sprintf(
				"%s holds the records %s from the start.", name, joinAnd(recs)),
				foldInfo{text: "{Self} holds the records " + joinAnd(recs) + " from the start."}, hc...))
		}
		if m.Arrives != nil {
			w, ac := b.triggerWords(m.Arrives, lineContext{})
			add(b.fstmt(KindArrives, about, bindPath+".arrives", fmt.Sprintf(
				"%s is not there at first: it arrives when %s.", name, w),
				foldInfo{text: "{Self} is not there at first: it arrives when " + w + ".", site: "the arrival of", who: d.ID}, ac...))
		}
		if bind.Behaviors != nil {
			for k, use := range *bind.Behaviors {
				s := fmt.Sprintf("%s runs the routine %s (as %s)", name, quote(use.Use), quote(use.ID))
				short := fmt.Sprintf("Runs the routine %s (as %s)", quote(use.Use), quote(use.ID))
				if bind.BehaviorState == use.ID {
					s += "; this routine's state decides its posts and when its body blocks"
					short += ", whose state decides its posts and blocking"
				}
				add(b.fstmt(KindRoutineUse, about, fmt.Sprintf("%s.behaviors[%d]", bindPath, k), s+".", foldInfo{text: short + "."},
					Connection{Kind: "routine", ID: use.Use, Role: "uses"}))
			}
		}
		if len(m.BlockingStates) > 0 {
			add(b.fstmt(KindBlocking, about, bindPath+".blockingStates", fmt.Sprintf(
				"While its routine is in the state %s, %s's body blocks the cell it stands on.",
				joinOr(quoteAll(m.BlockingStates)), name), foldInfo{text: fmt.Sprintf(
				"While its routine is in %s, {self}'s body blocks the cell it stands on.", joinOr(quoteAll(m.BlockingStates)))}))
		}
		for _, state := range sortedKeys(m.Posts) {
			g.Statements = append(g.Statements, b.postStatements(d.ID, state, m.Posts[state], bindPath+".posts."+state)...)
		}
		switch {
		case len(bind.Options) > 0:
			g.Statements = append(g.Statements, b.lineStatements(m.Options, bindPath+".options",
				lineTarget{name: name, short: b.shortName(d.ID), about: about, id: d.ID})...)
		case len(m.Options) > 0:
			fi, ok := b.factions[m.Faction]
			path := bindPath
			if ok {
				path = fmt.Sprintf("factions[%d].options", fi)
			}
			add(b.fstmt(KindBorrowLines, about, path, fmt.Sprintf(
				"%s has no lines of its own; the speaker can say to it the faction %s's lines (said under that faction).",
				name, quote(m.Faction)), foldInfo{text: fmt.Sprintf(
				"No lines of its own: the speaker can say to {self} the faction %s's lines (said under that faction).", quote(m.Faction))}, Connection{Kind: "faction", ID: m.Faction, Role: "speaks-lines-of"}))
		}
		groups = append(groups, g)
	}
	return groups
}

// postStatements says one post: the door it holds, from where, against
// which crossings, and who it lets through.
func (b *builder) postStatements(id, state string, p encounter.Post, path string) []Statement {
	name := b.creature(id)
	about := Subject{Kind: "creature", ID: id}
	door := b.unqualify(string(p.Door))
	var cells []string
	for _, c := range p.Cells {
		cells = append(cells, posWords(c))
	}
	side, sideName := "no authored region", "no authored region"
	if len(p.Cells) > 0 {
		side = b.regionAt(int(p.Cells[0].X), int(p.Cells[0].Y))
		sideName = side
		if i := strings.LastIndex(side, " ("); i > 0 {
			sideName = side[:i]
		}
	}
	against := "both ways: every crossing of it"
	switch p.Against {
	case encounter.PostEntering:
		against = fmt.Sprintf("one way: a crossing that starts on the post's side, %s, and ends on the far side "+
			"(the other way is not barred)", side)
	case encounter.PostLeaving:
		against = fmt.Sprintf("one way: a crossing that starts on the far side and ends on the post's side, %s "+
			"(the other way is not barred)", side)
	}
	s := fmt.Sprintf("While its routine is in the state %s and it stands, up, on %s, %s holds the door %s: "+
		"every creature not allied with %s is barred from crossing it, %s. Several holders hold one door: it is held while any of them holds it.",
		quote(state), joinOr(cells), name, quote(door), name, against)
	fagainst := "both ways"
	switch p.Against {
	case encounter.PostEntering:
		fagainst = fmt.Sprintf("one way: from its side (%s) to the far side", sideName)
	case encounter.PostLeaving:
		fagainst = fmt.Sprintf("one way: from the far side to its side (%s)", sideName)
	}
	short := fmt.Sprintf("and {self} stands, up, on %s, it holds the door %s against anyone not allied with it, %s",
		joinOr(cells), quote(door), fagainst)
	out := []Statement{b.fstmt(KindPost, about, path, s, foldInfo{text: short, state: state},
		Connection{Kind: "door", ID: door, Role: "holds"})}
	if len(p.Pass) > 0 {
		var reqs, freqs []string
		var conns []Connection
		for _, t := range p.Pass {
			w, c := b.triggerWords(t, lineContext{me: "the walker", other: name})
			fw, _ := b.triggerWords(t, lineContext{me: "the walker", other: "{self}"})
			fw = shortClause(fw)
			reqs = append(reqs, w)
			freqs = append(freqs, fw)
			conns = append(conns, c...)
		}
		out = append(out, b.fstmt(KindPass, about, path+".pass", fmt.Sprintf(
			"%s lets a walker through that post, as an ally walks, when %s.", name, strings.Join(reqs, " and ")),
			foldInfo{text: "it lets a walker through, as an ally, when " + strings.Join(freqs, " and "),
				state: state, site: "the posts", who: id}, conns...))
	}
	return out
}
