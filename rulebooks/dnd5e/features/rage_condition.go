// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// RageEndedEvent is published when rage ends
type RageEndedEvent struct {
	events.BaseEvent
	OwnerID string
	Reason  string
}

// RagingCondition represents the rage state for a barbarian
type RagingCondition struct {
	owner string
	level int
	bus   *events.Bus

	mu                sync.RWMutex
	ticksRemaining    int
	attackedThisRound bool
	wasHitThisRound   bool
	subscriptions     []string
	firstRound        bool
}

// NewRagingCondition creates a new rage condition
func NewRagingCondition(owner string, level int, bus *events.Bus) *RagingCondition {
	return &RagingCondition{
		owner:          owner,
		level:          level,
		bus:            bus,
		ticksRemaining: 10, // Rage lasts 10 rounds (1 minute)
		firstRound:     true,
	}
}

// GetID returns the condition ID
func (r *RagingCondition) GetID() string {
	return fmt.Sprintf("rage-%s", r.owner)
}

// GetType returns the condition type
func (r *RagingCondition) GetType() core.EntityType {
	return core.EntityType("condition")
}

// GetName returns the condition name
func (r *RagingCondition) GetName() string {
	return "Rage"
}

// GetDescription returns the condition description
func (r *RagingCondition) GetDescription() string {
	return "You have advantage on Strength checks and Strength saving throws. You gain damage resistance and a damage bonus."
}

// GetSourceCategory returns the source category
func (r *RagingCondition) GetSourceCategory() string {
	return "feature"
}

// GetSource returns the source ref
func (r *RagingCondition) GetSource() *core.Ref {
	ref, _ := core.ParseString("feature:barbarian:rage")
	return ref
}

// GetOwner returns the owner of the condition
func (r *RagingCondition) GetOwner() string {
	return r.owner
}

// GetStacks returns the number of stacks (rage doesn't stack)
func (r *RagingCondition) GetStacks() int {
	return 1
}

// SetStacks sets the number of stacks (no-op for rage)
func (r *RagingCondition) SetStacks(stacks int) {
	// Rage doesn't stack
}

// IsStackable returns whether the condition stacks (rage doesn't)
func (r *RagingCondition) IsStackable() bool {
	return false
}

// GetModifiers returns the modifiers from rage
func (r *RagingCondition) GetModifiers() []events.Modifier {
	// TODO: Implement modifiers once the modifier system is updated
	// For now, rage effects are handled via event handlers
	return nil
}

// calculateDamageBonus returns the rage damage bonus based on barbarian level
func (r *RagingCondition) calculateDamageBonus() int {
	switch {
	case r.level >= 16:
		return 4
	case r.level >= 9:
		return 3
	case r.level >= 1:
		return 2
	default:
		return 2
	}
}

// OnApply is called when the condition is applied
func (r *RagingCondition) OnApply() error {
	// Subscribe to relevant events
	r.mu.Lock()
	defer r.mu.Unlock()

	// Subscribe to turn end to check if rage should end
	if sub, err := r.bus.Subscribe(combat.TurnEndEventRef, r.handleTurnEnd); err == nil {
		r.subscriptions = append(r.subscriptions, sub)
	}

	// Subscribe to attack events to track if we attacked
	if sub, err := r.bus.Subscribe(combat.AttackEventRef, r.handleAttack); err == nil {
		r.subscriptions = append(r.subscriptions, sub)
	}

	// Subscribe to damage taken events to track if we were hit
	if sub, err := r.bus.Subscribe(combat.DamageTakenEventRef, r.handleDamageTaken); err == nil {
		r.subscriptions = append(r.subscriptions, sub)
	}

	return nil
}

// OnRemove is called when the condition is removed
func (r *RagingCondition) OnRemove() error {
	// Clean up subscriptions
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, sub := range r.subscriptions {
		_ = r.bus.Unsubscribe(sub)
	}
	r.subscriptions = nil

	return nil
}

// OnTick is called at the start of each round
func (r *RagingCondition) OnTick() (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we neither attacked nor were hit last round (before resetting flags)
	// But only after the first round
	if !r.firstRound && !r.attackedThisRound && !r.wasHitThisRound {
		// Rage ends early - no hostile action
		return true, nil // Remove condition
	}

	// No longer the first round
	r.firstRound = false

	// Reset flags for the new round
	r.attackedThisRound = false
	r.wasHitThisRound = false

	// Decrement duration
	r.ticksRemaining--

	// Check if rage duration expired
	if r.ticksRemaining <= 0 {
		return true, nil // Remove condition
	}

	return false, nil // Keep condition
}

// handleTurnEnd handles turn end events
func (r *RagingCondition) handleTurnEnd(e any) *events.DeferredAction {
	event := e.(*combat.TurnEndEvent)

	// Only care about our own turn ending
	if event.EntityID != r.owner {
		return nil
	}

	r.mu.Lock()
	shouldRemove := false

	// Check if rage should end (neither attacked nor was hit)
	// But only after the first round
	if !r.firstRound && !r.attackedThisRound && !r.wasHitThisRound {
		shouldRemove = true
	}

	// Prepare unsubscribe list if removing
	var unsubs []string
	if shouldRemove {
		unsubs = append([]string{}, r.subscriptions...)
	}
	r.mu.Unlock()

	if shouldRemove {
		// Use deferred action to safely remove condition
		action := events.NewDeferredAction()

		// Unsubscribe all handlers
		action.Unsubscribe(unsubs...)

		// Publish rage ended event
		ref, _ := core.ParseString("dnd5e:rage:ended")
		removedEvent := &RageEndedEvent{
			BaseEvent: *events.NewBaseEvent(ref),
			OwnerID:   r.owner,
			Reason:    "No hostile action",
		}
		action.Publish(removedEvent)

		return action
	}

	return nil
}

// handleAttack handles attack events
func (r *RagingCondition) handleAttack(e any) *events.DeferredAction {
	event := e.(*combat.AttackEvent)

	// Check if we made the attack
	if event.AttackerID == r.owner {
		r.mu.Lock()
		r.attackedThisRound = true
		r.mu.Unlock()
	}

	return nil
}

// handleDamageTaken handles damage taken events
func (r *RagingCondition) handleDamageTaken(e any) *events.DeferredAction {
	event := e.(*combat.DamageTakenEvent)

	// Check if we took damage
	if event.TargetID == r.owner && event.Amount > 0 {
		r.mu.Lock()
		r.wasHitThisRound = true
		r.mu.Unlock()
	}

	return nil
}

// Ensure RagingCondition implements the necessary interfaces
var _ core.Entity = (*RagingCondition)(nil)

