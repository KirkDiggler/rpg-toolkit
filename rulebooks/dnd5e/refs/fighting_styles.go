package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// FightingStyles provides type-safe, discoverable references to D&D 5e fighting styles.
// Use IDE autocomplete: refs.FightingStyles.<tab> to discover available fighting styles.
var FightingStyles = fightingStylesNS{}

type fightingStylesNS struct{}

// Archery returns a reference to the Archery fighting style.
// Grants +2 to attack rolls with ranged weapons.
func (fightingStylesNS) Archery() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "archery"}
}

// Defense returns a reference to the Defense fighting style.
// Grants +1 to AC while wearing armor.
func (fightingStylesNS) Defense() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "defense"}
}

// Dueling returns a reference to the Dueling fighting style.
// Grants +2 damage with one-handed weapons when no other weapon is held.
func (fightingStylesNS) Dueling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "dueling"}
}

// GreatWeaponFighting returns a reference to the Great Weapon Fighting style.
// Allows rerolling 1s and 2s on damage with two-handed weapons.
func (fightingStylesNS) GreatWeaponFighting() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "great_weapon_fighting"}
}

// Protection returns a reference to the Protection fighting style.
// Allows imposing disadvantage on attacks against nearby allies when wielding a shield.
func (fightingStylesNS) Protection() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "protection"}
}

// TwoWeaponFighting returns a reference to the Two-Weapon Fighting style.
// Adds ability modifier to off-hand damage when dual wielding.
func (fightingStylesNS) TwoWeaponFighting() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "two_weapon_fighting"}
}
