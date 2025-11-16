// Package combat provides D&D 5e combat mechanics implementation
package combat

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

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
var DamageChain = events.DefineChainedTopic[*DamageChainEvent]("dnd5e.combat.damage.chain")

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

// Validate validates the input
func (ai *AttackInput) Validate() error {
	if ai == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "AttackInput is nil")
	}

	if ai.Attacker == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Attacker is nil")
	}

	if ai.Defender == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Defender is nil")
	}

	if ai.Weapon == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Weapon is nil")
	}

	if ai.EventBus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "EventBus is nil")
	}

	return nil
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
	DamageRolls []int  // Individual damage dice rolls (flattened)
	DamageBonus int    // Total damage bonus
	TotalDamage int    // Final damage dealt
	DamageType  string // Type of damage
}

// ResolveAttack performs a complete attack resolution using the event chain system
//
//nolint:gocyclo // Attack resolution requires orchestrating multiple game rules stages
func ResolveAttack(ctx context.Context, input *AttackInput) (*AttackResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Use provided roller or default
	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	result := &AttackResult{
		DamageType: string(input.Weapon.DamageType),
	}

	// Step 1: Publish AttackEvent (before any rolls)
	attackTopic := dnd5e.AttackTopic.On(input.EventBus)
	err := attackTopic.Publish(ctx, dnd5e.AttackEvent{
		AttackerID: input.Attacker.GetID(),
		TargetID:   input.Defender.GetID(),
		WeaponRef:  input.Weapon.ID,
		IsMelee:    !input.Weapon.IsRanged(),
	})
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish attack event")
	}

	// Step 2: Roll attack (1d20)
	attackRoll, err := roller.Roll(ctx, 20)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to roll attack")
	}

	result.AttackRoll = attackRoll
	result.IsNaturalTwenty = (attackRoll == 20)
	result.IsNaturalOne = (attackRoll == 1)
	result.Critical = result.IsNaturalTwenty

	// Step 3: Calculate base attack bonus (ability modifier + proficiency)
	abilityMod := calculateAttackAbilityModifier(input.Weapon, input.AttackerScores)
	baseBonus := abilityMod + input.ProficiencyBonus

	// Step 4: Fire attack chain event to collect modifiers
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
	attackChain := events.NewStagedChain[AttackChainEvent](dnd5e.ModifierStages)
	attacks := AttackChain.On(input.EventBus)

	// Publish through chain to collect modifiers
	modifiedAttackChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish attack chain")
	}

	// Execute chain to get final attack event with all modifiers
	finalAttackEvent, err := modifiedAttackChain.Execute(ctx, attackEvent)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to execute attack chain")
	}

	// Update result with modified values
	result.AttackBonus = finalAttackEvent.AttackBonus
	result.TotalAttack = attackRoll + result.AttackBonus

	// Step 5: Determine hit/miss (natural 20 always hits, natural 1 always misses)
	switch {
	case result.IsNaturalOne:
		result.Hit = false
	case result.IsNaturalTwenty:
		result.Hit = true
	default:
		result.Hit = result.TotalAttack >= input.DefenderAC
	}

	// Step 6: If hit, calculate damage
	if result.Hit {
		// Parse weapon damage notation
		damagePool, err := dice.ParseNotation(input.Weapon.Damage)
		if err != nil {
			return nil, rpgerr.Wrap(err, fmt.Sprintf("invalid weapon damage %s", input.Weapon.Damage))
		}

		// Roll damage dice (double for crits)
		var damageRolls []int
		if result.Critical {
			// Critical: roll dice twice and combine
			damageRolls, err = rollDamageDice(ctx, damagePool, roller, 2)
		} else {
			// Normal: roll dice once
			damageRolls, err = rollDamageDice(ctx, damagePool, roller, 1)
		}
		if err != nil {
			return nil, err
		}
		result.DamageRolls = damageRolls

		// Sum base damage from dice
		baseDamage := 0
		for _, roll := range damageRolls {
			baseDamage += roll
		}

		// Apply damage chain for modifiers
		finalDamage, damageBonus, err := applyDamageChain(ctx, input, baseDamage, abilityMod, result.Critical)
		if err != nil {
			return nil, err
		}

		result.DamageBonus = damageBonus
		result.TotalDamage = finalDamage

		// Damage can't be negative
		if result.TotalDamage < 0 {
			result.TotalDamage = 0
		}

		// Step 8: Publish DamageReceivedEvent
		damageTopic := dnd5e.DamageReceivedTopic.On(input.EventBus)
		err = damageTopic.Publish(ctx, dnd5e.DamageReceivedEvent{
			TargetID:   input.Defender.GetID(),
			SourceID:   input.Attacker.GetID(),
			Amount:     result.TotalDamage,
			DamageType: string(input.Weapon.DamageType),
		})
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to publish damage received event")
		}
	}

	return result, nil
}

// rollDamageDice rolls the damage pool the specified number of times and combines results
func rollDamageDice(ctx context.Context, pool *dice.Pool, roller dice.Roller, times int) ([]int, error) {
	var allRolls []int
	for i := 0; i < times; i++ {
		result := pool.RollContext(ctx, roller)
		if result.Error() != nil {
			return nil, rpgerr.Wrap(result.Error(), "failed to roll damage")
		}
		// Flatten the roll groups
		for _, group := range result.Rolls() {
			allRolls = append(allRolls, group...)
		}
	}
	return allRolls, nil
}

// applyDamageChain applies the damage modifier chain and returns final damage and bonus
func applyDamageChain(
	ctx context.Context,
	input *AttackInput,
	baseDamage int,
	abilityMod int,
	isCritical bool,
) (finalDamage, damageBonus int, err error) {
	// Build damage chain event
	damageEvent := &DamageChainEvent{
		AttackerID:   input.Attacker.GetID(),
		TargetID:     input.Defender.GetID(),
		BaseDamage:   baseDamage,
		DamageBonus:  abilityMod,
		DamageType:   string(input.Weapon.DamageType),
		IsCritical:   isCritical,
		WeaponDamage: input.Weapon.Damage,
	}

	// Create and publish through damage chain
	damageChain := events.NewStagedChain[*DamageChainEvent](dnd5e.ModifierStages)
	damages := DamageChain.On(input.EventBus)

	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	if err != nil {
		return 0, 0, rpgerr.Wrap(err, "failed to publish damage chain")
	}

	// Execute chain to get final modifiers
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	if err != nil {
		return 0, 0, rpgerr.Wrap(err, "failed to execute damage chain")
	}

	return finalEvent.BaseDamage + finalEvent.DamageBonus, finalEvent.DamageBonus, nil
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
