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
				result, err := roller.Roll(size)
				if err != nil {
					t.Fatalf("Roll(%d) error = %v", size, err)
				}
				
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
			results, err := roller.RollN(tt.count, tt.size)
			if err != nil {
				t.Fatalf("RollN(%d, %d) error = %v", tt.count, tt.size, err)
			}

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

func TestCryptoRoller_Errors(t *testing.T) {
	roller := &CryptoRoller{}

	tests := []struct {
		name    string
		fn      func() error
		wantErr string
	}{
		{
			name: "Roll with zero size",
			fn: func() error {
				_, err := roller.Roll(0)
				return err
			},
			wantErr: "dice: invalid die size 0",
		},
		{
			name: "Roll with negative size",
			fn: func() error {
				_, err := roller.Roll(-1)
				return err
			},
			wantErr: "dice: invalid die size -1",
		},
		{
			name: "RollN with zero size",
			fn: func() error {
				_, err := roller.RollN(1, 0)
				return err
			},
			wantErr: "dice: invalid die size 0",
		},
		{
			name: "RollN with negative count",
			fn: func() error {
				_, err := roller.RollN(-1, 6)
				return err
			},
			wantErr: "dice: invalid die count -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Error("Expected error but got nil")
			} else if err.Error() != tt.wantErr {
				t.Errorf("Got error %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDefaultRoller(t *testing.T) {
	// Ensure DefaultRoller is set
	if DefaultRoller == nil {
		t.Fatal("DefaultRoller is nil")
	}

	// Test it works
	result, err := DefaultRoller.Roll(6)
	if err != nil {
		t.Fatalf("DefaultRoller.Roll(6) error = %v", err)
	}
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
	mockRoller.EXPECT().Roll(6).Return(4, nil)
	
	SetDefaultRoller(mockRoller)

	// Verify it was set
	result, err := DefaultRoller.Roll(6)
	if err != nil {
		t.Fatalf("DefaultRoller.Roll(6) error = %v", err)
	}
	if result != 4 {
		t.Errorf("DefaultRoller.Roll(6) = %d, want 4", result)
	}
}