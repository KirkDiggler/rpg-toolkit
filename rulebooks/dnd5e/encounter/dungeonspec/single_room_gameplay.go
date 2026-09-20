// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// single_room_gameplay.go is WHAT THE ROOM IS FOR, IN THE SINGLE-ROOM DIALECT
// (rpg-project#488, R1/R3/R5) — the root's `intel:`, the four gameplay keys a
// creature's orders gained, and the `propBindings:` block that is the fourth
// declaration kind.
//
// # Nothing here is a second dialect, and nothing here is a new capability
//
// Every key below is a v2 key reaching this dialect unchanged: the same Go
// shape off spec.go, the same sentences, judged by the one shared grammar
// (grammar.go) at THIS dialect's paths. single_room_site.go's whole preamble
// applies to this file too — paths are the contract, the grammar owns no
// geometry, and a refusal an author reads is the one they would have read in
// the other dialect.
//
// What each key maps to, and the v2 field it copies:
//
//	intel[<i>]                               Spec.Intel            (spec.go)
//	monsterBindings.<id>.holds[<j>]          PlaceSpec.Holds
//	monsterBindings.<id>.intimidate          PlaceSpec.Intimidate
//	monsterBindings.<id>.persuade            PlaceSpec.Persuade
//	monsterBindings.<id>.arrives             PlaceSpec.Arrives
//	propBindings.<id>.holdable               PlaceSpec.Holdable
//	propBindings.<id>.holds[<j>]             PlaceSpec.Holds
//	propBindings.<id>.arrives                PlaceSpec.Arrives
//
// # The two things this dialect says NO to, and why each is a sentence
//
// `intel[<i>].reveals.door` (R3) and the whole `propBindings` block are
// refused BY NAME rather than left as unknown keys. Both words mean
// something: a record CAN reveal the way to a door, and a placed prop CAN be
// picked up — in the engine, today, from the other dialect. What is missing
// is the thing underneath each. A revealed door wants a CONCEALED door on a
// crossing and this dialect has no crossings (the sites layer); a held prop
// wants a placed footprint that can be held and can arrive
// (rpg-toolkit#1854). So the author is told that, at the path they wrote it
// at, in the shape `doorBindings.<id>.concealed` already uses.
//
// THE TWO REFUSALS SIT AT DIFFERENT SEAMS, on purpose. `reveals.door` is a
// DOCUMENT defect — there is no legal document containing it — so it is
// refused by [validateSingleRoom] and a decode fails. `propBindings` is a
// legal document the engine cannot RUN yet, so it decodes with every refusal
// below applied and [CompileSingleRoom] is where it stops. That is what lets
// the World Builder author the block, round-trip it and grade it today
// ("carry the shape, let the engine grade it", rpg-dnd5e-web#1171) while the
// engine still fails closed and loudly rather than accepting a key it drops.

// The sentences this file adds, verbatim. Constants for
// single_room_doors.go's reason: the same defect always reports the same
// words, and a test pins the sentence an author reads rather than a
// paraphrase of it.
const (
	// intelRevealsDoor is `reveals: { door }` in this dialect (R3) — the word
	// means something, it simply needs a crossing to mean it.
	intelRevealsDoor = "a door is not something a single room can reveal yet: revealing the way to one needs a " +
		"concealed door on a crossing, and this dialect's doors are footprints standing in the open; " +
		"write `fact: <id>`, or wait for the sites layer"

	// intelRevealsNothing is a record the author started and did not finish.
	// v2's sentence with its `door:` half removed, because `door:` is not a
	// thing an author may write here at all.
	intelRevealsNothing = "does not say what it reveals — `fact: <id>`"

	// propNeedsDeclaration is a binding whose item declares no prop. The
	// prop's geometry IS its declaration, so this is not a missing field, it
	// is orders for something that does not exist.
	propNeedsDeclaration = "a prop's orders need the prop: declare it under propDeclarations"

	// propInArrangement is a binding that names an arrangement TEMPLATE
	// rather than a placed item — [doorInArrangement]'s refusal, one
	// declaration kind over (R1: "an arrangement's members get no orders,
	// the same refusal an arrangement door gets").
	propInArrangement = "a prop inside an arrangement is not something this build stamps yet: " +
		"this id names an arrangement template, so declare the orders on a placed item of its own, " +
		"or wait for the arrangement stamp"

	// propIsADoor is one id declared in both binding blocks. A door that is
	// also picked up has no meaning yet, and inventing one here would be this
	// slice deciding what a carried door leaves behind (R1).
	propIsADoor = "a door that is also picked up is not something this build plays: this id is a door under " +
		"doorBindings, so give the prop an id of its own, or drop one of the two bindings"

	// propBindingNoPrimitive is the COMPILE refusal (rpg-toolkit#1854): the
	// document is legal and the engine has nowhere to put it. Named at the
	// binding's own path so the World Builder draws it on the block the
	// author filled in.
	propBindingNoPrimitive = "a placed prop cannot be held and cannot arrive in this build: a v4 item compiles to a " +
		"footprint, and holdable/holds/arrives live on the legacy prop, which needs a content ref and an " +
		"anchor cell this dialect does not have; author the prop plain, or wait for rpg-toolkit#1854"
)

// # Source shape

// intelShape reads the optional root records. [factionShape]'s reasons: the
// typed decode refuses a value of the wrong KIND outright, but reads an
// authored `null` as the Go zero value without a word — a record with no id,
// a `reveals` the author deleted. Whether a REQUIRED word is present is
// deliberately not asked here, because [intelRecords] says it in an author's
// own words and two defects for one mistake sends them looking for a second
// problem.
func intelShape(doc *yaml.Node, add errSink) {
	records := optionalNode(doc, "intel", "", add)
	if records == nil {
		return
	}
	for i, rec := range records.Content {
		p := fmt.Sprintf("intel[%d]", i)
		rec = resolveNode(rec)
		if rec == nil || isNull(rec) {
			add(p, errNotNull)
			continue
		}
		if rec.Kind != yaml.MappingNode {
			continue
		}
		optionalReference(rec, "id", p, add)
		if reveals := optionalNode(rec, "reveals", p, add); reveals != nil && reveals.Kind == yaml.MappingNode {
			optionalReference(reveals, "door", p+".reveals", add)
			optionalReference(reveals, "fact", p+".reveals", add)
		}
	}
}

// propBindingsShape reads the optional prop orders, keyed by item id —
// [doorBindingsShape]'s walk, one declaration kind over, and for its reason:
// an authored `null` under a key that would otherwise read as the Go zero
// value is a binding that declares nothing.
func propBindingsShape(gp *yaml.Node, add errSink) {
	bindings := optionalNode(gp, "propBindings", "room.room", add)
	if bindings == nil || bindings.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(bindings.Content); i += 2 {
		p := "room.room.propBindings." + bindings.Content[i].Value
		b := resolveNode(bindings.Content[i+1])
		if b == nil || isNull(b) {
			add(p, errNotNull)
			continue
		}
		if b.Kind != yaml.MappingNode {
			continue
		}
		// `holdable` is a state rather than an unanswered question, exactly
		// as `closed` is: LEAVING IT OUT is a prop that stays scenery, and
		// writing null is an author who deleted the word.
		optionalNode(b, "holdable", p, add)
		optionalNode(b, "holds", p, add)
		optionalNode(b, "arrives", p, add)
	}
}

// # Decoded values

// intelRecords validates the root's knowledge records and hands back every id
// this document declares — the universe a `holds:` may name.
//
// [validation.intel]'s three rules, at this dialect's paths: an id, no two
// the same, and exactly one thing revealed. The fourth (a `door` that no door
// in the file has) is not asked, because `door` is refused outright here (R3)
// and asking whether a word this dialect forbids names something real would
// be a second defect for the one mistake.
//
// RUN BEFORE THE BINDINGS, which ask whether a holder names a record that
// exists — the ordering v2 keeps for the same reason.
func intelRecords(records []IntelSpec, add errSink) map[string]bool {
	declared := map[string]bool{}
	at := map[string]int{}
	for i, rec := range records {
		p := fmt.Sprintf("intel[%d]", i)
		switch prev, dup := at[rec.ID]; {
		case rec.ID == "":
			add(p+".id", "the intel record has no id")
		case dup:
			add(p+".id", fmt.Sprintf("intel %q is already declared at intel[%d]", rec.ID, prev))
		default:
			at[rec.ID], declared[rec.ID] = i, true
		}

		// THE DOOR REFUSAL COMES FIRST AND IS THE WHOLE ANSWER for a record
		// that names one. A record with both keys is an author who wrote a
		// word this dialect does not take beside one it does; telling them
		// the word is not built is the sentence that helps, and "a record
		// reveals exactly one thing" beside it is not.
		switch {
		case rec.Reveals.Door != "":
			add(p+".reveals.door", intelRevealsDoor)
		case rec.Reveals.Fact == "":
			add(p+".reveals", fmt.Sprintf("intel %q %s", rec.ID, intelRevealsNothing))
		}
	}

	return declared
}

// bindingHolds refuses a holder naming a record this document does not
// declare — [validation.place]'s refusal for [PlaceSpec.Holds], at the
// binding's own path and naming the binding's own id (the thing an author is
// looking at here is a creature or a prop by name, never a ref).
func bindingHolds(g *grammar, path, id string, holds []string, declared map[string]bool) {
	for j, rec := range holds {
		if declared[rec] {
			continue
		}
		g.fail(fmt.Sprintf("%s.holds[%d]", path, j),
			"%q holds intel %q, and no record in this dungeon has that id", id, rec)
	}
}

// bindingChecks refuses an authored `intimidate:` or `persuade:` with no way
// through it — [validation.place]'s two sentences, judged by the one shared
// [grammar.approaches] so every per-approach refusal is v2's as well.
//
// NIL IS NOT EMPTY, and that is the whole shape of this pair: a nil check is
// an author who said nothing, and the rulebook derives the DC from the stat
// block's passive Insight; `intimidate: []` is an author who said there IS a
// price and did not say what it is. [RoomDoorBinding.Locked]'s law, one
// binding kind over.
func bindingChecks(g *grammar, path string, b RoomMonsterBinding) {
	if b.Intimidate != nil {
		g.approaches(path+".intimidate",
			"this monster declares an intimidate check with no way through it — an ability and a DC",
			b.Intimidate)
	}
	if b.Persuade != nil {
		g.approaches(path+".persuade",
			"this monster declares a persuade check with no way through it — an ability and a DC",
			b.Persuade)
	}
}

// singleRoomArrivals validates every `arrives:` this document authored —
// [validation.arrivals] in this dialect, and in the same three parts: the
// predicate is one, a thing cannot wait on its own fall, and a RING of
// reserved things never arrives because each waits for a fall that cannot
// happen until it has arrived itself.
//
// ONLY CREATURES CAN BE IN A RING, and the graph is built from the monster
// bindings alone. A `{ down }` names a member and only a creature is one
// ([grammar.predicate] refuses the rest), so a prop can WAIT on a ring but
// can never be in one — which is the case v2's own walk reports as "a ring
// this placement leads to but is not in; its own members report it".
//
// RUN AFTER the factions and dispositions are indexed: a `{ stance }` is
// judged against the whole table.
func singleRoomArrivals(gp *RoomGameplaySource, g *grammar) {
	waitsOn := map[string]string{}
	for _, id := range sortedBindingIDs(gp.MonsterBindings) {
		b := gp.MonsterBindings[id]
		// A binding whose creature is gone is named once by [siteGrammar] and
		// not asked a second question here, for [bindingHolds]' reason.
		if _, live := g.members.indexOf(id); !live || b.Arrives == nil {
			continue
		}
		p := "room.room.monsterBindings." + id + ".arrives"
		g.predicate(p, b.Arrives, nil)
		if b.Arrives.Form() != predicateDown {
			continue
		}
		if b.Arrives.Down == id {
			g.fail(p+".down", "%q cannot wait for its own fall — it is not here to fall until it arrives", id)
			continue
		}
		// An edge only when the fall it waits for is itself reserved: a
		// creature standing there from the first frame can fall like anybody,
		// and a `{ down }` the check above already refused is not an edge.
		if _, live := g.members.indexOf(b.Arrives.Down); !live {
			continue
		}
		if waited, bound := gp.MonsterBindings[b.Arrives.Down]; bound && waited.Arrives != nil {
			waitsOn[id] = b.Arrives.Down
		}
	}
	reportArrivalRings(waitsOn, g)

	for _, id := range sortedPropBindingIDs(gp.PropBindings) {
		if a := gp.PropBindings[id].Arrives; a != nil {
			g.predicate("room.room.propBindings."+id+".arrives", a, nil)
		}
	}
}

// reportArrivalRings names every member of every cycle in the waits-for
// graph, at each member's own path — [validation.arrivals]' walk, keyed by id
// rather than by index because this dialect's bindings are a map.
//
// Walked from every reserved creature in SORTED order, so a file with a ring
// in it reports the same list every run (C8).
func reportArrivalRings(waitsOn map[string]string, g *grammar) {
	starts := make([]string, 0, len(waitsOn))
	for id := range waitsOn {
		starts = append(starts, id)
	}
	sort.Strings(starts)
	for _, id := range starts {
		seen := map[string]bool{id: true}
		for at := waitsOn[id]; ; {
			next, waits := waitsOn[at]
			if !waits {
				break
			}
			if next == id {
				g.fail("room.room.monsterBindings."+id+".arrives.down",
					"%q waits for %q to fall, and %q is waiting to arrive on a fall of its own that leads "+
						"back here — none of them can ever arrive", id, at, at)
				break
			}
			if seen[next] {
				break // a ring this creature leads to but is not in; its own members report it
			}
			seen[next] = true
			at = next
		}
	}
}

// propBindingValues judges every prop this room gives orders to: that the id
// names a prop that can take orders at all, and that what it holds and waits
// for is legal.
//
// [doorBindingValues]' order and its reason. A binding that names an
// arrangement template, a door, or nothing at all has no prop to talk about,
// so its OWNERSHIP is reported and the declaration is not looked at; its own
// fields are judged either way, because they are wrong wherever the prop ends
// up. Sorted, for [sortedBindingIDs]' reason.
//
// The `arrives:` predicate is judged by [singleRoomArrivals] rather than
// here, because a `{ stance }` needs the disposition table and this runs
// before it.
func propBindingValues(gp *RoomGameplaySource, g *grammar, declared map[string]bool) {
	templates := arrangementTemplateIDs(gp.ArrangementDeclarations)
	for _, id := range sortedPropBindingIDs(gp.PropBindings) {
		p := "room.room.propBindings." + id
		binding := gp.PropBindings[id]

		_, isProp := gp.PropDeclarations[id]
		_, isDoor := gp.DoorBindings[id]
		switch {
		case isDoor:
			g.fail(p, "%s", propIsADoor)
		case !isProp && templates[id]:
			g.fail(p, "%s", propInArrangement)
		case !isProp:
			g.fail(p, "%s", propNeedsDeclaration)
		}

		bindingHolds(g, p, id, binding.Holds, declared)
	}
}

// sortedPropBindingIDs orders the prop orders' ids, for [sortedBindingIDs]'
// reason: a defect list that depends on Go's map iteration is one no author
// can compare run to run.
func sortedPropBindingIDs(bindings map[string]RoomPropBinding) []string {
	out := make([]string, 0, len(bindings))
	for id := range bindings {
		out = append(out, id)
	}
	sort.Strings(out)

	return out
}

// # The lowering

// propBindingRefusals is what [CompileSingleRoom] answers for a document that
// gives a prop orders: one defect per binding, at its own path, naming the
// primitive that is missing (rpg-toolkit#1854).
//
// NOT CARRIED INERT, and that is the point. The alternative is a compile that
// accepts `holdable: true` and produces a world where nothing can be picked
// up — a key the engine reads and drops, which is the fail-silent this
// repository refuses. Nil for a document that binds no prop, so every room
// authored before this key existed compiles exactly as it did.
func propBindingRefusals(gp *RoomGameplaySource) []FieldError {
	var out []FieldError
	for _, id := range sortedPropBindingIDs(gp.PropBindings) {
		out = append(out, FieldError{Path: "room.room.propBindings." + id, Message: propBindingNoPrimitive})
	}

	return out
}
