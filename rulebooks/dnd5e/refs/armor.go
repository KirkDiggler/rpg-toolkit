//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Armor singletons - unexported for controlled access via methods
var (
	// Light Armor
	armorPadded         = &core.Ref{Module: Module, Type: TypeArmor, ID: "padded"}
	armorLeather        = &core.Ref{Module: Module, Type: TypeArmor, ID: "leather"}
	armorStuddedLeather = &core.Ref{Module: Module, Type: TypeArmor, ID: "studded-leather"}

	// Medium Armor
	armorHide        = &core.Ref{Module: Module, Type: TypeArmor, ID: "hide"}
	armorChainShirt  = &core.Ref{Module: Module, Type: TypeArmor, ID: "chain-shirt"}
	armorScaleMail   = &core.Ref{Module: Module, Type: TypeArmor, ID: "scale-mail"}
	armorBreastplate = &core.Ref{Module: Module, Type: TypeArmor, ID: "breastplate"}
	armorHalfPlate   = &core.Ref{Module: Module, Type: TypeArmor, ID: "half-plate"}

	// Heavy Armor
	armorRingMail  = &core.Ref{Module: Module, Type: TypeArmor, ID: "ring-mail"}
	armorChainMail = &core.Ref{Module: Module, Type: TypeArmor, ID: "chain-mail"}
	armorSplint    = &core.Ref{Module: Module, Type: TypeArmor, ID: "splint"}
	armorPlate     = &core.Ref{Module: Module, Type: TypeArmor, ID: "plate"}

	// Shield
	armorShield = &core.Ref{Module: Module, Type: TypeArmor, ID: "shield"}
)

// Armor provides type-safe, discoverable references to D&D 5e armor.
// Use IDE autocomplete: refs.Armor.<tab> to discover available armor.
// Methods return singleton pointers enabling identity comparison (ref == refs.Armor.Plate()).
var Armor = armorNS{}

type armorNS struct{}

// Light Armor
func (n armorNS) Padded() *core.Ref         { return armorPadded }
func (n armorNS) Leather() *core.Ref        { return armorLeather }
func (n armorNS) StuddedLeather() *core.Ref { return armorStuddedLeather }

// Medium Armor
func (n armorNS) Hide() *core.Ref        { return armorHide }
func (n armorNS) ChainShirt() *core.Ref  { return armorChainShirt }
func (n armorNS) ScaleMail() *core.Ref   { return armorScaleMail }
func (n armorNS) Breastplate() *core.Ref { return armorBreastplate }
func (n armorNS) HalfPlate() *core.Ref   { return armorHalfPlate }

// Heavy Armor
func (n armorNS) RingMail() *core.Ref  { return armorRingMail }
func (n armorNS) ChainMail() *core.Ref { return armorChainMail }
func (n armorNS) Splint() *core.Ref    { return armorSplint }
func (n armorNS) Plate() *core.Ref     { return armorPlate }

// Shield
func (n armorNS) Shield() *core.Ref { return armorShield }
