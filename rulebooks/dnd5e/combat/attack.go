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

// AttackHand indicates which hand is making the attack.
type AttackHand string

const (
	// AttackHandMain is the default - a main hand attack using a standard action.
	AttackHandMain AttackHand = "main"

	// AttackHandOff is an off-hand attack using a bonus action (two-weapon fighting).
	AttackHandOff AttackHand = "off"
)

// EquippedWeaponInfo provides weapon information needed for two-weapon fighting validation.
// This interface is satisfied by gamectx.EquippedWeapon.
type EquippedWeaponInfo struct {
	WeaponID weapons.WeaponID
}

// TwoWeaponContext provides character weapon and action economy information
// needed for two-weapon fighting validation.
type TwoWeaponContext interface {
	// GetMainHandWeapon returns the weapon in the main hand, or nil if none.
	GetMainHandWeapon(characterID string) *EquippedWeaponInfo

	// GetOffHandWeapon returns the weapon in the off hand (not shield), or nil if none.
	GetOffHandWeapon(characterID string) *EquippedWeaponInfo

	// GetActionEconomy returns the action economy for the character, or nil if not available.
	GetActionEconomy(characterID string) *ActionEconomy
}

// twoWeaponContextKey is the context key for TwoWeaponContext
type twoWeaponContextKey struct{}

// WithTwoWeaponContext adds a TwoWeaponContext to the context.
func WithTwoWeaponContext(ctx context.Context, twc TwoWeaponContext) context.Context {
	return context.WithValue(ctx, twoWeaponContextKey{}, twc)
}

// GetTwoWeaponContext retrieves the TwoWeaponContext from context.
func GetTwoWeaponContext(ctx context.Context) (TwoWeaponContext, bool) {
	twc, ok := ctx.Value(twoWeaponContextKey{}).(TwoWeaponContext)
	return twc, ok
}

// AttackInput provides all information needed to resolve an attack.
// Combatants are looked up from context using the CombatantLookup interface.
// Use WithCombatantLookup to add a combatant registry to the context.
type AttackInput struct {
	// AttackerID is the combatant performing the attack.
	// The attacker is looked up from context using GetCombatantFromContext.
	AttackerID string

	// TargetID is the combatant being attacked.
	// The target is looked up from context using GetCombatantFromContext.
	TargetID string

	// Weapon is the weapon being used for the attack.
	Weapon *weapons.Weapon

	// EventBus is required for publishing attack/damage events.
	EventBus events.EventBus

	// Roller is the dice roller for attack and damage rolls.
	// If nil, a default roller is used.
	Roller dice.Roller

	// AttackHand indicates which hand is making the attack.
	// Default (empty or AttackHandMain) is a main hand attack.
	// AttackHandOff triggers two-weapon fighting validation and consumes a bonus action.
	AttackHand AttackHand
}

// Validate validates the input.
// Note: This only validates the input fields, not that the combatants exist in context.
// Combatant lookup errors are returned by ResolveAttack.
func (ai *AttackInput) Validate() error {
	if ai == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "AttackInput is nil")
	}

	if ai.AttackerID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "AttackerID is required")
	}

	if ai.TargetID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "TargetID is required")
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

	// Look up attacker from context
	attacker, err := GetCombatantFromContext(ctx, input.AttackerID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to look up attacker %s", input.AttackerID)
	}

	// Look up defender from context
	defender, err := GetCombatantFromContext(ctx, input.TargetID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to look up defender %s", input.TargetID)
	}

	// Get attacker stats for attack calculations
	attackerScores := attacker.AbilityScores()
	proficiencyBonus := attacker.ProficiencyBonus()

	// Get defender's effective AC (uses AC chain for Characters, base AC for Monsters)
	defenderAC := GetEffectiveAC(ctx, defender)

	// Track if this is an off-hand attack for the damage chain
	isOffHandAttack := input.AttackHand == AttackHandOff

	// Validate two-weapon fighting requirements for off-hand attacks
	if isOffHandAttack {
		if err := validateOffHandAttack(ctx, input); err != nil {
			return nil, err
		}
	}

	// Use provided roller or default
	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	result := &AttackResult{
		DamageType: input.Weapon.DamageType,
		TargetAC:   defenderAC,
	}

	// Step 1: Calculate base attack bonus (ability modifier + proficiency)
	abilityMod := calculateAttackAbilityModifier(input.Weapon, attackerScores)
	baseBonus := abilityMod + proficiencyBonus

	// Step 2: Fire attack chain BEFORE the roll to collect advantage/disadvantage and modifiers
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:          input.AttackerID,
		TargetID:            input.TargetID,
		WeaponRef:           weaponToRef(input.Weapon),
		IsMelee:             !input.Weapon.IsRanged(),
		AttackType:          dnd5eEvents.AttackTypeStandard, // Default to standard attack
		AdvantageSources:    nil,
		DisadvantageSources: nil,
		CancellationSources: nil,
		AttackBonus:         baseBonus,
		TargetAC:            defenderAC,
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
		result.Hit = result.TotalAttack >= defenderAC
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
		abilityUsed := determineAbilityUsed(input.Weapon, attackerScores)

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
			AttackerID:      input.AttackerID,
			TargetID:        input.TargetID,
			Components:      []dnd5eEvents.DamageComponent{weaponComponent, abilityComponent},
			IsCritical:      result.Critical,
			HasAdvantage:    result.HasAdvantage,
			IsOffHandAttack: isOffHandAttack,
			AbilityModifier: abilityMod,
			EventBus:        input.EventBus,
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
			TargetID:   input.TargetID,
			SourceID:   input.AttackerID,
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

// validateOffHandAttack validates two-weapon fighting requirements for off-hand attacks.
// Returns an error if requirements are not met.
func validateOffHandAttack(ctx context.Context, input *AttackInput) error {
	// Get two-weapon context
	twc, ok := GetTwoWeaponContext(ctx)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "two-weapon context not available for off-hand attack validation")
	}

	characterID := input.AttackerID

	// Check main hand weapon
	mainHand := twc.GetMainHandWeapon(characterID)
	if mainHand == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no weapon in main hand")
	}

	// Look up main hand weapon properties
	mainWeapon, err := weapons.GetByID(mainHand.WeaponID)
	if err != nil {
		return rpgerr.Wrapf(err, "unknown main hand weapon: %s", mainHand.WeaponID)
	}

	if !mainWeapon.HasProperty(weapons.PropertyLight) {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "main hand weapon must be light for two-weapon fighting")
	}

	// Check off hand weapon
	offHand := twc.GetOffHandWeapon(characterID)
	if offHand == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no weapon in off hand")
	}

	// Look up off hand weapon properties
	offWeapon, err := weapons.GetByID(offHand.WeaponID)
	if err != nil {
		return rpgerr.Wrapf(err, "unknown off hand weapon: %s", offHand.WeaponID)
	}

	if !offWeapon.HasProperty(weapons.PropertyLight) {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "off hand weapon must be light for two-weapon fighting")
	}

	// Check and consume bonus action
	actionEconomy := twc.GetActionEconomy(characterID)
	if actionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy not available for off-hand attack")
	}

	if err := actionEconomy.UseBonusAction(); err != nil {
		return err // Already returns CodeResourceExhausted with "bonus action"
	}

	return nil
}
