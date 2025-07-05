// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ComposedCondition shows how conditions can use behavioral interfaces
// while maintaining their identity as conditions.
type ComposedCondition struct {
	*Core
	owner core.Entity

	// Composed behaviors - only include what's needed
	conditional ConditionalEffect
	temporary   TemporaryEffect
	stackable   StackableEffect
	dice        DiceModifier
	saves       SavingThrowEffect
}

// ComposedConditionConfig provides configuration for creating composed conditions
type ComposedConditionConfig struct {
	ID     string
	Type   string
	Source string
	Owner  core.Entity

	// Optional behaviors
	Conditional ConditionalEffect
	Temporary   TemporaryEffect
	Stackable   StackableEffect
	Dice        DiceModifier
	Saves       SavingThrowEffect

	// Apply/Remove functions that use the behaviors
	ApplyFunc  func(c *ComposedCondition, bus events.EventBus) error
	RemoveFunc func(c *ComposedCondition, bus events.EventBus) error
}

// NewComposedCondition creates a condition with composed behaviors
func NewComposedCondition(cfg ComposedConditionConfig) *ComposedCondition {
	cond := &ComposedCondition{
		owner:       cfg.Owner,
		conditional: cfg.Conditional,
		temporary:   cfg.Temporary,
		stackable:   cfg.Stackable,
		dice:        cfg.Dice,
		saves:       cfg.Saves,
	}

	// Create core with wrapped apply/remove that use behaviors
	cond.Core = NewCore(CoreConfig{
		ID:     cfg.ID,
		Type:   cfg.Type,
		Source: cfg.Source,
		ApplyFunc: func(bus events.EventBus) error {
			if cfg.ApplyFunc != nil {
				return cfg.ApplyFunc(cond, bus)
			}
			return cond.defaultApply(bus)
		},
		RemoveFunc: func(bus events.EventBus) error {
			if cfg.RemoveFunc != nil {
				return cfg.RemoveFunc(cond, bus)
			}
			return nil
		},
	})

	return cond
}

// defaultApply sets up standard behavior subscriptions
func (c *ComposedCondition) defaultApply(bus events.EventBus) error {
	// If it has dice modifiers, subscribe to relevant roll events
	if c.dice != nil {
		modType := c.dice.GetModifierType()
		eventTypes := c.getEventTypesForModifier(modType)

		for _, eventType := range eventTypes {
			c.Subscribe(bus, eventType, 50, events.HandlerFunc(func(ctx context.Context, e events.Event) error {
				// Check conditions if any
				if c.conditional != nil && !c.conditional.CheckCondition(ctx, e) {
					return nil
				}

				// Apply dice modifier if it should
				if c.dice.ShouldApply(ctx, e) {
					c.applyDiceModifier(ctx, e)
				}

				return nil
			}))
		}
	}

	// If temporary, set up expiration checking
	if c.temporary != nil {
		c.Subscribe(bus, "time.round_end", 10, events.HandlerFunc(func(ctx context.Context, e events.Event) error {
			if c.temporary.CheckExpiration(ctx, time.Now()) {
				return c.temporary.OnExpire(bus)
			}
			return nil
		}))
	}

	// If it requires saves, set up save checking
	if c.saves != nil {
		details := c.saves.GetSaveDetails()
		if details.RepeatType == SaveRepeat {
			c.Subscribe(bus, details.RepeatValue, 20, events.HandlerFunc(func(ctx context.Context, e events.Event) error {
				// Trigger a save event
				saveEvent := events.NewGameEvent("save.required", c, c.owner)
				saveEvent.Context().Set("ability", details.Ability)
				saveEvent.Context().Set("dc", details.DC)
				saveEvent.Context().Set("condition", c)
				_ = bus.Publish(ctx, saveEvent)
				return nil
			}))
		}
	}

	return nil
}

// Helper to map modifier types to event types
func (c *ComposedCondition) getEventTypesForModifier(modType ModifierType) []string {
	switch modType {
	case ModifierAttack:
		return []string{"attack.before", "attack.calculate"}
	case ModifierDamage:
		return []string{"damage.calculate"}
	case ModifierSave:
		return []string{"save.before", "save.calculate"}
	case ModifierSkill:
		return []string{"skill.check.before", "skill.check.calculate"}
	case ModifierAll:
		return []string{"roll.before", "roll.calculate"}
	default:
		return []string{}
	}
}

// applyDiceModifier adds the dice expression to the event
func (c *ComposedCondition) applyDiceModifier(ctx context.Context, e events.Event) {
	expr := c.dice.GetDiceExpression(ctx, e)

	// Add to event context
	if data, ok := e.(*events.GameEvent); ok {
		val, _ := data.Context().Get("modifiers")
		modifiers, _ := val.([]interface{})
		modifiers = append(modifiers, map[string]interface{}{
			"source":     c.GetID(),
			"expression": expr,
			"type":       "dice",
		})
		data.Context().Set("modifiers", modifiers)
	}
}

// Owner returns the entity that owns this condition
func (c *ComposedCondition) Owner() core.Entity {
	return c.owner
}

// CanStack checks if this condition can stack with another
func (c *ComposedCondition) CanStack(other core.Entity) bool {
	if c.stackable == nil {
		return false // Default: conditions don't stack
	}
	return c.stackable.CanStackWith(other)
}

// Example factory functions for common conditions

// CreateBlessCondition creates a Bless condition using composition
func CreateBlessCondition(owner core.Entity, source string) *ComposedCondition {
	return NewComposedCondition(ComposedConditionConfig{
		ID:     "bless-" + owner.GetID(),
		Type:   "condition.bless",
		Source: source,
		Owner:  owner,
		Temporary: &SimpleDuration{
			Duration: Duration{
				Type:  DurationMinutes,
				Value: 1,
			},
		},
		Dice: &SimpleDiceModifier{
			Expression: "1d4",
			ModType:    ModifierAttack,
			AppliesTo: func(ctx context.Context, e events.Event) bool {
				// Applies to attack rolls and saving throws
				return e.Type() == "attack.before" || e.Type() == "save.before"
			},
		},
		Stackable: &NoStacking{}, // Bless doesn't stack
	})
}

// CreatePoisonedCondition creates a Poisoned condition
func CreatePoisonedCondition(owner core.Entity, source string, poisonDC int) *ComposedCondition {
	return NewComposedCondition(ComposedConditionConfig{
		ID:     "poisoned-" + owner.GetID(),
		Type:   "condition.poisoned",
		Source: source,
		Owner:  owner,
		Saves: &SimpleSaveEffect{
			Details: SaveDetails{
				Ability:     "constitution",
				DC:          poisonDC,
				RepeatType:  SaveRepeat,
				RepeatValue: "turn_end",
			},
			OnSuccess: func(ctx context.Context, bus events.EventBus) error {
				// Remove condition on save
				return nil // Would trigger removal
			},
			OnFailure: func(ctx context.Context, bus events.EventBus) error {
				// Poisoned gives disadvantage on attacks
				return nil
			},
		},
		ApplyFunc: func(c *ComposedCondition, bus events.EventBus) error {
			// Subscribe to give disadvantage on attacks
			c.Subscribe(bus, "attack.before", 40, events.HandlerFunc(func(ctx context.Context, e events.Event) error {
				if data, ok := e.(*events.GameEvent); ok {
					val, _ := data.Context().Get("attacker")
					if attacker, ok := val.(core.Entity); ok && attacker.GetID() == owner.GetID() {
						data.Context().Set("disadvantage", true)
					}
				}
				return nil
			}))
			return nil
		},
	})
}

// Simple implementations of behaviors for examples

// SimpleDuration provides a basic duration implementation
type SimpleDuration struct {
	Duration  Duration
	StartTime time.Time
}

func (s *SimpleDuration) GetDuration() Duration { return s.Duration }
func (s *SimpleDuration) CheckExpiration(ctx context.Context, current time.Time) bool {
	if s.Duration.Type == DurationMinutes {
		elapsed := current.Sub(s.StartTime).Minutes()
		return elapsed >= float64(s.Duration.Value)
	}
	return false
}
func (s *SimpleDuration) OnExpire(bus events.EventBus) error { return nil }

type SimpleDiceModifier struct {
	Expression string
	ModType    ModifierType
	AppliesTo  func(context.Context, events.Event) bool
}

func (s *SimpleDiceModifier) GetDiceExpression(ctx context.Context, e events.Event) string {
	return s.Expression
}
func (s *SimpleDiceModifier) GetModifierType() ModifierType { return s.ModType }
func (s *SimpleDiceModifier) ShouldApply(ctx context.Context, e events.Event) bool {
	if s.AppliesTo != nil {
		return s.AppliesTo(ctx, e)
	}
	return true
}

type NoStacking struct{}

func (n *NoStacking) GetStackingRule() StackingRule       { return StackingNone }
func (n *NoStacking) CanStackWith(other core.Entity) bool { return false }
func (n *NoStacking) Stack(other core.Entity) error       { return nil }

type SimpleSaveEffect struct {
	Details   SaveDetails
	OnSuccess func(context.Context, events.EventBus) error
	OnFailure func(context.Context, events.EventBus) error
}

func (s *SimpleSaveEffect) GetSaveDetails() SaveDetails { return s.Details }
func (s *SimpleSaveEffect) OnSaveSuccess(ctx context.Context, bus events.EventBus) error {
	if s.OnSuccess != nil {
		return s.OnSuccess(ctx, bus)
	}
	return nil
}
func (s *SimpleSaveEffect) OnSaveFailure(ctx context.Context, bus events.EventBus) error {
	if s.OnFailure != nil {
		return s.OnFailure(ctx, bus)
	}
	return nil
}
