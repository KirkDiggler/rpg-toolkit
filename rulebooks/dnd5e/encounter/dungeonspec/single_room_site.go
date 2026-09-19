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
// Every shape is the v2 root's, every refusal is the v2 validator's, and the
// only thing this file writes is the ADAPTER between them: the single-room
// document seen as the [Spec] those validators already read ([siteSpec]), and
// the paths their defects are reported at. Paths are part of the contract —
// the World Builder draws each refusal on the thing it names — so they are
// this dialect's own (`room.room.monsters[2].faction`,
// `room.room.monsterBindings.goblin-1.on.time[0]`), which is why each
// validator is called with the path rather than having one rewritten after
// the fact.
//
// The precedent is [CompileTable], which already runs [validation.placeOn]
// over a `Spec{Orientation: "pointy"}` it builds for the purpose: one
// grammar, two callers, because the first thing a second copy would drift on
// is a refusal the author sees and the engine does not.
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

// errLiveRoomMonster is the orders block's owner rule, in
// [errLiveSceneProp]'s own shape: a declaration may not outlive the thing it
// declares. The web's room draft keeps the same discipline, dropping a
// binding when its creature is removed.
const errLiveRoomMonster = "must name a live room monster"

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
// [validation.factions] says "the faction has no id" and
// [validation.dispositions] says "the disposition does not say its stance" in
// an author's own words, and two defects for one mistake sends an author
// looking for a second problem. An unknown key inside any of these blocks is
// named by `KnownFields(true)`, with the key and the type that has no such
// field, exactly as everywhere else in this document.

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
	}
}

// # Decoded values
//
// siteSpec is the single-room document seen as the [Spec] the v2 faction
// validators and compilers read: the declared sides as they stand, and one
// synthetic [PlaceSpec] per authored monster carrying its id, its ref, its
// membership and — from its binding, when it has one — its orders.
//
// SYNTHETIC IN THE PLACEMENTS ONLY, and deliberately so: `faction`, `on`,
// `temper` and `actions` are the whole of what those validators and
// [monstersOf] read off a placement here. Geometry is NOT carried across —
// [PlaceSpec.At] stays zero — because the single room's cells are axial and
// its own compiler already judges them in the author's frame. Nothing in this
// file asks a geometric question, which is what makes that safe.
func siteSpec(s *SingleRoomSpec) *Spec {
	out := &Spec{Key: s.Key, Factions: s.Factions, Dispositions: s.Dispositions}
	out.Place = make([]PlaceSpec, 0, len(s.Room.Gameplay.Monsters))
	for _, m := range s.Room.Gameplay.Monsters {
		pl := PlaceSpec{ID: m.ID, Ref: m.Ref, Faction: m.Faction}
		if b, bound := s.Room.Gameplay.MonsterBindings[m.ID]; bound {
			pl.On, pl.Temper, pl.Actions = b.On, b.Temper, b.Actions
		}
		out.Place = append(out.Place, pl)
	}
	return out
}

// siteValues judges the site scope and the orders with the v2 validators, at
// this dialect's paths.
//
// The ORDER is [Validate]'s own, and each dependency is real: the factions are
// indexed before a membership can name one, the memberships are counted before
// a mind or a faction-of-one is judged, and the dispositions come last because
// an `until` is judged against the whole stance table.
func siteValues(s *SingleRoomSpec, add errSink) {
	gp := &s.Room.Gameplay
	// NO "DID THIS DOCUMENT AUTHOR ANY OF IT" SHORTCUT. Every validator below
	// is INERT on a document that declares nothing — no faction to index, no
	// membership but the reserved one, no orders block to judge — and the v3
	// fixture's committed picture is the evidence. A guard would buy nothing
	// and would silently skip whatever check is added here next.
	v := &validation{spec: siteSpec(s), cellRefusal: singleRoomCellSelector}
	v.factions()
	v.factionOrders()
	v.placeIDs = make(map[string]int, len(v.spec.Place))
	for i := range v.spec.Place {
		pl := v.spec.Place[i]
		if _, dup := v.placeIDs[pl.ID]; pl.ID != "" && !dup {
			v.placeIDs[pl.ID] = i
		}
		// EVERY PLACEMENT HERE IS A MONSTER. `monsters:` is the only list
		// this dialect places, and [monsterValues] already refuses a ref that
		// is not one, at `room.room.monsters[i].ref` — so asking refKind
		// again would report a second defect for the one bad ref.
		v.placeFaction(fmt.Sprintf("room.room.monsters[%d]", i), i, pl, typeMonsters)
		if _, bound := gp.MonsterBindings[pl.ID]; !bound {
			continue
		}
		at := "room.room.monsterBindings." + pl.ID
		v.placeOn(at, pl.On)
		v.placeTemper(at, pl)
		v.placeActions(at, pl)
	}
	v.minds()
	v.dispositions()
	// A binding whose creature is gone is refused the way a prop declaration
	// with no live owner is. Sorted, because a map's iteration order is not a
	// defect list an author can compare run to run.
	for _, id := range sortedBindings(gp.MonsterBindings) {
		if _, live := v.placeIDs[id]; !live {
			v.fail("room.room.monsterBindings."+id, "%s", errLiveRoomMonster)
		}
	}
	for _, e := range v.errs {
		add(e.Path, e.Message)
	}
}

// sortedBindings orders the orders blocks' ids, for [sortedKeys]' reason.
func sortedBindings(bindings map[string]RoomMonsterBinding) []string {
	out := make([]string, 0, len(bindings))
	for id := range bindings {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
