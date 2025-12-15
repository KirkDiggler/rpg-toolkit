// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

// Slot constants for weapon equipment positions
const (
	SlotMainHand = "main_hand"
	SlotOffHand  = "off_hand"
)

// EquippedWeapon represents a weapon equipped by a character.
// Purpose: Provides weapon information needed for combat calculations and
// feature eligibility checks (e.g., Dueling fighting style).
type EquippedWeapon struct {
	// ID is the unique identifier for this weapon instance
	ID string

	// Name is the display name of the weapon
	Name string

	// Slot indicates which hand the weapon is equipped in
	// Valid values: "main_hand", "off_hand"
	Slot string

	// IsShield indicates if this item is a shield (not a weapon for attack purposes)
	IsShield bool

	// IsTwoHanded indicates if this weapon requires both hands to wield
	IsTwoHanded bool

	// IsMelee indicates if this is a melee weapon (vs ranged)
	IsMelee bool
}

// CharacterWeapons holds weapon information for a character.
// Purpose: Provides methods to query equipped weapons for combat and feature eligibility checks.
type CharacterWeapons struct {
	mainHand *EquippedWeapon
	offHand  *EquippedWeapon
}

// NewCharacterWeapons creates a new CharacterWeapons from a slice of equipped weapons.
// Weapons are assigned to main hand or off hand based on their Slot field.
// If multiple weapons target the same slot, the last one wins.
func NewCharacterWeapons(weapons []*EquippedWeapon) *CharacterWeapons {
	cw := &CharacterWeapons{}
	for _, w := range weapons {
		if w == nil {
			continue
		}
		switch w.Slot {
		case SlotMainHand:
			cw.mainHand = w
		case SlotOffHand:
			cw.offHand = w
		}
	}
	return cw
}

// MainHand returns the weapon equipped in the main hand slot.
// Returns nil if no weapon is equipped in the main hand.
func (cw *CharacterWeapons) MainHand() *EquippedWeapon {
	return cw.mainHand
}

// OffHand returns the weapon equipped in the off-hand slot.
// Returns nil if no weapon is equipped, or if a shield is equipped.
// Purpose: Allows features like Dueling to check if a weapon (not shield) is in the off-hand.
func (cw *CharacterWeapons) OffHand() *EquippedWeapon {
	if cw.offHand == nil || cw.offHand.IsShield {
		return nil
	}
	return cw.offHand
}

// AllEquipped returns all equipped weapons (both main hand and off-hand).
// Excludes shields from the result.
// Always returns a non-nil slice, even if empty.
// Purpose: Provides complete weapon inventory for combat calculations.
func (cw *CharacterWeapons) AllEquipped() []*EquippedWeapon {
	weapons := make([]*EquippedWeapon, 0)

	if cw.mainHand != nil && !cw.mainHand.IsShield {
		weapons = append(weapons, cw.mainHand)
	}

	if cw.offHand != nil && !cw.offHand.IsShield {
		weapons = append(weapons, cw.offHand)
	}

	return weapons
}

// BasicCharacterRegistry is a concrete implementation of CharacterRegistry.
// Purpose: Provides in-memory storage for character state during event processing.
type BasicCharacterRegistry struct {
	characters    map[string]*CharacterWeapons
	abilityScores map[string]*AbilityScores
}

// NewBasicCharacterRegistry creates a new BasicCharacterRegistry.
func NewBasicCharacterRegistry() *BasicCharacterRegistry {
	return &BasicCharacterRegistry{
		characters:    make(map[string]*CharacterWeapons),
		abilityScores: make(map[string]*AbilityScores),
	}
}

// Add registers a character with their equipped weapons.
// If the character already exists, their weapons are replaced.
func (r *BasicCharacterRegistry) Add(characterID string, weapons *CharacterWeapons) {
	r.characters[characterID] = weapons
}

// Get retrieves the equipped weapons for a character.
// Returns the CharacterWeapons and true if found, nil and false otherwise.
func (r *BasicCharacterRegistry) Get(characterID string) (*CharacterWeapons, bool) {
	weapons, ok := r.characters[characterID]
	return weapons, ok
}

// GetCharacterWeapons retrieves weapon information for a character by ID.
// Returns nil if the character is not found.
// Purpose: Implements the CharacterRegistry interface.
func (r *BasicCharacterRegistry) GetCharacterWeapons(id string) *CharacterWeapons {
	return r.characters[id]
}

// AddAbilityScores registers a character's ability scores.
// If the character already has scores, they are replaced.
func (r *BasicCharacterRegistry) AddAbilityScores(characterID string, scores *AbilityScores) {
	r.abilityScores[characterID] = scores
}

// GetCharacterAbilityScores retrieves ability scores for a character by ID.
// Returns nil if the character is not found.
// Purpose: Allows features to query ability modifiers (e.g., Two-Weapon Fighting).
func (r *BasicCharacterRegistry) GetCharacterAbilityScores(id string) *AbilityScores {
	return r.abilityScores[id]
}
