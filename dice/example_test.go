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

// Example demonstrates dice that don't roll until observed.
func Example() {
	// Create dice - they haven't rolled yet!
	attack := dice.D20(1)
	weaponDamage := dice.D8(1)
	sneakAttack := dice.D6(3)

	// Dice can travel through your system as potential...
	// Imagine passing these through an event bus here

	// When we observe them, they roll and remember
	if attack.GetValue() >= 15 { // NOW the d20 rolls
		total := weaponDamage.GetValue() + sneakAttack.GetValue()
		fmt.Printf("Hit! Damage: %d\n", total)

		// The dice can explain what happened
		fmt.Printf("  Attack: %s\n", attack.GetDescription())
		fmt.Printf("  Weapon: %s\n", weaponDamage.GetDescription())
		fmt.Printf("  Sneak: %s\n", sneakAttack.GetDescription())
	} else {
		fmt.Printf("Miss with %s\n", attack.GetDescription())
		// Note: weaponDamage and sneakAttack never rolled!
	}

	// The output will vary due to randomness
}

// Example_lazyEvaluation shows how dice delay rolling until needed.
func Example_lazyEvaluation() {
	// Create three sources of damage
	sword := dice.D8(1)
	fireball := dice.D6(8)
	strengthBonus := 3

	// Build up total damage - dice still haven't rolled
	// Imagine these traveling through an event system:
	//   - sword damage modifier
	//   - fireball spell modifier
	//   - strength bonus modifier
	// Through damage reduction handlers...
	// Through resistance checks...

	// Finally, calculate actual damage
	total := sword.GetValue() + fireball.GetValue() + strengthBonus

	// Show the complete history
	fmt.Printf("Total damage: %d\n", total)
	fmt.Printf("  Sword: %s\n", sword.GetDescription())
	fmt.Printf("  Fireball: %s\n", fireball.GetDescription())
	fmt.Printf("  Strength: +%d\n", strengthBonus)

	// Second call returns same values - dice remember their fate
	sameTotal := sword.GetValue() + fireball.GetValue() + strengthBonus
	fmt.Printf("Still: %d (dice don't reroll)\n", sameTotal)

	// The output will vary due to randomness
}

// Example_damageWithReduction shows the complete journey of damage calculation.
func Example_damageWithReduction() {
	// Attacker rolls damage
	weaponDamage := dice.D10(1)
	flameDamage := dice.D6(2)

	// Defender has armor
	armorReduction := dice.D4(-1) // Negative dice!

	// Calculate final damage
	baseDamage := weaponDamage.GetValue() + flameDamage.GetValue()
	finalDamage := baseDamage + armorReduction.GetValue() // Subtracts!

	// Show the complete story
	fmt.Printf("Base damage: %d\n", baseDamage)
	fmt.Printf("  Weapon: %s\n", weaponDamage.GetDescription())
	fmt.Printf("  Flame: %s\n", flameDamage.GetDescription())
	fmt.Printf("  Armor: %s\n", armorReduction.GetDescription())
	fmt.Printf("Final damage: %d\n", finalDamage)

	// Will print something like:
	// Base damage: 14
	//   Weapon: +d10[8]=8
	//   Flame: +2d6[4,2]=6
	//   Armor: -d4[3]=-3
	// Final damage: 11
}

// Example_roguesSneakAttack shows the complete journey of a rogue's attack.
func Example_roguesSneakAttack() {
	// Morning: Rogue prepares their attack
	attackRoll := dice.D20(1)
	rapierDamage := dice.D8(1)
	dexterityBonus := 4
	sneakAttack := dice.D6(5) // 5d6 at level 9

	// These dice travel through the event system...
	// Through advantage/disadvantage handlers...
	// Through bless/bane modifiers...
	// But they still haven't rolled!

	// Combat resolution time - does the attack hit?
	targetAC := 16
	attackTotal := attackRoll.GetValue() + dexterityBonus // NOW it rolls!

	if attackTotal >= targetAC {
		// Hit! Calculate damage
		damage := rapierDamage.GetValue() + dexterityBonus
		damage += sneakAttack.GetValue() // Cascade of d6s!

		fmt.Printf("Hit! (rolled %d vs AC %d)\n", attackTotal, targetAC)
		fmt.Printf("Total damage: %d\n", damage)
		fmt.Printf("  Attack roll: %s +%d\n", attackRoll.GetDescription(), dexterityBonus)
		fmt.Printf("  Rapier: %s +%d\n", rapierDamage.GetDescription(), dexterityBonus)
		fmt.Printf("  Sneak Attack: %s\n", sneakAttack.GetDescription())
	} else {
		fmt.Printf("Miss! %s +%d = %d vs AC %d\n",
			attackRoll.GetDescription(), dexterityBonus, attackTotal, targetAC)
		// Notice: damage dice never rolled because we missed!
	}

	// The output will vary, but might look like:
	// Hit! (rolled 19 vs AC 16)
	// Total damage: 28
	//   Attack roll: +d20[15]=15 +4
	//   Rapier: +d8[5]=5 +4
	//   Sneak Attack: +5d6[6,4,3,3,3]=19
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
