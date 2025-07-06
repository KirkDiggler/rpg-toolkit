// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// SpellSlotTable defines spell slots by class and level.
type SpellSlotTable interface {
	GetSlots(classLevel int, spellLevel int) int
}

// SpellSlotPool manages spell slots for a caster.
type SpellSlotPool struct {
	pool  resources.Pool // Single pool containing all spell slot resources
	owner core.Entity
	class string
	level int
	table SpellSlotTable
}

// NewSpellSlotPool creates a new spell slot pool.
func NewSpellSlotPool(owner core.Entity, class string, level int, table SpellSlotTable) *SpellSlotPool {
	ssp := &SpellSlotPool{
		pool:  resources.NewSimplePool(owner),
		owner: owner,
		class: class,
		level: level,
		table: table,
	}

	// Initialize resources for each spell level
	for spellLevel := 1; spellLevel <= 9; spellLevel++ {
		slots := table.GetSlots(level, spellLevel)
		if slots > 0 {
			// Create a resource for this spell level
			resource := resources.NewSimpleResource(resources.SimpleResourceConfig{
				ID:      fmt.Sprintf("%s_spell_slots_%d", class, spellLevel),
				Type:    resources.ResourceType("spell_slot"),
				Owner:   owner,
				Key:     fmt.Sprintf("spell_slot_%d", spellLevel),
				Current: slots,
				Maximum: slots,
				RestoreTriggers: map[string]int{
					"long_rest": -1, // Full restore on long rest
				},
			})
			if err := ssp.pool.Add(resource); err != nil {
				// Log error but continue with other spell levels
				// In production, this should be logged properly
				continue
			}
		}
	}

	return ssp
}

// HasSlot checks if a slot is available at the given level.
func (ssp *SpellSlotPool) HasSlot(level int) bool {
	key := fmt.Sprintf("spell_slot_%d", level)
	if resource, ok := ssp.pool.Get(key); ok {
		return resource.Current() > 0
	}
	return false
}

// UseSlot consumes a spell slot of the given level.
func (ssp *SpellSlotPool) UseSlot(level int, bus events.EventBus) error {
	key := fmt.Sprintf("spell_slot_%d", level)
	if resource, ok := ssp.pool.Get(key); ok {
		if resource.Current() > 0 {
			return ssp.pool.Consume(key, 1, bus)
		}
		return fmt.Errorf("no spell slots available at level %d", level)
	}
	return fmt.Errorf("no spell slots at level %d", level)
}

// GetAvailableSlots returns the number of available slots at each level.
func (ssp *SpellSlotPool) GetAvailableSlots() map[int]int {
	slots := make(map[int]int)
	for level := 1; level <= 9; level++ {
		key := fmt.Sprintf("spell_slot_%d", level)
		if resource, ok := ssp.pool.Get(key); ok {
			slots[level] = resource.Current()
		}
	}
	return slots
}

// GetMaxSlots returns the maximum slots at each level.
func (ssp *SpellSlotPool) GetMaxSlots() map[int]int {
	slots := make(map[int]int)
	for level := 1; level <= 9; level++ {
		key := fmt.Sprintf("spell_slot_%d", level)
		if resource, ok := ssp.pool.Get(key); ok {
			slots[level] = resource.Maximum()
		}
	}
	return slots
}

// RestoreSlots restores spell slots based on rest type.
func (ssp *SpellSlotPool) RestoreSlots(restType string, bus events.EventBus) {
	// The pool will handle restoration for all resources based on their triggers
	ssp.pool.ProcessRestoration(restType, bus)
}

// SimpleSpellSlotTable provides a basic implementation for custom spell slot progressions.
type SimpleSpellSlotTable struct {
	progression map[int]map[int]int // [class level][spell level] = slots
}

// NewSimpleSpellSlotTable creates a custom spell slot table.
func NewSimpleSpellSlotTable(progression map[int]map[int]int) *SimpleSpellSlotTable {
	return &SimpleSpellSlotTable{
		progression: progression,
	}
}

// GetSlots returns the number of spell slots for a given class and spell level.
func (t *SimpleSpellSlotTable) GetSlots(classLevel int, spellLevel int) int {
	if slots, ok := t.progression[classLevel]; ok {
		return slots[spellLevel]
	}
	return 0
}
