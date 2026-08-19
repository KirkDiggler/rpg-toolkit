// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
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

	// Instances are simple damage amounts to apply (per damage type).
	// Use for spells, conditions, environment damage where dice breakdown isn't needed.
	// Either Instances OR Components must be provided, not both.
	Instances []DamageInstanceInput

	// Components are rich damage components with dice breakdown.
	// Use for attacks where combat log needs full transparency (dice rolls, rerolls, sources).
	// Either Instances OR Components must be provided, not both.
	Components []dnd5eEvents.DamageComponent

	// IsCritical indicates if this damage is from a critical hit
	IsCritical bool

	// HasAdvantage indicates if the attack had advantage (for sneak attack eligibility, etc.)
	HasAdvantage bool

	// AbilityUsed is which ability was used for the attack, if any (e.g. STR for a
	// melee weapon attack). Used by modifiers like Rage that only apply to
	// attacks using a specific ability. Leave empty for non-attack damage
	// (spells, conditions, environment).
	AbilityUsed abilities.Ability

	// IsMelee indicates this damage came from a melee attack, as opposed to a
	// ranged one. Used alongside AbilityUsed by modifiers like Rage that only
	// apply to melee weapon attacks. Leave false for non-attack damage.
	IsMelee bool

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

	hasInstances := len(d.Instances) > 0
	hasComponents := len(d.Components) > 0

	if !hasInstances && !hasComponents {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "either Instances or Components is required")
	}
	if hasInstances && hasComponents {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "provide either Instances or Components, not both")
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

	// FinalInstances are the damage instances after chain modifiers (simplified)
	FinalInstances []DamageInstanceInput

	// FinalComponents are the full damage components after chain modifiers.
	// Contains dice rolls, rerolls, sources - everything needed for combat log.
	FinalComponents []dnd5eEvents.DamageComponent
}

// DealDamage orchestrates the three-phase damage flow:
//   - RESOLVE: Publishes to DamageChain for modifiers (resistance, rage bonus, vulnerability)
//   - APPLY: Calls Target.ApplyDamage with the modified damage
//   - NOTIFY: Publishes DamageReceivedEvent for reactions (concentration checks, logging)
func DealDamage(ctx context.Context, input *DealDamageInput) (*DealDamageOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	targetID := input.Target.GetID()

	// Build initial damage components - either from rich Components or simple Instances
	var components []dnd5eEvents.DamageComponent

	if len(input.Components) > 0 {
		// Use pre-built components (from attack with full dice breakdown)
		components = input.Components
	} else {
		// Build simple components from instances (for spells, conditions, etc.)
		components = make([]dnd5eEvents.DamageComponent, 0, len(input.Instances))
		for _, inst := range input.Instances {
			components = append(components, dnd5eEvents.DamageComponent{
				Source:     dnd5eEvents.DamageSourceType(input.Source),
				FlatBonus:  inst.Amount,
				DamageType: inst.Type,
				IsCritical: input.IsCritical,
			})
		}
	}

	// RESOLVE: use shared ResolveDamage for chain processing
	resolveOutput, err := ResolveDamage(ctx, &ResolveDamageInput{
		AttackerID:   input.AttackerID,
		TargetID:     targetID,
		Components:   components,
		IsCritical:   input.IsCritical,
		HasAdvantage: input.HasAdvantage,
		AbilityUsed:  input.AbilityUsed,
		IsMelee:      input.IsMelee,
		EventBus:     input.EventBus,
	})
	if err != nil {
		return nil, err
	}

	primaryType := components[0].DamageType

	// APPLY: apply damage to target
	applyInstances := make([]DamageInstance, 0, len(resolveOutput.FinalInstances))
	for _, inst := range resolveOutput.FinalInstances {
		applyInstances = append(applyInstances, DamageInstance{
			Amount: inst.Amount,
			Type:   string(inst.Type),
		})
	}

	applyResult := input.Target.ApplyDamage(ctx, &ApplyDamageInput{
		Instances:  applyInstances,
		IsCritical: input.IsCritical,
	})

	// NOTIFY: publish DamageReceivedEvent for reactions
	//
	// Applying above and publishing here is a latent double-apply for a target
	// that treats this topic as an instruction: a bus-attached monster's sheet
	// keeper subscribes and calls TakeDamage, so the same damage lands twice.
	// Latent rather than live — nothing ships a DealDamage caller with a
	// subscribed monster target yet — and dispositioned in rpg-toolkit#1009
	// rather than changed here, because ApplyAttackOutcome's publish is the
	// opposite case (there it IS the only application path) and the two cannot
	// be fixed by the same edit.
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
		TotalDamage:     applyResult.TotalDamage,
		CurrentHP:       applyResult.CurrentHP,
		DroppedToZero:   applyResult.DroppedToZero,
		FinalInstances:  resolveOutput.FinalInstances,
		FinalComponents: resolveOutput.FinalComponents,
	}, nil
}

// ResolveDamageInput contains parameters for resolving damage through the chain.
// Use this when you need damage calculation without HP application (e.g., in ResolveAttack).
type ResolveDamageInput struct {
	// AttackerID is the ID of the entity dealing damage
	AttackerID string

	// TargetID is the ID of the entity receiving damage
	TargetID string

	// Components are rich damage components with dice breakdown
	Components []dnd5eEvents.DamageComponent

	// IsCritical indicates if this damage is from a critical hit
	IsCritical bool

	// HasAdvantage indicates if the attack had advantage
	HasAdvantage bool

	// IsOffHandAttack indicates this is a bonus action off-hand attack (two-weapon fighting).
	// When true, the Two-Weapon Fighting style condition will add ability modifier to damage.
	IsOffHandAttack bool

	// EventBus is the event bus for publishing chain events
	EventBus events.EventBus

	// Attack-specific fields (optional, used by modifiers like Great Weapon Fighting)

	// WeaponDamage is the weapon dice notation (e.g., "2d6") for reroll modifiers
	WeaponDamage string

	// AbilityUsed is which ability was used for the attack
	AbilityUsed abilities.Ability

	// WeaponRef is a reference to the weapon used
	WeaponRef *core.Ref

	// AbilityModifier is the ability modifier for this attack (STR or DEX mod).
	// Used by Two-Weapon Fighting style to add modifier to off-hand damage.
	AbilityModifier int

	// IsMelee indicates this is a melee attack (mirrors AttackChainEvent.IsMelee).
	// Used by modifiers like Rage that only apply to melee weapon attacks.
	IsMelee bool
}

// ResolveDamageOutput contains the result of damage resolution (before HP application).
type ResolveDamageOutput struct {
	// TotalDamage is the sum of all damage (after modifiers and multipliers)
	TotalDamage int

	// FinalInstances are the damage instances after chain modifiers (simplified)
	FinalInstances []DamageInstanceInput

	// FinalComponents are the full damage components after chain modifiers
	FinalComponents []dnd5eEvents.DamageComponent

	// AbilityUsed is the ability that was used for the attack after chain modifiers.
	// Conditions like Martial Arts may change this (e.g., STR -> DEX).
	AbilityUsed abilities.Ability
}

// ResolveDamage processes damage through the chain without applying HP changes.
// Use this from ResolveAttack to calculate damage, or use DealDamage for full flow.
func ResolveDamage(ctx context.Context, input *ResolveDamageInput) (*ResolveDamageOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "ResolveDamageInput is nil")
	}
	if len(input.Components) == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Components is required")
	}
	if input.EventBus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "EventBus is required")
	}

	// Publish through DamageChain for modifiers
	damageEvent := dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
		AttackerID:       input.AttackerID,
		TargetID:         input.TargetID,
		Components:       input.Components,
		WeaponDamageDice: input.WeaponDamage,
		WeaponDamageType: input.Components[0].DamageType,
		IsCritical:       input.IsCritical,
		HasAdvantage:     input.HasAdvantage,
		IsOffHandAttack:  input.IsOffHandAttack,
		AbilityModifier:  input.AbilityModifier,
		// Attack-specific fields (for modifiers like GWF that need weapon info)
		AbilityUsed: input.AbilityUsed,
		WeaponRef:   input.WeaponRef,
		IsMelee:     input.IsMelee,
	})

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](ModifierStages)
	damages := dnd5eEvents.DamageChain.On(input.EventBus)

	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish damage chain")
	}

	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to execute damage chain")
	}

	// Apply multipliers (resistance, vulnerability, immunity) and total them.
	// Shared with any caller that folds the chain on its own bus — one
	// implementation of the arithmetic rather than a copy per stack.
	finalInstances, totalDamage := FinalDamage(finalEvent.Components)

	return &ResolveDamageOutput{
		TotalDamage:     totalDamage,
		FinalInstances:  finalInstances,
		FinalComponents: finalEvent.Components,
		AbilityUsed:     finalEvent.AbilityUsed,
	}, nil
}
