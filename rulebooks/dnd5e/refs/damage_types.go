package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// DamageTypes provides type-safe, discoverable references to D&D 5e damage types.
// Use IDE autocomplete: refs.DamageTypes.<tab> to discover available damage types.
var DamageTypes = damageTypesNS{}

type damageTypesNS struct{}

// Physical Damage Types

// Bludgeoning returns a reference to the Bludgeoning damage type.
func (damageTypesNS) Bludgeoning() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "bludgeoning"}
}

// Piercing returns a reference to the Piercing damage type.
func (damageTypesNS) Piercing() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "piercing"}
}

// Slashing returns a reference to the Slashing damage type.
func (damageTypesNS) Slashing() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "slashing"}
}

// Elemental Damage Types

// Acid returns a reference to the Acid damage type.
func (damageTypesNS) Acid() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "acid"}
}

// Cold returns a reference to the Cold damage type.
func (damageTypesNS) Cold() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "cold"}
}

// Fire returns a reference to the Fire damage type.
func (damageTypesNS) Fire() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "fire"}
}

// Lightning returns a reference to the Lightning damage type.
func (damageTypesNS) Lightning() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "lightning"}
}

// Thunder returns a reference to the Thunder damage type.
func (damageTypesNS) Thunder() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "thunder"}
}

// Magical Damage Types

// Force returns a reference to the Force damage type.
func (damageTypesNS) Force() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "force"}
}

// Necrotic returns a reference to the Necrotic damage type.
func (damageTypesNS) Necrotic() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "necrotic"}
}

// Poison returns a reference to the Poison damage type.
func (damageTypesNS) Poison() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "poison"}
}

// Psychic returns a reference to the Psychic damage type.
func (damageTypesNS) Psychic() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "psychic"}
}

// Radiant returns a reference to the Radiant damage type.
func (damageTypesNS) Radiant() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "radiant"}
}
