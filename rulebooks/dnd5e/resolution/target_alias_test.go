// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func resolveTargetAliasAttack(
	t *testing.T, targetID string, targetIDs []string,
) StrikeOutcome {
	t.Helper()

	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(),
		AttackerID: wolfID,
		TargetID:   targetID,
		TargetIDs:  targetIDs,
		Roller:     &actionRoller{singles: []int{15}, damage: [][]int{{3}}},
	})
	require.NoError(t, err)

	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: actionHero()},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
	})
	require.NoError(t, err)

	return out.Outcome.(StrikeOutcome)
}

func resolveTargetAliasCast(
	t *testing.T, targetID string, targetIDs []string,
) CastOutcome {
	t.Helper()

	definition := spells.CastDefinition(spells.CastDefinitionInput{
		Spell: spells.ViciousMockery, SpellSaveDC: spellSaveDC,
	})
	require.NotNil(t, definition)
	machine, err := NewAction(&ActionInput{
		Definition: *definition,
		AttackerID: bardID,
		TargetID:   targetID,
		TargetIDs:  targetIDs,
		Roller:     facedRoller{d20: straightRoll, other: psychicFace},
	})
	require.NoError(t, err)

	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(t)
	fixtures.ctx = context.Background()
	out, err := fixtures.resolve(fixtures.saver(14), machine, nil, fixtures.bard(1))
	require.NoError(t, err)

	return out.Outcome.(CastOutcome)
}

func TestActionTargetAliasAndCanonicalFormsResolveEquivalently(t *testing.T) {
	t.Run("attack", func(t *testing.T) {
		legacy := resolveTargetAliasAttack(t, heroID, nil)
		canonical := resolveTargetAliasAttack(t, "", []string{heroID})

		require.Equal(t, legacy, canonical)
	})

	t.Run("cast", func(t *testing.T) {
		legacy := resolveTargetAliasCast(t, heroID, nil)
		canonical := resolveTargetAliasCast(t, "", []string{heroID})

		require.Equal(t, legacy, canonical)
	})
}

func TestNewActionRejectsConflictingTargetFormsBeforeRNG(t *testing.T) {
	for _, tc := range []struct {
		name       string
		definition combatActions.Definition
	}{
		{name: "attack", definition: validMeleeDefinition()},
		{
			name: "cast",
			definition: *spells.CastDefinition(spells.CastDefinitionInput{
				Spell: spells.ViciousMockery, SpellSaveDC: spellSaveDC,
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := []string{heroID}
			roller := &countingCastRoller{facedRoller: facedRoller{d20: straightRoll, other: psychicFace}}

			machine, err := NewAction(&ActionInput{
				Definition: tc.definition,
				AttackerID: wolfID,
				TargetID:   heroID,
				TargetIDs:  targets,
				Roller:     roller,
			})

			require.ErrorIs(t, err, ErrBadAction)
			require.Nil(t, machine)
			require.Contains(t, err.Error(), "TargetID")
			require.Contains(t, err.Error(), "TargetIDs")
			require.Zero(t, roller.calls)
			require.Equal(t, []string{heroID}, targets)
		})
	}
}

func TestNewActionValidatesAttackTargetCardinalityBeforeRNG(t *testing.T) {
	for _, tc := range []struct {
		name    string
		targets []string
	}{
		{name: "no target"},
		{name: "empty target", targets: []string{""}},
		{name: "multiple targets", targets: []string{heroID, wolfID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roller := &countingCastRoller{facedRoller: facedRoller{d20: straightRoll}}
			before := append([]string(nil), tc.targets...)

			machine, err := NewAction(&ActionInput{
				Definition: validMeleeDefinition(), AttackerID: wolfID,
				TargetIDs: tc.targets, Roller: roller,
			})

			require.ErrorIs(t, err, ErrBadAction)
			require.Nil(t, machine)
			require.Zero(t, roller.calls)
			require.Equal(t, before, tc.targets)
		})
	}
}

func TestNewActionCopiesCanonicalTargets(t *testing.T) {
	t.Run("attack", func(t *testing.T) {
		targets := []string{heroID}
		machine, err := NewAction(&ActionInput{
			Definition: validMeleeDefinition(), AttackerID: wolfID,
			TargetIDs: targets, Roller: &actionRoller{},
		})
		require.NoError(t, err)
		targets[0] = wolfID

		strike := machine.(*strikeMachine)
		require.Equal(t, heroID, strike.in.TargetID)
	})

	t.Run("cast", func(t *testing.T) {
		targets := []string{heroID}
		definition := spells.CastDefinition(spells.CastDefinitionInput{
			Spell: spells.TrueStrike, SpellSaveDC: spellSaveDC,
		})
		require.NotNil(t, definition)

		machine, err := NewAction(&ActionInput{
			Definition: *definition, AttackerID: bardID, TargetIDs: targets,
		})
		require.NoError(t, err)
		targets[0] = wolfID

		cast := machine.(*castMachine)
		require.Equal(t, heroID, cast.targets[0].targetID)
	})
}
