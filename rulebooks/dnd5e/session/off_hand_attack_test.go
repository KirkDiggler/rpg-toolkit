// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

func offHandFighter(id string) *character.Data {
	fighter := armedFighter(id)
	fighter.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponMartial}
	fighter.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Scimitar), Quantity: 1},
	}
	fighter.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Shortsword),
		character.SlotOffHand:  string(weapons.Scimitar),
	}
	return fighter
}

func attackDeclarations(declarations []session.Declaration) []session.Declaration {
	out := make([]session.Declaration, 0, 2)
	for _, declaration := range declarations {
		if declaration.Verb == session.VerbAttack {
			out = append(out, declaration)
		}
	}
	return out
}

func attackDeclarationForSlot(
	t *testing.T, declarations []session.Declaration, slot session.Slot,
) session.Declaration {
	t.Helper()
	var found *session.Declaration
	for i := range declarations {
		if declarations[i].Verb != session.VerbAttack || declarations[i].Slot != slot {
			continue
		}
		if found != nil {
			t.Fatalf("more than one Attack declaration uses slot %q", slot)
		}
		found = &declarations[i]
	}
	if found == nil {
		t.Fatalf("no Attack declaration uses slot %q", slot)
	}
	return *found
}

func affordOffHandFight(t *testing.T, mgr *session.Manager) *session.AffordOutput {
	t.Helper()
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	return out
}

func TestOffHandAttackExecutesWithBonusSlotAndOffHandWeapon(t *testing.T) {
	mgr, _, _, characters := aFight(t, offHandFighter("alice"), []int{15, 4, 15, 4})

	before := affordOffHandFight(t, mgr)
	main := attackDeclarationForSlot(t, before.Declarations, session.SlotAction)
	mainResult, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
	})
	require.NoError(t, err)
	require.True(t, mainResult.Hit)
	require.Equal(t, 7, mainResult.Damage)
	require.Equal(t, "dnd5e:weapons:shortsword", mainResult.Attack.Ref)

	storedAfterMain := characters.byID["alice"].ActionEconomy
	require.NotNil(t, storedAfterMain)
	require.Equal(t, 1, storedAfterMain.BonusActionsRemaining)
	require.Equal(t, 1, storedAfterMain.Granted[character.GrantedOffHandStrikes])

	afterMain := affordOffHandFight(t, mgr)
	bonus := attackDeclarationForSlot(t, afterMain.Declarations, session.SlotBonus)
	bonusResult, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: bonus.ID,
	})
	require.NoError(t, err)
	require.True(t, bonusResult.Hit)
	require.Equal(t, 4, bonusResult.Damage, "base off-hand damage omits the positive ability modifier")
	require.Equal(t, "dnd5e:weapons:scimitar", bonusResult.Attack.Ref)

	storedAfterBonus := characters.byID["alice"].ActionEconomy
	require.NotNil(t, storedAfterBonus)
	require.Zero(t, storedAfterBonus.BonusActionsRemaining)
	require.Zero(t, storedAfterBonus.Granted[character.GrantedOffHandStrikes])

	afterBonus := affordOffHandFight(t, mgr)
	require.Len(t, attackDeclarations(afterBonus.Declarations), 1,
		"consuming the granted capacity removes the off-hand declaration")
}

func TestSpentBonusActionKeepsDisabledOffHandDeclaration(t *testing.T) {
	mgr, _, _, characters := aFight(t, offHandFighter("alice"), []int{1})
	main := attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotAction)
	_, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
	})
	require.NoError(t, err)

	available := attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotBonus)
	characters.byID["alice"].ActionEconomy.BonusActionsRemaining = 0

	refreshed := affordOffHandFight(t, mgr)
	disabled := attackDeclarationForSlot(t, refreshed.Declarations, session.SlotBonus)
	require.False(t, disabled.Available)
	require.NotNil(t, disabled.Why)
	require.Equal(t, session.ShortfallNoBudget, disabled.Why.Reason)
	require.Equal(t, session.CurrencyBonus, disabled.Why.Currency)
	require.Equal(t, 1, disabled.Why.Needed)
	require.Zero(t, disabled.Why.Left)

	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: available.ID,
	})
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.Zero(t, characters.byID["alice"].ActionEconomy.BonusActionsRemaining)
	require.Equal(t, 1, characters.byID["alice"].ActionEconomy.Granted[character.GrantedOffHandStrikes])
}

func TestChangedEquipmentMakesOffHandSelectorStale(t *testing.T) {
	mgr, _, _, characters := aFight(t, offHandFighter("alice"), []int{1})
	main := attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotAction)
	_, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
	})
	require.NoError(t, err)
	bonus := attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotBonus)

	characters.byID["alice"].EquipmentSlots.Clear(character.SlotOffHand)
	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: bonus.ID,
	})

	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	require.Equal(t, 1, characters.byID["alice"].ActionEconomy.BonusActionsRemaining)
	require.Equal(t, 1, characters.byID["alice"].ActionEconomy.Granted[character.GrantedOffHandStrikes])
}

func TestTurnResetRemovesUnusedOffHandDeclaration(t *testing.T) {
	mgr, _, _, _ := aFight(t, offHandFighter("alice"), []int{1})
	main := attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotAction)
	_, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, attackDeclarationForSlot(t, affordOffHandFight(t, mgr).Declarations, session.SlotBonus).ID)

	end := currentEndTurnID(t, mgr, "sess", "alice")
	_, err = mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: end,
	})
	require.NoError(t, err)

	refreshed := affordOffHandFight(t, mgr)
	require.Len(t, attackDeclarations(refreshed.Declarations), 1)
	require.Equal(t, session.SlotAction, attackDeclarations(refreshed.Declarations)[0].Slot)
}

func TestQualifyingMissAddsBonusAttackDeclaration(t *testing.T) {
	mgr, _, _, _ := aFight(t, offHandFighter("alice"), []int{1})

	before := affordOffHandFight(t, mgr)
	beforeAttacks := attackDeclarations(before.Declarations)
	require.Len(t, beforeAttacks, 1)
	main := attackDeclarationForSlot(t, before.Declarations, session.SlotAction)
	require.Equal(t, "dnd5e:weapons:shortsword", main.Attack.Ref)

	result, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
	})
	require.NoError(t, err)
	require.False(t, result.Hit, "a natural 1 misses and still completes the Attack action")

	after := affordOffHandFight(t, mgr)
	afterAttacks := attackDeclarations(after.Declarations)
	require.Len(t, afterAttacks, 2)
	require.Equal(t, []session.Slot{session.SlotAction, session.SlotBonus}, []session.Slot{
		afterAttacks[0].Slot, afterAttacks[1].Slot,
	})
	bonus := attackDeclarationForSlot(t, after.Declarations, session.SlotBonus)
	require.True(t, bonus.Available)
	require.Equal(t, "dnd5e:weapons:scimitar", bonus.Attack.Ref)
	require.NotEqual(t, main.ID, bonus.ID)
	require.Equal(t, main.Candidates, bonus.Candidates)
}
