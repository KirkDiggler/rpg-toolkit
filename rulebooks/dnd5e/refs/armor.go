package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Armor provides type-safe, discoverable references to D&D 5e armor.
// Use IDE autocomplete: refs.Armor.<tab> to discover available armor.
var Armor = armorNS{}

type armorNS struct{}

// Light Armor

// Padded returns a reference to Padded armor.
func (armorNS) Padded() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "padded"}
}

// Leather returns a reference to Leather armor.
func (armorNS) Leather() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "leather"}
}

// StuddedLeather returns a reference to Studded Leather armor.
func (armorNS) StuddedLeather() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "studded-leather"}
}

// Medium Armor

// Hide returns a reference to Hide armor.
func (armorNS) Hide() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "hide"}
}

// ChainShirt returns a reference to Chain Shirt armor.
func (armorNS) ChainShirt() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "chain-shirt"}
}

// ScaleMail returns a reference to Scale Mail armor.
func (armorNS) ScaleMail() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "scale-mail"}
}

// Breastplate returns a reference to Breastplate armor.
func (armorNS) Breastplate() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "breastplate"}
}

// HalfPlate returns a reference to Half Plate armor.
func (armorNS) HalfPlate() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "half-plate"}
}

// Heavy Armor

// RingMail returns a reference to Ring Mail armor.
func (armorNS) RingMail() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "ring-mail"}
}

// ChainMail returns a reference to Chain Mail armor.
func (armorNS) ChainMail() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "chain-mail"}
}

// Splint returns a reference to Splint armor.
func (armorNS) Splint() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "splint"}
}

// Plate returns a reference to Plate armor.
func (armorNS) Plate() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "plate"}
}

// Shield

// Shield returns a reference to a Shield.
func (armorNS) Shield() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeArmor, ID: "shield"}
}
