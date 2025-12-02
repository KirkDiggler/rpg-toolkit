// Package dnd5e provides ref helper functions for type-safe references
package dnd5e

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref type constants for D&D 5e (used in core.Ref.Type field)
const (
	TypeConditions = "conditions"
	TypeFeatures   = "features"
	TypeClasses    = "classes"
	TypeRaces      = "races"
	TypeSpells     = "spells"
	TypeEquipment  = "equipment"
)

// ConditionRef creates a core.Ref for a D&D 5e condition
func ConditionRef(id string) *core.Ref {
	return &core.Ref{
		Module: Module,
		Type:   TypeConditions,
		ID:     id,
	}
}

// FeatureRef creates a core.Ref for a D&D 5e feature
func FeatureRef(id string) *core.Ref {
	return &core.Ref{
		Module: Module,
		Type:   TypeFeatures,
		ID:     id,
	}
}

// ClassRef creates a core.Ref for a D&D 5e class
func ClassRef(id string) *core.Ref {
	return &core.Ref{
		Module: Module,
		Type:   TypeClasses,
		ID:     id,
	}
}

// RaceRef creates a core.Ref for a D&D 5e race
func RaceRef(id string) *core.Ref {
	return &core.Ref{
		Module: Module,
		Type:   TypeRaces,
		ID:     id,
	}
}

// SpellRef creates a core.Ref for a D&D 5e spell
func SpellRef(id string) *core.Ref {
	return &core.Ref{
		Module: Module,
		Type:   TypeSpells,
		ID:     id,
	}
}

// EquipmentRef creates a core.Ref for a D&D 5e equipment item
func EquipmentRef(id string) *core.Ref {
	return &core.Ref{
		Module: Module,
		Type:   TypeEquipment,
		ID:     id,
	}
}
