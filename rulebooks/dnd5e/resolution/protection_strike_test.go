// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const protectorID = "protector-1"

// protector is a fighter with the Protection style, a shield in the off hand
// and its reaction unspent: eligible on every count the sheet answers, so
// placement alone decides.
func protector(t *testing.T) *character.Data {
	t.Helper()
	style, err := conditions.NewFightingStyleProtectionCondition(protectorID).ToJSON()
	require.NoError(t, err)
	return &character.Data{
		ID: protectorID, PlayerID: "player-2", Name: "Protector", Level: 1, ClassID: "fighter", RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12, ProficiencyBonus: 2,
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeArmor, ID: string(armor.Shield), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotOffHand: string(armor.Shield)},
		Conditions:     []json.RawMessage{style},
		ActionEconomy: &character.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
}

// protectionWorld places the wolf and the hero side by side and, when asked,
// the protector beside the hero.
func protectionWorld(t *testing.T, placeProtector bool) encounter.EncounterData {
	t.Helper()
	members := []encounter.MemberInput{
		{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 1}},
		{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
	}
	if placeProtector {
		members = append(members, encounter.MemberInput{
			ID: protectorID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1},
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: encounter.RefusingMover{},
		Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		Field:   encounter.FieldInput{Canvas: hexCanvas(), Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 10, 4)}},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	return enc.ToData()
}

// wolfBitesTheHeroBesideProtector runs the wolf's melee strike on the hero with the
// protector in the cast, placed or not.
func wolfBitesTheHeroBesideProtector(t *testing.T, placeProtector bool) (StrikeOutcome, *Output) {
	t.Helper()
	out, err := Resolve(context.Background(), &Input{
		World: protectionWorld(t, placeProtector),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: actionHero()}, {Character: protector(t)},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(),
			Roller: &actionRoller{singles: []int{15}, pairs: [][]int{{15, 15}}, damage: [][]int{{3}}},
		}),
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Roller: dice.NewRoller(),
	})
	require.NoError(t, err)
	outcome, ok := out.Outcome.(StrikeOutcome)
	require.True(t, ok)
	return outcome, out
}

func protectionImposed(sources []dnd5eEvents.AttackModifierSource) bool {
	for _, source := range sources {
		if source.SourceID == protectorID && source.SourceRef.Equals(refs.Conditions.FightingStyleProtection()) {
			return true
		}
	}
	return false
}

// protectorReactions is the protector's reactions as the resolution left
// them: its dirty sheet if one came back, otherwise untouched.
func protectorReactions(t *testing.T, out *Output) int {
	t.Helper()
	for _, sheet := range out.DirtyCharacters {
		if sheet.ID == protectorID {
			require.NotNil(t, sheet.ActionEconomy)
			return sheet.ActionEconomy.ReactionsRemaining
		}
	}
	return protector(t).ActionEconomy.ReactionsRemaining
}

// A protector in the cast but not on the map stands within 5 feet of nobody:
// the complete execution frame carries no pair for it, so the attack goes
// ahead without Protection and its reaction is not spent.
func TestAnUnplacedProtectorDoesNotProtect(t *testing.T) {
	outcome, out := wolfBitesTheHeroBesideProtector(t, false)

	require.False(t, protectionImposed(outcome.Folded.DisadvantageSources), "no disadvantage from an unplaced protector")
	require.Equal(t, 1, protectorReactions(t, out), "nothing was spent")
}

// The control: the same protector placed beside the hero imposes
// disadvantage and spends its reaction.
func TestAPlacedProtectorBesideTheTargetImposesDisadvantage(t *testing.T) {
	outcome, out := wolfBitesTheHeroBesideProtector(t, true)

	require.True(t, protectionImposed(outcome.Folded.DisadvantageSources), "Protection imposes disadvantage")
	require.Equal(t, 0, protectorReactions(t, out), "the reaction was spent")
}
