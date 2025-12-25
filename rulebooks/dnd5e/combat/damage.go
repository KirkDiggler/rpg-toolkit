// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// DamageSource identifies the origin of damage.
type DamageSource string

// Damage source constants.
const (
	// DamageSourceAttack indicates damage from a weapon attack.
	DamageSourceAttack DamageSource = "attack"

	// DamageSourceSpell indicates damage from a spell.
	DamageSourceSpell DamageSource = "spell"

	// DamageSourceCondition indicates damage from a condition (poison, ongoing fire, etc.).
	DamageSourceCondition DamageSource = "condition"

	// DamageSourceEnvironment indicates damage from environmental hazards.
	DamageSourceEnvironment DamageSource = "environment"
)

// DamageInstanceInput represents a single damage amount with its type.
// Multiple instances allow mixed-type damage (e.g., flametongue: slashing + fire).
type DamageInstanceInput struct {
	// Amount is the base damage before modifiers
	Amount int

	// Type is the damage type (slashing, fire, etc.)
	Type damage.Type
}

// DealDamageInput contains parameters for dealing damage via the event chain.
type DealDamageInput struct {
	// Target is the combatant receiving damage.
	// Caller is responsible for looking up the target (e.g., via gamectx.GetCombatant).
	Target Combatant

	// AttackerID is the ID of the entity dealing damage (optional, for modifier context)
	AttackerID string

	// Source identifies where the damage comes from
	Source DamageSource

	// Instances are the damage amounts to apply (per damage type)
	Instances []DamageInstanceInput

	// IsCritical indicates if this damage is from a critical hit
	IsCritical bool

	// EventBus is the event bus for publishing chain and notification events
	EventBus events.EventBus
}

// Validate validates the input.
func (d *DealDamageInput) Validate() error {
	if d == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "DealDamageInput is nil")
	}
	if d.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Target is required")
	}
	if d.EventBus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "EventBus is required")
	}
	if len(d.Instances) == 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "at least one damage instance is required")
	}
	return nil
}

// DealDamageOutput contains the result of dealing damage.
type DealDamageOutput struct {
	// TotalDamage is the sum of all damage applied (after modifiers)
	TotalDamage int

	// CurrentHP is the target's HP after damage
	CurrentHP int

	// DroppedToZero is true if this damage reduced the target to 0 HP
	DroppedToZero bool

	// FinalInstances are the damage instances after chain modifiers
	FinalInstances []DamageInstanceInput
}

// DealDamage orchestrates the two-phase damage flow:
// 1. RESOLVE: Publishes to DamageChain for modifiers (resistance, rage bonus, etc.)
// 2. APPLY: Calls Target.ApplyDamage with the modified damage
// 3. NOTIFY: Publishes DamageReceivedEvent for reactions (concentration, logging)
//
// The caller is responsible for looking up the target combatant (e.g., via gamectx.GetCombatant)
// and passing it in the input.
func DealDamage(ctx context.Context, input *DealDamageInput) (*DealDamageOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	targetID := input.Target.GetID()

	// Build initial damage components from input instances
	components := make([]dnd5eEvents.DamageComponent, 0, len(input.Instances))
	for _, inst := range input.Instances {
		components = append(components, dnd5eEvents.DamageComponent{
			Source:     dnd5eEvents.DamageSourceType(input.Source),
			FlatBonus:  inst.Amount,
			DamageType: inst.Type,
			IsCritical: input.IsCritical,
		})
	}

	// Determine primary damage type (first instance)
	primaryType := input.Instances[0].Type

	// PHASE 1: RESOLVE - publish through DamageChain for modifiers
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: input.AttackerID,
		TargetID:   targetID,
		Components: components,
		DamageType: primaryType,
		IsCritical: input.IsCritical,
	}

	// Create and publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](ModifierStages)
	damages := dnd5eEvents.DamageChain.On(input.EventBus)

	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish damage chain")
	}

	// Execute chain to get final modifiers
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to execute damage chain")
	}

	// Calculate total damage from all components after chain modifiers
	// Group components by damage type and apply multipliers
	finalInstances := calculateFinalDamage(finalEvent.Components)

	// PHASE 2: APPLY - apply damage to target
	applyInstances := make([]DamageInstance, 0, len(finalInstances))
	for _, inst := range finalInstances {
		applyInstances = append(applyInstances, DamageInstance{
			Amount: inst.Amount,
			Type:   string(inst.Type),
		})
	}

	applyResult := input.Target.ApplyDamage(ctx, &ApplyDamageInput{
		Instances:  applyInstances,
		IsCritical: input.IsCritical,
	})

	// PHASE 3: NOTIFY - publish DamageReceivedEvent for reactions
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(input.EventBus)
	err = damageTopic.Publish(ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   targetID,
		SourceID:   input.AttackerID,
		Amount:     applyResult.TotalDamage,
		DamageType: primaryType,
	})
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish damage received event")
	}

	return &DealDamageOutput{
		TotalDamage:    applyResult.TotalDamage,
		CurrentHP:      applyResult.CurrentHP,
		DroppedToZero:  applyResult.DroppedToZero,
		FinalInstances: finalInstances,
	}, nil
}

// calculateFinalDamage processes damage components and applies multipliers.
// In D&D 5e:
// - Resistance (0.5) halves damage, Vulnerability (2.0) doubles it, Immunity (0.0) negates
// - Multiple resistances don't stack (apply most beneficial once)
// - If both resistance and vulnerability exist for a type, they cancel out
func calculateFinalDamage(components []dnd5eEvents.DamageComponent) []DamageInstanceInput {
	// Group damage and multipliers by type
	type damageGroup struct {
		baseDamage  int
		multipliers []float64
	}
	byType := make(map[damage.Type]*damageGroup)

	for _, component := range components {
		dmgType := component.DamageType
		if byType[dmgType] == nil {
			byType[dmgType] = &damageGroup{}
		}

		// If component has a multiplier, it's a modifier (resistance/vulnerability)
		// Otherwise, it contributes base damage
		if component.Multiplier != 0 {
			byType[dmgType].multipliers = append(byType[dmgType].multipliers, component.Multiplier)
		} else {
			byType[dmgType].baseDamage += component.Total()
		}
	}

	// Apply multipliers to each damage type
	result := make([]DamageInstanceInput, 0, len(byType))
	for dmgType, group := range byType {
		finalDamage := group.baseDamage

		if len(group.multipliers) > 0 {
			// Apply D&D 5e stacking rules
			effectiveMultiplier := resolveMultipliers(group.multipliers)
			finalDamage = int(float64(finalDamage) * effectiveMultiplier)
		}

		if finalDamage > 0 {
			result = append(result, DamageInstanceInput{
				Amount: finalDamage,
				Type:   dmgType,
			})
		}
	}

	return result
}

// resolveMultipliers applies D&D 5e stacking rules for resistance/vulnerability.
// - Immunity (0.0) always wins
// - Resistance (0.5) and vulnerability (2.0) cancel out if both present
// - Multiple resistances don't stack (use 0.5 once)
// - Multiple vulnerabilities don't stack (use 2.0 once)
func resolveMultipliers(multipliers []float64) float64 {
	hasImmunity := false
	hasResistance := false
	hasVulnerability := false

	for _, m := range multipliers {
		switch {
		case m == 0.0:
			hasImmunity = true
		case m < 1.0:
			hasResistance = true
		case m > 1.0:
			hasVulnerability = true
		}
	}

	// Immunity trumps everything
	if hasImmunity {
		return 0.0
	}

	// Resistance and vulnerability cancel out
	if hasResistance && hasVulnerability {
		return 1.0
	}

	// Apply resistance (0.5) or vulnerability (2.0)
	if hasResistance {
		return 0.5
	}
	if hasVulnerability {
		return 2.0
	}

	return 1.0
}
