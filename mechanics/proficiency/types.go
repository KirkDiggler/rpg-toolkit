// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package proficiency

// Type represents different kinds of proficiencies in the game system.
type Type string

const (
	// TypeWeapon represents proficiency with weapons.
	TypeWeapon Type = "weapon"

	// TypeArmor represents proficiency with armor.
	TypeArmor Type = "armor"

	// TypeSkill represents proficiency with skills.
	TypeSkill Type = "skill"

	// TypeSave represents proficiency with saving throws.
	TypeSave Type = "saving_throw"

	// TypeTool represents proficiency with tools.
	TypeTool Type = "tool"

	// TypeInstrument represents proficiency with musical instruments.
	TypeInstrument Type = "instrument"
)

// Proficiency represents something an entity can be proficient with.
type Proficiency interface {
	// Type returns the proficiency type.
	Type() Type

	// Key returns the unique key for this proficiency (e.g., "shortsword", "athletics").
	Key() string

	// Name returns the display name for this proficiency.
	Name() string

	// Category returns the category (e.g., "simple-weapons", "martial-weapons").
	Category() string
}

// SimpleProficiency is a basic implementation of Proficiency.
type SimpleProficiency struct {
	profType Type
	key      string
	name     string
	category string
}

// NewProficiency creates a new simple proficiency.
func NewProficiency(profType Type, key, name, category string) *SimpleProficiency {
	return &SimpleProficiency{
		profType: profType,
		key:      key,
		name:     name,
		category: category,
	}
}

// Type returns the proficiency type.
func (p *SimpleProficiency) Type() Type {
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
