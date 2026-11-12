// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// builder holds what every section reads: the authored spec (for paths and
// names), the compiled result (what will run) and the authored tree.
type builder struct {
	spec *dungeonspec.SingleRoomSpec
	c    *dungeonspec.Compiled
	doc  document

	names     map[string]string // member id → "Guard (gate-1)"
	monsters  map[string]*dungeonspec.MonsterPlacement
	declIndex map[string]int
	factions  map[string]int // faction id → index in factions[]

	// carriers is, by conversation, every creature whose lines read or step
	// it, in declaration order.
	carriers map[string][]string

	// hubs collects every statement that grants, asks for, teaches or wakes
	// on a topic or a fact, for the hub sections.
	hubs []Statement

	// unsaid is what a section found it could not say.
	unsaid []Unsaid
}

func newBuilder(spec *dungeonspec.SingleRoomSpec, c *dungeonspec.Compiled, raw []byte) *builder {
	b := &builder{
		spec: spec, c: c, doc: newDocument(raw),
		names: map[string]string{}, monsters: map[string]*dungeonspec.MonsterPlacement{},
		declIndex: map[string]int{}, factions: map[string]int{}, carriers: map[string][]string{},
	}
	for i, f := range spec.Factions {
		b.factions[f.ID] = i
	}
	for i, d := range spec.Room.Gameplay.MonsterDeclarations {
		b.declIndex[d.ID] = i
		b.names[d.ID] = fmt.Sprintf("%s (%s)", b.kindName(d.Ref), d.ID)
	}
	for i := range c.Monsters {
		m := &c.Monsters[i]
		b.monsters[m.MemberID] = m
		if _, ok := b.names[m.MemberID]; !ok {
			b.names[m.MemberID] = fmt.Sprintf("%s (%s)", b.kindName(m.Ref), m.MemberID)
		}
	}
	for _, d := range spec.Room.Gameplay.MonsterDeclarations {
		m := b.monsters[d.ID]
		if m == nil {
			continue
		}
		seen := map[string]bool{}
		for _, o := range b.linesOf(m) {
			if id, ok := optionInteraction(o); ok && !seen[id] {
				seen[id] = true
				b.carriers[id] = append(b.carriers[id], d.ID)
			}
			for _, t := range o.Requires {
				if ti, ok := t.(encounter.TriggerInteraction); ok && !seen[string(ti.Interaction)] {
					seen[string(ti.Interaction)] = true
					b.carriers[string(ti.Interaction)] = append(b.carriers[string(ti.Interaction)], d.ID)
				}
			}
		}
	}
	return b
}

// kindName is the display name a ref gives a creature: the name of the
// stat block the file authors for it, else the ref's last word.
func (b *builder) kindName(ref string) string {
	id := ref[strings.LastIndex(ref, ":")+1:]
	if t, ok := b.spec.Templates[id]; ok && t.Name != "" {
		return t.Name
	}
	return capitalize(id)
}

// creature is a member's display name with its id.
func (b *builder) creature(id string) string {
	if n, ok := b.names[id]; ok {
		return n
	}
	return quote(id)
}

// creatures names several members.
func (b *builder) creatures(ids []string) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = b.creature(id)
	}
	return joinAnd(out)
}

// unqualify strips the site key a compiled id carries (`key/door`).
func (b *builder) unqualify(id string) string {
	return strings.TrimPrefix(id, b.c.Key+"/")
}

// linesOf is the lines a creature can be spoken to with, as compiled.
func (b *builder) linesOf(m *dungeonspec.MonsterPlacement) []encounter.Option { return m.Options }

// optionInteraction is the conversation a line reads or steps: its advance,
// else its first interaction requirement.
func optionInteraction(o encounter.Option) (string, bool) {
	if o.Advance != nil {
		return string(o.Advance.Interaction), true
	}
	for _, t := range o.Requires {
		if ti, ok := t.(encounter.TriggerInteraction); ok {
			return string(ti.Interaction), true
		}
	}
	return "", false
}

// regionAt is the authored region whose rows hold the axial cell, and its
// display words.
func (b *builder) regionAt(q, r int) string {
	for _, reg := range b.spec.Room.Gameplay.Regions {
		for _, row := range reg.Cells {
			if row.R != r {
				continue
			}
			for _, span := range row.Q {
				if q >= span[0] && q <= span[1] {
					return b.regionWords(reg.ID)
				}
			}
		}
	}
	return "no authored region"
}

// regionWords is a region's name with its id.
func (b *builder) regionWords(id string) string {
	for _, reg := range b.spec.Room.Gameplay.Regions {
		if reg.ID == id {
			return fmt.Sprintf("%s (%s)", reg.Name, reg.ID)
		}
	}
	return quote(id)
}

// stmt makes a statement and, when it touches a topic or a fact, keeps it
// for that hub.
func (b *builder) stmt(kind Kind, about Subject, path, sentence string, conns ...Connection) Statement {
	s := Statement{Kind: kind, About: about, Sentence: sentence, Path: path, Connects: dedupe(conns)}
	for _, c := range s.Connects {
		if c.Kind == "topic" || c.Kind == "fact" {
			b.hubs = append(b.hubs, s)
			break
		}
	}
	return s
}

// dedupe drops repeated connections, keeping the first.
func dedupe(conns []Connection) []Connection {
	if len(conns) == 0 {
		return nil
	}
	seen := map[Connection]bool{}
	out := make([]Connection, 0, len(conns))
	for _, c := range conns {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// build says the whole encounter.
func (b *builder) build() *ReadBack {
	rb := &ReadBack{Key: b.c.Key, Name: b.spec.Room.Name}
	rb.Sections = append(rb.Sections,
		Section{Title: "The place", Groups: b.place()},
		Section{Title: "Sides", Groups: b.sides()},
		Section{Title: "Creatures", Groups: b.creatureGroups()},
		Section{Title: "Conversations", Groups: b.conversations()},
		Section{Title: "Routines", Groups: b.routines()},
	)
	// The hubs last, from every statement above that touches them.
	rb.Sections = append(rb.Sections,
		Section{Title: "Topics", Groups: b.hubGroups("topic")},
		Section{Title: "Facts", Groups: b.hubGroups("fact")},
	)
	rb.NotYetSaid = b.notYetSaid()
	return rb
}

// sortedKeys is a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// statesInOrder is a recipe's states, the initial one first, then by name.
func statesInOrder(r machine.Recipe) []string {
	out := []string{}
	if _, ok := r.States[r.Initial]; ok {
		out = append(out, r.Initial)
	}
	for _, s := range sortedKeys(r.States) {
		if s != r.Initial {
			out = append(out, s)
		}
	}
	return out
}
