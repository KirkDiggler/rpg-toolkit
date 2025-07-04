// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Pool manages a collection of resources for an entity.
type Pool interface {
	// Owner returns the entity that owns this resource pool.
	Owner() core.Entity

	// Add adds a resource to the pool.
	Add(resource Resource) error

	// Remove removes a resource from the pool by key.
	Remove(key string) error

	// Get retrieves a resource by key.
	Get(key string) (Resource, bool)

	// GetByType retrieves all resources of a specific type.
	GetByType(resourceType ResourceType) []Resource

	// Consume attempts to consume from a specific resource.
	Consume(key string, amount int, bus events.EventBus) error

	// Restore restores a specific resource.
	Restore(key string, amount int, reason string, bus events.EventBus) error

	// ProcessShortRest processes a short rest for all resources.
	ProcessShortRest(bus events.EventBus)

	// ProcessLongRest processes a long rest for all resources.
	ProcessLongRest(bus events.EventBus)

	// GetSpellSlots returns spell slots organized by level.
	GetSpellSlots() map[int]Resource

	// ConsumeSpellSlot attempts to consume a spell slot of the specified level or higher.
	ConsumeSpellSlot(level int, bus events.EventBus) error
}

// SimplePool provides a basic implementation of the Pool interface.
type SimplePool struct {
	owner     core.Entity
	resources map[string]Resource
}

// NewSimplePool creates a new resource pool for an entity.
func NewSimplePool(owner core.Entity) *SimplePool {
	return &SimplePool{
		owner:     owner,
		resources: make(map[string]Resource),
	}
}

// Owner returns the entity that owns this pool
func (p *SimplePool) Owner() core.Entity {
	return p.owner
}

// Add adds a resource to the pool
func (p *SimplePool) Add(resource Resource) error {
	if resource == nil {
		return fmt.Errorf("cannot add nil resource")
	}

	if resource.Owner().GetID() != p.owner.GetID() {
		return fmt.Errorf("resource owner mismatch")
	}

	p.resources[resource.Key()] = resource
	return nil
}

// Remove removes a resource from the pool
func (p *SimplePool) Remove(key string) error {
	if _, exists := p.resources[key]; !exists {
		return fmt.Errorf("resource not found: %s", key)
	}

	delete(p.resources, key)
	return nil
}

// Get retrieves a resource by key
func (p *SimplePool) Get(key string) (Resource, bool) {
	resource, exists := p.resources[key]
	return resource, exists
}

// GetByType retrieves all resources of a specific type
func (p *SimplePool) GetByType(resourceType ResourceType) []Resource {
	var result []Resource
	for _, resource := range p.resources {
		if resource.GetType() == string(resourceType) {
			result = append(result, resource)
		}
	}
	return result
}

// Consume attempts to consume from a specific resource
func (p *SimplePool) Consume(key string, amount int, bus events.EventBus) error {
	resource, exists := p.resources[key]
	if !exists {
		return fmt.Errorf("resource not found: %s", key)
	}

	if err := resource.Consume(amount); err != nil {
		return err
	}

	// Publish consumption event
	if bus != nil {
		event := &ResourceConsumedEvent{
			GameEvent: events.NewGameEvent(EventResourceConsumed, p.owner, nil),
			Resource:  resource,
			Amount:    amount,
		}
		_ = bus.Publish(context.Background(), event)
	}

	return nil
}

// Restore restores a specific resource
func (p *SimplePool) Restore(key string, amount int, reason string, bus events.EventBus) error {
	resource, exists := p.resources[key]
	if !exists {
		return fmt.Errorf("resource not found: %s", key)
	}

	oldCurrent := resource.Current()
	resource.Restore(amount)
	actualRestored := resource.Current() - oldCurrent

	// Publish restoration event if anything was restored
	if bus != nil && actualRestored > 0 {
		event := &ResourceRestoredEvent{
			GameEvent: events.NewGameEvent(EventResourceRestored, p.owner, nil),
			Resource:  resource,
			Amount:    actualRestored,
			Reason:    reason,
		}
		_ = bus.Publish(context.Background(), event)
	}

	return nil
}

// ProcessShortRest processes a short rest for all resources
func (p *SimplePool) ProcessShortRest(bus events.EventBus) {
	for _, resource := range p.resources {
		restoreAmount := resource.RestoreOnShortRest()
		if restoreAmount > 0 {
			_ = p.Restore(resource.Key(), restoreAmount, "short_rest", bus)
		}
	}
}

// ProcessLongRest processes a long rest for all resources
func (p *SimplePool) ProcessLongRest(bus events.EventBus) {
	for _, resource := range p.resources {
		restoreAmount := resource.RestoreOnLongRest()
		if restoreAmount > 0 {
			_ = p.Restore(resource.Key(), restoreAmount, "long_rest", bus)
		}
	}
}

// GetSpellSlots returns spell slots organized by level
func (p *SimplePool) GetSpellSlots() map[int]Resource {
	slots := make(map[int]Resource)

	// Look for spell slots by standard keys
	for level := 1; level <= 9; level++ {
		key := fmt.Sprintf("spell_slots_%d", level)
		if resource, exists := p.resources[key]; exists {
			slots[level] = resource
		}
	}

	return slots
}

// ConsumeSpellSlot attempts to consume a spell slot of the specified level or higher
func (p *SimplePool) ConsumeSpellSlot(level int, bus events.EventBus) error {
	// Try to consume at the exact level first
	key := fmt.Sprintf("spell_slots_%d", level)
	if err := p.Consume(key, 1, bus); err == nil {
		return nil
	}

	// Try higher level slots
	for higherLevel := level + 1; higherLevel <= 9; higherLevel++ {
		key = fmt.Sprintf("spell_slots_%d", higherLevel)
		if resource, exists := p.resources[key]; exists && resource.IsAvailable() {
			return p.Consume(key, 1, bus)
		}
	}

	return fmt.Errorf("no spell slots available at level %d or higher", level)
}
