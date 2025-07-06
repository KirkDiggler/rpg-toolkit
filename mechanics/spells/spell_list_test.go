// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells/mock"
)

func TestSimpleSpellList_Cantrips(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	list := spells.NewSimpleSpellList(spells.SpellListConfig{
		MaxPreparedSpells: 5,
		PreparationStyle:  spells.PreparationStyleKnown,
	})

	// Create mock cantrips
	cantrip1 := mock.NewMockSpell(ctrl)
	cantrip1.EXPECT().GetID().Return("light").AnyTimes()
	cantrip1.EXPECT().Level().Return(0).AnyTimes()

	cantrip2 := mock.NewMockSpell(ctrl)
	cantrip2.EXPECT().GetID().Return("mage_hand").AnyTimes()
	cantrip2.EXPECT().Level().Return(0).AnyTimes()

	// Add cantrips
	err := list.AddCantrip(cantrip1)
	require.NoError(t, err)

	err = list.AddCantrip(cantrip2)
	require.NoError(t, err)

	// Check cantrips
	cantrips := list.GetCantrips()
	assert.Len(t, cantrips, 2)

	// Cantrips should be castable
	assert.True(t, list.CanCast("light"))
	assert.True(t, list.CanCast("mage_hand"))

	// Test duplicate cantrip
	err = list.AddCantrip(cantrip1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already known")
}

func TestSimpleSpellList_KnownSpells(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	list := spells.NewSimpleSpellList(spells.SpellListConfig{
		MaxPreparedSpells: 5,
		PreparationStyle:  spells.PreparationStyleKnown,
	})

	// Create mock spells
	spell1 := mock.NewMockSpell(ctrl)
	spell1.EXPECT().GetID().Return("magic_missile").AnyTimes()
	spell1.EXPECT().Level().Return(1).AnyTimes()

	spell2 := mock.NewMockSpell(ctrl)
	spell2.EXPECT().GetID().Return("shield").AnyTimes()
	spell2.EXPECT().Level().Return(1).AnyTimes()

	// Add known spells
	err := list.AddKnownSpell(spell1)
	require.NoError(t, err)

	err = list.AddKnownSpell(spell2)
	require.NoError(t, err)

	// Check known spells
	known := list.GetKnownSpells()
	assert.Len(t, known, 2)
	assert.True(t, list.IsKnown("magic_missile"))
	assert.True(t, list.IsKnown("shield"))

	// For known style, spells should be castable
	assert.True(t, list.CanCast("magic_missile"))
	assert.True(t, list.CanCast("shield"))

	// Remove a spell
	err = list.RemoveKnownSpell("shield")
	require.NoError(t, err)
	assert.False(t, list.IsKnown("shield"))
	assert.False(t, list.CanCast("shield"))

	// Test removing non-existent spell
	err = list.RemoveKnownSpell("fireball")
	assert.Error(t, err)
}

func TestSimpleSpellList_PreparedSpells(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	list := spells.NewSimpleSpellList(spells.SpellListConfig{
		MaxPreparedSpells: 3,
		PreparationStyle:  spells.PreparationStylePrepared,
	})

	// Create mock spells
	spell1 := mock.NewMockSpell(ctrl)
	spell1.EXPECT().GetID().Return("cure_wounds").AnyTimes()
	spell1.EXPECT().Level().Return(1).AnyTimes()

	spell2 := mock.NewMockSpell(ctrl)
	spell2.EXPECT().GetID().Return("bless").AnyTimes()
	spell2.EXPECT().Level().Return(1).AnyTimes()

	spell3 := mock.NewMockSpell(ctrl)
	spell3.EXPECT().GetID().Return("healing_word").AnyTimes()
	spell3.EXPECT().Level().Return(1).AnyTimes()

	spell4 := mock.NewMockSpell(ctrl)
	spell4.EXPECT().GetID().Return("shield_of_faith").AnyTimes()
	spell4.EXPECT().Level().Return(1).AnyTimes()

	// First add spells to known list
	err := list.AddKnownSpell(spell1)
	require.NoError(t, err)
	err = list.AddKnownSpell(spell2)
	require.NoError(t, err)
	err = list.AddKnownSpell(spell3)
	require.NoError(t, err)
	err = list.AddKnownSpell(spell4)
	require.NoError(t, err)

	// For prepared style, known spells are NOT castable until prepared
	assert.False(t, list.CanCast("cure_wounds"))

	// Prepare spells
	err = list.PrepareSpell(spell1)
	require.NoError(t, err)
	err = list.PrepareSpell(spell2)
	require.NoError(t, err)
	err = list.PrepareSpell(spell3)
	require.NoError(t, err)

	// Check prepared spells
	prepared := list.GetPreparedSpells()
	assert.Len(t, prepared, 3)
	assert.True(t, list.IsPrepared("cure_wounds"))
	assert.True(t, list.CanCast("cure_wounds"))

	// Try to prepare more than max
	err = list.PrepareSpell(spell4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max prepared spells")

	// Unprepare a spell
	err = list.UnprepareSpell("bless")
	require.NoError(t, err)
	assert.False(t, list.IsPrepared("bless"))

	// Now can prepare another
	err = list.PrepareSpell(spell4)
	require.NoError(t, err)
}

func TestSimpleSpellList_GetSpell(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	list := spells.NewSimpleSpellList(spells.SpellListConfig{
		MaxPreparedSpells: 5,
		PreparationStyle:  spells.PreparationStyleKnown,
	})

	// Create mock spell
	spell := mock.NewMockSpell(ctrl)
	spell.EXPECT().GetID().Return("fireball").AnyTimes()
	spell.EXPECT().Level().Return(3).AnyTimes()

	// Add spell
	err := list.AddKnownSpell(spell)
	require.NoError(t, err)

	// Get spell
	retrieved, ok := list.GetSpell("fireball")
	assert.True(t, ok)
	assert.Equal(t, spell, retrieved)

	// Get non-existent spell
	_, ok = list.GetSpell("meteor_swarm")
	assert.False(t, ok)
}

func TestSimpleSpellList_PreparationStyles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test that preparation style affects CanCast behavior
	knownList := spells.NewSimpleSpellList(spells.SpellListConfig{
		MaxPreparedSpells: 5,
		PreparationStyle:  spells.PreparationStyleKnown,
	})
	preparedList := spells.NewSimpleSpellList(spells.SpellListConfig{
		MaxPreparedSpells: 5,
		PreparationStyle:  spells.PreparationStylePrepared,
	})

	spell := mock.NewMockSpell(ctrl)
	spell.EXPECT().GetID().Return("magic_missile").AnyTimes()
	spell.EXPECT().Level().Return(1).AnyTimes()

	// Add to both lists
	err := knownList.AddKnownSpell(spell)
	require.NoError(t, err)
	err = preparedList.AddKnownSpell(spell)
	require.NoError(t, err)

	// Known style: can cast known spells
	assert.True(t, knownList.CanCast("magic_missile"))

	// Prepared style: cannot cast until prepared
	assert.False(t, preparedList.CanCast("magic_missile"))

	// Prepare the spell
	err = preparedList.PrepareSpell(spell)
	require.NoError(t, err)
	assert.True(t, preparedList.CanCast("magic_missile"))
}
