// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

func quarterstaffMonk(t *testing.T, id string) *character.Data {
	t.Helper()
	monk := armedFighter(id)
	monk.Level = 1
	monk.ClassID = classes.Monk
	monk.AbilityScores[abilities.STR] = 12
	monk.AbilityScores[abilities.DEX] = 16
	monk.AbilityScores[abilities.WIS] = 15
	monk.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponSimple}
	monk.Inventory = []character.InventoryItemData{{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Quarterstaff), Quantity: 1,
	}}
	monk.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Quarterstaff),
	}
	martialArts, err := conditions.NewMartialArtsCondition(conditions.MartialArtsInput{
		MemberID: id, MonkLevel: 1,
	}).ToJSON()
	require.NoError(t, err)
	monk.Conditions = append(monk.Conditions, martialArts)
	return monk
}

func TestChangedMartialArtsEligibilityMakesTheBonusSelectorStale(t *testing.T) {
	mgr, _, _, characters := aFight(t, quarterstaffMonk(t, "alice"), []int{1})
	main := attackDeclarationForSlot(
		t, affordOffHandFight(t, mgr).Declarations, session.SlotAction,
	)
	_, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
	})
	require.NoError(t, err)
	bonus := attackDeclarationForSlot(
		t, affordOffHandFight(t, mgr).Declarations, session.SlotBonus,
	)

	characters.byID["alice"].Conditions = nil
	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: bonus.ID,
	})

	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	stored := characters.byID["alice"].ActionEconomy
	require.Equal(t, 1, stored.BonusActionsRemaining)
	require.Equal(t, 1, stored.Granted[character.GrantedMartialArtsBonus])
}

func TestQuarterstaffAttackGrantsBonusUnarmedStrikeOnAMiss(t *testing.T) {
	mgr, _, _, characters := aFight(t, quarterstaffMonk(t, "alice"), []int{1, 1})

	before := affordOffHandFight(t, mgr)
	require.Len(t, attackDeclarations(before.Declarations), 1)
	main := attackDeclarationForSlot(t, before.Declarations, session.SlotAction)
	require.Equal(t, "dnd5e:weapons:quarterstaff", main.Attack.Ref)

	mainResult, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: main.ID,
	})
	require.NoError(t, err)
	require.False(t, mainResult.Hit, "a miss still completes the qualifying Attack action")
	require.Equal(t, "dnd5e:weapons:quarterstaff", mainResult.Attack.Ref)

	storedAfterMain := characters.byID["alice"].ActionEconomy
	require.NotNil(t, storedAfterMain)
	require.Zero(t, storedAfterMain.ActionsRemaining)
	require.Equal(t, 1, storedAfterMain.BonusActionsRemaining)
	require.Equal(t, 1, storedAfterMain.Granted[character.GrantedMartialArtsBonus])
	require.Zero(t, storedAfterMain.Granted[character.GrantedOffHandStrikes])

	afterMain := affordOffHandFight(t, mgr)
	bonus := attackDeclarationForSlot(t, afterMain.Declarations, session.SlotBonus)
	require.True(t, bonus.Available)
	require.Equal(t, "dnd5e:weapons:unarmed-strike", bonus.Attack.Ref)
	require.Equal(t, "Unarmed Strike", bonus.Attack.Name)
	require.NotEqual(t, main.ID, bonus.ID)
	require.Equal(t, main.Candidates, bonus.Candidates)

	bonusResult, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: bonus.ID,
	})
	require.NoError(t, err)
	require.False(t, bonusResult.Hit)
	require.Equal(t, "dnd5e:weapons:unarmed-strike", bonusResult.Attack.Ref,
		"the Quarterstaff remains equipped, but Martial Arts explicitly strikes unarmed")

	storedAfterBonus := characters.byID["alice"].ActionEconomy
	require.NotNil(t, storedAfterBonus)
	require.Zero(t, storedAfterBonus.BonusActionsRemaining)
	require.Zero(t, storedAfterBonus.Granted[character.GrantedMartialArtsBonus])
	require.Len(t, attackDeclarations(affordOffHandFight(t, mgr).Declarations), 1,
		"consuming the Martial Arts grant removes its declaration")
}
