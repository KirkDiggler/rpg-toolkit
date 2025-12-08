//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ability singletons - unexported for controlled access via methods
var (
	abilityStrength     = &core.Ref{Module: Module, Type: TypeAbilities, ID: "str"}
	abilityDexterity    = &core.Ref{Module: Module, Type: TypeAbilities, ID: "dex"}
	abilityConstitution = &core.Ref{Module: Module, Type: TypeAbilities, ID: "con"}
	abilityIntelligence = &core.Ref{Module: Module, Type: TypeAbilities, ID: "int"}
	abilityWisdom       = &core.Ref{Module: Module, Type: TypeAbilities, ID: "wis"}
	abilityCharisma     = &core.Ref{Module: Module, Type: TypeAbilities, ID: "cha"}
)

// Abilities provides type-safe, discoverable references to D&D 5e abilities.
// Use IDE autocomplete: refs.Abilities.<tab> to discover available abilities.
// Methods return singleton pointers enabling identity comparison (ref == refs.Abilities.Strength()).
var Abilities = abilitiesNS{}

type abilitiesNS struct{}

func (n abilitiesNS) Strength() *core.Ref     { return abilityStrength }
func (n abilitiesNS) Dexterity() *core.Ref    { return abilityDexterity }
func (n abilitiesNS) Constitution() *core.Ref { return abilityConstitution }
func (n abilitiesNS) Intelligence() *core.Ref { return abilityIntelligence }
func (n abilitiesNS) Wisdom() *core.Ref       { return abilityWisdom }
func (n abilitiesNS) Charisma() *core.Ref     { return abilityCharisma }
