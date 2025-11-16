// Package combat provides D&D 5e combat mechanics implementation
package combat

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Event chain stages for attack resolution
const (
	// StageBase applies base attack values (roll + proficiency + ability mod)
	StageBase chain.Stage = "base"
	// StageFeatures applies class feature modifiers (rage, sneak attack, etc.)
	StageFeatures chain.Stage = "features"
	// StageConditions applies condition modifiers (bless, bane, etc.)
	StageConditions chain.Stage = "conditions"
	// StageEquipment applies equipment bonuses (magic weapons, etc.)
	StageEquipment chain.Stage = "equipment"
	// StageFinal applies final adjustments
	StageFinal chain.Stage = "final"
)

// AttackStages defines the order of modifier application for attacks
var AttackStages = []chain.Stage{
	StageBase,
	StageFeatures,
	StageConditions,
	StageEquipment,
	StageFinal,
}

// AttackChainEvent represents an attack flowing through the modifier chain
type AttackChainEvent struct {
	AttackerID      string
	TargetID        string
	AttackRoll      int  // The d20 roll
	AttackBonus     int  // Base bonus before modifiers
	TargetAC        int  // Target's armor class
	IsNaturalTwenty bool // Natural 20 always hits
	IsNaturalOne    bool // Natural 1 always misses
}

// DamageChainEvent represents damage flowing through the modifier chain
type DamageChainEvent struct {
	AttackerID   string
	TargetID     string
	BaseDamage   int    // Base damage roll
	DamageBonus  int    // Base bonus before modifiers
	DamageType   string // Type of damage (slashing, piercing, etc.)
	IsCritical   bool   // Double damage dice on crit
	WeaponDamage string // Weapon damage dice (e.g., "1d8")
}

// AttackChain provides typed chained topic for attack roll modifiers
var AttackChain = events.DefineChainedTopic[AttackChainEvent]("dnd5e.combat.attack.chain")

// DamageChain provides typed chained topic for damage modifiers
var DamageChain = events.DefineChainedTopic[DamageChainEvent]("dnd5e.combat.damage.chain")

// AttackInput provides all information needed to resolve an attack
type AttackInput struct {
	Attacker         core.Entity
	Defender         core.Entity
	Weapon           *weapons.Weapon
	AttackerScores   shared.AbilityScores
	DefenderAC       int
	ProficiencyBonus int
	EventBus         events.EventBus
	Roller           dice.Roller // Dice roller for attack and damage
}

// AttackResult contains the complete outcome of an attack
type AttackResult struct {
	// Attack roll details
	AttackRoll      int  // The d20 roll
	AttackBonus     int  // Total bonus applied
	TotalAttack     int  // Roll + bonus
	Hit             bool // Did the attack hit?
	Critical        bool // Was it a critical hit?
	IsNaturalTwenty bool // Natural 20
	IsNaturalOne    bool // Natural 1

	// Damage details
	DamageRolls  []int  // Individual damage dice rolls
	DamageBonus  int    // Total damage bonus
	TotalDamage  int    // Final damage dealt
	DamageType   string // Type of damage

	// Modifier tracking
	AttackModifiers []ModifierInfo
	DamageModifiers []ModifierInfo
}

// ModifierInfo tracks what modifiers were applied
type ModifierInfo struct {
	Source string // What added this modifier (e.g., "rage", "bless")
	Stage  string // Which stage it was applied in
	Amount int    // How much was added
	Type   string // "attack" or "damage"
}

// ResolveAttack performs a complete attack resolution using the event chain system
func ResolveAttack(ctx context.Context, input *AttackInput) (*AttackResult, error) {
	if input.Attacker == nil {
		return nil, fmt.Errorf("attacker is required")
	}
	if input.Defender == nil {
		return nil, fmt.Errorf("defender is required")
	}
	if input.Weapon == nil {
		return nil, fmt.Errorf("weapon is required")
	}
	if input.EventBus == nil {
		return nil, fmt.Errorf("event bus is required")
	}

	// Use provided roller or default
	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	result := &AttackResult{
		DamageType:      string(input.Weapon.DamageType),
		AttackModifiers: []ModifierInfo{},
		DamageModifiers: []ModifierInfo{},
	}

	// Step 1: Roll attack (1d20)
	attackRoll, err := roller.Roll(ctx, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to roll attack: %w", err)
	}

	result.AttackRoll = attackRoll
	result.IsNaturalTwenty = (attackRoll == 20)
	result.IsNaturalOne = (attackRoll == 1)
	result.Critical = result.IsNaturalTwenty

	// Step 2: Calculate base attack bonus (ability modifier + proficiency)
	abilityMod := calculateAttackAbilityModifier(input.Weapon, input.AttackerScores)
	baseBonus := abilityMod + input.ProficiencyBonus

	// Step 3: Fire attack chain event to collect modifiers
	attackEvent := AttackChainEvent{
		AttackerID:      input.Attacker.GetID(),
		TargetID:        input.Defender.GetID(),
		AttackRoll:      attackRoll,
		AttackBonus:     baseBonus,
		TargetAC:        input.DefenderAC,
		IsNaturalTwenty: result.IsNaturalTwenty,
		IsNaturalOne:    result.IsNaturalOne,
	}

	// Create attack chain
	attackChain := events.NewStagedChain[AttackChainEvent](AttackStages)
	attacks := AttackChain.On(input.EventBus)

	// Publish through chain to collect modifiers
	modifiedAttackChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	if err != nil {
		return nil, fmt.Errorf("failed to publish attack chain: %w", err)
	}

	// Execute chain to get final attack event with all modifiers
	finalAttackEvent, err := modifiedAttackChain.Execute(ctx, attackEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to execute attack chain: %w", err)
	}

	// Update result with modified values
	result.AttackBonus = finalAttackEvent.AttackBonus
	result.TotalAttack = attackRoll + result.AttackBonus

	// Step 4: Determine hit/miss (natural 20 always hits, natural 1 always misses)
	if result.IsNaturalOne {
		result.Hit = false
	} else if result.IsNaturalTwenty {
		result.Hit = true
	} else {
		result.Hit = result.TotalAttack >= input.DefenderAC
	}

	// Step 5: If hit, calculate damage
	if result.Hit {
		// Roll damage dice
		damageRolls, damageTotal, err := rollWeaponDamage(ctx, input.Weapon, roller, result.Critical)
		if err != nil {
			return nil, fmt.Errorf("failed to roll damage: %w", err)
		}

		result.DamageRolls = damageRolls

		// Calculate base damage bonus (same ability modifier as attack)
		baseDamageBonus := abilityMod

		// Step 6: Fire damage chain event to collect modifiers
		damageEvent := DamageChainEvent{
			AttackerID:   input.Attacker.GetID(),
			TargetID:     input.Defender.GetID(),
			BaseDamage:   damageTotal,
			DamageBonus:  baseDamageBonus,
			DamageType:   string(input.Weapon.DamageType),
			IsCritical:   result.Critical,
			WeaponDamage: input.Weapon.Damage,
		}

		// Create damage chain
		damageChain := events.NewStagedChain[DamageChainEvent](AttackStages)
		damages := DamageChain.On(input.EventBus)

		// Publish through chain to collect modifiers
		modifiedDamageChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
		if err != nil {
			return nil, fmt.Errorf("failed to publish damage chain: %w", err)
		}

		// Execute chain to get final damage event with all modifiers
		finalDamageEvent, err := modifiedDamageChain.Execute(ctx, damageEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to execute damage chain: %w", err)
		}

		// Update result with modified values
		result.DamageBonus = finalDamageEvent.DamageBonus
		result.TotalDamage = finalDamageEvent.BaseDamage + result.DamageBonus

		// Damage can't be negative
		if result.TotalDamage < 0 {
			result.TotalDamage = 0
		}
	}

	return result, nil
}

// calculateAttackAbilityModifier determines which ability modifier to use for attack
func calculateAttackAbilityModifier(weapon *weapons.Weapon, scores shared.AbilityScores) int {
	// Finesse weapons can use STR or DEX (use whichever is higher)
	if weapon.HasProperty(weapons.PropertyFinesse) {
		strMod := scores.Modifier(abilities.STR)
		dexMod := scores.Modifier(abilities.DEX)
		if dexMod > strMod {
			return dexMod
		}
		return strMod
	}

	// Ranged weapons use DEX
	if weapon.IsRanged() {
		return scores.Modifier(abilities.DEX)
	}

	// Melee weapons use STR
	return scores.Modifier(abilities.STR)
}

// rollWeaponDamage rolls the weapon's damage dice
// On critical hits, damage dice are doubled (but not bonuses)
func rollWeaponDamage(ctx context.Context, weapon *weapons.Weapon, roller dice.Roller, isCritical bool) ([]int, int, error) {
	// Parse weapon damage (e.g., "1d8", "2d6")
	numDice, dieSize, err := parseDiceNotation(weapon.Damage)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid weapon damage notation %s: %w", weapon.Damage, err)
	}

	// Double dice on critical (not bonuses)
	if isCritical {
		numDice *= 2
	}

	// Roll the dice
	rolls, err := roller.RollN(ctx, numDice, dieSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to roll damage dice: %w", err)
	}

	// Calculate total
	total := 0
	for _, roll := range rolls {
		total += roll
	}

	return rolls, total, nil
}

// parseDiceNotation parses simple dice notation like "1d8" or "2d6"
// Returns (count, size, error)
func parseDiceNotation(notation string) (int, int, error) {
	// Handle simple cases: "1d8", "2d6", etc.
	var count, size int
	n, err := fmt.Sscanf(notation, "%dd%d", &count, &size)
	if err != nil || n != 2 {
		return 0, 0, fmt.Errorf("invalid dice notation: %s", notation)
	}
	return count, size, nil
}
