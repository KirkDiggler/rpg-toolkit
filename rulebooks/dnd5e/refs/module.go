// Package refs provides discoverable, type-safe references for D&D 5e content.
//
// Usage:
//
//	import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
//
//	// Type refs.<tab> to discover namespaces: Features, Conditions, Classes, etc.
//	// Type refs.Features.<tab> to discover features: Rage, SecondWind, etc.
//
//	source := refs.Features.Rage()        // *core.Ref for the Rage feature
//	condition := refs.Conditions.Raging() // *core.Ref for the Raging condition
//	class := refs.Classes.Barbarian()     // *core.Ref for the Barbarian class
//
// This package is a leaf package - it only imports core to ensure all other
// dnd5e packages can import it without cycles.
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Module is the module identifier for D&D 5e content
const Module core.Module = "dnd5e"

// Type constants for different content categories
const (
	TypeFeatures       core.Type = "features"
	TypeConditions     core.Type = "conditions"
	TypeClasses        core.Type = "classes"
	TypeRaces          core.Type = "races"
	TypeWeapons        core.Type = "weapons"
	TypeSpells         core.Type = "spells"
	TypeEquipment      core.Type = "equipment"
	TypeAbilities      core.Type = "abilities"
	TypeSkills         core.Type = "skills"
	TypeBackgrounds    core.Type = "backgrounds"
	TypeLanguages      core.Type = "languages"
	TypeFightingStyles core.Type = "fighting_styles"
	TypeDamageTypes    core.Type = "damage_types"
	TypeTools          core.Type = "tools"
	TypeArmor          core.Type = "armor"
)

// ns is a helper for building namespace refs. Embed this in namespace structs
// to get a ref() method that creates refs with the correct module and type.
type ns struct {
	t core.Type
}

func (n ns) ref(id core.ID) *core.Ref {
	return &core.Ref{Module: Module, Type: n.t, ID: id}
}
