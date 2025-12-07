//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Armor provides type-safe, discoverable references to D&D 5e armor.
// Use IDE autocomplete: refs.Armor.<tab> to discover available armor.
var Armor = armorNS{ns{TypeArmor}}

type armorNS struct{ ns }

// Light Armor
func (n armorNS) Padded() *core.Ref         { return n.ref("padded") }
func (n armorNS) Leather() *core.Ref        { return n.ref("leather") }
func (n armorNS) StuddedLeather() *core.Ref { return n.ref("studded-leather") }

// Medium Armor
func (n armorNS) Hide() *core.Ref        { return n.ref("hide") }
func (n armorNS) ChainShirt() *core.Ref  { return n.ref("chain-shirt") }
func (n armorNS) ScaleMail() *core.Ref   { return n.ref("scale-mail") }
func (n armorNS) Breastplate() *core.Ref { return n.ref("breastplate") }
func (n armorNS) HalfPlate() *core.Ref   { return n.ref("half-plate") }

// Heavy Armor
func (n armorNS) RingMail() *core.Ref  { return n.ref("ring-mail") }
func (n armorNS) ChainMail() *core.Ref { return n.ref("chain-mail") }
func (n armorNS) Splint() *core.Ref    { return n.ref("splint") }
func (n armorNS) Plate() *core.Ref     { return n.ref("plate") }

// Shield
func (n armorNS) Shield() *core.Ref { return n.ref("shield") }
