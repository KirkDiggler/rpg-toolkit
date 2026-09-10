// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package rolls_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
)

func TestResolveContributionsIsContentAgnosticAndPreservesPhysicalFacts(t *testing.T) {
	ctrl := gomock.NewController(t)
	roller := mock_dice.NewMockRoller(ctrl)
	gomock.InOrder(
		roller.EXPECT().RollN(gomock.Any(), 1, 4).Return([]int{3}, nil),
		roller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{5}, nil),
	)

	output, err := rolls.ResolveContributions(context.Background(), &rolls.ResolveContributionsInput{
		Roller: roller,
		Contributions: []dnd5eEvents.DiceContribution{
			{
				Source: dnd5eEvents.RollSource{
					Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a",
				},
				Dice: "1d4", Subtract: true,
			},
			{
				Source: dnd5eEvents.RollSource{
					Ref: refs.Spells.Bless(), Name: "Bless", SourceID: "cleric-a",
				},
				Dice: "1d6",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, output.Components, 2)
	require.Equal(t, []int{3}, output.Components[0].Dice.OriginalRolls)
	require.Equal(t, []int{3}, output.Components[0].Dice.FinalRolls)
	require.Equal(t, 3, output.Components[0].Dice.Subtotal)
	require.True(t, output.Components[0].SubtractDice)
	require.Equal(t, "bard-a", output.Components[0].Source.SourceID)
	require.Equal(t, []int{5}, output.Components[1].Dice.FinalRolls)
	require.False(t, output.Components[1].SubtractDice)
	require.Equal(t, "cleric-a", output.Components[1].Source.SourceID)
}

func TestResolveContributionsRequiresTheOperationRoller(t *testing.T) {
	output, err := rolls.ResolveContributions(context.Background(), &rolls.ResolveContributionsInput{
		Contributions: []dnd5eEvents.DiceContribution{{
			Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.Bless(), Name: "Bless", SourceID: "cleric-a",
			},
			Dice: "1d4",
		}},
	})
	require.ErrorContains(t, err, "roller is required")
	require.Nil(t, output)
}

func TestResolveContributionsRequiresResponsibleEntityForEveryDescription(t *testing.T) {
	ctrl := gomock.NewController(t)
	roller := mock_dice.NewMockRoller(ctrl)
	output, err := rolls.ResolveContributions(context.Background(), &rolls.ResolveContributionsInput{
		Roller: roller,
		Contributions: []dnd5eEvents.DiceContribution{{
			Source: dnd5eEvents.RollSource{Ref: refs.Spells.Bless(), Name: "Bless"},
			Dice:   "1d4",
		}},
	})
	require.ErrorContains(t, err, "source id is required")
	require.Nil(t, output)
}

func TestResolveContributionsValidatesEveryDescriptionBeforeRolling(t *testing.T) {
	ctrl := gomock.NewController(t)
	roller := mock_dice.NewMockRoller(ctrl)

	output, err := rolls.ResolveContributions(context.Background(), &rolls.ResolveContributionsInput{
		Roller: roller,
		Contributions: []dnd5eEvents.DiceContribution{
			{
				Source: dnd5eEvents.RollSource{Ref: refs.Spells.Bless(), Name: "Bless", SourceID: "cleric-a"},
				Dice:   "1d4",
			},
			{
				Source: dnd5eEvents.RollSource{
					Ref: refs.Conditions.ViciousMockery(), Name: "Vicious Mockery", SourceID: "bard-b",
				},
				Dice: "-1d4", Subtract: true,
			},
		},
	})
	require.Error(t, err)
	require.Nil(t, output)
}
