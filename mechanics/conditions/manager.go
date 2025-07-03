// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Manager tracks conditions for entities.
type Manager interface {
	// Add applies a condition to an entity.
	Add(entityID string, condition Condition) error

	// Remove removes a specific condition from an entity.
	Remove(entityID string, conditionID string) error

	// RemoveType removes all conditions of a specific type from an entity.
	RemoveType(entityID string, conditionType string) error

	// Get returns a specific condition on an entity.
	Get(entityID string, conditionID string) (Condition, bool)

	// GetByType returns all conditions of a specific type on an entity.
	GetByType(entityID string, conditionType string) []Condition

	// GetAll returns all conditions on an entity.
	GetAll(entityID string) []Condition

	// HasCondition checks if an entity has a specific condition type.
	HasCondition(entityID string, conditionType string) bool

	// Clear removes all conditions from an entity.
	Clear(entityID string)

	// ClearAll removes all conditions from all entities.
	ClearAll()
}

// EventManager is a Manager that integrates with the event system.
type EventManager struct {
	mu         sync.RWMutex
	conditions map[string]map[string]Condition // entityID -> conditionID -> Condition
	eventBus   events.EventBus
	entities   map[string]core.Entity // entityID -> Entity (for event publishing)
}

// NewEventManager creates a new condition manager with event integration.
func NewEventManager(eventBus events.EventBus) *EventManager {
	m := &EventManager{
		conditions: make(map[string]map[string]Condition),
		eventBus:   eventBus,
		entities:   make(map[string]core.Entity),
	}

	// Subscribe to events for duration tracking
	eventBus.SubscribeFunc(events.EventTurnEnd, 0, m.handleTurnEnd)
	eventBus.SubscribeFunc(events.EventRoundEnd, 0, m.handleRoundEnd)
	eventBus.SubscribeFunc(events.EventAfterDamage, 0, m.handleDamage)

	return m
}

// RegisterEntity associates an entity with its ID for event publishing.
func (m *EventManager) RegisterEntity(entity core.Entity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entities[entity.GetID()] = entity
}

// Add applies a condition to an entity.
func (m *EventManager) Add(entityID string, condition Condition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize map for entity if needed
	if _, exists := m.conditions[entityID]; !exists {
		m.conditions[entityID] = make(map[string]Condition)
	}

	// Add the condition
	m.conditions[entityID][condition.ID()] = condition

	// Get entity for events
	entity, hasEntity := m.entities[entityID]

	// Call OnApply
	if err := condition.OnApply(m.eventBus, entity); err != nil {
		delete(m.conditions[entityID], condition.ID())
		return fmt.Errorf("failed to apply condition: %w", err)
	}

	// Publish condition applied event
	if hasEntity {
		event := events.NewGameEvent(EventConditionApplied, entity, nil)
		event.Context().Set("condition_id", condition.ID())
		event.Context().Set("condition_type", condition.Type())
		if err := m.eventBus.Publish(context.Background(), event); err != nil {
			// Log error but don't fail the operation since condition is already applied
			// In a production system, this would use a proper logger
			_ = err
		}
	}

	return nil
}

// Remove removes a specific condition from an entity.
func (m *EventManager) Remove(entityID string, conditionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entityConditions, exists := m.conditions[entityID]
	if !exists {
		return nil
	}

	condition, exists := entityConditions[conditionID]
	if !exists {
		return nil
	}

	// Remove the condition
	delete(entityConditions, conditionID)

	// Clean up empty map
	if len(entityConditions) == 0 {
		delete(m.conditions, entityID)
	}

	// Get entity for events
	entity, hasEntity := m.entities[entityID]

	// Call OnRemove
	if err := condition.OnRemove(m.eventBus, entity); err != nil {
		// Log error but continue with removal since we're already committed
		// In a production system, this would use a proper logger
		_ = err
	}

	// Publish condition removed event
	if hasEntity {
		event := events.NewGameEvent(EventConditionRemoved, entity, nil)
		event.Context().Set("condition_id", conditionID)
		event.Context().Set("condition_type", condition.Type())
		if err := m.eventBus.Publish(context.Background(), event); err != nil {
			// Log error but don't fail the operation since condition is already removed
			// In a production system, this would use a proper logger
			_ = err
		}
	}

	return nil
}

// RemoveType removes all conditions of a specific type from an entity.
func (m *EventManager) RemoveType(entityID string, conditionType string) error {
	m.mu.RLock()
	toRemove := []string{}
	if entityConditions, exists := m.conditions[entityID]; exists {
		for id, cond := range entityConditions {
			if cond.Type() == conditionType {
				toRemove = append(toRemove, id)
			}
		}
	}
	m.mu.RUnlock()

	// Remove outside of read lock
	for _, id := range toRemove {
		if err := m.Remove(entityID, id); err != nil {
			// Continue removing other conditions even if one fails
			// In a production system, this would use a proper logger
			_ = err
		}
	}

	return nil
}

// Get returns a specific condition on an entity.
func (m *EventManager) Get(entityID string, conditionID string) (Condition, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entityConditions, exists := m.conditions[entityID]; exists {
		condition, exists := entityConditions[conditionID]
		return condition, exists
	}

	return nil, false
}

// GetByType returns all conditions of a specific type on an entity.
func (m *EventManager) GetByType(entityID string, conditionType string) []Condition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Condition
	if entityConditions, exists := m.conditions[entityID]; exists {
		for _, cond := range entityConditions {
			if cond.Type() == conditionType {
				result = append(result, cond)
			}
		}
	}

	return result
}

// GetAll returns all conditions on an entity.
func (m *EventManager) GetAll(entityID string) []Condition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Condition
	if entityConditions, exists := m.conditions[entityID]; exists {
		for _, cond := range entityConditions {
			result = append(result, cond)
		}
	}

	return result
}

// HasCondition checks if an entity has a specific condition type.
func (m *EventManager) HasCondition(entityID string, conditionType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entityConditions, exists := m.conditions[entityID]; exists {
		for _, cond := range entityConditions {
			if cond.Type() == conditionType {
				return true
			}
		}
	}

	return false
}

// Clear removes all conditions from an entity.
func (m *EventManager) Clear(entityID string) {
	m.mu.RLock()
	toRemove := []string{}
	if entityConditions, exists := m.conditions[entityID]; exists {
		for id := range entityConditions {
			toRemove = append(toRemove, id)
		}
	}
	m.mu.RUnlock()

	// Remove outside of read lock
	for _, id := range toRemove {
		if err := m.Remove(entityID, id); err != nil {
			// Continue removing other conditions even if one fails
			// In a production system, this would use a proper logger
			_ = err
		}
	}
}

// ClearAll removes all conditions from all entities.
func (m *EventManager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Call OnRemove for all conditions
	for entityID, entityConditions := range m.conditions {
		entity := m.entities[entityID]
		for _, condition := range entityConditions {
			if err := condition.OnRemove(m.eventBus, entity); err != nil {
				// Log error but continue clearing since we're already committed
				// In a production system, this would use a proper logger
				_ = err
			}
		}
	}

	// Clear the maps
	m.conditions = make(map[string]map[string]Condition)
}

// handleTurnEnd checks for expired conditions at turn end.
func (m *EventManager) handleTurnEnd(ctx context.Context, event events.Event) error {
	m.checkExpiredConditions(event)
	return nil
}

// handleRoundEnd checks for expired conditions at round end.
func (m *EventManager) handleRoundEnd(ctx context.Context, event events.Event) error {
	m.checkExpiredConditions(event)
	return nil
}

// handleDamage checks for conditions that expire on damage.
func (m *EventManager) handleDamage(ctx context.Context, event events.Event) error {
	m.checkExpiredConditions(event)
	return nil
}

// checkExpiredConditions removes any conditions that have expired.
func (m *EventManager) checkExpiredConditions(event events.Event) {
	m.mu.RLock()
	toRemove := make(map[string][]string) // entityID -> []conditionID

	for entityID, entityConditions := range m.conditions {
		for condID, condition := range entityConditions {
			if condition.IsExpired(event) {
				if toRemove[entityID] == nil {
					toRemove[entityID] = []string{}
				}
				toRemove[entityID] = append(toRemove[entityID], condID)
			}
		}
	}
	m.mu.RUnlock()

	// Remove expired conditions
	for entityID, conditionIDs := range toRemove {
		for _, condID := range conditionIDs {
			if err := m.Remove(entityID, condID); err != nil {
				// Continue removing other expired conditions even if one fails
				// In a production system, this would use a proper logger
				_ = err
			}
		}
	}
}

// Event types for conditions
const (
	EventConditionApplied = "condition_applied"
	EventConditionRemoved = "condition_removed"
	EventConditionExpired = "condition_expired"
)
