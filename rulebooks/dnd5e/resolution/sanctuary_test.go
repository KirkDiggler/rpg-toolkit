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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// clericWarder is a level-1 Cleric with a real, computable spell save DC —
// the standalone participant whose Sanctuary wards heroID in these tests.
// Never placed on the encounter map: [wardSaveDC] only needs the sheet, not
// a position.
func clericWarder() *character.Data {
	return &character.Data{
		ID: "cleric-1", PlayerID: "player-2", Name: "Warder", Level: 1, ClassID: "cleric", RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 10, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 16, abilities.CHA: 10,
		},
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 12, ProficiencyBonus: 2,
	}
}

func sanctuaryJSON(t *testing.T, memberID string) json.RawMessage {
	t.Helper()
	ward, err := conditions.NewSanctuaryCondition(conditions.NewSanctuaryConditionInput{
		MemberID: memberID, SourceID: "cleric-1", SourceRef: refs.Spells.Sanctuary(),
	})
	require.NoError(t, err)
	raw, err := ward.ToJSON()
	require.NoError(t, err)
	return raw
}

func sanctuaryImmuneJSON(t *testing.T, memberID string) json.RawMessage {
	t.Helper()
	immune, err := conditions.NewSanctuaryImmuneCondition(conditions.NewSanctuaryImmuneConditionInput{
		MemberID: memberID, SourceID: "cleric-1", SourceRef: refs.Spells.Sanctuary(),
	})
	require.NoError(t, err)
	raw, err := immune.ToJSON()
	require.NoError(t, err)
	return raw
}

func hasConditionRef(t *testing.T, blobs []json.RawMessage, ref string) bool {
	t.Helper()
	for _, blob := range blobs {
		loaded, err := conditions.LoadJSON(blob)
		require.NoError(t, err)
		if loaded.Ref().String() == ref {
			return true
		}
	}
	return false
}

func TestSanctuaryBlocksAnAttackOnAFailedWardSave(t *testing.T) {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}

	roller := &actionRoller{singles: []int{5}} // wolf's WIS save: low, fails against the cleric's DC
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: target}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
	})
	require.NoError(t, err)

	outcome := out.Outcome.(StrikeOutcome)
	require.NotNil(t, outcome.Warded)
	require.Equal(t, "cleric-1", outcome.Warded.SourceID)
	require.False(t, outcome.Warded.Save.Success)
	require.Zero(t, outcome.Roll, "no attack roll happened")
	require.Zero(t, outcome.Damage, "no damage happened")
	require.Equal(t, 1, roller.calls, "only the ward save was ever rolled")
}

func TestSanctuarySaveSuccessGrantsImmunityAndTheAttackProceeds(t *testing.T) {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}

	// Ward save rolls high and passes; the attack roll and damage that follow
	// are the same scripted values TestBaneAttackUsesOneSelectedContributionAndRecordsCalculation uses.
	roller := &actionRoller{singles: []int{20, 15}, damage: [][]int{{4}}}
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: target}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
	})
	require.NoError(t, err)

	outcome := out.Outcome.(StrikeOutcome)
	require.Nil(t, outcome.Warded)
	require.NotZero(t, outcome.Roll, "the attack proceeded exactly as if there were no ward")

	require.Len(t, out.DirtyMonsters, 1)
	require.True(t,
		hasConditionRef(t, out.DirtyMonsters[0].Conditions, refs.Conditions.SanctuaryImmune().String()),
		"the wolf earned immunity to cleric-1's Sanctuary by passing the save",
	)
}

func TestSanctuaryImmunitySkipsTheSaveEntirely(t *testing.T) {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}
	attacker := monsters.NewWolf(wolfID).ToData()
	attacker.Conditions = []json.RawMessage{sanctuaryImmuneJSON(t, wolfID)}

	// No save roll scripted at all: a queued read would fail the test outright.
	roller := &actionRoller{singles: []int{15}, damage: [][]int{{4}}}
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: attacker}, {Character: target}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
	})
	require.NoError(t, err)

	outcome := out.Outcome.(StrikeOutcome)
	require.Nil(t, outcome.Warded)
	require.NotZero(t, outcome.Roll)
	require.Equal(t, 2, roller.calls, "one attack roll and one damage roll — no ward save")
}

func TestAttackingEndsTheAttackersOwnSanctuary(t *testing.T) {
	attacker := monsters.NewWolf(wolfID).ToData()
	attacker.Conditions = []json.RawMessage{sanctuaryJSON(t, wolfID)}

	roller := &actionRoller{singles: []int{15}, damage: [][]int{{4}}}
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: wolfID, TargetID: heroID, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: attacker}, {Character: actionHero()}, {Character: clericWarder()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
	})
	require.NoError(t, err)

	require.Len(t, out.DirtyMonsters, 1)
	require.False(t,
		hasConditionRef(t, out.DirtyMonsters[0].Conditions, refs.Conditions.Sanctuary().String()),
		"the wolf's own Sanctuary ended the moment it attacked",
	)
}
