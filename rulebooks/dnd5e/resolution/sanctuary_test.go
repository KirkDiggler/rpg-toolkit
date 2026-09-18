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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
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

func TestSanctuarySaveSuccessDoesNotGrantAttackerImmunity(t *testing.T) {
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

	for _, dirty := range out.DirtyMonsters {
		require.False(t, hasConditionRef(t, dirty.Conditions, refs.Conditions.SanctuaryImmune().String()))
	}
}

func TestRecipientCooldownDoesNotSkipWardSaves(t *testing.T) {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}
	attacker := monsters.NewWolf(wolfID).ToData()
	attacker.Conditions = []json.RawMessage{sanctuaryImmuneJSON(t, wolfID)}

	// A recipient cooldown must not bypass the aggressor's ward save.
	roller := &actionRoller{singles: []int{20, 15}, damage: [][]int{{4}}}
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
	require.Equal(t, 3, roller.calls, "ward save, attack and damage all roll")
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

// castFixtures is [CastActionTestSuite.fixtures]'s own trick, used directly
// here rather than inside a suite: ContestDamageTestSuite's world (bard,
// hero, wolf) and monster fixture are reused rather than rebuilt, and
// SetT lets its suite.Require() calls work outside suite.Run.
func castFixtures(t *testing.T) *ContestDamageTestSuite {
	t.Helper()
	f := &ContestDamageTestSuite{}
	f.SetT(t)
	f.ctx = context.Background()
	return f
}

func TestSanctuaryBlocksABaneTargetOnAFailedWardSave(t *testing.T) {
	fixtures := castFixtures(t)
	wolf := fixtures.wolfData()
	wolf.Conditions = []json.RawMessage{sanctuaryJSON(t, wolfID)}

	roller := &actionRoller{singles: []int{5}} // bard's WIS ward save: fails
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{},
		World: fixtures.world(),
		Participants: []Participant{
			{Monster: wolf}, {Character: baneCaster(1, 2)}, {Character: clericWarder()},
		},
		Machine: machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	outcome := out.Outcome.(CastOutcome)
	require.Len(t, outcome.Targets, 1)
	require.NotNil(t, outcome.Targets[0].Warded)
	require.Equal(t, "cleric-1", outcome.Targets[0].Warded.SourceID)
	require.Nil(t, outcome.Targets[0].Save, "Bane's own contest never ran")
	require.Empty(t, outcome.Targets[0].Applied)
	require.Equal(t, 1, roller.calls, "only the ward save was ever rolled")

	payer := fixtures.sheet(out, bardID)
	require.Zero(t, payer.ActionEconomy.ActionsRemaining, "the action is spent regardless of the ward")
	require.Equal(t, 1, payer.Resources[resources.SpellSlotLevel1].Current, "and so is the slot")
}

func TestSanctuarySaveSuccessDoesNotGrantCasterImmunity(t *testing.T) {
	fixtures := castFixtures(t)
	wolf := fixtures.wolfData()
	wolf.Conditions = []json.RawMessage{sanctuaryJSON(t, wolfID)}

	roller := &actionRoller{singles: []int{20, 10}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{},
		World: fixtures.world(),
		Participants: []Participant{
			{Monster: wolf}, {Character: baneCaster(1, 2)}, {Character: clericWarder()},
		},
		Machine: machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	outcome := out.Outcome.(CastOutcome)
	require.Len(t, outcome.Targets, 1)
	require.Nil(t, outcome.Targets[0].Warded)
	require.NotNil(t, outcome.Targets[0].Save, "Bane's own contest ran exactly as if there were no ward")

	require.False(t,
		hasConditionRef(t, fixtures.sheet(out, bardID).Conditions, refs.Conditions.SanctuaryImmune().String()),
		"a successful ward save must not grant the hostile caster the recipient's cooldown",
	)
}

func TestCastingABaneEndsTheCastersOwnSanctuary(t *testing.T) {
	fixtures := castFixtures(t)
	caster := baneCaster(1, 2)
	caster.Conditions = []json.RawMessage{sanctuaryJSON(t, bardID)}

	roller := &actionRoller{singles: []int{10}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{},
		World:        fixtures.world(),
		Participants: []Participant{{Monster: fixtures.wolfData()}, {Character: caster}},
		Machine:      machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	require.False(t,
		hasConditionRef(t, fixtures.sheet(out, bardID).Conditions, refs.Conditions.Sanctuary().String()),
		"the bard's own Sanctuary ended the moment it cast a hostile spell",
	)
}

func TestSanctuaryDoesNotGateANonHostileCast(t *testing.T) {
	fixtures := castFixtures(t)
	saver := fixtures.saver(14)
	saver.Conditions = []json.RawMessage{sanctuaryJSON(t, heroID)}

	roller := &actionRoller{singles: []int{10}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID}, Roller: roller,
	})
	require.NoError(t, err)
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{},
		World: fixtures.world(),
		Participants: []Participant{
			{Character: saver}, {Monster: fixtures.wolfData()}, {Character: baneCaster(1, 2)},
		},
		Machine: machine, Cost: baneCost(),
	})
	require.NoError(t, err)

	outcome := out.Outcome.(CastOutcome)
	require.Len(t, outcome.Targets, 1)
	require.Nil(t, outcome.Targets[0].Warded, "bard and hero share a faction: never hostile, never gated")
	require.NotNil(t, outcome.Targets[0].Save)
	require.Equal(t, 1, roller.calls, "just Bane's own save — no ward save attempted, and Bane deals no damage")
}
