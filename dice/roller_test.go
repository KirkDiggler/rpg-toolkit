// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"fmt"
	"testing"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"go.uber.org/mock/gomock"
)

func TestCryptoRoller_Roll(t *testing.T) {
	roller := &CryptoRoller{}

	// Test various die sizes
	sizes := []int{4, 6, 8, 10, 12, 20, 100}
	
	for _, size := range sizes {
		t.Run(fmt.Sprintf("d%d", size), func(t *testing.T) {
			// Roll many times to ensure randomness
			results := make(map[int]int)
			iterations := size * 100

			for i := 0; i < iterations; i++ {
				result := roller.Roll(size)
				
				// Check bounds
				if result < 1 || result > size {
					t.Errorf("Roll(d%d) = %d, want between 1 and %d", size, result, size)
				}
				
				results[result]++
			}

			// Verify all possible values were rolled (with high probability)
			if len(results) != size {
				t.Errorf("Roll(d%d) after %d iterations hit %d different values, expected %d", 
					size, iterations, len(results), size)
			}
		})
	}
}

func TestCryptoRoller_RollN(t *testing.T) {
	roller := &CryptoRoller{}

	tests := []struct {
		name  string
		count int
		size  int
	}{
		{"3d6", 3, 6},
		{"2d20", 2, 20},
		{"0d6", 0, 6},
		{"1d100", 1, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := roller.RollN(tt.count, tt.size)

			if len(results) != tt.count {
				t.Errorf("RollN(%d, %d) returned %d results, want %d", 
					tt.count, tt.size, len(results), tt.count)
			}

			for i, result := range results {
				if result < 1 || result > tt.size {
					t.Errorf("RollN(%d, %d)[%d] = %d, want between 1 and %d", 
						tt.count, tt.size, i, result, tt.size)
				}
			}
		})
	}
}

func TestCryptoRoller_Panics(t *testing.T) {
	roller := &CryptoRoller{}

	tests := []struct {
		name      string
		fn        func()
		wantPanic string
	}{
		{
			name:      "Roll with zero size",
			fn:        func() { roller.Roll(0) },
			wantPanic: "dice: invalid die size 0",
		},
		{
			name:      "Roll with negative size",
			fn:        func() { roller.Roll(-1) },
			wantPanic: "dice: invalid die size -1",
		},
		{
			name:      "RollN with zero size",
			fn:        func() { roller.RollN(1, 0) },
			wantPanic: "dice: invalid die size 0",
		},
		{
			name:      "RollN with negative count",
			fn:        func() { roller.RollN(-1, 6) },
			wantPanic: "dice: invalid die count -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected panic but didn't get one")
				} else if r != tt.wantPanic {
					t.Errorf("Got panic %v, want %v", r, tt.wantPanic)
				}
			}()
			
			tt.fn()
		})
	}
}

func TestDefaultRoller(t *testing.T) {
	// Ensure DefaultRoller is set
	if DefaultRoller == nil {
		t.Fatal("DefaultRoller is nil")
	}

	// Test it works
	result := DefaultRoller.Roll(6)
	if result < 1 || result > 6 {
		t.Errorf("DefaultRoller.Roll(6) = %d, want between 1 and 6", result)
	}
}

func TestSetDefaultRoller(t *testing.T) {
	// Save original
	original := DefaultRoller
	defer func() { DefaultRoller = original }()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Set mock roller
	mockRoller := mock_dice.NewMockRoller(ctrl)
	mockRoller.EXPECT().Roll(6).Return(4)
	
	SetDefaultRoller(mockRoller)

	// Verify it was set
	result := DefaultRoller.Roll(6)
	if result != 4 {
		t.Errorf("DefaultRoller.Roll(6) = %d, want 4", result)
	}
}