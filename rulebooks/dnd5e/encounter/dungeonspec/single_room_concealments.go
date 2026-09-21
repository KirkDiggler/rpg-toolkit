// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// single_room_concealments.go is THE ROOT `concealments:` KEY IN THE
// SINGLE-ROOM DIALECT (rpg-project#490, R1/R2/R4/R7).
//
// # Nothing here is a second dialect
//
// The shape is [ConcealmentSpec] and the checks are judged by the one shared
// [grammar.approaches], at THIS dialect's paths. What this file writes is the
// dialect's half: the frame its cells are authored in (axial, like every
// other cell here), the universe its `props:` may name (`propDeclarations`),
// and the sentences an author reads at `concealments.<id>.…`.
//
// # What a concealment may not be, and why each one says so
//
// An unknown cell is a secret drawn on floor the room does not have. An
// unknown placed id is a secret hiding something that does not exist. A cell
// or an id in TWO concealments is the overlap R4 refuses, and the refusal
// NAMES BOTH so an author can see which two collide. No checks is a secret
// nobody can ever find; hiding nothing is a secret with nothing in it. Each
// is reported at the author's own path.

// The sentences this file adds, verbatim. Constants for
// single_room_doors.go's reason: the same defect always reports the same
// words, and a test pins the sentence an author reads rather than a
// paraphrase of it.
const (
	// concealmentNoChecks is a secret with no way in.
	concealmentNoChecks = "this concealment needs at least one way to find it — an ability and a DC under `checks`"

	// concealmentNoNotice is `notice: []` — [RoomDoorBinding.Locked]'s
	// nil-vs-empty law on the passive tell.
	concealmentNoNotice = "this concealment declares a notice with no way through it — an ability and a DC"

	// concealmentHidesNothing is a secret with no cells and no props.
	concealmentHidesNothing = "this concealment hides nothing: list the hexes under `cells`, " +
		"the placed things under `props`, or both"

	// concealmentUnknownCell is a cell the room's floor does not have.
	concealmentUnknownCell = "a concealment hides floor this room has: name a hex from walkableHexes"

	// concealmentUnknownProp is an id no `propDeclarations` entry owns.
	concealmentUnknownProp = "a concealment hides things this room places: declare it under propDeclarations"
)

// # Source shape

// concealmentsShape reads the optional root concealments, keyed by id.
// [intelShape]'s reasons: the typed decode refuses a value of the wrong KIND
// outright, but reads an authored `null` as the Go zero value without a word
// — a concealment the author emptied, a `checks:` they deleted. Whether a
// REQUIRED key is present is deliberately not asked here, because
// [singleRoomConcealmentValues] says it in an author's own words and two
// defects for one mistake sends them looking for a second problem.
func concealmentsShape(doc *yaml.Node, add errSink) {
	concealments := optionalNode(doc, "concealments", "", add)
	if concealments == nil || concealments.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(concealments.Content); i += 2 {
		p := "concealments." + concealments.Content[i].Value
		c := resolveNode(concealments.Content[i+1])
		if c == nil || isNull(c) {
			add(p, errNotNull)
			continue
		}
		if c.Kind != yaml.MappingNode {
			add(p, errNotAMapping)
			continue
		}
		optionalNode(c, "notice", p, add)
		optionalNode(c, "checks", p, add)
		optionalNode(c, "props", p, add)
		// THE CELLS ARE WALKED, unlike the two lists above, because a cell is
		// a MAPPING with two integer scalars in it and yaml.v3 would truncate
		// `q: 0.5` into the Go int 0 without a word — [cellShape]'s own
		// reason, at every other authored cell in this document.
		if cells := optionalNode(c, "cells", p, add); cells != nil {
			for j, cell := range cells.Content {
				cellShape(cell, fmt.Sprintf("%s.cells[%d]", p, j), add)
			}
		}
	}
}

// # Decoded values

// singleRoomConcealmentValues judges every secret this room declares, and
// hands back the ids it declared — the universe an `intel[<i>].reveals.
// concealment` may name.
//
// RUN AFTER the floor and the declarations are in hand, because every
// refusal below is about one of them. Sorted, for [sortedBindingIDs]' reason:
// a defect list that depends on Go's map iteration is one no author can
// compare run to run.
func singleRoomConcealmentValues(s *SingleRoomSpec, g *grammar, add errSink) map[string]bool {
	gp := &s.Room.Gameplay
	walkable := make(map[RoomCell]bool, len(gp.WalkableHexes))
	for _, c := range gp.WalkableHexes {
		walkable[c] = true
	}

	declared := make(map[string]bool, len(s.Concealments))
	cellAt := map[RoomCell]string{}
	propAt := map[string]string{}

	for _, id := range sortedConcealmentIDs(s.Concealments) {
		p := "concealments." + id
		c := s.Concealments[id]
		declared[id] = true

		if c.Checks == nil {
			add(p+".checks", concealmentNoChecks)
		} else {
			g.approaches(p+".checks", concealmentNoChecks, c.Checks)
		}
		if c.Notice != nil {
			g.approaches(p+".notice", concealmentNoNotice, c.Notice)
		}
		if len(c.Cells) == 0 && len(c.Props) == 0 {
			add(p, concealmentHidesNothing)
		}

		for j, cell := range c.Cells {
			at := fmt.Sprintf("%s.cells[%d]", p, j)
			if !walkable[cell] {
				add(at, concealmentUnknownCell)
				continue
			}
			// NAMING BOTH LINES is R4's own requirement: an author looking at
			// one of two colliding declarations cannot fix it without being
			// told which other one it collides with.
			if owner, taken := cellAt[cell]; taken {
				add(at, fmt.Sprintf(
					"this hex is already hidden by concealment %q, and a hex belongs to one concealment", owner))
				continue
			}
			cellAt[cell] = id
		}

		for j, named := range c.Props {
			at := fmt.Sprintf("%s.props[%d]", p, j)
			if _, isProp := gp.PropDeclarations[named]; !isProp {
				add(at, concealmentUnknownProp)
				continue
			}
			if owner, taken := propAt[named]; taken {
				add(at, fmt.Sprintf(
					"%q is already hidden by concealment %q, and a placed thing belongs to one concealment",
					named, owner))
				continue
			}
			propAt[named] = id
		}
	}

	return declared
}

// sortedConcealmentIDs orders the concealments' ids, for
// [sortedBindingIDs]' reason.
func sortedConcealmentIDs(concealments map[string]ConcealmentSpec) []string {
	out := make([]string, 0, len(concealments))
	for id := range concealments {
		out = append(out, id)
	}
	sort.Strings(out)

	return out
}

// # The lowering

// singleRoomConcealments is what this dialect compiles its secrets to: one
// [encounter.ConcealmentInput] per authored concealment, in sorted id order
// (C8 — a map's iteration order is not a thing a compiled field may depend
// on).
//
// THE AUTHOR WRITES ONE LIST OF PLACED IDS AND THE ENGINE TAKES TWO. A door
// is a prop in this dialect, so `props:` carries both and the split is asked
// of `doorBindings` here rather than of the author twice. A door's id is
// minted `<key>/<id>` because that is what [singleRoomDoors] mints; a prop's
// is the bare item id, because that is what [placedPropFrom] mints. Two
// namespaces, each spelled the way its own list spells it.
//
// THE ID IS `<key>/<id>`, the minting a door and an intel record already use,
// so two dungeons in one process cannot collide.
//
// Only reachable for a validated document: every cell is walkable and every
// id is declared, both refused by name above when they are not.
func singleRoomConcealments(
	key string, s *SingleRoomSpec, o spatial.HexOrientation,
) []encounter.ConcealmentInput {
	gp := &s.Room.Gameplay

	var out []encounter.ConcealmentInput
	for _, id := range sortedConcealmentIDs(s.Concealments) {
		c := s.Concealments[id]
		lowered := encounter.ConcealmentInput{
			ID:     encounter.ConcealmentID(key + "/" + id),
			Checks: approachesOf(c.Checks),
			Notice: approachesOf(c.Notice),
		}
		for _, cell := range c.Cells {
			lowered.Cells = append(lowered.Cells, axialOffset(cell, o))
		}
		for _, named := range c.Props {
			if _, isDoor := gp.DoorBindings[named]; isDoor {
				lowered.Doors = append(lowered.Doors, encounter.DoorID(key+"/"+named))
				continue
			}
			lowered.Props = append(lowered.Props, named)
		}
		out = append(out, lowered)
	}

	return out
}

// singleRoomConcealmentOf resolves what a record reveals in this dialect: the
// authored `concealment:` id, minted the way [singleRoomConcealments] mints
// it, or the empty string for a record that reveals a fact instead.
//
// A CLOSURE OVER THE KEY rather than a map, because the minting is one line
// and a map would be a second copy of it to keep in step.
func singleRoomConcealmentOf(key string) func(RevealsSpec) encounter.ConcealmentID {
	return func(rev RevealsSpec) encounter.ConcealmentID {
		if rev.Concealment == "" {
			return ""
		}

		return encounter.ConcealmentID(key + "/" + rev.Concealment)
	}
}
