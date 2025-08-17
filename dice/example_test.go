// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice_test

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
)

// Example demonstrates basic dice rolling
func Example() {
	// Roll a d20
	attack := dice.D20(1)
	fmt.Printf("Attack roll: %s\n", attack.GetDescription())

	// Roll 3d6 for damage
	damage := dice.D6(3)
	fmt.Printf("Damage: %d\n", damage.GetValue())

	// The output will vary due to randomness
}

// Example_customDice shows rolling non-standard dice
func Example_customDice() {
	// Create a d13 (non-standard die size)
	roll, err := dice.NewRoll(1, 13)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// The output will vary, but will be between 1 and 13
	fmt.Printf("d13 result: %d\n", roll.GetValue())
}

// Example_negativeModifier demonstrates penalty rolls
func Example_negativeModifier() {
	// A penalty represented as negative dice
	penalty := dice.D4(-1)

	// GetDescription shows the full notation
	fmt.Printf("Penalty: %s\n", penalty.GetDescription())
	// Will print something like: "-d4[3]=-3"
}

// TestWithMockRoller demonstrates how to test code that uses dice
func TestWithMockRoller(t *testing.T) {
	// Save the original roller and restore it after the test
	original := dice.DefaultRoller
	defer dice.SetDefaultRoller(original)

	// Create a mock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock roller with predictable results
	ctx := context.Background()
	mockRoller := mock_dice.NewMockRoller(ctrl)
	mockRoller.EXPECT().RollN(ctx, 1, 20).Return([]int{20}, nil) // Natural 20!
	mockRoller.EXPECT().RollN(ctx, 2, 6).Return([]int{6, 5}, nil)

	// Set the mock as the default
	dice.SetDefaultRoller(mockRoller)

	// Now test your game logic with predictable dice
	attackRoll := dice.D20(1).GetValue()
	if attackRoll != 20 {
		t.Errorf("Expected critical hit (20), got %d", attackRoll)
	}

	damage := dice.D6(2).GetValue()
	if damage != 11 {
		t.Errorf("Expected damage of 11, got %d", damage)
	}
}

// TestParallelSafety demonstrates testing without global state
func TestParallelSafety(t *testing.T) {
	// For parallel tests, use NewRollWithRoller to avoid global state
	t.Run("test1", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		mockRoller := mock_dice.NewMockRoller(ctrl)
		mockRoller.EXPECT().RollN(ctx, 1, 6).Return([]int{4}, nil)

		roll, err := dice.NewRollWithRoller(1, 6, mockRoller)
		if err != nil {
			t.Fatalf("Failed to create roll: %v", err)
		}

		if roll.GetValue() != 4 {
			t.Errorf("Expected 4, got %d", roll.GetValue())
		}
	})

	t.Run("test2", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		mockRoller := mock_dice.NewMockRoller(ctrl)
		mockRoller.EXPECT().RollN(ctx, 1, 6).Return([]int{2}, nil)

		roll, err := dice.NewRollWithRoller(1, 6, mockRoller)
		if err != nil {
			t.Fatalf("Failed to create roll: %v", err)
		}

		if roll.GetValue() != 2 {
			t.Errorf("Expected 2, got %d", roll.GetValue())
		}
	})
}
