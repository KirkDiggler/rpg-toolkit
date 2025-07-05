// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ConditionManager manages conditions on entities, handling immunity, suppression, and interactions.
type ConditionManager struct {
	mu         sync.RWMutex
	conditions map[string][]Condition     // entity ID -> active conditions
	immunities map[string][]ConditionType // entity ID -> condition immunities
	bus        events.EventBus

	// Optional relationship manager for concentration, auras, etc.
	relationships *RelationshipManager
}

// NewConditionManager creates a new condition manager.
func NewConditionManager(bus events.EventBus) *ConditionManager {
	cm := &ConditionManager{
		conditions: make(map[string][]Condition),
		immunities: make(map[string][]ConditionType),
		bus:        bus,
	}

	// Optionally create a relationship manager
	// cm.relationships = NewRelationshipManager(bus)

	return cm
}

// SetRelationshipManager sets a custom relationship manager.
func (cm *ConditionManager) SetRelationshipManager(rm *RelationshipManager) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.relationships = rm
}

// AddImmunity adds a condition immunity to an entity.
func (cm *ConditionManager) AddImmunity(entity core.Entity, condType ConditionType) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entityID := entity.GetID()
	cm.immunities[entityID] = append(cm.immunities[entityID], condType)

	// Remove any existing conditions of this type
	cm.removeConditionTypeUnsafe(entity, condType)
}

// RemoveImmunity removes a condition immunity from an entity.
func (cm *ConditionManager) RemoveImmunity(entity core.Entity, condType ConditionType) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entityID := entity.GetID()
	immunities := cm.immunities[entityID]
	for i, immunity := range immunities {
		if immunity == condType {
			cm.immunities[entityID] = append(immunities[:i], immunities[i+1:]...)
			break
		}
	}
}

// IsImmune checks if an entity is immune to a condition type.
func (cm *ConditionManager) IsImmune(entity core.Entity, condType ConditionType) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	immunities := cm.immunities[entity.GetID()]
	for _, immunity := range immunities {
		if immunity == condType {
			return true
		}
	}

	// Check if any active conditions grant immunity
	conditions := cm.conditions[entity.GetID()]
	for _, cond := range conditions {
		if enhanced, ok := cond.(*EnhancedCondition); ok {
			for _, immunity := range enhanced.definition.Immunities {
				if immunity == condType {
					return true
				}
			}
		}
	}

	return false
}

// CanApplyCondition checks if a condition can be applied to an entity.
func (cm *ConditionManager) CanApplyCondition(entity core.Entity, condType ConditionType) (bool, string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Check immunity
	if cm.IsImmune(entity, condType) {
		return false, fmt.Sprintf("immune to %s", condType)
	}

	// Check if a stronger condition exists
	conditions := cm.conditions[entity.GetID()]
	for _, cond := range conditions {
		if enhanced, ok := cond.(*EnhancedCondition); ok {
			// Check if this condition is suppressed by an existing one
			if cm.isSuppressedBy(condType, enhanced.conditionType) {
				return false, fmt.Sprintf("suppressed by stronger condition %s", enhanced.conditionType)
			}
		}
	}

	return true, ""
}

// ApplyCondition applies a condition to an entity.
func (cm *ConditionManager) ApplyCondition(condition Condition) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	target := condition.Target()

	// Get condition type if it's an enhanced condition
	var condType ConditionType
	var includes []ConditionType
	if enhanced, ok := condition.(*EnhancedCondition); ok {
		condType = enhanced.conditionType
		includes = enhanced.definition.Includes

		// Check if we can apply this condition
		if canApply, reason := cm.CanApplyCondition(target, condType); !canApply {
			return fmt.Errorf("cannot apply %s: %s", condType, reason)
		}

		// Games can implement special stacking logic for specific condition types

		// Remove weaker conditions that this suppresses
		cm.removeSuppressedConditionsUnsafe(target, condType)

		// Remove existing conditions of the same type (unless they stack)
		cm.removeConditionTypeUnsafe(target, condType)
	}

	// Apply the condition
	if err := condition.Apply(cm.bus); err != nil {
		return fmt.Errorf("failed to apply condition: %w", err)
	}

	// Track the condition
	entityID := target.GetID()
	cm.conditions[entityID] = append(cm.conditions[entityID], condition)

	// Apply included conditions (e.g., Paralyzed includes Incapacitated)
	for _, includedType := range includes {
		// Create a simple included condition
		included := NewSimpleCondition(SimpleConditionConfig{
			ID:     fmt.Sprintf("%s_included_%s", condition.GetID(), includedType),
			Type:   string(includedType),
			Target: target,
			Source: fmt.Sprintf("included by %s", condType),
		})

		// Apply it (this will handle immunity/suppression checks)
		if err := cm.ApplyCondition(included); err != nil {
			// Log but don't fail - included conditions are best-effort
			continue
		}
	}

	// Emit condition applied event
	event := events.NewGameEvent(
		events.EventOnConditionApplied,
		nil,
		target,
	)
	event.Context().Set("condition_type", string(condType))
	event.Context().Set("condition_id", condition.GetID())
	cm.bus.Publish(nil, event)

	return nil
}

// RemoveCondition removes a specific condition.
func (cm *ConditionManager) RemoveCondition(condition Condition) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.removeConditionUnsafe(condition)
}

// RemoveConditionByID removes a condition by ID.
func (cm *ConditionManager) RemoveConditionByID(entity core.Entity, conditionID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conditions := cm.conditions[entity.GetID()]
	for _, cond := range conditions {
		if cond.GetID() == conditionID {
			return cm.removeConditionUnsafe(cond)
		}
	}

	return fmt.Errorf("condition %s not found on entity %s", conditionID, entity.GetID())
}

// RemoveConditionType removes all conditions of a specific type from an entity.
func (cm *ConditionManager) RemoveConditionType(entity core.Entity, condType ConditionType) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.removeConditionTypeUnsafe(entity, condType)
}

// GetConditions returns all active conditions on an entity.
func (cm *ConditionManager) GetConditions(entity core.Entity) []Condition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conditions := cm.conditions[entity.GetID()]
	result := make([]Condition, len(conditions))
	copy(result, conditions)
	return result
}

// GetConditionsByType returns all conditions of a specific type on an entity.
func (cm *ConditionManager) GetConditionsByType(entity core.Entity, condType ConditionType) []Condition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []Condition
	conditions := cm.conditions[entity.GetID()]
	for _, cond := range conditions {
		if enhanced, ok := cond.(*EnhancedCondition); ok && enhanced.conditionType == condType {
			result = append(result, cond)
		} else if cond.GetType() == string(condType) {
			result = append(result, cond)
		}
	}

	return result
}

// HasCondition checks if an entity has a specific condition type.
func (cm *ConditionManager) HasCondition(entity core.Entity, condType ConditionType) bool {
	return len(cm.GetConditionsByType(entity, condType)) > 0
}

// Games can add their own helper methods for specific condition types

// Internal helper methods

func (cm *ConditionManager) removeConditionUnsafe(condition Condition) error {
	target := condition.Target()
	entityID := target.GetID()

	// Find and remove the condition
	conditions := cm.conditions[entityID]
	for i, cond := range conditions {
		if cond.GetID() == condition.GetID() {
			// Remove from event bus
			if err := cond.Remove(cm.bus); err != nil {
				return fmt.Errorf("failed to remove condition: %w", err)
			}

			// Remove from tracking
			cm.conditions[entityID] = append(conditions[:i], conditions[i+1:]...)

			// Emit condition removed event
			var condType string
			if enhanced, ok := cond.(*EnhancedCondition); ok {
				condType = string(enhanced.conditionType)
			} else {
				condType = cond.GetType()
			}

			event := events.NewGameEvent(
				events.EventOnConditionRemoved,
				nil,
				target,
			)
			event.Context().Set("condition_type", condType)
			event.Context().Set("condition_id", condition.GetID())
			cm.bus.Publish(nil, event)

			return nil
		}
	}

	return fmt.Errorf("condition not found")
}

func (cm *ConditionManager) removeConditionTypeUnsafe(entity core.Entity, condType ConditionType) error {
	conditions := cm.GetConditionsByType(entity, condType)
	var errs []error

	for _, cond := range conditions {
		if err := cm.removeConditionUnsafe(cond); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors removing conditions: %v", errs)
	}

	return nil
}

func (cm *ConditionManager) removeSuppressedConditionsUnsafe(entity core.Entity, suppressorType ConditionType) {
	conditions := cm.conditions[entity.GetID()]
	for _, cond := range conditions {
		if enhanced, ok := cond.(*EnhancedCondition); ok {
			if cm.isSuppressedBy(enhanced.conditionType, suppressorType) {
				_ = cm.removeConditionUnsafe(cond)
			}
		}
	}
}

func (cm *ConditionManager) isSuppressedBy(condType, suppressorType ConditionType) bool {
	// Get the suppressor definition
	suppressor, exists := GetConditionDefinition(suppressorType)
	if !exists {
		return false
	}

	// Check if the suppressor suppresses this condition
	for _, suppressed := range suppressor.Suppresses {
		if suppressed == condType {
			return true
		}
	}

	// Check if the suppressor includes this condition (which also suppresses it)
	for _, included := range suppressor.Includes {
		if included == condType {
			return true
		}
	}

	return false
}

// Games can implement their own special condition application logic
