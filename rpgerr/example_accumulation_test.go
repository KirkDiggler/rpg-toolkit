package rpgerr_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Example_errorAccumulation demonstrates the magic of automatic context accumulation.
// Watch how the error captures the complete story without manual passing.
func Example_errorAccumulation() {
	// Simulate an attack that flows through multiple game systems
	err := simulateCombatRound()

	// The error contains the ENTIRE journey
	meta := rpgerr.GetMeta(err)
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Round: %v\n", meta["round"])
	fmt.Printf("Attacker: %v\n", meta["attacker"])
	fmt.Printf("Weapon: %v\n", meta["weapon"])
	fmt.Printf("Distance: %v\n", meta["distance"])

	// Output:
	// Error: melee attack out of range
	// Round: 3
	// Attacker: fighter-001
	// Weapon: longsword
	// Distance: 35
}

func simulateCombatRound() error {
	// Combat system adds round context
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("round", 3),
		rpgerr.Meta("phase", "action"))

	// Execute player turn
	return executePlayerTurn(ctx, "fighter-001")
}

func executePlayerTurn(ctx context.Context, playerID string) error {
	// Turn system adds player context
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("attacker", playerID),
		rpgerr.Meta("action", "attack"))

	// Attempt melee attack
	return attemptMeleeAttack(ctx, "goblin-002")
}

func attemptMeleeAttack(ctx context.Context, targetID string) error {
	// Attack system adds weapon and target
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("target", targetID),
		rpgerr.Meta("weapon", "longsword"))

	// Check if in range
	return checkMeleeRange(ctx)
}

func checkMeleeRange(ctx context.Context) error {
	// Range check adds distance calculation
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("distance", 35),
		rpgerr.Meta("max_range", 5))

	// Too far! But the error will contain the whole story
	return rpgerr.OutOfRangeCtx(ctx, "melee attack")
}

// Example_spellcastingJourney shows how spell failures accumulate context through the magic system.
func Example_spellcastingJourney() {
	ctx := context.Background()

	// Magic system level
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("caster", "wizard-001"),
		rpgerr.Meta("caster_level", 5))

	// Spell preparation level
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("spell", "fireball"),
		rpgerr.Meta("spell_level", 3))

	// Resource check level
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("slots_available", map[string]int{
			"1st": 4,
			"2nd": 3,
			"3rd": 0, // No 3rd level slots!
		}))

	// Create error with full journey
	err := rpgerr.ResourceExhaustedCtx(ctx, "3rd level spell slots")

	meta := rpgerr.GetMeta(err)
	slots := meta["slots_available"].(map[string]int)

	fmt.Printf("Cannot cast %v - no level %v slots\n", meta["spell"], meta["spell_level"])
	fmt.Printf("Wizard %v (level %v) has slots: 1st=%d, 2nd=%d, 3rd=%d\n",
		meta["caster"], meta["caster_level"],
		slots["1st"], slots["2nd"], slots["3rd"])

	// Output:
	// Cannot cast fireball - no level 3 slots
	// Wizard wizard-001 (level 5) has slots: 1st=4, 2nd=3, 3rd=0
}

// Example_savingThrowChain demonstrates how a saving throw accumulates context
// through validation, rolling, and effect application.
func Example_savingThrowChain() {
	ctx := context.Background()

	// Spell cast context
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("spell", "hold_person"),
		rpgerr.Meta("save_type", "wisdom"),
		rpgerr.Meta("save_dc", 15),
		rpgerr.Meta("caster", "wizard-001"))

	// Target context
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("target", "fighter-001"),
		rpgerr.Meta("wisdom_modifier", 0),
		rpgerr.Meta("proficient_saves", []string{"strength", "constitution"}))

	// Roll context
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("roll", 12),
		rpgerr.Meta("total_save", 12)) // 12 + 0 modifier

	// Failed save - but look at all the context we have!
	err := rpgerr.NewCtx(ctx, rpgerr.CodeBlocked, "failed wisdom save vs hold person")

	meta := rpgerr.GetMeta(err)
	fmt.Printf("Spell: %v (DC %v)\n", meta["spell"], meta["save_dc"])
	fmt.Printf("Target rolled: %v (total: %v)\n", meta["roll"], meta["total_save"])
	fmt.Printf("Result: Failed (needed %v, got %v)\n", meta["save_dc"], meta["total_save"])

	// Output:
	// Spell: hold_person (DC 15)
	// Target rolled: 12 (total: 12)
	// Result: Failed (needed 15, got 12)
}

// Example_damageReductionPipeline shows deep nesting where each pipeline stage
// adds its context, creating a complete picture of why damage was modified.
func Example_damageReductionPipeline() {
	// Attack hits and enters damage calculation
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("attacker", "barbarian-001"),
		rpgerr.Meta("rage_active", true))

	// Base damage calculation
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("weapon", "greataxe"),
		rpgerr.Meta("damage_roll", 8),
		rpgerr.Meta("strength_bonus", 4),
		rpgerr.Meta("rage_bonus", 2),
		rpgerr.Meta("total_damage", 14))

	// Target defenses
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("target", "stone_golem"),
		rpgerr.Meta("damage_type", "slashing"),
		rpgerr.Meta("target_immunities", []string{"poison", "psychic"}),
		rpgerr.Meta("target_resistances", []string{"slashing", "piercing", "bludgeoning"}))

	// Non-magical weapon check
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("weapon_magical", false),
		rpgerr.Meta("bypass_resistance", false),
		rpgerr.Meta("final_damage", 7)) // Halved from 14

	// Create an informational "error" showing the reduction
	err := rpgerr.NewCtx(ctx, rpgerr.CodeBlocked,
		"damage reduced by resistance to non-magical slashing")

	// The complete damage story is captured
	meta := rpgerr.GetMeta(err)
	fmt.Printf("Attack: %v with %v dealt %v damage\n",
		meta["attacker"], meta["weapon"], meta["damage_roll"])
	fmt.Printf("With bonuses: %v total damage\n", meta["total_damage"])
	fmt.Printf("After %v resistance: %v damage\n",
		meta["damage_type"], meta["final_damage"])

	// Output:
	// Attack: barbarian-001 with greataxe dealt 8 damage
	// With bonuses: 14 total damage
	// After slashing resistance: 7 damage
}
