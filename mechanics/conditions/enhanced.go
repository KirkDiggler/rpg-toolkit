// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// EnhancedCondition provides a full-featured condition with mechanical effects.
type EnhancedCondition struct {
	*SimpleCondition
	conditionType ConditionType
	definition    *ConditionDefinition
	level         int                    // For exhaustion
	saveDC        int                    // For conditions that allow saves
	metadata      map[string]interface{} // Additional condition-specific data
}

// EnhancedConditionConfig holds configuration for creating an enhanced condition.
type EnhancedConditionConfig struct {
	ID            string
	ConditionType ConditionType
	Target        core.Entity
	Source        string
	Level         int                    // For exhaustion (1-6)
	SaveDC        int                    // DC for saving throws to end condition
	Duration      events.Duration        // How long the condition lasts
	Metadata      map[string]interface{} // Additional data
}

// NewEnhancedCondition creates a new enhanced condition with full mechanical effects.
func NewEnhancedCondition(cfg EnhancedConditionConfig) (*EnhancedCondition, error) {
	// Get the condition definition
	def, exists := GetConditionDefinition(cfg.ConditionType)
	if !exists {
		return nil, fmt.Errorf("unknown condition type: %s", cfg.ConditionType)
	}

	// Games can add their own validation for specific condition types

	ec := &EnhancedCondition{
		conditionType: cfg.ConditionType,
		definition:    def,
		level:         cfg.Level,
		saveDC:        cfg.SaveDC,
		metadata:      cfg.Metadata,
	}

	// Create the simple condition with our apply/remove handlers
	simpleConfig := SimpleConditionConfig{
		ID:     cfg.ID,
		Type:   string(cfg.ConditionType),
		Target: cfg.Target,
		Source: cfg.Source,
		ApplyFunc: func(_ *SimpleCondition, bus events.EventBus) error {
			return ec.applyEffects(bus)
		},
		RemoveFunc: func(_ *SimpleCondition, _ events.EventBus) error {
			// Remove is handled by SimpleCondition's subscription tracking
			return nil
		},
	}

	ec.SimpleCondition = NewSimpleCondition(simpleConfig)
	return ec, nil
}

// GetConditionType returns the type of this condition.
func (ec *EnhancedCondition) GetConditionType() ConditionType {
	return ec.conditionType
}

// GetLevel returns the level (for exhaustion).
func (ec *EnhancedCondition) GetLevel() int {
	return ec.level
}

// GetSaveDC returns the save DC to end this condition.
func (ec *EnhancedCondition) GetSaveDC() int {
	return ec.saveDC
}

// GetMetadata returns condition-specific metadata.
func (ec *EnhancedCondition) GetMetadata(key string) (interface{}, bool) {
	if ec.metadata == nil {
		return nil, false
	}
	val, exists := ec.metadata[key]
	return val, exists
}

// applyEffects registers all mechanical effects with the event bus.
func (ec *EnhancedCondition) applyEffects(bus events.EventBus) error {
	// Get effects based on condition type
	effects := ec.definition.Effects

	// Games can implement special effect handling based on condition type and level

	// Apply each effect
	for _, effect := range effects {
		if err := ec.applyEffect(bus, effect); err != nil {
			return fmt.Errorf("failed to apply effect %s: %w", effect.Type, err)
		}
	}

	// Handle included conditions (e.g., Paralyzed includes Incapacitated)
	// This is handled by the condition manager to avoid circular dependencies

	return nil
}

// applyEffect applies a single mechanical effect.
func (ec *EnhancedCondition) applyEffect(bus events.EventBus, effect ConditionEffect) error {
	switch effect.Type {
	case EffectAdvantage:
		return ec.applyAdvantageEffect(bus, effect)
	case EffectDisadvantage:
		return ec.applyDisadvantageEffect(bus, effect)
	case EffectAutoFail:
		return ec.applyAutoFailEffect(bus, effect)
	case EffectSpeedZero:
		return ec.applySpeedEffect(bus, effect, 0)
	case EffectSpeedReduction:
		if factor, ok := effect.Value.(float64); ok {
			return ec.applySpeedEffect(bus, effect, factor)
		}
		return fmt.Errorf("speed reduction requires float64 value")
	case EffectIncapacitated:
		return ec.applyIncapacitatedEffect(bus)
	case EffectNoReactions:
		return ec.applyNoReactionsEffect(bus)
	case EffectCantSpeak, EffectCantHear, EffectCantSee:
		// These are informational and handled by specific game implementations
		return nil
	case EffectResistance, EffectVulnerability, EffectImmunity:
		return ec.applyDamageModificationEffect(bus, effect)
	default:
		// Unknown effects are ignored (allows for custom effects)
		return nil
	}
}

// applyAdvantageEffect applies advantage to specific rolls.
func (ec *EnhancedCondition) applyAdvantageEffect(bus events.EventBus, effect ConditionEffect) error {
	handler := func(_ context.Context, event events.Event) error {
		// Check if this event matches the target
		if !ec.eventMatchesTarget(event, effect.Target) {
			return nil
		}

		// Check if the entity is involved
		if !ec.isEntityInvolved(event, effect.Target) {
			return nil
		}

		// Add advantage modifier
		event.Context().AddModifier(events.NewModifier(
			fmt.Sprintf("%s_advantage", ec.conditionType),
			events.ModifierAdvantage,
			events.IntValue(1),
			100, // High priority
		))

		return nil
	}

	// Subscribe to relevant events
	eventTypes := ec.getEventTypesForTarget(effect.Target)
	for _, eventType := range eventTypes {
		ec.Subscribe(bus, eventType, 100, handler)
	}

	return nil
}

// applyDisadvantageEffect applies disadvantage to specific rolls.
func (ec *EnhancedCondition) applyDisadvantageEffect(bus events.EventBus, effect ConditionEffect) error {
	handler := func(_ context.Context, event events.Event) error {
		if !ec.eventMatchesTarget(event, effect.Target) {
			return nil
		}

		if !ec.isEntityInvolved(event, effect.Target) {
			return nil
		}

		event.Context().AddModifier(events.NewModifier(
			fmt.Sprintf("%s_disadvantage", ec.conditionType),
			events.ModifierDisadvantage,
			events.IntValue(1),
			100,
		))

		return nil
	}

	eventTypes := ec.getEventTypesForTarget(effect.Target)
	for _, eventType := range eventTypes {
		ec.Subscribe(bus, eventType, 100, handler)
	}

	return nil
}

// applyAutoFailEffect makes specific checks automatically fail.
func (ec *EnhancedCondition) applyAutoFailEffect(bus events.EventBus, effect ConditionEffect) error {
	handler := func(_ context.Context, event events.Event) error {
		if !ec.eventMatchesTarget(event, effect.Target) {
			return nil
		}

		if !ec.isEntityInvolved(event, effect.Target) {
			return nil
		}

		// Set auto-fail flag
		event.Context().Set("auto_fail", true)
		event.Context().Set("auto_fail_reason", fmt.Sprintf("%s condition", ec.conditionType))

		return nil
	}

	eventTypes := ec.getEventTypesForTarget(effect.Target)
	for _, eventType := range eventTypes {
		ec.Subscribe(bus, eventType, 50, handler) // Earlier priority to ensure it's set
	}

	return nil
}

// applySpeedEffect modifies movement speed.
func (ec *EnhancedCondition) applySpeedEffect(bus events.EventBus, _ ConditionEffect, factor float64) error {
	handler := func(_ context.Context, event events.Event) error {
		if event.Type() != EventOnMovement {
			return nil
		}

		if event.Source() != ec.Target() {
			return nil
		}

		if factor == 0 {
			event.Context().Set("speed_multiplier", 0.0)
			event.Context().Set("speed_zero_reason", fmt.Sprintf("%s condition", ec.conditionType))
		} else {
			// Get current multiplier and apply our factor
			currentMult := 1.0
			if mult, exists := event.Context().GetFloat64("speed_multiplier"); exists {
				currentMult = mult
			}
			event.Context().Set("speed_multiplier", currentMult*factor)
		}

		return nil
	}

	ec.Subscribe(bus, EventOnMovement, 100, handler)
	return nil
}

// applyIncapacitatedEffect prevents actions and reactions.
func (ec *EnhancedCondition) applyIncapacitatedEffect(bus events.EventBus) error {
	handler := func(_ context.Context, event events.Event) error {
		// Check for action/reaction attempts
		if event.Type() != EventBeforeAction && event.Type() != EventBeforeReaction {
			return nil
		}

		if event.Source() != ec.Target() {
			return nil
		}

		// Cancel the action/reaction
		event.Cancel()
		event.Context().Set("cancel_reason", fmt.Sprintf("incapacitated by %s", ec.conditionType))

		return nil
	}

	ec.Subscribe(bus, EventBeforeAction, 50, handler)
	ec.Subscribe(bus, EventBeforeReaction, 50, handler)
	return nil
}

// applyNoReactionsEffect prevents reactions.
func (ec *EnhancedCondition) applyNoReactionsEffect(bus events.EventBus) error {
	handler := func(_ context.Context, event events.Event) error {
		if event.Type() != EventBeforeReaction {
			return nil
		}

		if event.Source() != ec.Target() {
			return nil
		}

		event.Cancel()
		event.Context().Set("cancel_reason", fmt.Sprintf("no reactions due to %s", ec.conditionType))

		return nil
	}

	ec.Subscribe(bus, EventBeforeReaction, 50, handler)
	return nil
}

// applyDamageModificationEffect applies resistance, vulnerability, or immunity.
func (ec *EnhancedCondition) applyDamageModificationEffect(bus events.EventBus, effect ConditionEffect) error {
	handler := func(_ context.Context, event events.Event) error {
		if event.Type() != events.EventBeforeTakeDamage {
			return nil
		}

		if event.Target() != ec.Target() {
			return nil
		}

		// Get damage type
		damageType, _ := event.Context().GetString(events.ContextKeyDamageType)

		// Check if this effect applies to this damage type
		if effectDamageType, ok := effect.Value.(string); ok {
			if effectDamageType != "all" && effectDamageType != damageType {
				return nil
			}
		}

		// Apply the modification
		switch effect.Type {
		case EffectResistance:
			event.Context().AddModifier(events.NewModifier(
				fmt.Sprintf("%s_resistance", ec.conditionType),
				"damage_resistance",
				events.NewRawValue(50, string(ec.conditionType)), // 50% reduction
				200,
			))
		case EffectVulnerability:
			event.Context().AddModifier(events.NewModifier(
				fmt.Sprintf("%s_vulnerability", ec.conditionType),
				"damage_vulnerability",
				events.NewRawValue(200, string(ec.conditionType)), // 200% damage
				200,
			))
		case EffectImmunity:
			event.Context().Set("damage_immunity", true)
			event.Context().Set("immunity_source", string(ec.conditionType))
		}

		return nil
	}

	ec.Subscribe(bus, events.EventBeforeTakeDamage, 100, handler)
	return nil
}

// Helper methods

// eventMatchesTarget checks if an event matches the effect target.
func (ec *EnhancedCondition) eventMatchesTarget(event events.Event, target EffectTarget) bool {
	eventType := event.Type()

	switch target {
	case TargetAttackRolls:
		return eventType == events.EventOnAttackRoll
	case TargetSavingThrows, TargetAllSaves:
		return eventType == events.EventOnSavingThrow
	case TargetAbilityChecks, TargetAllChecks:
		return eventType == events.EventOnAbilityCheck
	case TargetDexSaves, TargetStrSaves:
		if eventType != events.EventOnSavingThrow {
			return false
		}
		// Check save type in context
		saveType, _ := event.Context().GetString("save_type")
		return (target == TargetDexSaves && saveType == "dexterity") ||
			(target == TargetStrSaves && saveType == "strength")
	case TargetSight, TargetHearing:
		// Check if ability check requires sight/hearing
		if eventType != events.EventOnAbilityCheck {
			return false
		}
		checkType, _ := event.Context().GetString("check_type")
		return (target == TargetSight && checkType == "perception_sight") ||
			(target == TargetHearing && checkType == "perception_hearing")
	case TargetMovement:
		return eventType == EventOnMovement
	case TargetAttacksAgainst:
		return eventType == events.EventOnAttackRoll
	default:
		return false
	}
}

// isEntityInvolved checks if the target entity is involved in the event correctly.
func (ec *EnhancedCondition) isEntityInvolved(event events.Event, target EffectTarget) bool {
	if target == TargetAttacksAgainst {
		// For attacks against, the condition target should be the event target
		return event.Target() == ec.Target()
	}
	// For most effects, the condition target should be the event source
	return event.Source() == ec.Target()
}

// getEventTypesForTarget returns the event types to subscribe to for an effect target.
func (ec *EnhancedCondition) getEventTypesForTarget(target EffectTarget) []string {
	switch target {
	case TargetAttackRolls, TargetAttacksAgainst:
		return []string{events.EventOnAttackRoll}
	case TargetSavingThrows, TargetAllSaves, TargetDexSaves, TargetStrSaves:
		return []string{events.EventOnSavingThrow}
	case TargetAbilityChecks, TargetAllChecks, TargetSight, TargetHearing:
		return []string{events.EventOnAbilityCheck}
	case TargetMovement:
		return []string{EventOnMovement}
	case TargetActions:
		return []string{EventBeforeAction}
	case TargetReactions:
		return []string{EventBeforeReaction}
	case TargetDamage:
		return []string{events.EventBeforeTakeDamage}
	default:
		return []string{}
	}
}
