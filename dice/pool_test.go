// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
)

func TestNewPool(t *testing.T) {
	tests := []struct {
		name         string
		dice         []Spec
		modifier     int
		wantNotation string
	}{
		{
			name:         "single dice type with modifier",
			dice:         []Spec{{Count: 2, Size: 6}},
			modifier:     3,
			wantNotation: "2d6+3",
		},
		{
			name:         "single die no modifier",
			dice:         []Spec{{Count: 1, Size: 20}},
			modifier:     0,
			wantNotation: "d20",
		},
		{
			name:         "multiple dice types",
			dice:         []Spec{{Count: 2, Size: 6}, {Count: 1, Size: 4}},
			modifier:     2,
			wantNotation: "2d6+d4+2",
		},
		{
			name:         "negative modifier",
			dice:         []Spec{{Count: 3, Size: 8}},
			modifier:     -2,
			wantNotation: "3d8-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPool(tt.dice, tt.modifier)
			if pool.Notation() != tt.wantNotation {
				t.Errorf("Pool.Notation() = %q, want %q", pool.Notation(), tt.wantNotation)
			}
		})
	}
}

func TestSimplePool(t *testing.T) {
	pool := SimplePool(2, 6, 3)
	if pool.Notation() != "2d6+3" {
		t.Errorf("SimplePool(2, 6, 3).Notation() = %q, want %q", pool.Notation(), "2d6+3")
	}
}

func TestPool_Roll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRoller := mock_dice.NewMockRoller(ctrl)
	ctx := context.Background()

	// Expect rolls for 2d6+3
	mockRoller.EXPECT().RollN(ctx, 2, 6).Return([]int{4, 5}, nil)

	pool := SimplePool(2, 6, 3)
	result := pool.RollContext(ctx, mockRoller)

	if result.Error() != nil {
		t.Fatalf("Pool.Roll() error = %v", result.Error())
	}

	if result.Total() != 12 { // 4 + 5 + 3
		t.Errorf("Pool.Roll() total = %d, want 12", result.Total())
	}

	if result.Modifier() != 3 {
		t.Errorf("Result.Modifier() = %d, want 3", result.Modifier())
	}

	rolls := result.Rolls()
	if len(rolls) != 1 || len(rolls[0]) != 2 {
		t.Errorf("Result.Rolls() = %v, want [[4, 5]]", rolls)
	}
}

func TestPool_Statistics(t *testing.T) {
	tests := []struct {
		name        string
		pool        *Pool
		wantAverage float64
		wantMin     int
		wantMax     int
	}{
		{
			name:        "2d6+3",
			pool:        SimplePool(2, 6, 3),
			wantAverage: 10, // (3.5 * 2) + 3
			wantMin:     5,  // 2 + 3
			wantMax:     15, // 12 + 3
		},
		{
			name:        "d20",
			pool:        SimplePool(1, 20, 0),
			wantAverage: 10.5, // (20 + 1) / 2
			wantMin:     1,
			wantMax:     20,
		},
		{
			name:        "3d4-2",
			pool:        SimplePool(3, 4, -2),
			wantAverage: 5.5, // (2.5 * 3) - 2
			wantMin:     1,   // 3 - 2
			wantMax:     10,  // 12 - 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if avg := tt.pool.Average(); avg != tt.wantAverage {
				t.Errorf("Pool.Average() = %v, want %v", avg, tt.wantAverage)
			}
			if minValue := tt.pool.Min(); minValue != tt.wantMin {
				t.Errorf("Pool.Min() = %v, want %v", minValue, tt.wantMin)
			}
			if maxValue := tt.pool.Max(); maxValue != tt.wantMax {
				t.Errorf("Pool.Max() = %v, want %v", maxValue, tt.wantMax)
			}
		})
	}
}

func TestPool_MultipleRolls(t *testing.T) {
	// Test that Pool produces fresh results each time
	pool := SimplePool(1, 6, 0)
	roller := NewRoller()

	results := make(map[int]bool)
	for i := 0; i < 20; i++ {
		result := pool.Roll(roller)
		if result.Error() != nil {
			t.Fatalf("Roll %d failed: %v", i, result.Error())
		}
		results[result.Total()] = true
	}

	// With 20 rolls of a d6, we should see at least 2 different results
	if len(results) < 2 {
		t.Errorf("After 20 rolls, only saw %d different results, expected variety", len(results))
	}
}
