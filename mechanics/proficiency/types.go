// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package proficiency

// ProficiencyType represents different kinds of proficiencies in the game system.
type ProficiencyType string //nolint:revive // Named for clarity when used across packages

const (
	// ProficiencyTypeWeapon represents proficiency with weapons.
	ProficiencyTypeWeapon ProficiencyType = "weapon"

	// ProficiencyTypeArmor represents proficiency with armor.
	ProficiencyTypeArmor ProficiencyType = "armor"

	// ProficiencyTypeSkill represents proficiency with skills.
	ProficiencyTypeSkill ProficiencyType = "skill"

	// ProficiencyTypeSave represents proficiency with saving throws.
	ProficiencyTypeSave ProficiencyType = "saving_throw"

	// ProficiencyTypeTool represents proficiency with tools.
	ProficiencyTypeTool ProficiencyType = "tool"

	// ProficiencyTypeInstrument represents proficiency with musical instruments.
	ProficiencyTypeInstrument ProficiencyType = "instrument"
)

// Proficiency represents something an entity can be proficient with.
type Proficiency interface {
	// Type returns the proficiency type.
	Type() ProficiencyType

	// Key returns the unique key for this proficiency (e.g., "shortsword", "athletics").
	Key() string

	// Name returns the display name for this proficiency.
	Name() string

	// Category returns the category (e.g., "simple-weapons", "martial-weapons").
	Category() string
}

// SimpleProficiencyConfig holds the configuration for creating a SimpleProficiency.
type SimpleProficiencyConfig struct {
	Type     ProficiencyType
	Key      string
	Name     string
	Category string
}

// SimpleProficiency is a basic implementation of Proficiency.
type SimpleProficiency struct {
	profType ProficiencyType
	key      string
	name     string
	category string
}

// NewSimpleProficiency creates a new simple proficiency from a config.
func NewSimpleProficiency(cfg SimpleProficiencyConfig) *SimpleProficiency {
	return &SimpleProficiency{
		profType: cfg.Type,
		key:      cfg.Key,
		name:     cfg.Name,
		category: cfg.Category,
	}
}

// Type returns the proficiency type.
func (p *SimpleProficiency) Type() ProficiencyType {
	return p.profType
}

// Key returns the unique key for this proficiency.
func (p *SimpleProficiency) Key() string {
	return p.key
}

// Name returns the display name for this proficiency.
func (p *SimpleProficiency) Name() string {
	return p.name
}

// Category returns the category for this proficiency.
func (p *SimpleProficiency) Category() string {
	return p.category
}

// NewProficiency is a convenience function that creates a proficiency using the old API.
// Deprecated: Use NewSimpleProficiency with SimpleProficiencyConfig instead.
func NewProficiency(profType ProficiencyType, key, name, category string) *SimpleProficiency {
	return NewSimpleProficiency(SimpleProficiencyConfig{
		Type:     profType,
		Key:      key,
		Name:     name,
		Category: category,
	})
}
