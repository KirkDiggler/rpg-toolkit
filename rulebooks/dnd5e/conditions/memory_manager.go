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

// MemoryManager is an in-memory implementation of the condition Manager
type MemoryManager struct {
	mu         sync.RWMutex
	conditions map[string][]Condition // key is entity ID
	handlers   map[ConditionType]ConditionHandler
	bus        *events.Bus
}

// ConditionHandler defines the interface for condition-specific behavior
type ConditionHandler interface {
	// OnApply is called when the condition is applied
	OnApply(ctx context.Context, target core.Entity, condition *Condition) error
	// OnRemove is called when the condition is removed
	OnRemove(ctx context.Context, target core.Entity, condition *Condition) error
}

// NewMemoryManager creates a new in-memory condition manager
func NewMemoryManager(bus *events.Bus) *MemoryManager {
	m := &MemoryManager{
		conditions: make(map[string][]Condition),
		handlers:   make(map[ConditionType]ConditionHandler),
		bus:        bus,
	}

	// Register default handlers
	m.RegisterHandler(Raging, NewRagingConditionHandler(bus))

	return m
}

// RegisterHandler registers a handler for a specific condition type
func (m *MemoryManager) RegisterHandler(conditionType ConditionType, handler ConditionHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[conditionType] = handler
}

// ApplyCondition applies a condition to a target entity
func (m *MemoryManager) ApplyCondition(ctx context.Context, input *ApplyConditionInput) (*ApplyConditionOutput, error) {
	if input == nil || input.Target == nil {
		return nil, fmt.Errorf("invalid input: target is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entityID := input.Target.GetID()
	output := &ApplyConditionOutput{Applied: false}

	// Get existing conditions for this entity
	entityConditions := m.conditions[entityID]
	if entityConditions == nil {
		entityConditions = []Condition{}
	}

	// Check if condition already exists from the same source
	for i, existing := range entityConditions {
		if existing.Type == input.Condition.Type && existing.Source == input.Condition.Source {
			// Replace existing condition
			output.PreviousRef = &existing
			entityConditions[i] = input.Condition
			m.conditions[entityID] = entityConditions

			// Call handler for removal of old and application of new
			if handler, ok := m.handlers[existing.Type]; ok {
				_ = handler.OnRemove(ctx, input.Target, &existing)
				if err := handler.OnApply(ctx, input.Target, &entityConditions[i]); err != nil {
					// Rollback on failure
					entityConditions[i] = existing
					m.conditions[entityID] = entityConditions
					_ = handler.OnApply(ctx, input.Target, &existing)
					return nil, fmt.Errorf("failed to apply condition handler: %w", err)
				}
			}

			output.Applied = true
			return output, nil
		}
	}

	// Add new condition
	entityConditions = append(entityConditions, input.Condition)
	m.conditions[entityID] = entityConditions

	// Call handler if registered
	if handler, ok := m.handlers[input.Condition.Type]; ok {
		conditionRef := &entityConditions[len(entityConditions)-1]
		if err := handler.OnApply(ctx, input.Target, conditionRef); err != nil {
			// Rollback on failure
			m.conditions[entityID] = entityConditions[:len(entityConditions)-1]
			return nil, fmt.Errorf("failed to apply condition handler: %w", err)
		}
	}

	output.Applied = true
	return output, nil
}

// RemoveCondition removes a condition from a target
func (m *MemoryManager) RemoveCondition(ctx context.Context, input *RemoveConditionInput) (*RemoveConditionOutput, error) {
	if input == nil || input.Target == nil {
		return nil, fmt.Errorf("invalid input: target is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entityID := input.Target.GetID()
	output := &RemoveConditionOutput{Removed: false}

	entityConditions := m.conditions[entityID]
	if entityConditions == nil {
		return output, nil
	}

	// Find and remove the condition
	filtered := []Condition{}
	for _, condition := range entityConditions {
		shouldRemove := condition.Type == input.Type
		if input.Source != "" {
			shouldRemove = shouldRemove && condition.Source == input.Source
		}

		if shouldRemove {
			output.Removed = true
			output.Condition = &condition

			// Call handler if registered
			if handler, ok := m.handlers[condition.Type]; ok {
				_ = handler.OnRemove(ctx, input.Target, &condition)
			}
		} else {
			filtered = append(filtered, condition)
		}
	}

	m.conditions[entityID] = filtered
	return output, nil
}

// TickDuration decrements duration for time-based conditions
func (m *MemoryManager) TickDuration(ctx context.Context, input *TickDurationInput) (*TickDurationOutput, error) {
	if input == nil || input.Target == nil {
		return nil, fmt.Errorf("invalid input: target is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entityID := input.Target.GetID()
	output := &TickDurationOutput{ExpiredConditions: []Condition{}}

	entityConditions := m.conditions[entityID]
	if entityConditions == nil {
		return output, nil
	}

	// Process conditions and track which ones expire
	filtered := []Condition{}
	for _, condition := range entityConditions {
		// Only tick conditions with matching duration type
		if condition.DurationType == input.DurationType && condition.Remaining > 0 {
			condition.Remaining -= input.Amount
			if condition.Remaining <= 0 {
				// Condition expired
				output.ExpiredConditions = append(output.ExpiredConditions, condition)

				// Call handler if registered
				if handler, ok := m.handlers[condition.Type]; ok {
					_ = handler.OnRemove(ctx, input.Target, &condition)
				}
				continue // Don't add to filtered list
			}
		}
		filtered = append(filtered, condition)
	}

	m.conditions[entityID] = filtered
	return output, nil
}

// GetConditions returns all conditions on a target
func (m *MemoryManager) GetConditions(ctx context.Context, target core.Entity) ([]Condition, error) {
	if target == nil {
		return nil, fmt.Errorf("invalid target")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entityID := target.GetID()
	conditions := m.conditions[entityID]
	if conditions == nil {
		return []Condition{}, nil
	}

	// Return a copy to prevent external modification
	result := make([]Condition, len(conditions))
	copy(result, conditions)
	return result, nil
}

// HasCondition checks if target has a specific condition
func (m *MemoryManager) HasCondition(ctx context.Context, target core.Entity, conditionType ConditionType) (bool, error) {
	if target == nil {
		return false, fmt.Errorf("invalid target")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entityID := target.GetID()
	entityConditions := m.conditions[entityID]
	if entityConditions == nil {
		return false, nil
	}

	for _, condition := range entityConditions {
		if condition.Type == conditionType {
			return true, nil
		}
	}

	return false, nil
}