//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// DamageTypes provides type-safe, discoverable references to D&D 5e damage types.
// Use IDE autocomplete: refs.DamageTypes.<tab> to discover available damage types.
var DamageTypes = damageTypesNS{ns{TypeDamageTypes}}

type damageTypesNS struct{ ns }

// Physical Damage Types
func (n damageTypesNS) Bludgeoning() *core.Ref { return n.ref("bludgeoning") }
func (n damageTypesNS) Piercing() *core.Ref    { return n.ref("piercing") }
func (n damageTypesNS) Slashing() *core.Ref    { return n.ref("slashing") }

// Elemental Damage Types
func (n damageTypesNS) Acid() *core.Ref      { return n.ref("acid") }
func (n damageTypesNS) Cold() *core.Ref      { return n.ref("cold") }
func (n damageTypesNS) Fire() *core.Ref      { return n.ref("fire") }
func (n damageTypesNS) Lightning() *core.Ref { return n.ref("lightning") }
func (n damageTypesNS) Thunder() *core.Ref   { return n.ref("thunder") }

// Magical Damage Types
func (n damageTypesNS) Force() *core.Ref    { return n.ref("force") }
func (n damageTypesNS) Necrotic() *core.Ref { return n.ref("necrotic") }
func (n damageTypesNS) Poison() *core.Ref   { return n.ref("poison") }
func (n damageTypesNS) Psychic() *core.Ref  { return n.ref("psychic") }
func (n damageTypesNS) Radiant() *core.Ref  { return n.ref("radiant") }
