// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// v4_run_test.go is THE PRIZE AND THE WAY OUT, PROVED TWICE (rpg-project#488,
// slice 1) — the v4 dialect's `exits:`, `endings:` and `scenarios:`.
//
// # The claim, and how it is tested
//
// Each of the three is a v2 root key reaching this dialect unchanged, so the
// proof is v4_gameplay_test.go's: author the SAME gameplay in both dialects,
// compile both, and require the fields the slice adds EQUAL. Populating a
// field proves nothing on its own.
//
// ONE OF THE THREE CARRIES A CELL, and that is the only half either dialect
// owns (R2): v2 writes an absolute `at: [col, row]` and this dialect writes
// axial `cell: {q, r}`. Both fixtures below therefore author their exit at
// the axial cell that LOWERS to the offset cell the v2 file names, which is
// what makes `Field.Exits` comparable at all — and what the comparison
// proves is that the two frames land in one place.
//
// WHAT IS DELIBERATELY NOT COMPARED IS GEOMETRY, for v4_gameplay_test.go's
// reason: v2 paints regions and walls them, this dialect has one room and no
// walls, and comparing whole worlds would be a test that cannot pass.
//
// NO COLLECTION LENGTH IS PINNED anywhere here.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// # The heirloom tomb — the way out, and what the room is for

// The tomb's way out compiles identically in both dialects: the same id, and
// the same cell, reached from two different authored frames.
//
// THE CELL IS THE POINT OF THIS PAIR. v2 writes `at: [1,3]` and the v4 twin
// writes `cell: {q: 0, r: 3}`; each dialect spends its own conversion and
// both arrive at the one offset cell [encounter.FieldExit] takes. A frame
// that drifted would show up here as two exits in two places, which is
// exactly the failure R2's "an exit has partyStart's shape" is a ruling
// about.
func TestTheHeirloomTombDeclaresTheSameWayOutInBothDialects(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-tomb-heirloom.yaml")
	v4 := loadContent(t, "testdata/world-builder-v4-tomb-heirloom.yaml")

	require.Equal(t, []encounter.FieldExit{{ID: "entrance", At: offsetCell(1, 3)}}, v2.Field.Exits,
		"the v2 tomb's way out, at the cell the party comes in on")
	require.Equal(t, v2.Field.Exits, v4.Field.Exits,
		"and the v4 twin's, from axial {q: 0, r: 3}")

	// THE WAY OUT RIDES THE FIELD, which is the copy that survives being
	// stored: an exit kept only on Compiled would be lost the moment the
	// dungeon was saved, and a live session could never answer for it.
	require.Equal(t, v4.Field.Exits[0].At, v4.PartyStart[0].At,
		"this tomb's entrance is also its exit, and the two cells agree")
}

// And its scenario binding is carried through exactly as authored, in both.
//
// CARRIED, NOT INTERPRETED. `artifact` and `exit` mean nothing to this
// package: what it checked is that `heirloom` and `entrance` name something
// the document declares, and what the two KEYS mean is the scenario
// package's own question (design law C1).
func TestTheHeirloomTombBindsTheSameScenarioInBothDialects(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-tomb-heirloom.yaml")
	v4 := loadContent(t, "testdata/world-builder-v4-tomb-heirloom.yaml")

	require.Equal(t, map[string]map[string]string{
		"recover-the-artifact": {"artifact": "heirloom", "exit": "entrance"},
	}, v2.Scenarios, "the v2 tomb binds the artifact and the way out")
	require.Equal(t, v2.Scenarios, v4.Scenarios, "and the v4 twin binds the same two")

	// THE PRIZE IS A PLACED PROP IN THIS DIALECT, and the binding names it by
	// the id its declaration owns — which is what makes `artifact: heirloom`
	// resolvable at all here, where v2 resolves it against a `place:` entry.
	var heirloom encounter.PlacedPropInput
	for _, p := range v4.Field.Placed {
		if p.ID == "heirloom" {
			heirloom = p
		}
	}
	require.Equal(t, encounter.PropID("heirloom"), heirloom.ID, "the binding names a prop this room places")
	require.True(t, heirloom.Holdable, "and one the party can carry out")
}

// A binding is DEEP-COPIED out of the spec, so a host mutating the map it is
// handed cannot reach back into the document it was compiled from.
func TestAScenarioBindingCannotBeMutatedBackIntoTheSpec(t *testing.T) {
	first := loadContent(t, "testdata/world-builder-v4-tomb-heirloom.yaml")
	first.Scenarios["recover-the-artifact"]["artifact"] = "something-else"

	again := loadContent(t, "testdata/world-builder-v4-tomb-heirloom.yaml")
	require.Equal(t, "heirloom", again.Scenarios["recover-the-artifact"]["artifact"],
		"the document still binds what it authored")
}

// # The raider camp — the gate, the binding, and an ending of its own

// The camp's way out and its scenario binding compile identically in both
// dialects, for the tomb's reasons one file over.
func TestTheCampDeclaresTheSameWayOutAndBindingInBothDialects(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-raider-camp.yaml")
	v4 := loadContent(t, "testdata/world-builder-v4-raider-camp.yaml")

	require.Equal(t, []encounter.FieldExit{{ID: "front-gate", At: offsetCell(1, 3)}}, v2.Field.Exits,
		"the v2 camp's gate")
	require.Equal(t, v2.Field.Exits, v4.Field.Exits, "and the v4 twin's, from axial {q: 0, r: 3}")

	// A FACTION IS BINDABLE, which is what `convince` needs: the hold-out
	// scenario is about a SIDE standing down, and nothing else in either
	// document is named `raiders`.
	require.Equal(t, map[string]map[string]string{"hold-out": {"convince": "raiders"}}, v2.Scenarios)
	require.Equal(t, v2.Scenarios, v4.Scenarios)
}

// heldOutInV2 is the smallest v2 document authoring the camp's ending —
// written here rather than added to the reference camp, which declares none
// of its own: `hold-out` is a SCENARIO's ending there, and this slice is
// about the key a DOCUMENT can author.
const heldOutInV2 = `
version: 2
key: held-out-in-v2
name: Held Out
orientation: pointy
void: opaque
regions:
  - id: yard
    name: The Yard
    archetype: crypt
    lighting: { intensity: 0.6 }
    cells:
      - [[0,0],[1,0],[2,0]]
      - [[0,1],[1,1],[2,1]]
start: [0, 0]
endings:
  - { id: held-out, when: { round: 6 } }
`

// An ending a v4 document authors compiles to the same [encounter.EndingInput]
// the same line compiles to in v2 — one predicate grammar, one [predicateOf],
// two dialects.
func TestARootEndingCompilesTheSameInBothDialects(t *testing.T) {
	v2 := loadSource(t, heldOutInV2)
	v4 := loadContent(t, "testdata/world-builder-v4-raider-camp.yaml")

	require.Equal(t, []encounter.EndingInput{{Key: "held-out", Trigger: encounter.TriggerRound{Round: 6}}},
		v2.Endings, "the v2 document's ending")
	require.Equal(t, v2.Endings, v4.Endings, "and the v4 camp's, from the same authored line")
}

// A document that authors none of the three compiles to none of them, which
// is what keeps every room written before this slice picturing as it did.
func TestADocumentThatAuthorsNoneOfThemCarriesNone(t *testing.T) {
	v4 := loadContent(t, "testdata/world-builder-v4-site.yaml")

	require.Nil(t, v4.Field.Exits, "no way out was authored")
	require.Nil(t, v4.Endings, "and no ending")
	require.Nil(t, v4.Scenarios, "and nothing is bound — nil rather than an empty map")
}

// # The refusals, each at its exact path

// An exit is refused as v2 refuses one: an id it must have, and an id no two
// may share.
func TestAnExitIsRefusedAsV2RefusesOne(t *testing.T) {
	t.Run("no id", func(t *testing.T) {
		errs := refusals(t, v4With("exits:\n  - { cell: { q: 0, r: 0 } }", ""))
		requireExactDefect(t, errs, "exits[0].id", "the exit has no id")
	})

	t.Run("two ways out under one id", func(t *testing.T) {
		errs := refusals(t, v4With(
			"exits:\n  - { id: entrance, cell: { q: 0, r: 0 } }\n  - { id: entrance, cell: { q: 1, r: 0 } }", ""))
		requireExactDefect(t, errs, "exits[1].id", `exit "entrance" is already declared at exits[0]`)
	})
}

// An exit's CELL is answered by this dialect's own standability — the one
// `partyStart` and every monster cell already go through — at the exit's own
// path and in the author's own axial frame (R2).
//
// THE SENTENCES ARE THIS DIALECT'S, NOT v2's, and that is the one place the
// two rules could not be one function. v2 says "which is scenery" and "where
// <wall> leaves no room to stand", and this dialect has neither scenery nor
// walls: a room is the hexes its author painted, and what takes a cell away
// is a footprint standing on it.
func TestAnExitNobodyCanStandOnIsRefused(t *testing.T) {
	t.Run("a cell nobody painted", func(t *testing.T) {
		errs := refusals(t, v4With("exits:\n  - { id: entrance, cell: { q: 3, r: 3 } }", ""))
		requireExactDefect(t, errs, "exits[0].cell", "at author's axial q=3 r=3: is not standable")
	})

	t.Run("a cell a footprint fills", func(t *testing.T) {
		errs := refusals(t, theBlockedRoom("exits:\n  - { id: entrance, cell: { q: 0, r: 1 } }"))
		requireExactDefect(t, errs, "exits[0].cell",
			`at author's axial q=0 r=1: is occupied by footprint "table"`)
	})
}

// theBlockedRoom is a room whose one declared prop BLOCKS MOVEMENT and whose
// footprint covers the cell `{q: 0, r: 1}` — the fixture the standability
// refusal needs, since [v4With]'s table is deliberately non-blocking.
func theBlockedRoom(root string) string {
	return fmt.Sprintf(`
version: 4
key: blocked-fixture
play: { void: transparent, lighting: bright, standing: centre-covered }
%s
room:
  version: 3
  id: room-1
  name: A Room
  coordinateFrame: { hexRadius: 1 }
  workspace: { hexRadius: 6 }
  scene:
    version: 1
    id: scene-1
    name: A Room
    items:
      - { id: table, kind: prop, transform: { x: 0, y: 0, z: 2, rotationY: 0 } }
    groups: []
  room:
    implicitRegionId: room-1-region
    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0}, {q: 0, r: 1}, {q: 1, r: 1}]
    propDeclarations:
      table: { blocksMovement: true, blocksLineOfSight: false, footprint: { width: 2, depth: 2, offsetX: 0, offsetZ: 0 } }
    arrangementDeclarations: {}
    partyStart: { q: 0, r: 0 }
    monsters: []
`, root)
}

// An ending that does not say when it fires is one nothing can fire, and
// nothing is defaulted.
func TestAnEndingThatNeverSaysWhenIsRefused(t *testing.T) {
	errs := refusals(t, v4With("endings:\n  - { id: held-out }", ""))
	requireExactDefect(t, errs, "endings[0].when",
		"the ending does not say when it fires — a predicate is exactly one of { round: N }, "+
			"{ down: <placement id> }, { fact: <id> }, or "+
			"{ stance: { between: [a, b], is: hostile|neutral|allied } }")
}

// An ending nobody can reach is refused by the one predicate grammar, which
// is the same liveness rule an `until` and an `arrives` meet.
func TestAnEndingNobodyCanReachIsRefused(t *testing.T) {
	t.Run("a round counted from zero", func(t *testing.T) {
		errs := refusals(t, v4With("endings:\n  - { id: held-out, when: { round: 0 } }", ""))
		requireExactDefect(t, errs, "endings[0].when.round", "round 0: a round is counted from 1")
	})

	t.Run("a stance the pair can never hold", func(t *testing.T) {
		errs := refusals(t, v4With(
			"factions:\n  - { id: raiders }\n"+
				"dispositions:\n  - { between: [raiders, party], stance: hostile }\n"+
				"endings:\n  - { id: turned, when: { stance: { between: [raiders, party], is: neutral } } }", ""))
		requireExactDefect(t, errs, "endings[0].when.stance",
			"party and raiders can never be neutral: they are hostile, and nothing turns them")
	})

	// AND TWO ENDINGS UNDER ONE ID is the same refusal an exit and a record
	// get: what the `ended` beat names has to name one thing.
	t.Run("two endings under one id", func(t *testing.T) {
		errs := refusals(t, v4With(
			"endings:\n  - { id: held-out, when: { round: 6 } }\n  - { id: held-out, when: { round: 9 } }", ""))
		requireExactDefect(t, errs, "endings[1].id", `ending "held-out" is already declared at endings[0]`)
	})
}

// A binding that names nothing at all is a dangling reference whatever
// scenario reads it.
func TestAScenarioBindingToNothingIsRefused(t *testing.T) {
	errs := refusals(t, v4With("scenarios:\n  recover-the-artifact:\n    artifact:", ""))
	requireExactDefect(t, errs, "scenarios.recover-the-artifact.artifact",
		`scenario "recover-the-artifact" binds artifact to nothing`)
}

// And one that names an id this document does not declare is refused by name.
//
// THE UNIVERSE IS THIS DIALECT'S FOUR KINDS: a placed creature, a declared
// prop, an exit, or a faction. An ARRANGEMENT TEMPLATE is not one of them —
// nothing stamps one yet — and the second case below is what says so.
func TestAScenarioBindingToNothingInThisDocumentIsRefused(t *testing.T) {
	t.Run("an id nothing declares", func(t *testing.T) {
		errs := refusals(t, v4With("scenarios:\n  recover-the-artifact: { artifact: nowhere }", ""))
		requireExactDefect(t, errs, "scenarios.recover-the-artifact.artifact",
			`scenario "recover-the-artifact" binds artifact to "nowhere", and nothing in this dungeon has that id`)
	})

	t.Run("an id inside an arrangement template", func(t *testing.T) {
		errs := refusals(t, v4With("scenarios:\n  recover-the-artifact: { artifact: stamped-chair }", ""))
		requireExactDefect(t, errs, "scenarios.recover-the-artifact.artifact",
			`scenario "recover-the-artifact" binds artifact to "stamped-chair", `+
				"and nothing in this dungeon has that id")
	})
}

// Every kind of id this document declares IS bindable, and the compile
// carries each through untouched — the other half of the refusal above,
// without which "nothing has that id" could be true of everything.
func TestEveryKindOfIDThisDocumentDeclaresIsBindable(t *testing.T) {
	compiled := loadSource(t, v4With(
		"factions:\n  - { id: raiders }\n"+
			"exits:\n  - { id: entrance, cell: { q: 0, r: 0 } }\n"+
			"scenarios:\n  every-kind:\n    exit: entrance\n    artifact: table\n"+
			"    quarry: goblin-1\n    convince: raiders", ""))

	require.Equal(t, map[string]string{
		"exit":     "entrance",
		"artifact": "table",
		"quarry":   "goblin-1",
		"convince": "raiders",
	}, compiled.Scenarios["every-kind"],
		"an exit, a declared prop, a placed creature and a faction all bind")
}

// offsetCell is one authored cell in the offset frame [encounter.FieldExit]
// takes, written the way a v2 file writes it.
func offsetCell(col, row int) spatial.Position {
	return spatial.Position{X: float64(col), Y: float64(row)}
}
