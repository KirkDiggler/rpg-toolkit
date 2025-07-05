// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ExhaustionCondition is a specialized condition for handling exhaustion levels.
type ExhaustionCondition struct {
	*EnhancedCondition
}

// NewExhaustionCondition creates a new exhaustion condition with the specified level.
func NewExhaustionCondition(target core.Entity, level int, source string) (*ExhaustionCondition, error) {
	if level < 1 || level > 6 {
		return nil, fmt.Errorf("exhaustion level must be between 1 and 6, got %d", level)
	}

	// Create the enhanced condition
	config := EnhancedConditionConfig{
		ID:            fmt.Sprintf("exhaustion_%s_%d", target.GetID(), generateID()),
		ConditionType: ConditionExhaustion,
		Target:        target,
		Source:        source,
		Level:         level,
	}

	enhanced, err := NewEnhancedCondition(config)
	if err != nil {
		return nil, err
	}

	return &ExhaustionCondition{
		EnhancedCondition: enhanced,
	}, nil
}

// IncreaseLevel increases the exhaustion level by the specified amount.
func (ec *ExhaustionCondition) IncreaseLevel(amount int) (int, error) {
	newLevel := ec.level + amount
	if newLevel > 6 {
		newLevel = 6
	}

	if newLevel == ec.level {
		return ec.level, nil // No change
	}

	ec.level = newLevel

	// If the condition is active, we need to reapply effects
	if ec.IsActive() {
		// This would typically be handled by the condition manager
		// to properly remove old effects and apply new ones
		return newLevel, fmt.Errorf("cannot modify active exhaustion - remove and reapply")
	}

	return newLevel, nil
}

// DecreaseLevel decreases the exhaustion level by the specified amount.
func (ec *ExhaustionCondition) DecreaseLevel(amount int) (int, error) {
	newLevel := ec.level - amount
	if newLevel < 0 {
		newLevel = 0
	}

	if newLevel == ec.level {
		return ec.level, nil // No change
	}

	ec.level = newLevel

	// If the condition is active, we need to reapply effects
	if ec.IsActive() {
		return newLevel, fmt.Errorf("cannot modify active exhaustion - remove and reapply")
	}

	return newLevel, nil
}

// GetEffectDescription returns a human-readable description of the current exhaustion effects.
func (ec *ExhaustionCondition) GetEffectDescription() string {
	descriptions := []string{
		"Level 1: Disadvantage on ability checks",
		"Level 2: Speed halved",
		"Level 3: Disadvantage on attack rolls and saving throws",
		"Level 4: Hit point maximum halved",
		"Level 5: Speed reduced to 0",
		"Level 6: Death",
	}

	// Return all effects up to current level (they're cumulative)
	result := fmt.Sprintf("Exhaustion Level %d:\n", ec.level)
	for i := 0; i < ec.level && i < len(descriptions); i++ {
		result += fmt.Sprintf("- %s\n", descriptions[i])
	}

	return result
}

// ExhaustionManager provides convenience methods for managing exhaustion.
type ExhaustionManager struct {
	conditionManager *ConditionManager
}

// NewExhaustionManager creates a new exhaustion manager.
func NewExhaustionManager(cm *ConditionManager) *ExhaustionManager {
	return &ExhaustionManager{
		conditionManager: cm,
	}
}

// GetExhaustionLevel returns the current exhaustion level for an entity.
func (em *ExhaustionManager) GetExhaustionLevel(entity core.Entity) int {
	return em.conditionManager.GetExhaustionLevel(entity)
}

// AddExhaustion adds exhaustion levels to an entity.
func (em *ExhaustionManager) AddExhaustion(entity core.Entity, levels int, source string) error {
	if levels <= 0 {
		return nil
	}

	currentLevel := em.GetExhaustionLevel(entity)
	newLevel := currentLevel + levels
	if newLevel > 6 {
		newLevel = 6
	}

	// If already at max, nothing to do
	if currentLevel >= 6 {
		return nil
	}

	// Create new exhaustion condition
	exhaustion, err := NewExhaustionCondition(entity, newLevel, source)
	if err != nil {
		return err
	}

	// Apply it (this will replace existing exhaustion)
	return em.conditionManager.ApplyCondition(exhaustion)
}

// RemoveExhaustion removes exhaustion levels from an entity.
func (em *ExhaustionManager) RemoveExhaustion(entity core.Entity, levels int) error {
	if levels <= 0 {
		return nil
	}

	currentLevel := em.GetExhaustionLevel(entity)
	if currentLevel == 0 {
		return nil // No exhaustion to remove
	}

	newLevel := currentLevel - levels
	if newLevel <= 0 {
		// Remove exhaustion completely
		return em.conditionManager.RemoveConditionType(entity, ConditionExhaustion)
	}

	// Create new exhaustion condition with reduced level
	exhaustion, err := NewExhaustionCondition(entity, newLevel, "exhaustion_reduced")
	if err != nil {
		return err
	}

	// Apply it (this will replace existing exhaustion)
	return em.conditionManager.ApplyCondition(exhaustion)
}

// ClearExhaustion removes all exhaustion from an entity.
func (em *ExhaustionManager) ClearExhaustion(entity core.Entity) error {
	return em.conditionManager.RemoveConditionType(entity, ConditionExhaustion)
}

// ApplyExhaustionOnRest handles exhaustion recovery during rests.
func (em *ExhaustionManager) ApplyExhaustionOnRest(entity core.Entity, restType string) error {
	switch restType {
	case "long":
		// Long rest removes 1 level of exhaustion
		return em.RemoveExhaustion(entity, 1)
	case "short":
		// Short rest doesn't affect exhaustion
		return nil
	default:
		return fmt.Errorf("unknown rest type: %s", restType)
	}
}

// CheckExhaustionDeath checks if an entity has died from exhaustion.
func (em *ExhaustionManager) CheckExhaustionDeath(entity core.Entity) bool {
	return em.GetExhaustionLevel(entity) >= 6
}

// Helper for creating exhaustion from various sources

// CreateExhaustionFromEnvironment creates exhaustion from environmental effects.
func CreateExhaustionFromEnvironment(target core.Entity, environment string) (*ExhaustionCondition, error) {
	sources := map[string]int{
		"extreme_heat": 1,
		"extreme_cold": 1,
		"forced_march": 1,
		"starvation":   1,
		"dehydration":  2,
		"suffocation":  1,
	}

	levels, exists := sources[environment]
	if !exists {
		return nil, fmt.Errorf("unknown environmental exhaustion source: %s", environment)
	}

	return NewExhaustionCondition(target, levels, fmt.Sprintf("environmental_%s", environment))
}

// CreateExhaustionFromSpell creates exhaustion from spell effects.
func CreateExhaustionFromSpell(target core.Entity, spellName string, levels int) (*ExhaustionCondition, error) {
	return NewExhaustionCondition(target, levels, fmt.Sprintf("spell_%s", spellName))
}

// CreateExhaustionFromAbility creates exhaustion from class abilities or features.
func CreateExhaustionFromAbility(target core.Entity, abilityName string, levels int) (*ExhaustionCondition, error) {
	return NewExhaustionCondition(target, levels, fmt.Sprintf("ability_%s", abilityName))
}
