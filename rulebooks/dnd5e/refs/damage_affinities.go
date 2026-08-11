package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

var (
	damageAffinityResistance    = &core.Ref{Module: Module, Type: TypeDamageAffinities, ID: "resistance"}
	damageAffinityVulnerability = &core.Ref{Module: Module, Type: TypeDamageAffinities, ID: "vulnerability"}
	damageAffinityImmunity      = &core.Ref{Module: Module, Type: TypeDamageAffinities, ID: "immunity"}
)

// DamageAffinities provides refs for reusable resistance, vulnerability, and
// immunity behaviors. They can be granted by creatures, items, or spells.
var DamageAffinities = damageAffinitiesNS{}

type damageAffinitiesNS struct{}

func (damageAffinitiesNS) Resistance() *core.Ref    { return damageAffinityResistance }
func (damageAffinitiesNS) Vulnerability() *core.Ref { return damageAffinityVulnerability }
func (damageAffinitiesNS) Immunity() *core.Ref      { return damageAffinityImmunity }
