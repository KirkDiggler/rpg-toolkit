// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"fmt"
	"sync"
)

// Preparation style constants for spell casters.
const (
	PreparationStyleKnown    = "known"    // For sorcerers, bards, etc.
	PreparationStylePrepared = "prepared" // For wizards, clerics, etc.
)

//go:generate mockgen -destination=mock/mock_spell_list.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/spells SpellList

// SpellList manages known and prepared spells for a caster.
type SpellList interface {
	// Known spells (sorcerer, bard, ranger)
	AddKnownSpell(spell Spell) error
	RemoveKnownSpell(spellID string) error
	GetKnownSpells() []Spell
	IsKnown(spellID string) bool

	// Prepared spells (wizard, cleric, druid)
	PrepareSpell(spell Spell) error
	UnprepareSpell(spellID string) error
	GetPreparedSpells() []Spell
	IsPrepared(spellID string) bool
	MaxPreparedSpells() int

	// Cantrips
	AddCantrip(spell Spell) error
	GetCantrips() []Spell

	// General queries
	GetSpell(spellID string) (Spell, bool)
	CanCast(spellID string) bool
}

// SimpleSpellList provides a basic spell list implementation.
type SimpleSpellList struct {
	mu               sync.RWMutex
	knownSpells      map[string]Spell
	preparedSpells   map[string]Spell
	cantrips         map[string]Spell
	maxPrepared      int
	preparationStyle string // PreparationStyleKnown or PreparationStylePrepared
}

// SpellListConfig configures a spell list.
type SpellListConfig struct {
	MaxPreparedSpells int
	PreparationStyle  string // PreparationStyleKnown (sorcerer) or PreparationStylePrepared (wizard)
}

// NewSimpleSpellList creates a new spell list.
func NewSimpleSpellList(config SpellListConfig) *SimpleSpellList {
	return &SimpleSpellList{
		knownSpells:      make(map[string]Spell),
		preparedSpells:   make(map[string]Spell),
		cantrips:         make(map[string]Spell),
		maxPrepared:      config.MaxPreparedSpells,
		preparationStyle: config.PreparationStyle,
	}
}

// AddKnownSpell adds a spell to the known spells list.
func (sl *SimpleSpellList) AddKnownSpell(spell Spell) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if spell.Level() == 0 {
		return fmt.Errorf("use AddCantrip for cantrips")
	}

	sl.knownSpells[spell.GetID()] = spell

	// For "known" casters, known spells are always prepared
	if sl.preparationStyle == PreparationStyleKnown {
		sl.preparedSpells[spell.GetID()] = spell
	}

	return nil
}

// RemoveKnownSpell removes a spell from the known spells list.
func (sl *SimpleSpellList) RemoveKnownSpell(spellID string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if _, ok := sl.knownSpells[spellID]; !ok {
		return fmt.Errorf("spell %s is not known", spellID)
	}

	delete(sl.knownSpells, spellID)
	delete(sl.preparedSpells, spellID)

	return nil
}

// GetKnownSpells returns all known spells.
func (sl *SimpleSpellList) GetKnownSpells() []Spell {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	spells := make([]Spell, 0, len(sl.knownSpells))
	for _, spell := range sl.knownSpells {
		spells = append(spells, spell)
	}
	return spells
}

// IsKnown checks if a spell is known.
func (sl *SimpleSpellList) IsKnown(spellID string) bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	_, ok := sl.knownSpells[spellID]
	return ok
}

// PrepareSpell prepares a known spell for casting.
func (sl *SimpleSpellList) PrepareSpell(spell Spell) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	// Known casters don't prepare spells
	if sl.preparationStyle == PreparationStyleKnown {
		return fmt.Errorf("this caster doesn't prepare spells")
	}

	// Check if spell is known
	if _, ok := sl.knownSpells[spell.GetID()]; !ok {
		return fmt.Errorf("can only prepare known spells")
	}

	// Check preparation limit
	if len(sl.preparedSpells) >= sl.maxPrepared {
		return fmt.Errorf("already at max prepared spells (%d)", sl.maxPrepared)
	}

	sl.preparedSpells[spell.GetID()] = spell
	return nil
}

// UnprepareSpell removes a spell from the prepared list.
func (sl *SimpleSpellList) UnprepareSpell(spellID string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.preparationStyle == PreparationStyleKnown {
		return fmt.Errorf("this caster doesn't prepare spells")
	}

	if _, ok := sl.preparedSpells[spellID]; !ok {
		return fmt.Errorf("spell %s is not prepared", spellID)
	}

	delete(sl.preparedSpells, spellID)
	return nil
}

// GetPreparedSpells returns all prepared spells.
func (sl *SimpleSpellList) GetPreparedSpells() []Spell {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	spells := make([]Spell, 0, len(sl.preparedSpells))
	for _, spell := range sl.preparedSpells {
		spells = append(spells, spell)
	}
	return spells
}

// IsPrepared checks if a spell is prepared.
func (sl *SimpleSpellList) IsPrepared(spellID string) bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	_, ok := sl.preparedSpells[spellID]
	return ok
}

// MaxPreparedSpells returns the maximum number of spells that can be prepared.
func (sl *SimpleSpellList) MaxPreparedSpells() int {
	return sl.maxPrepared
}

// AddCantrip adds a cantrip to the spell list.
func (sl *SimpleSpellList) AddCantrip(spell Spell) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if spell.Level() != 0 {
		return fmt.Errorf("spell %s is not a cantrip", spell.GetID())
	}

	if _, exists := sl.cantrips[spell.GetID()]; exists {
		return fmt.Errorf("cantrip %s is already known", spell.GetID())
	}

	sl.cantrips[spell.GetID()] = spell
	return nil
}

// GetCantrips returns all cantrips.
func (sl *SimpleSpellList) GetCantrips() []Spell {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	spells := make([]Spell, 0, len(sl.cantrips))
	for _, spell := range sl.cantrips {
		spells = append(spells, spell)
	}
	return spells
}

// GetSpell retrieves a spell by ID.
func (sl *SimpleSpellList) GetSpell(spellID string) (Spell, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	// Check cantrips first
	if spell, ok := sl.cantrips[spellID]; ok {
		return spell, true
	}

	// Then known spells
	if spell, ok := sl.knownSpells[spellID]; ok {
		return spell, true
	}

	return nil, false
}

// CanCast checks if a spell can be cast (is prepared or a cantrip).
func (sl *SimpleSpellList) CanCast(spellID string) bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	// Cantrips can always be cast
	if _, ok := sl.cantrips[spellID]; ok {
		return true
	}

	// Otherwise must be prepared
	_, ok := sl.preparedSpells[spellID]
	return ok
}
