// Package damage provides D&D 5e damage type constants and utilities
package damage

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Type represents types of damage in D&D 5e
type Type string

// Damage type constants
const (
	// Physical damage types
	Bludgeoning Type = "bludgeoning"
	Piercing    Type = "piercing"
	Slashing    Type = "slashing"

	// Elemental damage types
	Acid      Type = "acid"
	Cold      Type = "cold"
	Fire      Type = "fire"
	Lightning Type = "lightning"
	Thunder   Type = "thunder"

	// Magical damage types
	Force    Type = "force"
	Necrotic Type = "necrotic"
	Poison   Type = "poison"
	Psychic  Type = "psychic"
	Radiant  Type = "radiant"

	None Type = "none"
)

// All contains all damage types mapped by ID for O(1) lookup
var All = map[string]Type{
	"bludgeoning": Bludgeoning,
	"piercing":    Piercing,
	"slashing":    Slashing,
	"acid":        Acid,
	"cold":        Cold,
	"fire":        Fire,
	"lightning":   Lightning,
	"thunder":     Thunder,
	"force":       Force,
	"necrotic":    Necrotic,
	"poison":      Poison,
	"psychic":     Psychic,
	"radiant":     Radiant,
	"none":        None,
}

// Physical returns all physical damage types
func Physical() []Type {
	return []Type{Bludgeoning, Piercing, Slashing}
}

// Elemental returns all elemental damage types
func Elemental() []Type {
	return []Type{Acid, Cold, Fire, Lightning, Thunder}
}

// Magical returns all magical damage types
func Magical() []Type {
	return []Type{Force, Necrotic, Poison, Psychic, Radiant}
}

// GetByID returns a damage type by its ID
func GetByID(id string) (Type, error) {
	damageType, ok := All[id]
	if !ok {
		return "", rpgerr.New(rpgerr.CodeNotFound, "damage type not found",
			rpgerr.WithMeta("provided", id))
	}
	return damageType, nil
}

// IsPhysical returns true if this is a physical damage type
func (t Type) IsPhysical() bool {
	switch t {
	case Bludgeoning, Piercing, Slashing:
		return true
	default:
		return false
	}
}

// IsElemental returns true if this is an elemental damage type
func (t Type) IsElemental() bool {
	switch t {
	case Acid, Cold, Fire, Lightning, Thunder:
		return true
	default:
		return false
	}
}

// IsMagical returns true if this is a magical damage type
func (t Type) IsMagical() bool {
	switch t {
	case Force, Necrotic, Poison, Psychic, Radiant:
		return true
	default:
		return false
	}
}

// Display returns the human-readable name of the damage type
func (t Type) Display() string {
	switch t {
	case Acid:
		return "Acid"
	case Bludgeoning:
		return "Bludgeoning"
	case Cold:
		return "Cold"
	case Fire:
		return "Fire"
	case Force:
		return "Force"
	case Lightning:
		return "Lightning"
	case Necrotic:
		return "Necrotic"
	case Piercing:
		return "Piercing"
	case Poison:
		return "Poison"
	case Psychic:
		return "Psychic"
	case Radiant:
		return "Radiant"
	case Slashing:
		return "Slashing"
	case Thunder:
		return "Thunder"
	default:
		return string(t)
	}
}
