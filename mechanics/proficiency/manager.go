// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package proficiency provides a comprehensive system for handling proficiencies
// in RPG games, particularly designed for D&D 5e mechanics.
package proficiency

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Manager handles proficiency checking and bonus calculation for entities.
//
//go:generate mockgen -destination=mock/mock_manager.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency Manager
type Manager interface {
	// Check proficiency
	HasProficiency(entity core.Entity, profType Type, key string) bool
	HasWeaponProficiency(entity core.Entity, weaponKey string) bool
	HasSkillProficiency(entity core.Entity, skillKey string) bool

	// Get bonuses
	GetProficiencyBonus(entity core.Entity) int
	GetWeaponAttackBonus(entity core.Entity, weaponKey string) int
	GetSkillBonus(entity core.Entity, skillKey string) int
	GetSaveBonus(entity core.Entity, abilityKey string) int

	// Manage proficiencies
	AddProficiency(entity core.Entity, prof Proficiency)
	RemoveProficiency(entity core.Entity, profKey string)
	GetAllProficiencies(entity core.Entity) []Proficiency
}

// Storage interface for persisting proficiencies.
type Storage interface {
	// GetProficiencies returns all proficiencies for an entity.
	GetProficiencies(entityID string) ([]Proficiency, error)

	// SaveProficiency adds or updates a proficiency for an entity.
	SaveProficiency(entityID string, prof Proficiency) error

	// RemoveProficiency removes a proficiency from an entity.
	RemoveProficiency(entityID string, profKey string) error

	// HasProficiency checks if an entity has a specific proficiency.
	HasProficiency(entityID string, profType Type, key string) (bool, error)
}

// LevelProvider provides entity level information for bonus calculation.
type LevelProvider interface {
	// GetLevel returns the level of an entity.
	GetLevel(entity core.Entity) int
}

// DefaultManager is the standard implementation of Manager.
type DefaultManager struct {
	storage       Storage
	levelProvider LevelProvider
}

// NewManager creates a new proficiency manager.
func NewManager(storage Storage, levelProvider LevelProvider) *DefaultManager {
	return &DefaultManager{
		storage:       storage,
		levelProvider: levelProvider,
	}
}

// HasProficiency checks if an entity has a specific proficiency.
func (m *DefaultManager) HasProficiency(entity core.Entity, profType Type, key string) bool {
	if entity == nil {
		return false
	}

	hasProficiency, err := m.storage.HasProficiency(entity.GetID(), profType, key)
	if err != nil {
		return false
	}

	// Check for category-based proficiencies
	if !hasProficiency {
		proficiencies, err := m.storage.GetProficiencies(entity.GetID())
		if err != nil {
			return false
		}

		for _, prof := range proficiencies {
			if prof.Type() == profType && prof.Category() != "" {
				// Check if the key matches the category
				if matchesCategory(key, prof.Category()) {
					return true
				}
			}
		}
	}

	return hasProficiency
}

// HasWeaponProficiency checks if an entity has proficiency with a specific weapon.
func (m *DefaultManager) HasWeaponProficiency(entity core.Entity, weaponKey string) bool {
	return m.HasProficiency(entity, TypeWeapon, weaponKey)
}

// HasSkillProficiency checks if an entity has proficiency with a specific skill.
func (m *DefaultManager) HasSkillProficiency(entity core.Entity, skillKey string) bool {
	return m.HasProficiency(entity, TypeSkill, skillKey)
}

// GetProficiencyBonus returns the proficiency bonus based on entity level.
func (m *DefaultManager) GetProficiencyBonus(entity core.Entity) int {
	if entity == nil || m.levelProvider == nil {
		return 2 // Default proficiency bonus
	}

	level := m.levelProvider.GetLevel(entity)
	return calculateProficiencyBonus(level)
}

// GetWeaponAttackBonus returns the attack bonus for a weapon (proficiency bonus if proficient).
func (m *DefaultManager) GetWeaponAttackBonus(entity core.Entity, weaponKey string) int {
	if m.HasWeaponProficiency(entity, weaponKey) {
		return m.GetProficiencyBonus(entity)
	}
	return 0
}

// GetSkillBonus returns the skill bonus (proficiency bonus if proficient).
func (m *DefaultManager) GetSkillBonus(entity core.Entity, skillKey string) int {
	if m.HasSkillProficiency(entity, skillKey) {
		return m.GetProficiencyBonus(entity)
	}
	return 0
}

// GetSaveBonus returns the saving throw bonus (proficiency bonus if proficient).
func (m *DefaultManager) GetSaveBonus(entity core.Entity, abilityKey string) int {
	if m.HasProficiency(entity, TypeSave, abilityKey) {
		return m.GetProficiencyBonus(entity)
	}
	return 0
}

// AddProficiency adds a proficiency to an entity.
func (m *DefaultManager) AddProficiency(entity core.Entity, prof Proficiency) {
	if entity == nil || prof == nil {
		return
	}

	_ = m.storage.SaveProficiency(entity.GetID(), prof)
}

// RemoveProficiency removes a proficiency from an entity.
func (m *DefaultManager) RemoveProficiency(entity core.Entity, profKey string) {
	if entity == nil {
		return
	}

	_ = m.storage.RemoveProficiency(entity.GetID(), profKey)
}

// GetAllProficiencies returns all proficiencies for an entity.
func (m *DefaultManager) GetAllProficiencies(entity core.Entity) []Proficiency {
	if entity == nil {
		return nil
	}

	proficiencies, err := m.storage.GetProficiencies(entity.GetID())
	if err != nil {
		return nil
	}

	return proficiencies
}

// calculateProficiencyBonus returns the proficiency bonus for a given level.
// Standard D&D 5e progression: +2 at levels 1-4, +3 at 5-8, +4 at 9-12, +5 at 13-16, +6 at 17-20.
func calculateProficiencyBonus(level int) int {
	if level <= 0 {
		return 2
	}

	// Formula: 2 + (level-1)/4
	return 2 + (level-1)/4
}

// matchesCategory checks if a key matches a category.
// For example, "shortsword" matches "simple-weapons".
// This is a placeholder that should be customized based on game rules.
func matchesCategory(key, category string) bool {
	// TODO: Implement actual category matching logic based on game rules
	// For now, this is a simple placeholder that game implementations should override

	// Example categories for D&D 5e:
	simpleWeapons := map[string]bool{
		"club":         true,
		"dagger":       true,
		"handaxe":      true,
		"javelin":      true,
		"mace":         true,
		"quarterstaff": true,
		"shortsword":   true,
		"spear":        true,
	}

	martialWeapons := map[string]bool{
		"battleaxe":   true,
		"flail":       true,
		"glaive":      true,
		"greataxe":    true,
		"greatsword":  true,
		"halberd":     true,
		"lance":       true,
		"longsword":   true,
		"maul":        true,
		"morningstar": true,
		"pike":        true,
		"rapier":      true,
		"scimitar":    true,
		"shortsword":  true,
		"trident":     true,
		"warhammer":   true,
		"whip":        true,
	}

	switch category {
	case "simple-weapons":
		return simpleWeapons[key]
	case "martial-weapons":
		return martialWeapons[key]
	default:
		return false
	}
}
