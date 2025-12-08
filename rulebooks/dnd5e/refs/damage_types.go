//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// DamageType singletons - unexported for controlled access via methods
var (
	// Physical Damage Types
	damageTypeBludgeoning = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "bludgeoning"}
	damageTypePiercing    = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "piercing"}
	damageTypeSlashing    = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "slashing"}

	// Elemental Damage Types
	damageTypeAcid      = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "acid"}
	damageTypeCold      = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "cold"}
	damageTypeFire      = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "fire"}
	damageTypeLightning = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "lightning"}
	damageTypeThunder   = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "thunder"}

	// Magical Damage Types
	damageTypeForce    = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "force"}
	damageTypeNecrotic = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "necrotic"}
	damageTypePoison   = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "poison"}
	damageTypePsychic  = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "psychic"}
	damageTypeRadiant  = &core.Ref{Module: Module, Type: TypeDamageTypes, ID: "radiant"}
)

// DamageTypes provides type-safe, discoverable references to D&D 5e damage types.
// Use IDE autocomplete: refs.DamageTypes.<tab> to discover available damage types.
// Methods return singleton pointers enabling identity comparison (ref == refs.DamageTypes.Fire()).
var DamageTypes = damageTypesNS{}

type damageTypesNS struct{}

// Physical Damage Types
func (n damageTypesNS) Bludgeoning() *core.Ref { return damageTypeBludgeoning }
func (n damageTypesNS) Piercing() *core.Ref    { return damageTypePiercing }
func (n damageTypesNS) Slashing() *core.Ref    { return damageTypeSlashing }

// Elemental Damage Types
func (n damageTypesNS) Acid() *core.Ref      { return damageTypeAcid }
func (n damageTypesNS) Cold() *core.Ref      { return damageTypeCold }
func (n damageTypesNS) Fire() *core.Ref      { return damageTypeFire }
func (n damageTypesNS) Lightning() *core.Ref { return damageTypeLightning }
func (n damageTypesNS) Thunder() *core.Ref   { return damageTypeThunder }

// Magical Damage Types
func (n damageTypesNS) Force() *core.Ref    { return damageTypeForce }
func (n damageTypesNS) Necrotic() *core.Ref { return damageTypeNecrotic }
func (n damageTypesNS) Poison() *core.Ref   { return damageTypePoison }
func (n damageTypesNS) Psychic() *core.Ref  { return damageTypePsychic }
func (n damageTypesNS) Radiant() *core.Ref  { return damageTypeRadiant }
