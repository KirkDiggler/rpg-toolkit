// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// sharedlines.go says A CONVERSATION'S LINES WRITTEN ONCE (rpg-project#1025):
// each root `conversations:` entry is one group about {lines, <id>}, its
// lines said once at their own paths and its carriers connected
// {creature|prop|faction, <id>, carries}; a carrier says each conversation it
// uses as one statement at its `use:` entry, connected {lines, <id>,
// speaks-lines-of}, the way a creature with no lines of its own points at
// its faction's.

// lineCarrierOf is one carrier's list of lines: its subject, its path, its
// authored list and its compiled lines.
type lineCarrierOf struct {
	about    Subject
	at       string
	authored []dungeonspec.OptionSpec
	compiled []encounter.Option
}

// lineCarriersOf is every carrier of lines in the file: the factions in
// authored order, then the creatures in declaration order, then the props by
// id.
func (b *builder) lineCarriersOf() []lineCarrierOf {
	var out []lineCarrierOf
	for i, f := range b.spec.Factions {
		out = append(out, lineCarrierOf{about: Subject{Kind: "faction", ID: f.ID}, at: fmt.Sprintf("factions[%d]", i),
			authored: f.Options, compiled: b.factionLines(f.ID)})
	}
	for _, d := range b.spec.Room.Gameplay.MonsterDeclarations {
		bind := b.spec.Room.Gameplay.MonsterBindings[d.ID]
		var compiled []encounter.Option
		if m := b.monsters[d.ID]; m != nil {
			compiled = m.Options
		}
		out = append(out, lineCarrierOf{about: Subject{Kind: "creature", ID: d.ID},
			at: "room.room.monsterBindings." + d.ID, authored: bind.Options, compiled: compiled})
	}
	for _, id := range sortedKeys(b.spec.Room.Gameplay.PropBindings) {
		out = append(out, lineCarrierOf{about: Subject{Kind: "prop", ID: id}, at: "room.room.propBindings." + id,
			authored: b.spec.Room.Gameplay.PropBindings[id].Options, compiled: b.propLines(id)})
	}
	return out
}

// propLines is a prop's compiled lines, cell prop or placed footprint.
func (b *builder) propLines(id string) []encounter.Option {
	for _, p := range b.c.Field.Props {
		if b.unqualify(string(p.ID)) == id {
			return p.Options
		}
	}
	for _, p := range b.c.Field.Placed {
		if b.unqualify(string(p.ID)) == id {
			return p.Options
		}
	}
	return nil
}

// indexConversations fills, for each conversation, who carries it and its
// compiled lines (read off the first carrier that compiled them: a
// conversation's lines compile alike on every carrier).
func (b *builder) indexConversations() {
	for _, c := range b.lineCarriersOf() {
		sources := dungeonspec.LineSources(b.spec, c.at, c.authored)
		aligned := len(c.compiled) == len(sources)
		seen := map[string]bool{}
		for i, src := range sources {
			conv := src.Conversation
			if conv == "" {
				continue
			}
			if !seen[conv] {
				seen[conv] = true
				b.convCarriers[conv] = append(b.convCarriers[conv], Connection{Kind: c.about.Kind, ID: c.about.ID, Role: "carries"})
				if aligned && b.convFrom[conv] == "" {
					b.convFrom[conv] = c.at
				}
			}
			if aligned && b.convFrom[conv] == c.at {
				b.convLines[conv] = append(b.convLines[conv], c.compiled[i])
				b.convLinePaths[conv] = append(b.convLinePaths[conv], src.Path)
			}
		}
	}
}

// carrierName is how a carrier is named in a conversation's sentence.
func (b *builder) carrierName(c Connection) string {
	switch c.Kind {
	case "creature":
		return b.creature(c.ID)
	case "prop":
		return "the prop " + b.propName(c.ID)
	default:
		return fmt.Sprintf("any creature of the faction %s that has no lines of its own", quote(c.ID))
	}
}

// sharedLineGroups says every conversation: what it is, who carries it, and
// each of its lines once, at its own path.
func (b *builder) sharedLineGroups() []Group {
	var groups []Group
	for _, id := range sortedKeys(b.spec.Conversations) {
		about := Subject{Kind: "lines", ID: id}
		at := "conversations." + id
		g := Group{Title: "The lines of " + quote(id), About: about}
		carriers := b.convCarriers[id]
		var names []string
		allProps := len(carriers) > 0
		for _, c := range carriers {
			names = append(names, b.carrierName(c))
			allProps = allProps && c.Kind == "prop"
		}
		var ids []string
		for _, line := range b.spec.Conversations[id].Lines {
			ids = append(ids, quote(line.ID))
		}
		who := "nobody"
		if len(names) > 0 {
			who = joinAnd(names)
		}
		s := fmt.Sprintf("The conversation %s is lines written once: %s. It is carried by %s, %s; each offers these lines "+
			"as if they were written on it, beside any lines of its own.", quote(id), joinAnd(ids),
			count(len(carriers), "carrier", "carriers"), who)
		g.Statements = append(g.Statements, b.fstmt(KindSharedLines, about, at, s, foldInfo{text: fmt.Sprintf(
			"Lines written once: %s. Carried by %s (%s).", joinAnd(ids), count(len(carriers), "carrier", "carriers"), who)},
			carriers...))
		lines := b.convLines[id]
		if lines == nil {
			b.unsaid = append(b.unsaid, Unsaid{Path: at + ".lines", Kind: "conversation lines",
				Why: "no carrier of the conversation speaks them, so they are not in the compiled encounter to be said"})
		}
		g.Statements = append(g.Statements, b.lineStatementsAt(lines, b.convLinePaths[id], lineTarget{
			name: "the carrier", short: "the carrier",
			about: about, prop: allProps})...)
		groups = append(groups, g)
	}
	return groups
}

// carrierLineStatements says one carrier's lines: each of its own in full
// at its own path, and each conversation it uses as one statement at its
// `use:` entry, naming the lines it brings. A list the sources do not match
// (not a compiled site's) is said at the expanded paths.
func (b *builder) carrierLineStatements(opts []encounter.Option, at string, authored []dungeonspec.OptionSpec, t lineTarget) []Statement {
	sources := dungeonspec.LineSources(b.spec, at, authored)
	if len(sources) != len(opts) {
		return b.lineStatements(opts, at+".options", t)
	}
	var out []Statement
	for i := 0; i < len(sources); i++ {
		src := sources[i]
		if src.Conversation == "" {
			out = append(out, b.lineStatementsAt(opts[i:i+1], []string{src.Path}, t)...)
			continue
		}
		var ids []string
		for ; i < len(sources) && sources[i].Use == src.Use; i++ {
			ids = append(ids, quote(opts[i].ID))
		}
		i--
		s := fmt.Sprintf("%s carries the conversation %s: it offers that conversation's lines %s, each said once under "+
			"the conversation.", capitalize(t.name), quote(src.Conversation), joinAnd(ids))
		out = append(out, b.fstmt(KindUsesLines, t.about, src.Use, s, foldInfo{text: fmt.Sprintf(
			"Carries the conversation %s: its lines %s (said under that conversation).", quote(src.Conversation), joinAnd(ids))},
			Connection{Kind: "lines", ID: src.Conversation, Role: "speaks-lines-of"}))
	}
	return out
}

// lineSourcesOf is where each of a creature's compiled lines was written:
// its own list's sources, else its faction's.
func (b *builder) lineSourcesOf(member string) []dungeonspec.LineSource {
	bind := b.spec.Room.Gameplay.MonsterBindings[member]
	if len(bind.Options) > 0 {
		return dungeonspec.LineSources(b.spec, "room.room.monsterBindings."+member, bind.Options)
	}
	m := b.monsters[member]
	if m == nil {
		return nil
	}
	fi, ok := b.factions[m.Faction]
	if !ok {
		return nil
	}
	return dungeonspec.LineSources(b.spec, fmt.Sprintf("factions[%d]", fi), b.spec.Factions[fi].Options)
}
