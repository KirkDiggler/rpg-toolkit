// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// RelationshipType defines how conditions relate to their sources
type RelationshipType string

const (
	// RelationshipConcentration - caster can only maintain one group, broken by damage
	RelationshipConcentration RelationshipType = "concentration"

	// RelationshipAura - conditions exist while source is in range
	RelationshipAura RelationshipType = "aura"

	// RelationshipChanneled - requires continuous action from source
	RelationshipChanneled RelationshipType = "channeled"

	// RelationshipMaintained - costs resources each turn to maintain
	RelationshipMaintained RelationshipType = "maintained"

	// RelationshipLinked - conditions that must be removed together
	RelationshipLinked RelationshipType = "linked"

	// RelationshipDependent - condition exists only while another does
	RelationshipDependent RelationshipType = "dependent"
)

// Relationship tracks how conditions relate to their sources or each other
type Relationship struct {
	Type       RelationshipType
	Source     core.Entity    // The entity maintaining this relationship
	Conditions []Condition    // The conditions in this relationship
	Metadata   map[string]any // Additional data (range for auras, cost for maintained, etc)
}

// RelationshipManager handles all condition relationships in the game
type RelationshipManager struct {
	mu sync.RWMutex
	// Track relationships by source entity
	bySource map[string][]*Relationship
	// Track which relationship a condition belongs to
	byCondition map[string]*Relationship
	// Event bus for notifications
	bus events.EventBus
}

// NewRelationshipManager creates a new relationship manager
func NewRelationshipManager(bus events.EventBus) *RelationshipManager {
	rm := &RelationshipManager{
		bySource:    make(map[string][]*Relationship),
		byCondition: make(map[string]*Relationship),
		bus:         bus,
	}

	// Subscribe to relevant events
	// Could listen for damage events to check concentration saves
	// Could listen for movement to update auras
	// etc.

	return rm
}

// CreateRelationship establishes a new relationship
func (rm *RelationshipManager) CreateRelationship(
	relType RelationshipType,
	source core.Entity,
	conditions []Condition,
	metadata map[string]any,
) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Handle type-specific logic
	switch relType {
	case RelationshipConcentration:
		// Break any existing concentration
		if err := rm.breakConcentrationUnsafe(source); err != nil {
			return err
		}

	case RelationshipChanneled:
		// Might check if source has available actions

	case RelationshipAura:
		// Might validate range metadata
		if metadata == nil || metadata["range"] == nil {
			return fmt.Errorf("aura relationships require 'range' metadata")
		}
	}

	// Create the relationship
	rel := &Relationship{
		Type:       relType,
		Source:     source,
		Conditions: conditions,
		Metadata:   metadata,
	}

	// Track it
	rm.bySource[source.GetID()] = append(rm.bySource[source.GetID()], rel)
	for _, cond := range conditions {
		rm.byCondition[cond.GetID()] = rel
	}

	return nil
}

// BreakRelationship removes a specific relationship and all its conditions
func (rm *RelationshipManager) BreakRelationship(rel *Relationship) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	return rm.breakRelationshipUnsafe(rel)
}

// BreakAllRelationships removes all relationships for a source
func (rm *RelationshipManager) BreakAllRelationships(source core.Entity) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rels := rm.bySource[source.GetID()]
	var errs []error

	for _, rel := range rels {
		if err := rm.breakRelationshipUnsafe(rel); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors breaking relationships: %v", errs)
	}

	return nil
}

// GetRelationship returns the relationship a condition belongs to
func (rm *RelationshipManager) GetRelationship(condition Condition) *Relationship {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.byCondition[condition.GetID()]
}

// GetRelationshipsByType returns all relationships of a specific type for a source
func (rm *RelationshipManager) GetRelationshipsByType(source core.Entity, relType RelationshipType) []*Relationship {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []*Relationship
	for _, rel := range rm.bySource[source.GetID()] {
		if rel.Type == relType {
			result = append(result, rel)
		}
	}

	return result
}

// UpdateAuras checks and updates all aura relationships based on positions
func (rm *RelationshipManager) UpdateAuras() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var toRemove []*Relationship

	// Check each aura relationship
	for _, rels := range rm.bySource {
		for _, rel := range rels {
			if rel.Type != RelationshipAura {
				continue
			}

			// TODO: Implement actual range checking
			// This requires:
			// 1. Position data for source and target entities
			// 2. Range value from rel.Metadata["range"]
			// 3. Distance calculation logic
			// For now, auras never expire due to range
			inRange := true

			if !inRange {
				toRemove = append(toRemove, rel)
			}
		}
	}

	// Remove out-of-range auras
	for _, rel := range toRemove {
		if err := rm.breakRelationshipUnsafe(rel); err != nil {
			return err
		}
	}

	return nil
}

// Internal helpers

func (rm *RelationshipManager) breakConcentrationUnsafe(source core.Entity) error {
	// Find all concentration relationships
	var concentrations []*Relationship
	for _, rel := range rm.bySource[source.GetID()] {
		if rel.Type == RelationshipConcentration {
			concentrations = append(concentrations, rel)
		}
	}

	// Break them all
	for _, rel := range concentrations {
		if err := rm.breakRelationshipUnsafe(rel); err != nil {
			return err
		}
	}

	return nil
}

func (rm *RelationshipManager) breakRelationshipUnsafe(rel *Relationship) error {
	// Remove all conditions
	var errs []error
	for _, cond := range rel.Conditions {
		if err := cond.Remove(rm.bus); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove condition %s: %w", cond.GetID(), err))
		}
		delete(rm.byCondition, cond.GetID())
	}

	// Remove from source tracking
	sourceRels := rm.bySource[rel.Source.GetID()]
	for i, r := range sourceRels {
		if r == rel {
			rm.bySource[rel.Source.GetID()] = append(sourceRels[:i], sourceRels[i+1:]...)
			break
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors breaking relationship: %v", errs)
	}

	return nil
}
