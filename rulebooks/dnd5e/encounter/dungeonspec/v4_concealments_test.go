// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// v4_concealments_test.go is ONE NOUN, PROVED IN BOTH DIALECTS
// (rpg-project#490, E5).
//
// # The claim, and how it is tested
//
// The v2 dialect says a secret with two words — `regions[].concealed: true`
// and `doors[].concealed: [...]` — and the single-room dialect says it with
// one root block. Both lower to the SAME [encounter.ConcealmentInput], and a
// claim of that shape is not proved by asserting a field is populated: it is
// proved by authoring the same secret in both documents and requiring the
// same compiled answer.
//
// testdata/world-builder-v4-tomb-vault.yaml is the reference heirloom tomb's
// vault re-authored in v4, and the first test below compiles the two beside
// each other. IDS ARE NOT COMPARED — each dialect mints under its own key,
// which is the same reason a door's id and a record's id are not compared
// anywhere else here.
//
// ONE FIELD DIFFERS ON PURPOSE, and it is the point of the noun: the v4
// document names the HEIRLOOM as a member. v2 can only hide the prize by
// standing it on hidden floor; naming it is what lets a bookcase in plain
// sight be part of a secret, and the test says so rather than comparing
// around it.
//
// The rest of the file is the refusals, each at the author's own path.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

const v4VaultPath = "testdata/world-builder-v4-tomb-vault.yaml"

// TestTheTwoDialectsHideTheSameVault is the equivalence claim.
func TestTheTwoDialectsHideTheSameVault(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-tomb-heirloom.yaml")
	v4 := loadContent(t, v4VaultPath)

	require.Len(t, v2.Concealments, 1, "the v2 tomb hides one thing")
	require.Len(t, v4.Concealments, 1, "and so does the room it was re-authored as")
	hidden2, hidden4 := v2.Concealments[0], v4.Concealments[0]

	require.Equal(t, hidden2.Checks, hidden4.Checks,
		"the same two ways to find it — spotted, or reasoned out — in the same authored order")
	require.Len(t, hidden4.Cells, len(hidden2.Cells), "the same six cells of hidden floor")
	require.Len(t, hidden4.Doors, 1, "and the same one door, a member of the secret rather than a flagged thing")
	require.Len(t, hidden2.Doors, 1)

	require.Equal(t, []string{"heirloom"}, hidden4.Props,
		"and the prize named outright, which is the thing v2 can only say by standing it on hidden floor")
	require.Nil(t, hidden2.Props)

	require.Equal(t, []encounter.CheckApproach{{Ability: "investigation", DC: 12}}, hidden4.Notice,
		"the passive tell rides the lowering, carried and unread until slice 2")
	require.Nil(t, hidden2.Notice, "the v2 dialect has no word for it yet, and says nothing rather than empty")

	// AND WHAT A RECORD GIVES AWAY IS THE SAME SECRET IN BOTH (R7). v2 writes
	// `reveals: { door: vault }` and this dialect writes
	// `reveals: { concealment: vault }`; each resolves to its own document's
	// concealment, which is the one thing a record can name now.
	for _, rec := range v2.Intel {
		require.Equal(t, hidden2.ID, rec.Reveals.Concealment)
	}
	for _, rec := range v4.Intel {
		require.Equal(t, hidden4.ID, rec.Reveals.Concealment)
	}

	require.Equal(t, v4.Concealments, v4.Field.Concealments,
		"the same list the field carries, surfaced for a host that reads what a document declares")
}

// TestTheV4VaultIsPlayable is the other half of the fixture's claim: what it
// lowers to is a field the composition accepts, with the secret's door
// standing as the footprint its prop declaration draws.
func TestTheV4VaultIsPlayable(t *testing.T) {
	v4 := loadContent(t, v4VaultPath)

	require.Len(t, v4.Field.Doors, 1)
	door := v4.Field.Doors[0]
	require.Equal(t, encounter.DoorID("tomb-vault-v4/vault-door"), door.ID)
	require.NotNil(t, door.Placement, "a door in this dialect stands as a rectangle, not in a crossing")
	require.Equal(t, encounter.DoorClosed, door.State.Kind())
	require.Equal(t, []encounter.DoorID{door.ID}, v4.Concealments[0].Doors,
		"and the secret names it by the id the doors list mints, never the raw item id")

	// THE ITEM IS IN BOTH LISTS, under the two ids each list mints — the
	// rectangle the map draws and the door whose state decides what it
	// blocks. Nothing here has to say that twice: the lowering sorts one
	// authored `props:` list into the two.
	placed := map[encounter.PropID]bool{}
	for _, p := range v4.Field.Placed {
		placed[p.ID] = true
	}
	require.True(t, placed["vault-door"], "the leaf is drawn")
	require.True(t, placed["heirloom"], "and so is the prize")
}

// # The refusals, each at the author's own path

// TestAConcealmentIsRefusedForWhatItCannotBe covers every sentence the root
// block adds.
func TestAConcealmentIsRefusedForWhatItCannotBe(t *testing.T) {
	t.Run("no checks", func(t *testing.T) {
		errs := refusals(t, v4With("concealments:\n  vault: { cells: [{q: 1, r: 0}] }", ""))
		requireExactDefect(t, errs, "concealments.vault.checks",
			"this concealment needs at least one way to find it — an ability and a DC under `checks`")
	})

	t.Run("an empty notice", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n  vault: { notice: [], checks: [{ability: perception, dc: 15}], cells: [{q: 1, r: 0}] }", ""))
		requireExactDefect(t, errs, "concealments.vault.notice",
			"this concealment declares a notice with no way through it — an ability and a DC")
	})

	t.Run("hides nothing", func(t *testing.T) {
		errs := refusals(t, v4With("concealments:\n  vault: { checks: [{ability: perception, dc: 15}] }", ""))
		requireExactDefect(t, errs, "concealments.vault",
			"this concealment hides nothing: list the hexes under `cells`, "+
				"the placed things under `props`, or both")
	})

	t.Run("a cell this room's floor does not have", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n  vault: { checks: [{ability: perception, dc: 15}], cells: [{q: 5, r: 5}] }", ""))
		requireExactDefect(t, errs, "concealments.vault.cells[0]",
			"a concealment hides floor this room has: name a hex from walkableHexes")
	})

	t.Run("a placed thing this room does not declare", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n  vault: { checks: [{ability: perception, dc: 15}], props: [bookcase] }", ""))
		requireExactDefect(t, errs, "concealments.vault.props[0]",
			"a concealment hides things this room places: declare it under propDeclarations")
	})

	// R4, AND THE REFUSAL NAMES BOTH. An author looking at one of two
	// colliding declarations cannot fix it without being told which other one
	// it collides with.
	t.Run("one hex in two concealments", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n"+
				"  alcove: { checks: [{ability: perception, dc: 15}], cells: [{q: 1, r: 0}] }\n"+
				"  vault: { checks: [{ability: perception, dc: 15}], cells: [{q: 1, r: 0}] }", ""))
		requireExactDefect(t, errs, "concealments.vault.cells[0]",
			`this hex is already hidden by concealment "alcove", and a hex belongs to one concealment`)
	})

	t.Run("one placed thing in two concealments", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n"+
				"  alcove: { checks: [{ability: perception, dc: 15}], props: [table] }\n"+
				"  vault: { checks: [{ability: perception, dc: 15}], props: [table] }", ""))
		requireExactDefect(t, errs, "concealments.vault.props[0]",
			`"table" is already hidden by concealment "alcove", and a placed thing belongs to one concealment`)
	})

	t.Run("an authored null", func(t *testing.T) {
		errs := refusals(t, v4With("concealments:\n  vault:", ""))
		requireExactDefect(t, errs, "concealments.vault", "must not be null")
	})

	t.Run("a typo inside one is named at its own path", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n  vault: { cheks: [{ability: perception, dc: 15}], cells: [{q: 1, r: 0}] }", ""))
		requireExactDefect(t, errs, "concealments.vault.cheks",
			`"cheks" is not a key this build reads: they are cells, checks, notice, props`)
	})

	t.Run("a record naming no such concealment", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n  vault: { checks: [{ability: perception, dc: 15}], cells: [{q: 1, r: 0}] }\n"+
				"intel:\n  - { id: map, reveals: { concealment: cellar } }", ""))
		requireExactDefect(t, errs, "intel[0].reveals.concealment",
			`intel "map" reveals concealment "cellar", and no concealment in this room has that id`)
	})

	t.Run("a record revealing both a concealment and a fact", func(t *testing.T) {
		errs := refusals(t, v4With(
			"concealments:\n  vault: { checks: [{ability: perception, dc: 15}], cells: [{q: 1, r: 0}] }\n"+
				"intel:\n  - { id: map, reveals: { concealment: vault, fact: a-fact } }", ""))
		requireExactDefect(t, errs, "intel[0].reveals",
			`intel "map" reveals both a concealment and a fact, and a record reveals exactly one thing`)
	})

	// AND THE ONE THAT IS LEGAL: a secret hiding only a placed thing, no
	// cells at all. A bookcase that is not what it looks like.
	t.Run("hiding only a placed thing is legal", func(t *testing.T) {
		compiled := loadSource(t, v4With(
			"concealments:\n  vault: { checks: [{ability: perception, dc: 15}], props: [table] }", ""))
		require.Len(t, compiled.Concealments, 1)
		require.Empty(t, compiled.Concealments[0].Cells)
		require.Equal(t, []string{"table"}, compiled.Concealments[0].Props)
	})
}

// TestTheUnknownKeyRefusalOffersConcealments is rpg-project#481 R2 reaching
// the new root key — offered by REFLECTION off the dialect root
// (unknown_key.go), so a key added to a shape is offered the day it exists.
func TestTheUnknownKeyRefusalOffersConcealments(t *testing.T) {
	errs := refusals(t, v4With("concealmets:\n  vault: { checks: [] }", ""))
	requireExactDefect(t, errs, "concealmets",
		`"concealmets" is not a key this build reads: they are concealments, dispositions, endings, `+
			"exits, factions, intel, key, play, room, scenarios, tables, version")
}

// TestADoorIsHiddenByBelongingRatherThanByAFlag is where the retired
// `doorBindings.<id>.concealed` went (rpg-project#490). The word moved to the
// root rather than staying refused, so writing it on a door binding earns the
// unknown-key sentence — which says what that block DOES take.
func TestADoorIsHiddenByBelongingRatherThanByAFlag(t *testing.T) {
	compiled := loadSource(t, v4With(
		"concealments:\n  vault: { checks: [{ability: perception, dc: 15}], props: [table] }",
		"    doorBindings:\n      table: { closed: true }"))

	require.Len(t, compiled.Field.Doors, 1)
	require.Equal(t, encounter.DoorID("refusal-fixture/table"), compiled.Field.Doors[0].ID)
	require.Equal(t, []encounter.DoorID{"refusal-fixture/table"}, compiled.Concealments[0].Doors,
		"the author wrote the item id in `props:` and the lowering sorted it into the doors")
	require.Empty(t, compiled.Concealments[0].Props, "so it is not also a prop member")
}
