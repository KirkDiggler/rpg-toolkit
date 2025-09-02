// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
)

func TestCryptoRoller_Roll(t *testing.T) {
	roller := &CryptoRoller{}
	ctx := context.Background()

	// Test various die sizes
	sizes := []int{4, 6, 8, 10, 12, 20, 100}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("d%d", size), func(t *testing.T) {
			// Roll many times to ensure randomness
			results := make(map[int]int)
			iterations := size * 100

			for i := 0; i < iterations; i++ {
				result, err := roller.Roll(ctx, size)
				if err != nil {
					t.Fatalf("Roll(%d) error = %v", size, err)
				}

				// Check bounds
				if result < 1 || result > size {
					t.Errorf("Roll(d%d) = %d, want between 1 and %d", size, result, size)
				}

				results[result]++
			}

			// Verify we hit a reasonable number of different values
			// For large dice, we may not hit every face in our iterations
			minExpected := size * 3 / 4 // Expect at least 75% of faces
			if size > 20 {
				minExpected = size * 2 / 3 // For larger dice, expect at least 66%
			}
			if len(results) < minExpected {
				t.Errorf("Roll(d%d) after %d iterations hit only %d different values, expected at least %d",
					size, iterations, len(results), minExpected)
			}
		})
	}
}

func TestCryptoRoller_RollN(t *testing.T) {
	roller := &CryptoRoller{}
	ctx := context.Background()

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
			results, err := roller.RollN(ctx, tt.count, tt.size)
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
	ctx := context.Background()

	tests := []struct {
		name    string
		fn      func() error
		wantErr string
	}{
		{
			name: "Roll with zero size",
			fn: func() error {
				_, err := roller.Roll(ctx, 0)
				return err
			},
			wantErr: "dice: invalid die size 0",
		},
		{
			name: "Roll with negative size",
			fn: func() error {
				_, err := roller.Roll(ctx, -1)
				return err
			},
			wantErr: "dice: invalid die size -1",
		},
		{
			name: "RollN with zero size",
			fn: func() error {
				_, err := roller.RollN(ctx, 1, 0)
				return err
			},
			wantErr: "dice: invalid die size 0",
		},
		{
			name: "RollN with negative count",
			fn: func() error {
				_, err := roller.RollN(ctx, -1, 6)
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

func TestNewRoller(t *testing.T) {
	ctx := context.Background()
	// Create a new roller
	roller := NewRoller()
	if roller == nil {
		t.Fatal("NewRoller() returned nil")
	}

	// Test it works
	result, err := roller.Roll(ctx, 6)
	if err != nil {
		t.Fatalf("roller.Roll(6) error = %v", err)
	}
	if result < 1 || result > 6 {
		t.Errorf("roller.Roll(6) = %d, want between 1 and 6", result)
	}
}

func TestNewMockableRoller(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test with mock roller
	mockRoller := mock_dice.NewMockRoller(ctrl)
	mockRoller.EXPECT().Roll(ctx, 6).Return(4, nil)

	roller := NewMockableRoller(mockRoller)

	// Verify mock is used
	result, err := roller.Roll(ctx, 6)
	if err != nil {
		t.Fatalf("roller.Roll(6) error = %v", err)
	}
	if result != 4 {
		t.Errorf("roller.Roll(6) = %d, want 4", result)
	}

	// Test with nil returns default
	defaultRoller := NewMockableRoller(nil)
	if defaultRoller == nil {
		t.Fatal("NewMockableRoller(nil) returned nil")
	}
}
