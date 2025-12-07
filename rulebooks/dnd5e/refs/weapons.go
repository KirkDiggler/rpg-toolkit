package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Weapons provides type-safe, discoverable references to D&D 5e weapons.
// Use IDE autocomplete: refs.Weapons.<tab> to discover available weapons.
var Weapons = weaponsNS{}

type weaponsNS struct{}

// Simple Melee Weapons

// Club returns a reference to the Club weapon.
func (weaponsNS) Club() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "club"}
}

// Dagger returns a reference to the Dagger weapon.
func (weaponsNS) Dagger() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "dagger"}
}

// Greatclub returns a reference to the Greatclub weapon.
func (weaponsNS) Greatclub() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "greatclub"}
}

// Handaxe returns a reference to the Handaxe weapon.
func (weaponsNS) Handaxe() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "handaxe"}
}

// Javelin returns a reference to the Javelin weapon.
func (weaponsNS) Javelin() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "javelin"}
}

// LightHammer returns a reference to the Light Hammer weapon.
func (weaponsNS) LightHammer() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "light-hammer"}
}

// Mace returns a reference to the Mace weapon.
func (weaponsNS) Mace() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "mace"}
}

// Quarterstaff returns a reference to the Quarterstaff weapon.
func (weaponsNS) Quarterstaff() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "quarterstaff"}
}

// Sickle returns a reference to the Sickle weapon.
func (weaponsNS) Sickle() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "sickle"}
}

// Spear returns a reference to the Spear weapon.
func (weaponsNS) Spear() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "spear"}
}

// Simple Ranged Weapons

// LightCrossbow returns a reference to the Light Crossbow weapon.
func (weaponsNS) LightCrossbow() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "light-crossbow"}
}

// Dart returns a reference to the Dart weapon.
func (weaponsNS) Dart() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "dart"}
}

// Shortbow returns a reference to the Shortbow weapon.
func (weaponsNS) Shortbow() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "shortbow"}
}

// Sling returns a reference to the Sling weapon.
func (weaponsNS) Sling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "sling"}
}

// Martial Melee Weapons

// Battleaxe returns a reference to the Battleaxe weapon.
func (weaponsNS) Battleaxe() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "battleaxe"}
}

// Flail returns a reference to the Flail weapon.
func (weaponsNS) Flail() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "flail"}
}

// Glaive returns a reference to the Glaive weapon.
func (weaponsNS) Glaive() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "glaive"}
}

// Greataxe returns a reference to the Greataxe weapon.
func (weaponsNS) Greataxe() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "greataxe"}
}

// Greatsword returns a reference to the Greatsword weapon.
func (weaponsNS) Greatsword() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "greatsword"}
}

// Halberd returns a reference to the Halberd weapon.
func (weaponsNS) Halberd() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "halberd"}
}

// Lance returns a reference to the Lance weapon.
func (weaponsNS) Lance() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "lance"}
}

// Longsword returns a reference to the Longsword weapon.
func (weaponsNS) Longsword() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "longsword"}
}

// Maul returns a reference to the Maul weapon.
func (weaponsNS) Maul() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "maul"}
}

// Morningstar returns a reference to the Morningstar weapon.
func (weaponsNS) Morningstar() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "morningstar"}
}

// Pike returns a reference to the Pike weapon.
func (weaponsNS) Pike() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "pike"}
}

// Rapier returns a reference to the Rapier weapon.
func (weaponsNS) Rapier() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "rapier"}
}

// Scimitar returns a reference to the Scimitar weapon.
func (weaponsNS) Scimitar() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "scimitar"}
}

// Shortsword returns a reference to the Shortsword weapon.
func (weaponsNS) Shortsword() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "shortsword"}
}

// Trident returns a reference to the Trident weapon.
func (weaponsNS) Trident() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "trident"}
}

// WarPick returns a reference to the War Pick weapon.
func (weaponsNS) WarPick() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "war-pick"}
}

// Warhammer returns a reference to the Warhammer weapon.
func (weaponsNS) Warhammer() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "warhammer"}
}

// Whip returns a reference to the Whip weapon.
func (weaponsNS) Whip() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "whip"}
}

// Martial Ranged Weapons

// Blowgun returns a reference to the Blowgun weapon.
func (weaponsNS) Blowgun() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "blowgun"}
}

// HandCrossbow returns a reference to the Hand Crossbow weapon.
func (weaponsNS) HandCrossbow() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "hand-crossbow"}
}

// HeavyCrossbow returns a reference to the Heavy Crossbow weapon.
func (weaponsNS) HeavyCrossbow() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "heavy-crossbow"}
}

// Longbow returns a reference to the Longbow weapon.
func (weaponsNS) Longbow() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "longbow"}
}

// Net returns a reference to the Net weapon.
func (weaponsNS) Net() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "net"}
}

// Category placeholders

// AnySimpleWeapon returns a reference for any simple weapon.
func (weaponsNS) AnySimpleWeapon() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "simple-weapon"}
}

// AnyMartialWeapon returns a reference for any martial weapon.
func (weaponsNS) AnyMartialWeapon() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "martial-weapon"}
}

// AnyWeapon returns a reference for any weapon.
func (weaponsNS) AnyWeapon() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeWeapons, ID: "any-weapon"}
}
