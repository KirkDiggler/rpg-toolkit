// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
)

const secondSwingReason = "second attack of a multiattack"

var sequenceRef = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "test-multiattack"}

// twoClaws is the goblin boss's shape with the test roster's claw: two swings
// at one target, and only the second one penalised.
func twoClaws() combatActions.Definition {
	claw := validMeleeDefinition()

	return combatActions.Definition{
		Ref:  sequenceRef,
		Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: claw.Ref},
				{Action: claw.Ref, Disadvantage: secondSwingReason},
			},
		},
	}
}

// resolveSequenceAgainst drives one sequence over a hero the caller shapes,
// so a test that needs the target to drop can say so in hit points rather
// than in damage dice.
func resolveSequenceAgainst(
	t *testing.T, sequence combatActions.Definition, components []combatActions.Definition,
	hero *character.Data, roller dice.Roller,
) (*Output, error) {
	t.Helper()

	machine, err := NewAction(&ActionInput{
		Definition: sequence, AttackerID: wolfID, TargetID: heroID,
		Components: components, Roller: roller,
	})
	require.NoError(t, err)

	return Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: hero},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		TurnDriver: passDriver{}, Roller: dice.NewRoller(),
	})
}

func TestNewActionDispatchesTheSequenceArmToItsOwnMachine(t *testing.T) {
	machine, err := NewAction(&ActionInput{
		Definition: twoClaws(), AttackerID: wolfID, TargetID: heroID,
		Components: []combatActions.Definition{validMeleeDefinition()},
	})

	require.NoError(t, err)
	require.IsType(t, &sequenceMachine{}, machine)
}

// TestSequenceRollsOneD20PerStepAndOnlyTheDeclaredOneKeepsLowest is the whole
// mechanism in one run: two swings, in order, and the disadvantage lands on
// exactly the step that declared it — naming the sequence, not the claw.
func TestSequenceRollsOneD20PerStepAndOnlyTheDeclaredOneKeepsLowest(t *testing.T) {
	// 18 lands the first blow. The pair is the second step's: 4 is kept over
	// 17, which is what turns a hit into a miss and makes the keep rule
	// decide something rather than merely be recorded.
	roller := &actionRoller{singles: []int{18}, pairs: [][]int{{17, 4}}, damage: [][]int{{5}}}

	out, err := resolveSequenceAgainst(t, twoClaws(),
		[]combatActions.Definition{validMeleeDefinition(), twoClaws()}, actionHero(), roller)
	require.NoError(t, err)

	sequence, ok := out.Outcome.(SequenceOutcome)
	require.True(t, ok, "the sequence arm answers with its own outcome, not a strike's")
	require.Equal(t, sequenceRef, sequence.Action, "the outcome names the script, not the claw")
	require.Equal(t, wolfID, sequence.AttackerID)
	require.Equal(t, heroID, sequence.TargetID)
	require.Len(t, sequence.Steps, 2)
	require.Zero(t, sequence.Unswung)

	first := sequence.Steps[0]
	require.Equal(t, 18, first.Roll)
	require.True(t, first.Hit)
	require.Empty(t, first.Folded.DisadvantageSources,
		"the first swing of a multiattack is an ordinary attack")
	firstDice := first.Calculation.Components[0].Dice
	require.Equal(t, []int{18}, firstDice.OriginalRolls, "one die, because nothing was imposed on it")
	require.Nil(t, firstDice.Keep, "no rule met on this pool, and the zero value says so")

	second := sequence.Steps[1]
	require.Equal(t, 4, second.Roll, "the low face is the one that counts")
	require.False(t, second.Hit)
	require.Len(t, second.Folded.DisadvantageSources, 1)
	imposed := second.Folded.DisadvantageSources[0]
	require.Equal(t, secondSwingReason, imposed.Reason)
	require.Equal(t, wolfID, imposed.SourceID, "whose die it is")
	require.Equal(t, &sequenceRef, imposed.SourceRef, "and whose rule threw it — the script, not the claw")

	secondDice := second.Calculation.Components[0].Dice
	require.Equal(t, []int{17, 4}, secondDice.OriginalRolls)
	require.Equal(t, []int{1}, secondDice.KeptIndices)
	require.NotNil(t, secondDice.Keep)
	require.Equal(t, dnd5eEvents.KeepDisadvantage, secondDice.Keep.Rule)
	require.Len(t, secondDice.Keep.Imposed, 1)
	require.Equal(t, secondSwingReason, secondDice.Keep.Imposed[0].Name,
		"the reason the content declared is the name a player reads on the keep record")

	require.Empty(t, roller.singles, "the first step rolled its single d20")
	require.Empty(t, roller.pairs, "the second step rolled its pair")
	require.Empty(t, roller.damage, "one hit, one damage pool")
}

// TestSequenceStepsPreserveTheDeclaredOrder pins the ordering with two
// DIFFERENT components, which the two-scimitar case structurally cannot: a
// machine that ran the steps backwards would pass every assertion above.
func TestSequenceStepsPreserveTheDeclaredOrder(t *testing.T) {
	bite := validMeleeDefinition()
	bite.Ref = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "test-bite"}
	bite.Name = "Test Bite"
	bite.Attack.Damage = []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 1}}
	claw := validMeleeDefinition()

	sequence := combatActions.Definition{
		Ref:  sequenceRef,
		Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{{Action: bite.Ref}, {Action: claw.Ref}},
		},
	}

	roller := &actionRoller{singles: []int{19, 3}, damage: [][]int{{2, 2}}}
	out, err := resolveSequenceAgainst(t, sequence,
		[]combatActions.Definition{sequence, claw, bite}, actionHero(), roller)
	require.NoError(t, err)

	sequenceOut := out.Outcome.(SequenceOutcome)
	require.Len(t, sequenceOut.Steps, 2)
	require.Equal(t, 19, sequenceOut.Steps[0].Roll, "the bite is declared first and rolls first")
	require.True(t, sequenceOut.Steps[0].Hit)
	require.Equal(t, 5, sequenceOut.Steps[0].Damage, "2d4+1 rolled 2 and 2")
	require.Equal(t, 3, sequenceOut.Steps[1].Roll)
	require.False(t, sequenceOut.Steps[1].Hit)
}

// TestAMissDoesNotEndASequence is half the stop rule. "The goblin makes two
// attacks with its scimitar" is two swings whatever the first one rolled, and
// a machine that quit on a miss would quietly halve every boss in the roster.
func TestAMissDoesNotEndASequence(t *testing.T) {
	roller := &actionRoller{singles: []int{2}, pairs: [][]int{{15, 14}}, damage: [][]int{{6}}}

	out, err := resolveSequenceAgainst(t, twoClaws(),
		[]combatActions.Definition{validMeleeDefinition(), twoClaws()}, actionHero(), roller)
	require.NoError(t, err)

	sequence := out.Outcome.(SequenceOutcome)
	require.Len(t, sequence.Steps, 2, "the second swing happened")
	require.False(t, sequence.Steps[0].Hit)
	require.True(t, sequence.Steps[1].Hit)
	require.Zero(t, sequence.Unswung, "nothing was cancelled, so nothing is reported as cancelled")
}

// TestADownedTargetEndsASequence is the other half, and the roller is the
// assertion: nothing is scripted for a second swing, so a machine that took
// one would fail rather than roll something plausible.
func TestADownedTargetEndsASequence(t *testing.T) {
	hero := actionHero()
	hero.HitPoints = 4

	roller := &actionRoller{singles: []int{18}, damage: [][]int{{6}}}

	out, err := resolveSequenceAgainst(t, twoClaws(),
		[]combatActions.Definition{validMeleeDefinition(), twoClaws()}, hero, roller)
	require.NoError(t, err)

	sequence := out.Outcome.(SequenceOutcome)
	require.Len(t, sequence.Steps, 1)
	require.True(t, sequence.Steps[0].Hit)
	require.Equal(t, 8, sequence.Steps[0].Damage, "1d6+2 rolled 6, against four hit points")
	require.Equal(t, 1, sequence.Unswung,
		"the swing that never came is reported, not left to be reconstructed")
	require.Empty(t, roller.pairs, "no second d20 was ever asked for")
}

// TestSequencePreflightsEveryStepBeforeAnythingIsRolled is why the components
// are resolved and started at the door: half a multiattack cannot be taken
// back, so a script with an unreachable step is refused whole.
func TestSequencePreflightsEveryStepBeforeAnythingIsRolled(t *testing.T) {
	// A sixty-foot bite the hero is well inside, and a five-foot claw they
	// are two cells outside. The FIRST step could run; the second could not.
	// That asymmetry is the test — a machine that preflighted lazily would
	// roll the bite and then discover the claw.
	bite := validMeleeDefinition()
	bite.Ref = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "test-long-bite"}
	bite.Name = "Test Long Bite"
	bite.Attack.Delivery = combatActions.AttackDelivery{
		Ranged: &combatActions.RangedDelivery{NormalFeet: 60},
	}
	claw := validMeleeDefinition()

	sequence := combatActions.Definition{
		Ref:  sequenceRef,
		Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{{Action: bite.Ref}, {Action: claw.Ref}},
		},
	}

	roller := &actionRoller{singles: []int{18}, damage: [][]int{{5}}}
	machine, err := NewAction(&ActionInput{
		Definition: sequence, AttackerID: wolfID, TargetID: heroID,
		Components: []combatActions.Definition{sequence, bite, claw}, Roller: roller,
	})
	require.NoError(t, err)

	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 3),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: actionHero()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		TurnDriver: passDriver{}, Roller: dice.NewRoller(),
	})

	require.ErrorIs(t, err, ErrOutOfRange)
	require.ErrorContains(t, err, "step 1", "the reachable first step is not what was refused")
	require.Nil(t, out)
	require.Zero(t, roller.calls, "nothing was rolled for a script that could not finish")
}

func TestSequenceRefusals(t *testing.T) {
	claw := validMeleeDefinition()

	t.Run("a step the actor does not carry", func(t *testing.T) {
		_, err := NewAction(&ActionInput{
			Definition: twoClaws(), AttackerID: wolfID, TargetID: heroID,
			Components: []combatActions.Definition{twoClaws()},
		})
		require.ErrorIs(t, err, ErrBadAction)
		require.ErrorContains(t, err, "which the actor does not carry")
	})

	t.Run("a step that is itself a sequence", func(t *testing.T) {
		inner := twoClaws()
		inner.Ref = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "test-inner-multiattack"}
		outer := combatActions.Definition{
			Ref:  sequenceRef,
			Name: "Multiattack",
			Sequence: &combatActions.SequenceProfile{
				Steps: []combatActions.SequenceStep{{Action: inner.Ref}, {Action: claw.Ref}},
			},
		}

		_, err := NewAction(&ActionInput{
			Definition: outer, AttackerID: wolfID, TargetID: heroID,
			Components: []combatActions.Definition{outer, inner, claw},
		})
		require.ErrorIs(t, err, ErrBadAction)
		require.ErrorContains(t, err, "which is itself a sequence")
	})

	t.Run("no attacker", func(t *testing.T) {
		_, err := NewAction(&ActionInput{
			Definition: twoClaws(), TargetID: heroID,
			Components: []combatActions.Definition{twoClaws(), claw},
		})
		require.ErrorIs(t, err, ErrBadAction)
		require.ErrorContains(t, err, "performed by nobody")
	})

	t.Run("two targets", func(t *testing.T) {
		_, err := NewAction(&ActionInput{
			Definition: twoClaws(), AttackerID: wolfID, TargetIDs: []string{heroID, wolfID},
			Components: []combatActions.Definition{twoClaws(), claw},
		})
		require.ErrorIs(t, err, ErrBadAction)
		require.ErrorContains(t, err, "requires exactly one target")
	})

	t.Run("no roller", func(t *testing.T) {
		machine, err := NewAction(&ActionInput{
			Definition: twoClaws(), AttackerID: wolfID, TargetID: heroID,
			Components: []combatActions.Definition{twoClaws(), claw},
		})
		require.NoError(t, err)
		_, err = machine.Start(context.Background(), nil)
		require.ErrorIs(t, err, ErrNoRoller)
	})
}
