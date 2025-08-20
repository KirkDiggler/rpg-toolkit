// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"errors"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
)

// RageConfig contains configuration for creating a Rage feature
type RageConfig struct {
	ID    string          // Unique identifier for this feature instance
	Level int             // Barbarian level (affects damage bonus)
	Uses  int             // Number of uses per long rest
	Bus   events.EventBus // Event bus for publishing events
}

// Rage implements a barbarian's rage feature
type Rage struct {
	id    string
	uses  int
	level int
	bus   events.EventBus

	// Protect concurrent access to state
	mu sync.RWMutex

	// Track current state (protected by mu)
	currentUses int
}

// NewRage creates a new rage feature with the given configuration
func NewRage(config RageConfig) *Rage {
	return &Rage{
		id:          config.ID,
		uses:        config.Uses,
		level:       config.Level,
		bus:         config.Bus,
		currentUses: config.Uses,
	}
}

// GetID returns the entity's unique identifier
func (r *Rage) GetID() string { return r.id }

// GetType returns the entity type (feature)
func (r *Rage) GetType() core.EntityType { return "feature" }

// GetResourceType returns what resource this feature consumes
func (r *Rage) GetResourceType() ResourceType { return ResourceTypeRageUses }

// ResetsOn returns when this feature's uses reset
func (r *Rage) ResetsOn() ResetType { return ResetTypeLongRest }

// CanActivate checks if rage can be activated
func (r *Rage) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.currentUses <= 0 {
		return errors.New("no rage uses remaining")
	}

	// Check if already raging by querying the character's conditions
	if char, ok := owner.(interface{ HasCondition(string) bool }); ok {
		if char.HasCondition("dnd5e:conditions:raging") {
			return errors.New("already raging")
		}
	}

	return nil
}

// Activate enters rage mode by publishing a condition event
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	r.mu.Lock()
	r.currentUses--
	r.mu.Unlock()

	// Create the actual raging condition
	ragingCondition := conditions.NewRagingCondition(conditions.RagingConditionInput{
		CharacterID: owner.GetID(),
		DamageBonus: r.getDamageBonus(),
		Level:       r.level,
		Source:      r.id,
	})

	// Publish the condition applied event
	topic := character.ConditionAppliedTopic.On(r.bus)
	err := topic.Publish(ctx, character.ConditionAppliedEvent{
		CharacterID: owner.GetID(),
		Condition:   ragingCondition,
		Source:      r.id,
	})
	if err != nil {
		// Rollback state on error
		r.mu.Lock()
		r.currentUses++
		r.mu.Unlock()
		return err
	}

	return nil
}

// getDamageBonus returns the rage damage bonus based on barbarian level
func (r *Rage) getDamageBonus() int {
	if r.level >= 16 {
		return 4
	} else if r.level >= 9 {
		return 3
	}
	return 2
}
