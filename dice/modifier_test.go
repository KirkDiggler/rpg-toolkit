// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"strings"
	"testing"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"go.uber.org/mock/gomock"
)

func TestRoll_GetValue(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		size     int
		rolls    []int
		expected int
	}{
		{
			name:     "single d6",
			count:    1,
			size:     6,
			rolls:    []int{4},
			expected: 4,
		},
		{
			name:     "multiple dice",
			count:    3,
			size:     6,
			rolls:    []int{1, 4, 6},
			expected: 11,
		},
		{
			name:     "negative count",
			count:    -2,
			size:     6,
			rolls:    []int{3, 5},
			expected: -8,
		},
		{
			name:     "zero dice",
			count:    0,
			size:     6,
			rolls:    []int{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRoller := mock_dice.NewMockRoller(ctrl)
			
			if len(tt.rolls) > 0 {
				mockRoller.EXPECT().RollN(gomock.Any(), tt.size).Return(tt.rolls)
			}

			roll := NewRollWithRoller(tt.count, tt.size, mockRoller)

			// First call should roll
			result := roll.GetValue()
			if result != tt.expected {
				t.Errorf("GetValue() = %d, want %d", result, tt.expected)
			}

			// Second call should return same cached value
			result2 := roll.GetValue()
			if result2 != result {
				t.Errorf("Second GetValue() = %d, want %d (cached)", result2, result)
			}
		})
	}
}

func TestRoll_GetDescription(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		size     int
		rolls    []int
		expected string
	}{
		{
			name:     "single d6",
			count:    1,
			size:     6,
			rolls:    []int{4},
			expected: "+d6[4]=4",
		},
		{
			name:     "multiple dice",
			count:    2,
			size:     8,
			rolls:    []int{3, 7},
			expected: "+2d8[3,7]=10",
		},
		{
			name:     "negative single",
			count:    -1,
			size:     4,
			rolls:    []int{3},
			expected: "-d4[3]=-3",
		},
		{
			name:     "negative multiple",
			count:    -3,
			size:     6,
			rolls:    []int{2, 4, 1},
			expected: "-3d6[2,4,1]=-7",
		},
		{
			name:     "zero dice",
			count:    0,
			size:     6,
			rolls:    []int{},
			expected: "+0d6[]=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRoller := mock_dice.NewMockRoller(ctrl)
			
			if len(tt.rolls) > 0 {
				mockRoller.EXPECT().RollN(gomock.Any(), tt.size).Return(tt.rolls)
			}

			roll := NewRollWithRoller(tt.count, tt.size, mockRoller)

			description := roll.GetDescription()
			if description != tt.expected {
				t.Errorf("GetDescription() = %q, want %q", description, tt.expected)
			}
		})
	}
}

func TestRoll_CachedBehavior(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRoller := mock_dice.NewMockRoller(ctrl)
	// Expect only one call to RollN, proving caching works
	mockRoller.EXPECT().RollN(2, 6).Return([]int{1, 2}).Times(1)

	roll := NewRollWithRoller(2, 6, mockRoller)

	// First GetDescription should roll and cache
	desc1 := roll.GetDescription()
	if desc1 != "+2d6[1,2]=3" {
		t.Errorf("First GetDescription() = %q, want %q", desc1, "+2d6[1,2]=3")
	}

	// GetValue should return cached value, not re-roll
	value := roll.GetValue()
	if value != 3 {
		t.Errorf("GetValue() after GetDescription() = %d, want 3", value)
	}

	// Second GetDescription should return same cached result
	desc2 := roll.GetDescription()
	if desc2 != desc1 {
		t.Errorf("Second GetDescription() = %q, want %q", desc2, desc1)
	}
}

func TestRoll_HelperFunctions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test with mock roller to ensure deterministic results
	mockRoller := mock_dice.NewMockRoller(ctrl)
	original := DefaultRoller
	DefaultRoller = mockRoller
	defer func() { DefaultRoller = original }()

	tests := []struct {
		name     string
		fn       func(int) *Roll
		count    int
		wantSize int
	}{
		{"D4", D4, 1, 4},
		{"D6", D6, 2, 6},
		{"D8", D8, 1, 8},
		{"D10", D10, 1, 10},
		{"D12", D12, 1, 12},
		{"D20", D20, 1, 20},
		{"D100", D100, 1, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roll := tt.fn(tt.count)
			
			// Check internal state
			if roll.count != tt.count {
				t.Errorf("%s(%d).count = %d, want %d", tt.name, tt.count, roll.count, tt.count)
			}
			if roll.size != tt.wantSize {
				t.Errorf("%s(%d).size = %d, want %d", tt.name, tt.count, roll.size, tt.wantSize)
			}
			
			// Verify it uses DefaultRoller
			if roll.roller != DefaultRoller {
				t.Errorf("%s(%d) not using DefaultRoller", tt.name, tt.count)
			}
		})
	}
}

func TestRoll_RealRandom(t *testing.T) {
	// Test with real randomness to ensure it works
	roll := NewRoll(2, 6)
	
	value := roll.GetValue()
	if value < 2 || value > 12 {
		t.Errorf("2d6 rolled %d, want between 2 and 12", value)
	}
	
	desc := roll.GetDescription()
	// Should contain the format we expect
	if !strings.Contains(desc, "+2d6[") {
		t.Errorf("Description doesn't contain expected format: %s", desc)
	}
	if !strings.Contains(desc, "]=") {
		t.Errorf("Description doesn't contain expected format: %s", desc)
	}
}