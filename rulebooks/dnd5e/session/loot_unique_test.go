// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// loot_unique_test.go plays "a character owns one of a custom item"
// (rpg-project#1177, story rpg-project#850): a looter who already owns a
// site's custom item does not take another, and it stays on the body for the
// next looter. The scenes are lieutenants_cloak_test.go's.

const twoLieutenantsSitePath = "testdata/two-lieutenants-walk.yaml"

func (s *cloakScene) lootBody(member, body string) (*session.LootOutput, error) {
	return s.mgr.Loot(context.Background(), &session.LootInput{Session: s.id, Member: member, Target: body, Range: 3})
}

// equippedOn is the ids of the items the saved encounter says the body wears.
func (s *cloakScene) equippedOn(body string) []string {
	s.t.Helper()
	enc, err := s.mgr.LoadWorldForTest(context.Background(), s.id)
	require.NoError(s.t, err)
	data := enc.ToData()
	out := []string{}
	for _, m := range data.Members {
		if string(m.ID) == body {
			for _, g := range m.Equipment {
				out = append(out, g.ID)
			}
		}
	}
	return out
}

// Test 1. One fighter plays the site twice, with the same saved character.
func TestACharacterWhoOwnsTheCloakDoesNotTakeAnotherInALaterRun(t *testing.T) {
	ctx := context.Background()
	first := newCloakSceneOn(t, cloakSite(t), "sess-1", nil, "alice")
	_, err := first.loot("alice")
	require.NoError(t, err)
	owned := ownedCloaks(first.stored("alice"))
	require.Len(t, owned, 1)

	// The cloak goes on and is used: it is at 0 of 1.
	_, err = first.mgr.Equip(ctx, &session.EquipInput{Character: "alice", Slot: string(character.SlotCloak), Item: owned[0]})
	require.NoError(t, err)
	rows := first.secondWindRows("alice")
	require.Len(t, rows, 2)
	_, err = first.mgr.Activate(ctx, &session.ActivateInput{Session: "sess-1", Member: "alice", DeclarationID: rows[1].ID})
	require.NoError(t, err)
	require.Zero(t, itemLeft(first.stored("alice"), owned[0]), "the cloak is at 0 of 1")

	// The second run, the same saved character. A launch is the character's
	// first-admission long rest, which refills the cloak to 1 of 1 (found while
	// building: the issue expected it to stay at 0), so the cloak is used again
	// in this run, and only then is the body looted.
	second := newCloakSceneOn(t, cloakSite(t), "sess-2", first.chars, "alice")
	require.Equal(t, owned, ownedCloaks(second.stored("alice")), "the saved sheet came into the second run")
	require.Equal(t, 1, itemLeft(second.stored("alice"), owned[0]), "the launch's long rest refilled it")
	rows = second.secondWindRows("alice")
	require.Len(t, rows, 2, "the cloak is still on from the first run")
	_, err = second.mgr.Activate(ctx, &session.ActivateInput{Session: "sess-2", Member: "alice", DeclarationID: rows[1].ID})
	require.NoError(t, err)
	require.Zero(t, itemLeft(second.stored("alice"), owned[0]), "the cloak is at 0 of 1 in the second run")

	_, err = second.loot("alice")
	require.NoError(t, err, "a looter who has the cloak loots as any looter does")

	saved := second.stored("alice")
	require.Equal(t, owned, ownedCloaks(saved), "exactly one cloak, with the id it had")
	require.Zero(t, itemLeft(saved, owned[0]), "a second loot is not a fresh charge")
	require.Equal(t, []string{"lieutenants-cloak"}, second.equippedOn("guard"), "it stayed on the body")
}

// Test 2. What the looter left is on the body for the next looter.
func TestWhatALooterLeftStaysForAnotherPlayer(t *testing.T) {
	first := newCloakSceneOn(t, cloakSite(t), "sess-1", nil, "alice")
	_, err := first.loot("alice")
	require.NoError(t, err)

	second := newCloakSceneOn(t, cloakSite(t), "sess-2", first.chars, "alice", "bob")
	require.Equal(t, []string{"lieutenants-cloak"}, second.equippedOn("guard"), "the body wears the cloak")
	require.Empty(t, ownedCloaks(second.stored("bob")))

	_, err = second.loot("alice")
	require.NoError(t, err)
	require.Equal(t, []string{"lieutenants-cloak"}, second.equippedOn("guard"), "alice left it on the body")
	require.Len(t, ownedCloaks(second.stored("alice")), 1)

	_, err = second.lootBody("bob", "guard")
	require.NoError(t, err)
	require.Len(t, ownedCloaks(second.stored("bob")), 1, "bob has the cloak")
	require.Empty(t, second.equippedOn("guard"), "the body has nothing")
}

// Test 3. Two bodies in one run: one cloak for the looter, and the second
// body keeps its own.
func TestALooterOfTwoBodiesTakesOneCloak(t *testing.T) {
	raw, err := os.ReadFile(twoLieutenantsSitePath)
	require.NoError(t, err)
	s := newCloakScene(t, string(raw), "alice")
	require.Equal(t, []string{"lieutenants-cloak"}, s.equippedOn("guard"))
	require.Equal(t, []string{"lieutenants-cloak"}, s.equippedOn("guard-2"))

	_, err = s.lootBody("alice", "guard")
	require.NoError(t, err)
	_, err = s.lootBody("alice", "guard-2")
	require.NoError(t, err)

	require.Len(t, ownedCloaks(s.stored("alice")), 1, "one cloak on the sheet")
	require.Empty(t, s.equippedOn("guard"), "the first body gave its cloak")
	require.Equal(t, []string{"lieutenants-cloak"}, s.equippedOn("guard-2"), "the second body keeps its cloak")
}

// Test 4. THE LAW: the answer is the same whether the looter took the cloak,
// left it, or the body had none.
func TestLootAnswersTheSameWhetherTheLooterTookOrLeftTheCloak(t *testing.T) {
	// A looter who owns the cloak.
	owner := newCloakSceneOn(t, cloakSite(t), "sess", nil, "alice")
	_, err := owner.loot("alice")
	require.NoError(t, err)
	left := newCloakSceneOn(t, cloakSite(t), "sess", owner.chars, "alice")
	gotLeft, err := left.loot("alice")
	require.NoError(t, err)
	require.Len(t, ownedCloaks(left.stored("alice")), 1)
	require.Equal(t, []string{"lieutenants-cloak"}, left.equippedOn("guard"), "it was left")

	// A looter who does not.
	took := newCloakScene(t, cloakSite(t), "alice")
	gotTook, err := took.loot("alice")
	require.NoError(t, err)
	require.Empty(t, took.equippedOn("guard"), "it was taken")

	// The same site with no equipment line.
	none := newCloakScene(t, cloakSite(t, "        equipment: [lieutenants-cloak]\n", ""), "alice")
	gotNone, err := none.loot("alice")
	require.NoError(t, err)

	require.Equal(t, gotTook, gotLeft, "the whole LootOutput of a looter who left it and one who took it")
	require.Equal(t, gotTook, gotNone, "and of a body that carried nothing")
	require.Equal(t, []string{"character:alice", "encounter:sess"}, gotLeft.Saved.Written)
}
