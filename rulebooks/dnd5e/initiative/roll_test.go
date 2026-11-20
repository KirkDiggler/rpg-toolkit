// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package initiative_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func warmedUp() *core.Ref { return &core.Ref{Module: "dnd5e", Type: "conditions", ID: "warmed-up"} }

func dexterityMod(amount int) initiative.Modifier {
	return initiative.Modifier{
		Source: dnd5eEvents.RollSource{Ref: refs.Abilities.Dexterity(), Name: "Dexterity"},
		Amount: amount,
	}
}

func warmedUpMod(amount int) initiative.Modifier {
	return initiative.Modifier{
		Source: dnd5eEvents.RollSource{Ref: warmedUp(), Name: "Warmed up"},
		Amount: amount,
	}
}

func rollerReturning(t *testing.T, face int) *mock_dice.MockRoller {
	ctrl := gomock.NewController(t)
	roller := mock_dice.NewMockRoller(ctrl)
	roller.EXPECT().Roll(gomock.Any(), 20).Return(face, nil)
	return roller
}

// Catches a record that drops the die, loses a modifier's source, reorders the
// parts, or totals them differently from the roll.
func TestRollOneRecordsTheDieAndEveryModifier(t *testing.T) {
	out, err := initiative.RollOne(context.Background(), &initiative.RollOneInput{
		MemberID:  "alice",
		Modifiers: []initiative.Modifier{dexterityMod(2), warmedUpMod(1)},
		Roller:    rollerReturning(t, 14),
	})
	require.NoError(t, err)
	assert.Equal(t, 14, out.Face)
	assert.Equal(t, 17, out.Total)
	require.NotNil(t, out.Calculation)
	assert.Equal(t, 17, out.Calculation.Total)
	require.Len(t, out.Calculation.Components, 3)

	names := []string{}
	for _, c := range out.Calculation.Components {
		names = append(names, c.Source.Name)
	}
	assert.Equal(t, []string{"Initiative", "Dexterity", "Warmed up"}, names)

	die := out.Calculation.Components[0]
	require.NotNil(t, die.Dice)
	assert.Equal(t, 20, die.Dice.DieSize)
	assert.Equal(t, 14, die.Dice.Subtotal)
	assert.Nil(t, die.Modifier)
	assert.Equal(t, "dnd5e:rules:initiative", die.Source.Ref.String())
	assert.Equal(t, "alice", die.Source.SourceID)

	for i, want := range []int{2, 1} {
		c := out.Calculation.Components[i+1]
		assert.Nil(t, c.Dice)
		require.NotNil(t, c.Modifier)
		assert.Equal(t, want, *c.Modifier)
	}
}

// Catches a zero modifier dropped from the record, or a negative one added.
func TestRollOneKeepsAZeroAndANegativeModifier(t *testing.T) {
	out, err := initiative.RollOne(context.Background(), &initiative.RollOneInput{
		MemberID:  "alice",
		Modifiers: []initiative.Modifier{dexterityMod(0), warmedUpMod(-1)},
		Roller:    rollerReturning(t, 10),
	})
	require.NoError(t, err)
	assert.Equal(t, 9, out.Total)
	require.Len(t, out.Calculation.Components, 3)
	require.NotNil(t, out.Calculation.Components[1].Modifier)
	assert.Equal(t, 0, *out.Calculation.Components[1].Modifier)
	assert.Equal(t, -1, *out.Calculation.Components[2].Modifier)
}

// Catches a die-only roll that is not valid or adds something.
func TestRollOneWithNoModifiersIsTheDieAlone(t *testing.T) {
	out, err := initiative.RollOne(context.Background(), &initiative.RollOneInput{
		MemberID: "alice", Roller: rollerReturning(t, 7),
	})
	require.NoError(t, err)
	assert.Equal(t, out.Face, out.Total)
	assert.Equal(t, 7, out.Total)
	assert.Len(t, out.Calculation.Components, 1)
}

// Catches a refusal that is missing, a die spent on a roll that is then
// refused, or a roller's error that is swallowed.
func TestRollOneRefusals(t *testing.T) {
	ctrl := gomock.NewController(t)
	failing := mock_dice.NewMockRoller(ctrl)
	failing.EXPECT().Roll(gomock.Any(), 20).Return(0, errors.New("dice broke"))
	// No EXPECT: any roll through this roller fails the test.
	idle := mock_dice.NewMockRoller(ctrl)

	cases := map[string]*initiative.RollOneInput{
		"nil input":    nil,
		"blank member": {MemberID: "  ", Roller: idle},
		"nil roller":   {MemberID: "alice"},
		"no ref": {MemberID: "alice", Roller: idle, Modifiers: []initiative.Modifier{
			{Source: dnd5eEvents.RollSource{Name: "Dexterity"}, Amount: 1}}},
		"invalid ref": {MemberID: "alice", Roller: idle, Modifiers: []initiative.Modifier{
			{Source: dnd5eEvents.RollSource{Ref: &core.Ref{}, Name: "Dexterity"}, Amount: 1}}},
		"no name": {MemberID: "alice", Roller: idle, Modifiers: []initiative.Modifier{
			{Source: dnd5eEvents.RollSource{Ref: refs.Abilities.Dexterity()}, Amount: 1}}},
		"roller error": {MemberID: "alice", Roller: failing},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := initiative.RollOne(context.Background(), in)
			require.Error(t, err)
			assert.Nil(t, out)
		})
	}
}

// Catches a record that holds the caller's pointers, so a later edit rewrites
// what the roll said it was.
func TestRollOneDoesNotShareTheCallersSources(t *testing.T) {
	mod := warmedUpMod(1)
	out, err := initiative.RollOne(context.Background(), &initiative.RollOneInput{
		MemberID: "alice", Modifiers: []initiative.Modifier{mod}, Roller: rollerReturning(t, 5),
	})
	require.NoError(t, err)
	mod.Source.Ref.ID = "tampered"
	assert.Equal(t, "dnd5e:conditions:warmed-up", out.Calculation.Components[1].Source.Ref.String())
}
