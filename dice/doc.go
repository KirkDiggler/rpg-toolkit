// Package dice provides cryptographically secure random number generation
// for RPG mechanics without implementing any game-specific rules.
//
// Purpose:
// This package offers deterministic and non-deterministic dice rolling
// capabilities with modifier support, ensuring fair and unpredictable
// game outcomes when needed while supporting testing scenarios.
//
// Scope:
//   - Dice notation parsing (e.g., "3d6+2", "1d20-1")
//   - Cryptographically secure random generation
//   - Modifier system for bonuses and penalties
//   - Roll history and individual die results
//   - Deterministic rolling for testing
//   - Support for standard polyhedral dice (d4, d6, d8, d10, d12, d20, d100)
//   - Mathematical operations on roll results
//
// Non-Goals:
//   - Game-specific roll types: Advantage/disadvantage belong in games
//   - Roll result interpretation: Critical hits/failures are game rules
//   - Dice pool mechanics: Counting successes is game-specific
//   - Reroll mechanics: When to reroll is game logic
//   - Probability calculations: Use external statistics packages
//   - Dice UI/visualization: This is pure logic
//   - Custom dice faces: Non-numeric dice are game-specific
//
// Integration:
// This package is used by:
//   - Combat systems for attack and damage rolls
//   - Skill systems for ability checks
//   - Loot systems for random generation
//   - Any game mechanic requiring random numbers
//
// The dice package provides the randomness foundation but makes no
// assumptions about how rolls are used or interpreted.
//
// Example:
//
//	// Create a crypto-secure roller
//	roller := dice.NewCryptoRoller()
//
//	// Roll 3d6+2
//	result, err := roller.Roll("3d6+2")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Rolled %d (dice: %v, modifier: %d)\n",
//	    result.Total, result.Rolls, result.Modifier)
//
//	// For testing, use deterministic roller
//	testRoller := dice.NewFixedRoller([]int{6, 5, 4}) // Always rolls 6, 5, 4
//	result, _ = testRoller.Roll("3d6")
//	// result.Total = 15, result.Rolls = [6, 5, 4]
//
//	// Games implement their own mechanics
//	if gameRules.IsCriticalHit(result.Rolls[0]) {
//	    // Game-specific critical logic
//	}
package dice
