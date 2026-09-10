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

func TestResolveContributionsPreservesPhysicalFacesAndOperator(t *testing.T) {
	ctrl := gomock.NewController(t)
	roller := mock_dice.NewMockRoller(ctrl)
	roller.EXPECT().RollN(gomock.Any(), 1, 4).Return([]int{3}, nil)

	output, err := rolls.ResolveContributions(context.Background(), &rolls.ResolveContributionsInput{
		Roller: roller,
		Contributions: []dnd5eEvents.DiceContribution{{
			Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a",
			},
			Dice: "1d4", Subtract: true,
		}},
	})
	require.NoError(t, err)
	require.Len(t, output.Components, 1)
	require.Equal(t, []int{3}, output.Components[0].Dice.OriginalRolls)
	require.Equal(t, []int{3}, output.Components[0].Dice.FinalRolls)
	require.Equal(t, 3, output.Components[0].Dice.Subtotal)
	require.True(t, output.Components[0].SubtractDice)
	require.Equal(t, "bard-a", output.Components[0].Source.SourceID)
}

func TestResolveContributionsValidatesEveryDescriptionBeforeRolling(t *testing.T) {
	ctrl := gomock.NewController(t)
	roller := mock_dice.NewMockRoller(ctrl)

	output, err := rolls.ResolveContributions(context.Background(), &rolls.ResolveContributionsInput{
		Roller: roller,
		Contributions: []dnd5eEvents.DiceContribution{
			{
				Source: dnd5eEvents.RollSource{Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a"},
				Dice:   "1d4", Subtract: true,
			},
			{
				Source: dnd5eEvents.RollSource{Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-b"},
				Dice:   "-1d4", Subtract: true,
			},
		},
	})
	require.Error(t, err)
	require.Nil(t, output)
}
