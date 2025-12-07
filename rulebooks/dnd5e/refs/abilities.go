package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Abilities provides type-safe, discoverable references to D&D 5e abilities.
// Use IDE autocomplete: refs.Abilities.<tab> to discover available abilities.
var Abilities = abilitiesNS{}

type abilitiesNS struct{}

// Strength returns a reference to the Strength ability.
func (abilitiesNS) Strength() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeAbilities, ID: "str"}
}

// Dexterity returns a reference to the Dexterity ability.
func (abilitiesNS) Dexterity() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeAbilities, ID: "dex"}
}

// Constitution returns a reference to the Constitution ability.
func (abilitiesNS) Constitution() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeAbilities, ID: "con"}
}

// Intelligence returns a reference to the Intelligence ability.
func (abilitiesNS) Intelligence() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeAbilities, ID: "int"}
}

// Wisdom returns a reference to the Wisdom ability.
func (abilitiesNS) Wisdom() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeAbilities, ID: "wis"}
}

// Charisma returns a reference to the Charisma ability.
func (abilitiesNS) Charisma() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeAbilities, ID: "cha"}
}
