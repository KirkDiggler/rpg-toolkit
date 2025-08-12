// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

//go:generate mockgen -destination=mock/mock_pool.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/resources Pool

// Pool manages a collection of resources for an entity.
type Pool interface {
	// Owner returns the entity that owns this resource pool.
	Owner() core.Entity

	// Add adds a resource to the pool.
	Add(resource Resource) error

	// Remove removes a resource from the pool by key.
	Remove(key *core.Ref) error

	// Get retrieves a resource by key.
	Get(key *core.Ref) (Resource, bool)

	// GetByType retrieves all resources of a specific type.
	GetByType(resourceType ResourceType) []Resource

	// Consume attempts to consume from a specific resource.
	Consume(key *core.Ref, amount int, bus events.EventBus) error

	// Restore restores a specific resource.
	Restore(key *core.Ref, amount int, reason string, bus events.EventBus) error

	// ProcessShortRest processes a short rest for all resources.
	// Deprecated: Use ProcessRestoration("short_rest", bus) instead
	ProcessShortRest(bus events.EventBus)

	// ProcessLongRest processes a long rest for all resources.
	// Deprecated: Use ProcessRestoration("long_rest", bus) instead
	ProcessLongRest(bus events.EventBus)

	// ProcessRestoration processes a restoration trigger for all resources.
	// The trigger is a game-specific string that resources may respond to.
	// Examples: "short_rest", "long_rest", "dawn", "milestone", "prayer_cast"
	ProcessRestoration(trigger string, bus events.EventBus)

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

	p.resources[resource.Key().String()] = resource
	return nil
}

// Remove removes a resource from the pool
func (p *SimplePool) Remove(key *core.Ref) error {
	keyStr := key.String()
	if _, exists := p.resources[keyStr]; !exists {
		return fmt.Errorf("resource not found: %s", keyStr)
	}

	delete(p.resources, keyStr)
	return nil
}

// Get retrieves a resource by key
func (p *SimplePool) Get(key *core.Ref) (Resource, bool) {
	resource, exists := p.resources[key.String()]
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
func (p *SimplePool) Consume(key *core.Ref, amount int, bus events.EventBus) error {
	keyStr := key.String()
	resource, exists := p.resources[keyStr]
	if !exists {
		return fmt.Errorf("resource not found: %s", keyStr)
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
func (p *SimplePool) Restore(key *core.Ref, amount int, reason string, bus events.EventBus) error {
	keyStr := key.String()
	resource, exists := p.resources[keyStr]
	if !exists {
		return fmt.Errorf("resource not found: %s", keyStr)
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
// Deprecated: Use ProcessRestoration("short_rest", bus) instead
func (p *SimplePool) ProcessShortRest(bus events.EventBus) {
	p.ProcessRestoration("short_rest", bus)
}

// ProcessLongRest processes a long rest for all resources
// Deprecated: Use ProcessRestoration("long_rest", bus) instead
func (p *SimplePool) ProcessLongRest(bus events.EventBus) {
	p.ProcessRestoration("long_rest", bus)
}

// ProcessRestoration processes a restoration trigger for all resources
func (p *SimplePool) ProcessRestoration(trigger string, bus events.EventBus) {
	for _, resource := range p.resources {
		restoreAmount := resource.RestoreOnTrigger(trigger)
		if restoreAmount > 0 {
			_ = p.Restore(resource.Key(), restoreAmount, trigger, bus)
		}
	}
}

// GetSpellSlots returns spell slots organized by level
func (p *SimplePool) GetSpellSlots() map[int]Resource {
	slots := make(map[int]Resource)

	// Look for spell slots by standard keys
	for level := 1; level <= 9; level++ {
		key := core.MustNewRef(core.RefInput{
			Module: "core",
			Type:   "spell_slot",
			Value:  fmt.Sprintf("level_%d", level),
		})
		if resource, exists := p.resources[key.String()]; exists {
			slots[level] = resource
		}
	}

	return slots
}

// ConsumeSpellSlot attempts to consume a spell slot of the specified level or higher
func (p *SimplePool) ConsumeSpellSlot(level int, bus events.EventBus) error {
	// Try to consume at the exact level first
	key := core.MustNewRef(core.RefInput{
		Module: "core",
		Type:   "spell_slot",
		Value:  fmt.Sprintf("level_%d", level),
	})
	if err := p.Consume(key, 1, bus); err == nil {
		return nil
	}

	// Try higher level slots
	for higherLevel := level + 1; higherLevel <= 9; higherLevel++ {
		higherKey := core.MustNewRef(core.RefInput{
			Module: "core",
			Type:   "spell_slot",
			Value:  fmt.Sprintf("level_%d", higherLevel),
		})
		keyStr := higherKey.String()
		if resource, exists := p.resources[keyStr]; exists && resource.IsAvailable() {
			return p.Consume(higherKey, 1, bus)
		}
	}

	return fmt.Errorf("no spell slots available at level %d or higher", level)
}
