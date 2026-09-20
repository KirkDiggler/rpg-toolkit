// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// single_room_site.go is THE SITE SCOPE AND THE CREATURE'S ORDERS IN THE
// SINGLE-ROOM DIALECT (rpg-project#477, Decision 4; rpg-toolkit#1826) — the
// root's `factions:` and `dispositions:`, a creature's `faction:`, and the
// `monsterBindings:` block that carries what a faction would otherwise
// supply.
//
// # Nothing here is a second dialect
//
// Every shape is the v2 root's and every refusal is the one gameplay
// grammar's (grammar.go). What this file writes is the dialect's HALF of that
// call: the cast it placed, the frame it has for a cell, and the paths its
// defects are reported at. Paths are part of the contract — the World Builder
// draws each refusal on the thing it names — so they are this dialect's own
// (`room.room.monsters[2].faction`,
// `room.room.monsterBindings.goblin-1.on.time[0]`), which is why each
// grammar function is called with the path rather than having one rewritten
// after the fact.
//
// This used to reach the grammar by BUILDING A FAKE v2 DOCUMENT and running
// the v2 validator over it. The types were always shared; only the functions
// were not, and a fake owner was standing in for the ownership question
// (rpg-project#484). They are shared now, and nothing here impersonates
// anything.
//
// # The one refusal this dialect adds
//
// `{ at: [col, row] }` inside an orders block. [SelectorSpec.At] is an offset
// resolved through the document's `orientation`, and the single-room dialect
// has none — its cells are axial {q, r} — so the same bytes would mean two
// different cells in the two dialects. The only shipped use of `at` is a
// reserve walking in from another room, and a single room has no elsewhere
// yet: refused now, fail closed, defined against the room's own frame when
// the sites layer gives a creature somewhere to walk to (rpg-toolkit#1826,
// ruling 1).

// singleRoomCellSelector is that refusal, verbatim. One sentence, saying what
// is wrong, what to write instead, and what would make it legal.
const singleRoomCellSelector = "at is not a place a single room can name yet: its cells are axial and the selector's " +
	"cell is an offset in a document orientation this dialect does not have; name enemy, attacker or actor, " +
	"or wait for the sites layer"

// singleRoomCells is this dialect's answer to [cells]: it names no cell at
// all. Not framed, so the refusal above is the whole defect and the word
// carrying the selector is never judged beside it.
type singleRoomCells struct{}

func (singleRoomCells) cellAt([2]int) cellAnswer {
	return cellAnswer{refusal: singleRoomCellSelector}
}

// # Source shape
//
// The walk below covers every new key so none of them is a hole in the strict
// read. What it judges is what the TYPED DECODE CANNOT: yaml.v3 refuses a
// value of the wrong kind outright, and [DecodeSingleRoom] returns on that
// error before this walk runs at all — but it reads an authored `null` as the
// Go zero value without a word, which would be an orders block that orders
// nothing, a table that was never written, a faction with no mind. Each of
// those is named at its path here, and an empty NAME is refused beside it,
// because "" and absence are the same bytes meaning different things.
//
// It deliberately does NOT ask whether a required word is present.
// [grammar.factions] says "the faction has no id" and
// [grammar.dispositions] says "the disposition does not say its stance" in
// an author's own words, and two defects for one mistake sends an author
// looking for a second problem. An unknown key inside any of these blocks is
// caught by `KnownFields(true)` and named at its own path, with the keys that
// block does take — exactly as everywhere else in this document, and in either
// dialect (rpg-project#481, R2; unknown_key.go).

// siteScopeShape reads the optional root scope: `factions` and
// `dispositions`, each a list of mappings.
func siteScopeShape(doc *yaml.Node, add errSink) {
	if factions := optionalNode(doc, "factions", "", add); factions != nil {
		for i, fa := range factions.Content {
			factionShape(fa, fmt.Sprintf("factions[%d]", i), add)
		}
	}
	if dispositions := optionalNode(doc, "dispositions", "", add); dispositions != nil {
		for i, d := range dispositions.Content {
			dispositionShape(d, fmt.Sprintf("dispositions[%d]", i), add)
		}
	}
}

func factionShape(fa *yaml.Node, p string, add errSink) {
	fa = resolveNode(fa)
	if fa == nil || fa.Kind != yaml.MappingNode {
		return
	}
	optionalNode(fa, "id", p, add)
	optionalReference(fa, "mind", p, add)
	optionalNode(fa, "on", p, add)
	// `temper` is a WORD OR A MIX, so only its null-ness is judged here; the
	// two spellings and the sealed words are [TemperSpec.UnmarshalYAML]'s.
	optionalNode(fa, "temper", p, add)
}

func dispositionShape(d *yaml.Node, p string, add errSink) {
	d = resolveNode(d)
	if d == nil || d.Kind != yaml.MappingNode {
		return
	}
	optionalNode(d, "between", p, add)
	optionalNode(d, "stance", p, add)
	optionalNode(d, "until", p, add)
}

// monsterBindingsShape reads the optional orders blocks, keyed by creature id.
func monsterBindingsShape(gp *yaml.Node, add errSink) {
	bindings := optionalNode(gp, "monsterBindings", "room.room", add)
	if bindings == nil || bindings.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(bindings.Content); i += 2 {
		p := "room.room.monsterBindings." + bindings.Content[i].Value
		b := resolveNode(bindings.Content[i+1])
		if b == nil || isNull(b) {
			add(p, errNotNull)
			continue
		}
		if b.Kind != yaml.MappingNode {
			continue
		}
		optionalNode(b, "on", p, add)
		// A binding's `temper` is a placement's own WORD, so an empty one is
		// [optionalReference]'s case: a name that names nothing.
		optionalReference(b, "temper", p, add)
		optionalNode(b, "actions", p, add)
		// AND THE FOUR GAMEPLAY KEYS (rpg-project#488,
		// single_room_gameplay.go). Each is judged here for the same one
		// thing: an authored `null` that the typed decode would read as the
		// Go zero value — a `holds:` the author emptied, a check they
		// deleted, a predicate that is now no predicate at all.
		optionalNode(b, "holds", p, add)
		optionalNode(b, "intimidate", p, add)
		optionalNode(b, "persuade", p, add)
		optionalNode(b, "arrives", p, add)
	}
}

// # Decoded values
//
// roomMembers is the cast this dialect hands the grammar: every authored
// creature, in authored order, as who it is, what it is and which side it is
// on. Index-aligned with `monsters:`, so a defect about a membership is
// reported at `room.room.monsters[i].faction`.
//
// EVERY MEMBER HERE IS A MONSTER. `monsters:` is the only list this dialect
// places; its props are scenery, declared under their own key and nameable by
// nothing that takes a member id.
func roomMembers(gp *RoomGameplaySource) members {
	all := make([]member, 0, len(gp.Monsters))
	for _, m := range gp.Monsters {
		all = append(all, member{id: m.ID, ref: m.Ref, faction: m.Faction})
	}

	return newMembers(all)
}

// siteGrammar judges the site scope and the orders with the one gameplay
// grammar, at this dialect's paths.
//
// The ORDER is [Validate]'s own, and each dependency is real: the factions are
// indexed before a membership can name one, the memberships are counted before
// a mind or a faction-of-one is judged, and the dispositions come last because
// an `until` is judged against the whole stance table.
func siteGrammar(s *SingleRoomSpec, add errSink) {
	gp := &s.Room.Gameplay
	// NO "DID THIS DOCUMENT AUTHOR ANY OF IT" SHORTCUT. Every function below
	// is INERT on a document that declares nothing — no faction to index, no
	// membership but the reserved one, no orders block to judge — and the v3
	// fixture's committed picture is the evidence. A guard would buy nothing
	// and would silently skip whatever check is added here next.
	g := newGrammar(grammarInput{
		Add:          add,
		Factions:     s.Factions,
		Dispositions: s.Dispositions,
		Members:      roomMembers(gp),
		Cells:        singleRoomCells{},
	})
	g.factions()
	g.factionOrders()
	// THE RECORDS BEFORE ANY HOLDER, which is [validation.intel]'s own
	// ordering reason: the orders loop below asks whether a `holds:` names a
	// record that exists, so the universe it may name has to be indexed
	// first (rpg-project#488, single_room_gameplay.go).
	declared := intelRecords(s.Intel, add)
	for i, m := range g.members.all {
		// EVERY MEMBER HERE IS A MONSTER, and the kind says so rather than
		// being read back off the ref: [monsterValues] already refuses a ref
		// that is not one, at `room.room.monsters[i].ref`, so asking refKind
		// again would report a second defect for the one bad ref.
		g.placeFaction(fmt.Sprintf("room.room.monsters[%d]", i), i, m, typeMonsters)
		b, bound := gp.MonsterBindings[m.id]
		if !bound {
			continue
		}
		at := "room.room.monsterBindings." + m.id
		g.placeOn(at, b.On)
		g.placeTemper(at, b.Temper)
		g.placeActions(at, b.Actions)
		// AND THE GAMEPLAY KEYS THE SAME BINDING CARRIES (rpg-project#488
		// R1/R5). In this loop rather than in one of their own, so they are
		// judged for the LIVE cast only: a binding whose creature is gone is
		// named once, below, and asking a second question about its contents
		// would send an author looking for a second problem.
		bindingHolds(g, at, m.id, b.Holds, declared)
		bindingChecks(g, at, b)
	}
	g.minds()
	g.dispositions()
	// THE PROP ORDERS AND THE ARRIVALS (rpg-project#488,
	// single_room_gameplay.go). Here rather than in the loop above because
	// neither is about the cast: a prop binding is about a placed thing, and
	// an `arrives:` predicate may name a `{ stance }`, which is judged
	// against the whole table [grammar.dispositions] just built.
	propBindingValues(gp, g, declared)
	singleRoomArrivals(gp, g)
	// A binding whose creature is gone is refused the way a prop declaration
	// with no live owner is — a declaration may not outlive the thing it
	// declares, and the web's room draft keeps the same discipline by dropping
	// a binding when its creature is removed. Sorted, because a map's
	// iteration order is not a defect list an author can compare run to run.
	for _, id := range sortedBindingIDs(gp.MonsterBindings) {
		if _, live := g.members.indexOf(id); !live {
			add("room.room.monsterBindings."+id, "must name a live room monster")
		}
	}
	// AND THE DOORS, judged by the same grammar's state half
	// (single_room_doors.go). Last, because a door is about a PLACED thing
	// rather than about the cast, and nothing above it depends on one.
	doorBindingValues(gp, g, add)
}

// sortedBindingIDs orders the orders blocks' ids, for [sortedKeys]' reason.
func sortedBindingIDs(bindings map[string]RoomMonsterBinding) []string {
	out := make([]string, 0, len(bindings))
	for id := range bindings {
		out = append(out, id)
	}
	sort.Strings(out)

	return out
}
