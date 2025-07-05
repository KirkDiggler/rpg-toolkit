// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// Example: Bless spell - temporary effect that adds dice to attack rolls
type BlessEffect struct {
	*effects.Core
	duration  effects.Duration
	targets   []core.Entity
	startTime time.Time
}

// Implement TemporaryEffect
func (b *BlessEffect) GetDuration() effects.Duration {
	return b.duration
}

func (b *BlessEffect) CheckExpiration(_ context.Context, currentTime time.Time) bool {
	// For rounds-based duration, this would check round count
	// For time-based, check elapsed time
	if b.duration.Type == effects.DurationMinutes {
		elapsed := currentTime.Sub(b.startTime).Minutes()
		return elapsed >= float64(b.duration.Value)
	}
	return false
}

func (b *BlessEffect) OnExpire(bus events.EventBus) error {
	// Clean up when bless expires
	return b.Remove(bus)
}

// Implement DiceModifier
func (b *BlessEffect) GetDiceExpression(_ context.Context, _ events.Event) string {
	return "1d4" // Fresh roll each time
}

func (b *BlessEffect) GetModifierType() effects.ModifierType {
	return effects.ModifierAttack
}

func (b *BlessEffect) ShouldApply(_ context.Context, event events.Event) bool {
	// Check if the attacker is blessed
	if attackEvent, ok := event.(*AttackEvent); ok {
		for _, target := range b.targets {
			if target.GetID() == attackEvent.Attacker.GetID() {
				return true
			}
		}
	}
	return false
}

// Implement TargetedEffect
func (b *BlessEffect) GetTargets() []core.Entity {
	return b.targets
}

func (b *BlessEffect) AddTarget(target core.Entity) error {
	b.targets = append(b.targets, target)
	return nil
}

func (b *BlessEffect) RemoveTarget(target core.Entity) error {
	for i, t := range b.targets {
		if t.GetID() == target.GetID() {
			b.targets = append(b.targets[:i], b.targets[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("target not found")
}

func (b *BlessEffect) IsValidTarget(target core.Entity) bool {
	// Bless can target any creature
	return target.GetType() == "creature" || target.GetType() == "character"
}

// Example: Rage - conditional, temporary, resource-consuming effect
type RageEffect struct {
	*effects.Core
	barbarian core.Entity
	endTime   time.Time
	damage    int // Track if damage dealt/taken this turn
}

// Implement ConditionalEffect
func (r *RageEffect) CheckCondition(_ context.Context, event events.Event) bool {
	// Rage ends if you haven't attacked or taken damage
	if event.Type() == "turn.end" {
		return r.damage > 0
	}
	// Rage benefits apply to strength-based attacks
	if event.Type() == "attack.before" {
		return true // Would check if STR-based
	}
	return true
}

// Implement ResourceConsumer
func (r *RageEffect) GetResourceRequirements() []effects.ResourceRequirement {
	return []effects.ResourceRequirement{
		{
			Key:      "rage_uses",
			Amount:   1,
			Optional: false,
		},
	}
}

func (r *RageEffect) ConsumeResources(_ context.Context, _ events.EventBus) error {
	// This would actually consume the rage use from the character's pool
	// For now, just return success
	return nil
}

// Implement TemporaryEffect
func (r *RageEffect) GetDuration() effects.Duration {
	return effects.Duration{
		Type:  effects.DurationMinutes,
		Value: 1,
	}
}

func (r *RageEffect) CheckExpiration(_ context.Context, currentTime time.Time) bool {
	return currentTime.After(r.endTime)
}

func (r *RageEffect) OnExpire(bus events.EventBus) error {
	// Publish rage ended event
	endEvent := events.NewGameEvent("rage.ended", r, nil)
	endEvent.Context().Set("barbarian", r.barbarian)
	_ = bus.Publish(context.Background(), endEvent)
	return r.Remove(bus)
}

// Example: Poison - saving throw effect with damage over time
type PoisonEffect struct {
	*effects.Core
	victim   core.Entity
	severity int
}

// Implement SavingThrowEffect
func (p *PoisonEffect) GetSaveDetails() effects.SaveDetails {
	return effects.SaveDetails{
		Ability:     "constitution",
		DC:          12 + p.severity,
		RepeatType:  effects.SaveRepeat,
		RepeatValue: "turn_end",
	}
}

func (p *PoisonEffect) OnSaveSuccess(_ context.Context, bus events.EventBus) error {
	// Remove poison on successful save
	return p.Remove(bus)
}

func (p *PoisonEffect) OnSaveFailure(ctx context.Context, bus events.EventBus) error {
	// Deal poison damage
	damageEvent := events.NewGameEvent("damage.poison", p, p.victim)
	damageEvent.Context().Set("amount", fmt.Sprintf("%dd6", p.severity))
	_ = bus.Publish(ctx, damageEvent)
	return nil
}

// Example: Shield spell - triggered effect
type ShieldSpell struct {
	*effects.Core
	caster  core.Entity
	acBonus int
}

// Implement TriggeredEffect
func (s *ShieldSpell) GetTriggers() []effects.TriggerCondition {
	return []effects.TriggerCondition{
		{
			EventType: "attack.before_hit",
			Condition: func(_ context.Context, event events.Event) bool {
				// Trigger when caster is attacked
				if attackEvent, ok := event.(*AttackEvent); ok {
					return attackEvent.AttackTarget.GetID() == s.caster.GetID()
				}
				return false
			},
			Priority: 100, // High priority to modify AC
		},
	}
}

func (s *ShieldSpell) OnTrigger(_ context.Context, event events.Event, _ events.EventBus) error {
	// Add +5 to AC
	if attackEvent, ok := event.(*AttackEvent); ok {
		attackEvent.TargetAC += s.acBonus
	}
	return nil
}

// Example: Powerful Build - permanent conditional effect
type PowerfulBuildTrait struct {
	*effects.Core
}

// Implement ConditionalEffect
func (p *PowerfulBuildTrait) CheckCondition(_ context.Context, event events.Event) bool {
	// Only applies to carrying capacity and push/drag/lift
	eventType := event.Type()
	return eventType == "capacity.calculate" ||
		eventType == "strength.check.push" ||
		eventType == "strength.check.drag" ||
		eventType == "strength.check.lift"
}

// Apply doubles carrying capacity
func (p *PowerfulBuildTrait) Apply(bus events.EventBus) error {
	// Subscribe to relevant events
	p.Subscribe(bus, "capacity.calculate", 50, events.HandlerFunc(func(ctx context.Context, e events.Event) error {
		if p.CheckCondition(ctx, e) {
			// Double the capacity
			if capEvent, ok := e.(*CapacityEvent); ok {
				capEvent.Multiplier *= 2
			}
		}
		return nil
	}))

	return p.Core.Apply(bus)
}

// Example: ExclusiveEffect - demonstrates stacking rules
type ExclusiveEffect struct {
	*effects.Core
}

// Implement StackableEffect
func (e *ExclusiveEffect) GetStackingRule() effects.StackingRule {
	return effects.StackingNone // Can't maintain multiple exclusive effects
}

func (e *ExclusiveEffect) CanStackWith(other core.Entity) bool {
	// Can never stack exclusive effects of same type
	if _, ok := other.(*ExclusiveEffect); ok {
		return false
	}
	return true
}

func (e *ExclusiveEffect) Stack(_ core.Entity) error {
	// This would never be called due to CanStackWith returning false
	return fmt.Errorf("cannot maintain multiple exclusive effects")
}

// Mock types for examples
type AttackEvent struct {
	*events.GameEvent
	Attacker     core.Entity
	AttackTarget core.Entity
	TargetAC     int
}

func (a *AttackEvent) Source() core.Entity { return a.Attacker }
func (a *AttackEvent) Target() core.Entity { return a.AttackTarget }

type CapacityEvent struct {
	*events.GameEvent
	Character  core.Entity
	Base       int
	Multiplier float64
}

func (c *CapacityEvent) Source() core.Entity { return c.Character }
func (c *CapacityEvent) Target() core.Entity { return nil }

// Example usage showing composition
func Example() {
	bus := events.NewBus()
	wizard := &MockEntity{id: "wizard-1", typ: "character"}
	fighter := &MockEntity{id: "fighter-1", typ: "character"}

	// Bless combines: TemporaryEffect + DiceModifier + TargetedEffect
	bless := &BlessEffect{
		Core: effects.NewCore(effects.CoreConfig{
			ID:   "bless-1",
			Type: "spell_effect",
		}),
		duration: effects.Duration{
			Type:  effects.DurationMinutes,
			Value: 1,
		},
		targets:   []core.Entity{wizard, fighter},
		startTime: time.Now(),
	}

	// Apply the bless effect
	_ = bless.Apply(bus)

	// When an attack happens, bless would add 1d4 through its subscriptions
}
