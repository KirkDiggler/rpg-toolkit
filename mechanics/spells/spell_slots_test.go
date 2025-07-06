// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	coremock "github.com/KirkDiggler/rpg-toolkit/core/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	eventsmock "github.com/KirkDiggler/rpg-toolkit/events/mock"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells/mock"
)

func TestSimpleSpellSlotTable(t *testing.T) {
	// Test a simple table with fixed slots
	slots := map[int]map[int]int{
		1: {1: 2, 2: 0, 3: 0}, // Level 1: 2 1st level slots
		2: {1: 3, 2: 0, 3: 0}, // Level 2: 3 1st level slots
		3: {1: 4, 2: 2, 3: 0}, // Level 3: 4 1st, 2 2nd level slots
	}

	table := spells.NewSimpleSpellSlotTable(slots)

	tests := []struct {
		classLevel int
		spellLevel int
		expected   int
	}{
		{1, 1, 2},
		{1, 2, 0},
		{2, 1, 3},
		{3, 1, 4},
		{3, 2, 2},
		{3, 3, 0},
		{4, 1, 0}, // No data for level 4
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := table.GetSlots(tt.classLevel, tt.spellLevel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSpellSlotPool_Creation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOwner := coremock.NewMockEntity(ctrl)
	mockTable := mock.NewMockSpellSlotTable(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()

	// Mock owner GetID calls - resources.NewSimplePool calls it
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()

	// Mock the table to return slots for a level 3 wizard
	mockTable.EXPECT().GetSlots(3, 1).Return(4).Times(1)
	mockTable.EXPECT().GetSlots(3, 2).Return(2).Times(1)
	mockTable.EXPECT().GetSlots(3, 3).Return(0).Times(1)
	mockTable.EXPECT().GetSlots(3, 4).Return(0).Times(1)
	mockTable.EXPECT().GetSlots(3, 5).Return(0).Times(1)
	mockTable.EXPECT().GetSlots(3, 6).Return(0).Times(1)
	mockTable.EXPECT().GetSlots(3, 7).Return(0).Times(1)
	mockTable.EXPECT().GetSlots(3, 8).Return(0).Times(1)
	mockTable.EXPECT().GetSlots(3, 9).Return(0).Times(1)

	pool := spells.NewSpellSlotPool(mockOwner, "wizard", 3, mockTable)
	require.NotNil(t, pool)

	// Check that resources were created
	slots := pool.GetAvailableSlots()
	assert.Equal(t, 4, slots[1])
	assert.Equal(t, 2, slots[2])
	assert.Equal(t, 0, slots[3]) // No 3rd level slots
}

func TestSpellSlotPool_UseSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOwner := coremock.NewMockEntity(ctrl)
	mockTable := mock.NewMockSpellSlotTable(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()
	mockBus := eventsmock.NewMockEventBus(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()

	// Setup table expectations
	for i := 1; i <= 9; i++ {
		if i <= 2 {
			mockTable.EXPECT().GetSlots(3, i).Return(2).Times(1)
		} else {
			mockTable.EXPECT().GetSlots(3, i).Return(0).Times(1)
		}
	}

	pool := spells.NewSpellSlotPool(mockOwner, "wizard", 3, mockTable)

	// Expect resource consumed event
	mockBus.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, event events.Event) error {
			assert.Equal(t, "resource.consumed", event.Type())
			return nil
		},
	).Times(1)

	// Use a 1st level slot
	err := pool.UseSlot(1, mockBus)
	require.NoError(t, err)

	// Check remaining slots
	slots := pool.GetAvailableSlots()
	assert.Equal(t, 1, slots[1])
	assert.Equal(t, 2, slots[2])
}

func TestSpellSlotPool_UseSlot_Insufficient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOwner := coremock.NewMockEntity(ctrl)
	mockTable := mock.NewMockSpellSlotTable(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()
	mockBus := eventsmock.NewMockEventBus(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()

	// Setup table with no 3rd level slots
	for i := 1; i <= 9; i++ {
		mockTable.EXPECT().GetSlots(1, i).Return(0).Times(1)
	}

	pool := spells.NewSpellSlotPool(mockOwner, "wizard", 1, mockTable)

	// Try to use a 3rd level slot
	err := pool.UseSlot(3, mockBus)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no spell slots")
}

func TestSpellSlotPool_RestoreSlots(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOwner := coremock.NewMockEntity(ctrl)
	mockTable := mock.NewMockSpellSlotTable(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()
	mockBus := eventsmock.NewMockEventBus(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()

	// Setup table expectations
	for i := 1; i <= 9; i++ {
		if i == 1 {
			mockTable.EXPECT().GetSlots(2, i).Return(3).Times(1)
		} else {
			mockTable.EXPECT().GetSlots(2, i).Return(0).Times(1)
		}
	}

	pool := spells.NewSpellSlotPool(mockOwner, "wizard", 2, mockTable)

	// Use all slots first
	mockBus.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil).Times(3)
	for i := 0; i < 3; i++ {
		err := pool.UseSlot(1, mockBus)
		require.NoError(t, err)
	}

	// Verify all slots are used
	slots := pool.GetAvailableSlots()
	assert.Equal(t, 0, slots[1])

	// Restore all slots on long rest
	mockBus.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, event events.Event) error {
			assert.Equal(t, "resource.restored", event.Type())
			return nil
		},
	).Times(1)

	pool.RestoreSlots("long_rest", mockBus)

	// Verify all slots are restored
	slots = pool.GetAvailableSlots()
	assert.Equal(t, 3, slots[1])
}

func TestSpellSlotPool_HasSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOwner := coremock.NewMockEntity(ctrl)
	mockTable := mock.NewMockSpellSlotTable(ctrl)

	// Mock owner GetID calls
	mockOwner.EXPECT().GetID().Return("wizard-1").AnyTimes()

	// Setup table expectations
	for i := 1; i <= 9; i++ {
		if i <= 2 {
			mockTable.EXPECT().GetSlots(3, i).Return(2).Times(1)
		} else {
			mockTable.EXPECT().GetSlots(3, i).Return(0).Times(1)
		}
	}

	pool := spells.NewSpellSlotPool(mockOwner, "wizard", 3, mockTable)

	assert.True(t, pool.HasSlot(1))
	assert.True(t, pool.HasSlot(2))
	assert.False(t, pool.HasSlot(3))
	assert.False(t, pool.HasSlot(9))
	assert.False(t, pool.HasSlot(10)) // Out of range
}
