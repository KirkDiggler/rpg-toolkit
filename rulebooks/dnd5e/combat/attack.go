// Package combat provides D&D 5e combat mechanics implementation
package combat

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

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

// DamageBreakdown provides detailed component breakdown of damage calculation
type DamageBreakdown struct {
	Components  []dnd5eEvents.DamageComponent
	AbilityUsed abilities.Ability // Use abilities.Ability type, not string
	TotalDamage int
}

// AttackResult contains the complete outcome of an attack
type AttackResult struct {
	// Attack roll details
	AttackRoll      int   // The d20 roll (final result after advantage/disadvantage)
	AttackBonus     int   // Total bonus applied
	TotalAttack     int   // Roll + bonus
	TargetAC        int   // Target's armor class
	Hit             bool  // Did the attack hit?
	Critical        bool  // Was it a critical hit?
	IsNaturalTwenty bool  // Natural 20
	IsNaturalOne    bool  // Natural 1
	AllRolls        []int // All d20 rolls (2 if advantage/disadvantage, 1 otherwise)
	HasAdvantage    bool  // True if rolled with advantage
	HasDisadvantage bool  // True if rolled with disadvantage

	// Damage details
	DamageRolls []int       // Individual damage dice rolls (flattened)
	DamageBonus int         // Total damage bonus
	TotalDamage int         // Final damage dealt
	DamageType  damage.Type // Type of damage

	// Detailed breakdown
	Breakdown *DamageBreakdown // Detailed damage breakdown (nil if attack missed)
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
		DamageType: input.Weapon.DamageType,
		TargetAC:   input.DefenderAC,
	}

	// Step 1: Calculate base attack bonus (ability modifier + proficiency)
	abilityMod := calculateAttackAbilityModifier(input.Weapon, input.AttackerScores)
	baseBonus := abilityMod + input.ProficiencyBonus

	// Step 2: Fire attack chain BEFORE the roll to collect advantage/disadvantage and modifiers
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:          input.Attacker.GetID(),
		TargetID:            input.Defender.GetID(),
		WeaponRef:           weaponToRef(input.Weapon),
		IsMelee:             !input.Weapon.IsRanged(),
		AdvantageSources:    nil,
		DisadvantageSources: nil,
		AttackBonus:         baseBonus,
		TargetAC:            input.DefenderAC,
		CriticalThreshold:   20, // Default threshold (can be modified by conditions)
		ReactionsConsumed:   nil,
	}

	// Create attack chain
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(input.EventBus)

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

	// Step 3: Determine advantage/disadvantage and roll
	// D&D 5e rule: any advantage + any disadvantage = cancel out to normal roll
	hasAdvantage := len(finalAttackEvent.AdvantageSources) > 0
	hasDisadvantage := len(finalAttackEvent.DisadvantageSources) > 0

	var attackRoll int
	var allRolls []int

	switch {
	case hasAdvantage && hasDisadvantage:
		// They cancel out - roll normally
		attackRoll, err = roller.Roll(ctx, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack")
		}
		allRolls = []int{attackRoll}
		// Clear both flags since they cancelled
		hasAdvantage = false
		hasDisadvantage = false
	case hasAdvantage:
		// D&D 5e advantage: roll 2d20, take higher
		allRolls, err = roller.RollN(ctx, 2, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack with advantage")
		}
		attackRoll = max(allRolls[0], allRolls[1])
	case hasDisadvantage:
		// D&D 5e disadvantage: roll 2d20, take lower
		allRolls, err = roller.RollN(ctx, 2, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack with disadvantage")
		}
		attackRoll = min(allRolls[0], allRolls[1])
	default:
		// Normal roll
		attackRoll, err = roller.Roll(ctx, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack")
		}
		allRolls = []int{attackRoll}
	}

	result.AttackRoll = attackRoll
	result.AllRolls = allRolls
	result.HasAdvantage = hasAdvantage
	result.HasDisadvantage = hasDisadvantage
	result.IsNaturalTwenty = (attackRoll == 20)
	result.IsNaturalOne = (attackRoll == 1)

	// Update result with modified values from chain
	result.AttackBonus = finalAttackEvent.AttackBonus
	result.TotalAttack = attackRoll + result.AttackBonus

	// Determine critical hit based on threshold (modified by conditions like Improved Critical)
	result.Critical = attackRoll >= finalAttackEvent.CriticalThreshold

	// Step 4: Publish ReactionUsedEvents for any reactions consumed during the chain
	if len(finalAttackEvent.ReactionsConsumed) > 0 {
		reactionTopic := dnd5eEvents.ReactionUsedTopic.On(input.EventBus)
		for _, reaction := range finalAttackEvent.ReactionsConsumed {
			// ReactionConsumption has same structure as ReactionUsedEvent
			err = reactionTopic.Publish(ctx, dnd5eEvents.ReactionUsedEvent(reaction))
			if err != nil {
				return nil, rpgerr.Wrap(err, "failed to publish reaction used event")
			}
		}
	}

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

		// Determine which ability was used
		abilityUsed := determineAbilityUsed(input.Weapon, input.AttackerScores)

		// Build damage components for the chain
		weaponComponent := dnd5eEvents.DamageComponent{
			Source:            dnd5eEvents.DamageSourceWeapon,
			SourceRef:         weaponToRef(input.Weapon),
			OriginalDiceRolls: damageRolls,
			FinalDiceRolls:    damageRolls, // No rerolls yet
			DamageType:        input.Weapon.DamageType,
			IsCritical:        result.Critical,
		}

		abilityComponent := dnd5eEvents.DamageComponent{
			Source:     dnd5eEvents.DamageSourceAbility,
			SourceRef:  abilityToRef(abilityUsed),
			FlatBonus:  abilityMod,
			DamageType: input.Weapon.DamageType,
			IsCritical: result.Critical,
		}

		// RESOLVE: Use shared ResolveDamage for chain processing and multipliers
		resolveOutput, err := ResolveDamage(ctx, &ResolveDamageInput{
			AttackerID:   input.Attacker.GetID(),
			TargetID:     input.Defender.GetID(),
			Components:   []dnd5eEvents.DamageComponent{weaponComponent, abilityComponent},
			IsCritical:   result.Critical,
			HasAdvantage: result.HasAdvantage,
			EventBus:     input.EventBus,
			// Attack-specific fields for modifiers like Great Weapon Fighting
			WeaponDamage: input.Weapon.Damage,
			AbilityUsed:  abilityUsed,
			WeaponRef:    weaponToRef(input.Weapon),
		})
		if err != nil {
			return nil, err
		}

		result.TotalDamage = resolveOutput.TotalDamage
		result.DamageBonus = abilityMod // Keep for backward compatibility

		// Damage can't be negative
		if result.TotalDamage < 0 {
			result.TotalDamage = 0
		}

		// Set breakdown from resolve output
		result.Breakdown = &DamageBreakdown{
			Components:  resolveOutput.FinalComponents,
			AbilityUsed: abilityUsed,
			TotalDamage: resolveOutput.TotalDamage,
		}

		// NOTIFY: Publish DamageReceivedEvent with proper source info
		damageTopic := dnd5eEvents.DamageReceivedTopic.On(input.EventBus)
		err = damageTopic.Publish(ctx, dnd5eEvents.DamageReceivedEvent{
			TargetID:   input.Defender.GetID(),
			SourceID:   input.Attacker.GetID(),
			SourceRef:  weaponToRef(input.Weapon),
			Amount:     result.TotalDamage,
			DamageType: input.Weapon.DamageType,
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

// determineAbilityUsed determines which ability is used for the attack
func determineAbilityUsed(weapon *weapons.Weapon, scores shared.AbilityScores) abilities.Ability {
	// Finesse weapons can use STR or DEX (use whichever is higher)
	if weapon.HasProperty(weapons.PropertyFinesse) {
		strMod := scores.Modifier(abilities.STR)
		dexMod := scores.Modifier(abilities.DEX)
		if dexMod > strMod {
			return abilities.DEX
		}
		return abilities.STR
	}

	// Ranged weapons use DEX
	if weapon.IsRanged() {
		return abilities.DEX
	}

	// Melee weapons use STR
	return abilities.STR
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

// weaponToRef converts a weapon to its singleton core.Ref.
// Returns the singleton ref for pointer identity comparison, or nil if weapon is nil.
func weaponToRef(weapon *weapons.Weapon) *core.Ref {
	if weapon == nil {
		return nil
	}
	return refs.Weapons.ByID(weapon.ID)
}

// abilityToRef converts an ability to its core.Ref
func abilityToRef(ability abilities.Ability) *core.Ref {
	switch ability {
	case abilities.STR:
		return refs.Abilities.Strength()
	case abilities.DEX:
		return refs.Abilities.Dexterity()
	case abilities.CON:
		return refs.Abilities.Constitution()
	case abilities.INT:
		return refs.Abilities.Intelligence()
	case abilities.WIS:
		return refs.Abilities.Wisdom()
	case abilities.CHA:
		return refs.Abilities.Charisma()
	default:
		return nil
	}
}
