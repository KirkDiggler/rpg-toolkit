// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
)

func TestNewLazy(t *testing.T) {
	pool := SimplePool(2, 6, 3)
	lazy := NewLazy(pool)

	if lazy.Pool() != pool {
		t.Error("NewLazy() did not store pool correctly")
	}

	// Test that it has a roller
	if lazy.roller == nil {
		t.Error("NewLazy() did not create roller")
	}
}

func TestNewLazyWithRoller(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pool := SimplePool(1, 20, 0)
	mockRoller := mock_dice.NewMockRoller(ctrl)

	// Test with mock roller
	lazy := NewLazyWithRoller(pool, mockRoller)
	if lazy.roller != mockRoller {
		t.Error("NewLazyWithRoller() did not use provided roller")
	}

	// Test with nil roller (should create default)
	lazy2 := NewLazyWithRoller(pool, nil)
	if lazy2.roller == nil {
		t.Error("NewLazyWithRoller(nil) did not create default roller")
	}
}

func TestLazy_GetValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockRoller := mock_dice.NewMockRoller(ctrl)

	// Set up expectations for multiple calls
	mockRoller.EXPECT().RollN(ctx, 1, 6).Return([]int{4}, nil).Times(1)
	mockRoller.EXPECT().RollN(ctx, 1, 6).Return([]int{2}, nil).Times(1)
	mockRoller.EXPECT().RollN(ctx, 1, 6).Return([]int{6}, nil).Times(1)

	pool := SimplePool(1, 6, 0)
	lazy := NewLazyWithRoller(pool, mockRoller)

	// Test that each call produces fresh results
	results := []int{
		lazy.GetValue(),
		lazy.GetValue(),
		lazy.GetValue(),
	}

	expected := []int{4, 2, 6}
	for i, result := range results {
		if result != expected[i] {
			t.Errorf("Call %d: GetValue() = %d, want %d", i+1, result, expected[i])
		}
	}
}

func TestLazy_GetDescription(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockRoller := mock_dice.NewMockRoller(ctrl)

	// Set up expectation for description
	mockRoller.EXPECT().RollN(ctx, 2, 6).Return([]int{4, 5}, nil)

	pool := SimplePool(2, 6, 3)
	lazy := NewLazyWithRoller(pool, mockRoller)

	desc := lazy.GetDescription()

	// Should have a + prefix and show the roll details
	if !strings.HasPrefix(desc, "+") {
		t.Errorf("GetDescription() = %q, expected to start with +", desc)
	}

	if !strings.Contains(desc, "2d6") {
		t.Errorf("GetDescription() = %q, expected to contain '2d6'", desc)
	}

	if !strings.Contains(desc, "12") { // 4+5+3=12
		t.Errorf("GetDescription() = %q, expected to contain total '12'", desc)
	}
}

func TestLazy_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockRoller := mock_dice.NewMockRoller(ctrl)

	// Set up roller to return error
	mockRoller.EXPECT().RollN(ctx, 1, 20).Return(nil, ErrInvalidDieSize).AnyTimes()

	pool := SimplePool(1, 20, 0)
	lazy := NewLazyWithRoller(pool, mockRoller)

	// GetValue should return 0 on error
	if value := lazy.GetValue(); value != 0 {
		t.Errorf("GetValue() with error = %d, want 0", value)
	}

	// GetDescription should show error
	desc := lazy.GetDescription()
	if !strings.Contains(desc, "ERROR") {
		t.Errorf("GetDescription() with error = %q, expected to contain 'ERROR'", desc)
	}
}

func TestLazy_FreshRolls(t *testing.T) {
	// Test with real roller to ensure fresh rolls
	pool := SimplePool(1, 6, 0)
	lazy := NewLazy(pool)

	// Roll many times and track results
	results := make(map[int]int)
	for i := 0; i < 60; i++ {
		value := lazy.GetValue()
		if value < 1 || value > 6 {
			t.Errorf("GetValue() = %d, want between 1 and 6", value)
		}
		results[value]++
	}

	// Should see multiple different values
	if len(results) < 3 {
		t.Errorf("After 60 rolls, only saw %d different values, expected more variety", len(results))
	}
}

func TestLazyFromNotation(t *testing.T) {
	tests := []struct {
		name     string
		notation string
		wantErr  bool
	}{
		{
			name:     "valid notation",
			notation: "2d6+3",
			wantErr:  false,
		},
		{
			name:     "invalid notation",
			notation: "invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lazy, err := LazyFromNotation(tt.notation)
			if (err != nil) != tt.wantErr {
				t.Errorf("LazyFromNotation(%q) error = %v, wantErr %v", tt.notation, err, tt.wantErr)
				return
			}
			if !tt.wantErr && lazy == nil {
				t.Error("LazyFromNotation() returned nil without error")
			}
		})
	}
}
