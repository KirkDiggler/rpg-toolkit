// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"gopkg.in/yaml.v3"
)

// single_room_doors.go is A DOOR IN THE SINGLE-ROOM DIALECT (rpg-project#485;
// rpg-toolkit#1850) — the `doorBindings:` block, the refusals it earns, and
// the one thing it compiles to.
//
// # A door here is a prop plus a state
//
// The v2 dialect's door is a POSITION ON A WALL. This dialect has no walls —
// Kirk, from inside the World Builder: "walls are currently just props that
// block los and movement. Placing a door 'in' a wall no longer makes sense" —
// so the v2 geometry is not unused here, it is inapplicable.
//
// What this dialect does have is the prop precedent, and a door uses all of
// it. The door's SHAPE is its `propDeclarations` entry, lowered by the same
// [placedPropFrom] every table and barrel goes through; its STATE is the
// binding below; and what its footprint blocks follows that state rather than
// the declaration's two flags. One door grammar, two geometries: an edge in
// v2, a footprint here.
//
// # State wins, which is why the flags are refused (R3)
//
// A fresh declaration asserts nothing — the World Builder seeds
// `blocksMovement: false` — and that is exactly right for a door, because
// the door's state is what decides. Setting either flag TRUE on a door item
// is the collision rpg-toolkit#1846 raised, ruled the World Builder lane's
// way: it would build a wall that never opens, since nothing consults door
// state to clear an authored flag. So a `true` is refused at its own path,
// and a `false` is the honest "this declaration asserts nothing" it already
// was.
//
// # What a door may not be yet, and why each one says so
//
// A door with no `propDeclarations` entry has no shape at all. A door id that
// names an ARRANGEMENT template is a door inside a stamp, which nothing
// remaps yet (R5). Each is refused by name, in the shape
// [singleRoomCellSelector] takes: what is wrong, what to write instead, and
// what would make it legal.
//
// `concealed` USED TO BE THE THIRD ONE, and is not any more
// (rpg-project#490). A hidden rectangle in the middle of a room was the
// picture question the builder had not asked; the concealment primitive is
// the answer, and the word moved to the root — a door is hidden by being
// listed in a `concealments.<id>.props`. It is not a key on this block at
// all now, so writing it earns the unknown-key refusal with the keys this
// block does take.

// The three refusals this dialect adds, verbatim. Constants so the same
// defect always reports the same words, and so a test can pin the sentence an
// author reads rather than a paraphrase of it.
const (
	// doorNeedsFootprint is a binding whose item declares no shape. The
	// door's geometry IS its prop declaration, so this one is not a missing
	// field, it is a door that does not exist anywhere.
	doorNeedsFootprint = "a door needs a footprint: declare it under propDeclarations"

	// doorInArrangement is a binding that names an arrangement TEMPLATE
	// rather than a placed item. The prop path remaps a stamped template's
	// id to the live item's; a door has no remap yet, and inventing one
	// here would be this slice deciding what a stamped door's state belongs
	// to.
	doorInArrangement = "a door inside an arrangement is not something this build stamps yet: " +
		"this id names an arrangement template, so declare the door on a placed item of its own, " +
		"or wait for the arrangement door"

	// doorStateDecidesBlocking is either blocking flag authored TRUE on a
	// door item (R3, rpg-toolkit#1846).
	doorStateDecidesBlocking = "a door's state decides what it blocks, and this would be a wall that never opens: " +
		"leave it false and say `closed` under doorBindings"
)

// # Source shape

// doorBindingsShape reads the optional door bindings, keyed by item id. What
// it judges is what the typed decode cannot: an authored `null` under a key
// that would otherwise read as the Go zero value — a binding that declares
// nothing, a lock that was never written. [monsterBindingsShape]'s reasons,
// one declaration kind over.
func doorBindingsShape(gp *yaml.Node, add errSink) {
	bindings := optionalNode(gp, "doorBindings", "room.room", add)
	if bindings == nil || bindings.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(bindings.Content); i += 2 {
		p := "room.room.doorBindings." + bindings.Content[i].Value
		b := resolveNode(bindings.Content[i+1])
		if b == nil || isNull(b) {
			add(p, errNotNull)
			continue
		}
		if b.Kind != yaml.MappingNode {
			continue
		}
		// `closed` is a state, not an unanswered question: LEAVING IT OUT
		// is an open doorway ([DoorSpec.Closed]'s own rule), and writing
		// null is an author who deleted the word — the distinction every
		// optional key in this document keeps.
		optionalNode(b, "closed", p, add)
		optionalNode(b, "locked", p, add)
		optionalNode(b, "concealed", p, add)
	}
}

// # Decoded values

// doorBindingValues judges every door this room declares: that the thing it
// names can be a door at all, that its declaration leaves the blocking to the
// state, and that the state itself is legal.
//
// The order is the order an author fixes them in. A binding that names an
// arrangement template or nothing at all has no door to talk about, so its
// geometry is reported and its declaration is not looked at; the STATE keys
// are judged either way, because they are this binding's own fields and wrong
// wherever the door ends up.
//
// Sorted, for [sortedBindingIDs]' reason: a defect list that depends on Go's
// map iteration is one no author can compare run to run.
func doorBindingValues(gp *RoomGameplaySource, g *grammar, add errSink) {
	templates := arrangementTemplateIDs(gp.ArrangementDeclarations)
	for _, id := range sortedDoorIDs(gp.DoorBindings) {
		p := "room.room.doorBindings." + id
		binding := gp.DoorBindings[id]

		decl, declared := gp.PropDeclarations[id]
		switch {
		case !declared && templates[id]:
			add(p, doorInArrangement)
		case !declared:
			add(p, doorNeedsFootprint)
		default:
			// STATE WINS (R3). The declaration's own path, because that is
			// the field the World Builder draws the refusal on.
			declPath := "room.room.propDeclarations." + id
			if decl.BlocksMovement != nil && *decl.BlocksMovement {
				add(declPath+".blocksMovement", doorStateDecidesBlocking)
			}
			if decl.BlocksLineOfSight != nil && *decl.BlocksLineOfSight {
				add(declPath+".blocksLineOfSight", doorStateDecidesBlocking)
			}
		}

		// AND THE STATE ITSELF, by the one shared grammar: the lock a v2
		// door gets, with v2's sentences, at this dialect's path. Whether
		// the door is HIDDEN is not a door key in either dialect's engine
		// any more, so nothing is handed on beside the lock.
		g.doorState(p, binding.Locked, nil)
	}
}

// arrangementTemplateIDs is every template id any arrangement declares — the
// set a door binding may NOT name (R5). Built from the declarations rather
// than from the scene, because an arrangement is a template block and its ids
// need never have been stamped at all.
func arrangementTemplateIDs(arrangements map[string]map[string]RoomPropDeclaration) map[string]bool {
	out := map[string]bool{}
	for _, templates := range arrangements {
		for id := range templates {
			out[id] = true
		}
	}

	return out
}

// sortedDoorIDs orders the door bindings' ids, for [sortedBindingIDs]' reason.
func sortedDoorIDs(bindings map[string]RoomDoorBinding) []string {
	out := make([]string, 0, len(bindings))
	for id := range bindings {
		out = append(out, id)
	}
	sort.Strings(out)

	return out
}

// # The lowering

// singleRoomDoors is what this dialect compiles its doors to: one
// [encounter.DoorInput] per binding, standing as the FOOTPRINT its prop
// declaration draws and in the state the binding declares.
//
// THE GEOMETRY IS THE PROP'S, converted by the one adapter
// (single_room_placement.go): the door's rectangle and the table's go through
// [placedPropFrom] identically, and only the blocking differs — a prop's is
// two authored flags, a door's is its state. Nothing here re-derives a pose
// and nothing scales twice.
//
// THE ID IS V2'S MINTING, `<key>/<item id>`, so two dungeons in one process
// cannot collide and so the web can derive it from the item id and the
// dungeon key it already fetched by (R4). Sorted by id (C8).
//
// Only reachable for a validated document: every binding here has a
// declaration and a pose, both refused by name above when it does not.
func singleRoomDoors(key string, gp *RoomGameplaySource, read roomRead) []encounter.DoorInput {
	var out []encounter.DoorInput
	for _, id := range sortedDoorIDs(gp.DoorBindings) {
		decl, declared := gp.PropDeclarations[id]
		pose, posed := read.Poses[id]
		if !declared || !posed {
			continue
		}
		binding := gp.DoorBindings[id]
		placement := placedFootprintFrom(decl, pose)
		out = append(out, encounter.DoorInput{
			ID:        encounter.DoorID(key + "/" + id),
			Placement: &placement,
			State:     doorStateOf(binding.Locked, binding.Closed),
		})
	}

	return out
}
