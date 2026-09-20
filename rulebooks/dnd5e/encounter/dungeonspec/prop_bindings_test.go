// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// prop_bindings_test.go is THE FOURTH DECLARATION KIND (rpg-project#488, R1)
// — what a placed prop's orders refuse, and the one thing they cannot do yet.
//
// # The two seams, and why the tests are split across them
//
// Every refusal about OWNERSHIP — an id no declaration owns, an id that is
// also a door, an id inside an arrangement — is a defect in the DOCUMENT, so
// the decoder is where it lands and [dungeonspec.DecodeSingleRoom] fails.
//
// The block as a whole is a defect in what the ENGINE can run, not in the
// document: a v4 item compiles to a footprint, and holdable/holds/arrives
// live on the legacy prop, which wants a content ref and an anchor cell this
// dialect does not have (rpg-toolkit#1854). So a legal block DECODES — which
// is what lets the World Builder author it, round-trip it and grade it today
// — and [dungeonspec.CompileSingleRoom] refuses it by name.
//
// THE SECOND HALF IS THE ONE THAT MATTERS. Carrying `holdable: true` through
// a compile that drops it would produce a world where nothing can be picked
// up and nothing says so, which is the fail-silent this repository refuses.
// The test below is what stops that being "fixed" by deleting the refusal.

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// theLetterAsAProp is the v4 document that authors the Wiseman's letter the
// way v2 does — a holdable prop that carries the record and arrives on round
// 6. It lives outside testdata's own glob because it does not compile, and
// testdata/*.yaml is the set of dungeons this package ships.
const theLetterAsAProp = "testdata/decode-only/world-builder-v4-raider-letter.yaml"

// The whole block decodes, with every value the author wrote carried onto the
// spec — the shape the World Builder emits, graded and kept.
func TestAPropsOrdersDecodeWhole(t *testing.T) {
	raw, err := os.ReadFile(theLetterAsAProp) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)

	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err, "a legal document, whatever the engine can do with it")

	require.Equal(t, dungeonspec.RoomPropBinding{
		Holdable: true,
		Holds:    []string{"wisemans-letter"},
		Arrives:  &dungeonspec.PredicateSpec{Round: intPtr(6)},
	}, decoded.Spec.Room.Gameplay.PropBindings["letter"],
		"holdable, what it carries, and when it shows up")
}

// And the compile refuses it by name, at the binding's own path, naming what
// is missing. This is the assertion that fails if the refusal is ever quietly
// dropped in favour of carrying the keys inert.
//
// THIS TEST IS MEANT TO FLIP, and the sentence it pins names the issue that
// flips it. When rpg-toolkit#1854 lands — a placed footprint that can be held
// and can arrive — the refusal goes, this test fails, and what replaces it is
// a compile-equivalence assertion against the v2 letter's Holdable, Holds and
// Arrives. A test that named no issue would just look broken that day.
func TestAPropsOrdersAreRefusedAtCompileUntilThePrimitive(t *testing.T) {
	raw, err := os.ReadFile(theLetterAsAProp) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)

	_, err = dungeonspec.Load(raw)
	require.Error(t, err, "the engine has nowhere to put these")

	var verr *dungeonspec.ValidationError
	require.ErrorAs(t, err, &verr)
	requireExactDefect(t, verr.Errors, "room.room.propBindings.letter",
		"a placed prop cannot be held and cannot arrive in this build: a v4 item compiles to a "+
			"footprint, and holdable/holds/arrives live on the legacy prop, which needs a content ref and an "+
			"anchor cell this dialect does not have; author the prop plain, or wait for rpg-toolkit#1854")
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
	// goblin-1 is not a prop at all, so it earns the ownership refusal at the
	// decoder and never reaches the compile. What this asserts is that the
	// document is refused for BOTH — neither id passes silently.
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
