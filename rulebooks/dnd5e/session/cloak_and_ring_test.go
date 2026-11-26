// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// cloak_and_ring_test.go plays "a looted item is one per character only when
// its author says unique" (rpg-project#1182, story rpg-project#850) on
// testdata/cloak-and-ring-walk.yaml: the Lieutenant wears a unique cloak and
// a ring that is not unique.

const cloakAndRingSitePath = "testdata/cloak-and-ring-walk.yaml"

const ringName = "Garrison ring"

func ringSite(t *testing.T, pairs ...string) string {
	t.Helper()
	raw, err := os.ReadFile(cloakAndRingSitePath)
	require.NoError(t, err)
	text := string(raw)
	for i := 0; i < len(pairs); i += 2 {
		require.Contains(t, text, pairs[i], "the site has the text being changed")
		text = strings.Replace(text, pairs[i], pairs[i+1], 1)
	}
	return text
}

// ownedNamed is the sheet's items whose rules name is name, in sheet order.
func ownedNamed(data *character.Data, name string) []string {
	var out []string
	for _, item := range data.Items {
		if item.Description.Name == name {
			out = append(out, item.ID)
		}
	}
	return out
}

// Tests 1 and 2. One fighter plays the site twice.
func TestAUniqueCloakIsOnePerCharacterAndANotUniqueRingIsEveryTime(t *testing.T) {
	ctx := context.Background()
	first := newCloakSceneOn(t, ringSite(t), "sess-1", nil, "alice")
	_, err := first.loot("alice")
	require.NoError(t, err)
	saved := first.stored("alice")
	cloaks, rings := ownedNamed(saved, cloakName), ownedNamed(saved, ringName)
	require.Len(t, cloaks, 1)
	require.Len(t, rings, 1, "the first loot takes both")
	require.Empty(t, first.equippedOn("guard"), "the body gave both")

	// The second run, the same saved character, a new Lieutenant wearing both.
	// A launch is a long rest, so the first ring is spent inside this run.
	second := newCloakSceneOn(t, ringSite(t), "sess-2", first.chars, "alice", "bob")
	require.ElementsMatch(t, []string{"lieutenants-cloak", "garrison-ring"}, second.equippedOn("guard"))
	_, err = second.mgr.Equip(ctx, &session.EquipInput{Character: "alice", Slot: string(character.SlotRingLeft), Item: rings[0]})
	require.NoError(t, err)
	rows := second.secondWindRows("alice")
	require.Len(t, rows, 2, "her own Second Wind and the ring's")
	_, err = second.mgr.Activate(ctx, &session.ActivateInput{Session: "sess-2", Member: "alice", DeclarationID: rows[1].ID})
	require.NoError(t, err)
	require.Zero(t, itemLeft(second.stored("alice"), rings[0]), "the first ring is at 0 of 1")

	_, err = second.loot("alice")
	require.NoError(t, err)
	saved = second.stored("alice")
	require.Equal(t, cloaks, ownedNamed(saved, cloakName), "still exactly one cloak, with the id it had")
	got := ownedNamed(saved, ringName)
	require.Len(t, got, 2, "a second ring")
	require.Equal(t, rings[0], got[0])
	require.NotEqual(t, got[0], got[1], "each ring is its own item")
	require.Zero(t, itemLeft(saved, got[0]), "the first ring reads 0 of 1")
	require.Equal(t, 1, itemLeft(saved, got[1]), "the second reads 1 of 1")

	// Test 2. The cloak stayed on the body; the ring did not.
	require.Equal(t, []string{"lieutenants-cloak"}, second.equippedOn("guard"))
	_, err = second.lootBody("bob", "guard")
	require.NoError(t, err)
	bob := second.stored("bob")
	require.Len(t, ownedNamed(bob, cloakName), 1, "bob gets the cloak")
	require.Empty(t, ownedNamed(bob, ringName), "and no ring: it was taken")
	require.Empty(t, second.equippedOn("guard"))
}

// Test 4. THE LAW: the whole answer is the same whatever the looter took.
func TestLootAnswersTheSameWhateverTheLooterOwnedOfCloakAndRing(t *testing.T) {
	owner := newCloakSceneOn(t, ringSite(t), "sess-1", nil, "alice")
	_, err := owner.loot("alice")
	require.NoError(t, err)
	owns := newCloakSceneOn(t, ringSite(t), "sess", owner.chars, "alice")
	gotOwns, err := owns.loot("alice")
	require.NoError(t, err)
	require.Equal(t, []string{"lieutenants-cloak"}, owns.equippedOn("guard"), "cloak left, ring taken")
	require.Len(t, ownedNamed(owns.stored("alice"), ringName), 2)

	fresh := newCloakScene(t, ringSite(t), "alice")
	gotFresh, err := fresh.loot("alice")
	require.NoError(t, err)
	require.Empty(t, fresh.equippedOn("guard"), "both taken")

	none := newCloakScene(t, ringSite(t, "        equipment: [lieutenants-cloak, garrison-ring]\n", ""), "alice")
	gotNone, err := none.loot("alice")
	require.NoError(t, err)

	require.Equal(t, gotFresh, gotOwns, "the whole LootOutput of a looter who left the cloak and one who took both")
	require.Equal(t, gotFresh, gotNone, "and of a body that carried nothing")
}

// Test 7. A sheet saved before the word existed holds the cloak under the
// site's id with no `unique` on its copied description. The cloak is now
// `unique: true`: the looter still leaves it, the loot is not refused, and
// the sheet still has one cloak.
func TestASheetFromBeforeTheWordStillLeavesTheCloak(t *testing.T) {
	first := newCloakSceneOn(t, cloakSite(t), "sess-1", nil, "alice")
	_, err := first.loot("alice")
	require.NoError(t, err)
	old := first.stored("alice")
	require.Len(t, old.Items, 1)
	require.True(t, old.Items[0].Description.Unique, "saved now, with the word")
	id := old.Items[0].ID
	old.Items[0].Description.Unique = false // as session v0.214.0 saved it
	require.NoError(t, first.chars.SaveCharacter(context.Background(), old))

	second := newCloakSceneOn(t, cloakSite(t), "sess-2", first.chars, "alice")
	_, err = second.loot("alice")
	require.NoError(t, err, "the loot is not refused as already owned")
	require.Equal(t, []string{id}, ownedCloaks(second.stored("alice")), "still exactly one cloak")
	require.Equal(t, []string{"lieutenants-cloak"}, second.equippedOn("guard"), "left on the body")
}
