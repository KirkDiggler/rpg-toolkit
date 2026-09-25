// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// prop_bindings_test.go is THE FOURTH DECLARATION KIND (rpg-project#488, R1)
// — what a placed prop's orders refuse, and what they compile to.
//
// # The two seams, and why the tests are split across them
//
// Every refusal about OWNERSHIP — an id no declaration owns, an id that is
// also a door, an id inside an arrangement — is a defect in the DOCUMENT, so
// the decoder is where it lands and [dungeonspec.DecodeSingleRoom] fails.
//
// The block itself COMPILES (rpg-toolkit#1854). It used to be refused at
// [dungeonspec.CompileSingleRoom] by name, because a v4 item compiled to a
// footprint and holdable/holds/arrives lived only on the legacy prop, which
// wants a content ref and an anchor cell this dialect does not have. A placed
// footprint can now be held and can now arrive, so the three keys are laid
// onto the placement the item's own declaration produced.
//
// WHAT REPLACED THE REFUSAL TEST IS THE EQUIVALENCE IT PROMISED. The old test
// named rpg-toolkit#1854 and said in so many words that when the primitive
// landed it would become "a compile-equivalence assertion against the v2
// letter's Holdable, Holds and Arrives". That is what is below.

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// theLetterAsAProp is the v4 document that authors the Wiseman's letter the
// way v2 does — a holdable prop that carries the record and arrives on round
// 6 — with the item's geometry as a footprint rather than an anchor cell.
const theLetterAsAProp = "testdata/world-builder-v4-raider-letter.yaml"

// The whole block decodes, with every value the author wrote carried onto the
// spec — the shape the World Builder emits, graded and kept.
func TestAPropsOrdersDecodeWhole(t *testing.T) {
	raw, err := os.ReadFile(theLetterAsAProp) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)

	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err, "a legal document")

	require.Equal(t, dungeonspec.RoomPropBinding{
		Holdable: true,
		Holds:    []string{"wisemans-letter"},
		Arrives:  &dungeonspec.PredicateSpec{Round: intPtr(6)},
	}, decoded.Spec.Room.Gameplay.PropBindings["letter"],
		"holdable, what it carries, and when it shows up")
}

// And the compile lands all three on the placement, equal to what the v2
// letter compiles to as a legacy prop.
//
// THE RECORD ID DIFFERS BY ONE PREFIX AND NOTHING ELSE, for the reason
// TestTheRaiderCampDeclaresTheSameRecordInBothDialects states: a record is
// minted `<key>/<id>` in both dialects, so the key is a fact about the file
// rather than about the record.
//
// THIS IS THE TEST THE REFUSAL SAID IT WOULD BECOME. It fails if `holdable`,
// `holds` or `arrives` is ever dropped on the way through the compile, which
// is the fail-silent the refusal existed to prevent.
func TestAPropsOrdersCompileEqualToTheV2Letter(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-raider-camp.yaml")
	v4 := loadContent(t, theLetterAsAProp)

	var legacy encounter.PropInput
	for _, p := range v2.Field.Props {
		if p.ID == "letter" {
			legacy = p
		}
	}
	require.Equal(t, encounter.PropID("letter"), legacy.ID, "the v2 camp still authors the letter as a prop")

	var placed encounter.PlacedPropInput
	for _, p := range v4.Field.Placed {
		if p.ID == "letter" {
			placed = p
		}
	}
	require.Equal(t, encounter.PropID("letter"), placed.ID, "the v4 document authors it as a placement")

	require.True(t, legacy.Holdable, "the v2 letter can be picked up")
	require.Equal(t, legacy.Holdable, placed.Holdable, "and so can the v4 one")

	require.Equal(t, []encounter.IntelID{"reference-raider-camp/wisemans-letter"}, legacy.Holds,
		"the v2 letter carries the record")
	require.Equal(t, []encounter.IntelID{"raider-letter-v4/wisemans-letter"}, placed.Holds,
		"the v4 letter carries the same record, this file's key")

	require.Equal(t, encounter.Trigger(encounter.TriggerRound{Round: 6}), legacy.Arrives,
		"the v2 letter is dropped at the gate on round 6")
	require.Equal(t, legacy.Arrives, placed.Arrives, "and so is the v4 one")
}

// A document that binds no prop compiles to placements with none of the three
// set — every room authored before this key existed.
func TestADocumentThatBindsNoPropTakesNoOrders(t *testing.T) {
	v4 := loadContent(t, "testdata/world-builder-v4-site.yaml")

	for _, p := range v4.Field.Placed {
		require.False(t, p.Holdable, "%q was given no orders", p.ID)
		require.Nil(t, p.Holds, "%q was given no orders", p.ID)
		require.Nil(t, p.Arrives, "%q was given no orders", p.ID)
	}
}

// twoPropsOneBound is a room with two declared items and a binding on ONE of
// them — the fixture the keying claim needs, since a document with no
// bindings at all cannot tell "keyed" from "applied to everything".
const twoPropsOneBound = `
version: 4
key: two-props
play: { void: transparent, lighting: bright, standing: centre-covered }
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
      - { id: cup, kind: prop, transform: { x: 0, y: 0, z: 2, rotationY: 0 } }
      - { id: plinth, kind: prop, transform: { x: 2, y: 0, z: 2, rotationY: 0 } }
    groups: []
  room:
    implicitRegionId: room-1-region
    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0}, {q: 0, r: 1}, {q: 1, r: 1}]
    propDeclarations:
      cup: { blocksMovement: false, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
      plinth: { blocksMovement: true, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
    arrangementDeclarations: {}
    partyStart: { q: 0, r: 0 }
    monsterDeclarations: []
    propBindings:
      cup: { holdable: true }
`

// The orders land on the item the binding NAMES and on no other — the
// declaration law's own keying, which is what stops one holdable cup making
// the whole room portable.
func TestAPropsOrdersLandOnTheItemTheyName(t *testing.T) {
	compiled := loadSource(t, twoPropsOneBound)

	byID := map[encounter.PropID]encounter.PlacedPropInput{}
	for _, p := range compiled.Field.Placed {
		byID[p.ID] = p
	}
	require.Contains(t, byID, encounter.PropID("cup"))
	require.Contains(t, byID, encounter.PropID("plinth"))

	require.True(t, byID["cup"].Holdable, "the bound item takes its orders")
	require.False(t, byID["plinth"].Holdable, "and the item beside it takes none")
}

// Every binding is named, not just the first: an author fixing this has to
// see the whole block, and a list that stopped at one would send them round
// again.
func TestEveryPropsOrdersAreNamed(t *testing.T) {
	_, err := dungeonspec.Load([]byte(v4With("", "    propBindings:\n"+
		"      table: { holdable: true }\n"+
		"      goblin-1: { holdable: true }")))
	require.Error(t, err)

	var verr *dungeonspec.ValidationError
	require.ErrorAs(t, err, &verr)
	// goblin-1 is not a prop at all, so it earns the ownership refusal; the
	// table has a declaration of its own and compiles. What this asserts is
	// that the id nothing owns is named rather than passing silently.
	paths := make([]string, 0, len(verr.Errors))
	for _, e := range verr.Errors {
		paths = append(paths, e.Path)
	}
	require.Contains(t, paths, "room.room.propBindings.goblin-1")
}

// # Ownership: the three ids a prop binding may not name

// A binding naming an id nothing declares has no prop to give orders to.
func TestAPropBindingWithNoDeclarationIsRefused(t *testing.T) {
	requireExactDefect(t, refusals(t, v4With("", "    propBindings:\n      nowhere: { holdable: true }")),
		"room.room.propBindings.nowhere",
		"a prop's orders need the prop: declare it under propDeclarations")
}

// An id in BOTH binding blocks is a door somebody picks up, and nothing says
// what that means yet (R1).
func TestAPropBindingOnADoorIsRefused(t *testing.T) {
	requireExactDefect(t, refusals(t, v4With("",
		"    doorBindings:\n      table: { closed: true }\n"+
			"    propBindings:\n      table: { holdable: true }")),
		"room.room.propBindings.table",
		"a door that is also picked up is not something this build plays: this id is a door under "+
			"doorBindings, so give the prop an id of its own, or drop one of the two bindings")
}

// An arrangement's members get no orders — the same refusal an arrangement
// door gets, one declaration kind over.
func TestAPropBindingInsideAnArrangementIsRefused(t *testing.T) {
	requireExactDefect(t, refusals(t, v4With("",
		"    propBindings:\n      stamped-chair: { holdable: true }")),
		"room.room.propBindings.stamped-chair",
		"a prop inside an arrangement is not something this build stamps yet: "+
			"this id names an arrangement template, so declare the orders on a placed item of its own, "+
			"or wait for the arrangement stamp")
}

// intPtr is the one pointer a predicate's round form needs.
func intPtr(v int) *int { return &v }
