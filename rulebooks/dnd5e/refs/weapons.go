//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Weapons provides type-safe, discoverable references to D&D 5e weapons.
// Use IDE autocomplete: refs.Weapons.<tab> to discover available weapons.
var Weapons = weaponsNS{ns{TypeWeapons}}

type weaponsNS struct{ ns }

// Simple Melee Weapons
func (n weaponsNS) Club() *core.Ref         { return n.ref("club") }
func (n weaponsNS) Dagger() *core.Ref       { return n.ref("dagger") }
func (n weaponsNS) Greatclub() *core.Ref    { return n.ref("greatclub") }
func (n weaponsNS) Handaxe() *core.Ref      { return n.ref("handaxe") }
func (n weaponsNS) Javelin() *core.Ref      { return n.ref("javelin") }
func (n weaponsNS) LightHammer() *core.Ref  { return n.ref("light-hammer") }
func (n weaponsNS) Mace() *core.Ref         { return n.ref("mace") }
func (n weaponsNS) Quarterstaff() *core.Ref { return n.ref("quarterstaff") }
func (n weaponsNS) Sickle() *core.Ref       { return n.ref("sickle") }
func (n weaponsNS) Spear() *core.Ref        { return n.ref("spear") }

// Simple Ranged Weapons
func (n weaponsNS) LightCrossbow() *core.Ref { return n.ref("light-crossbow") }
func (n weaponsNS) Dart() *core.Ref          { return n.ref("dart") }
func (n weaponsNS) Shortbow() *core.Ref      { return n.ref("shortbow") }
func (n weaponsNS) Sling() *core.Ref         { return n.ref("sling") }

// Martial Melee Weapons
func (n weaponsNS) Battleaxe() *core.Ref   { return n.ref("battleaxe") }
func (n weaponsNS) Flail() *core.Ref       { return n.ref("flail") }
func (n weaponsNS) Glaive() *core.Ref      { return n.ref("glaive") }
func (n weaponsNS) Greataxe() *core.Ref    { return n.ref("greataxe") }
func (n weaponsNS) Greatsword() *core.Ref  { return n.ref("greatsword") }
func (n weaponsNS) Halberd() *core.Ref     { return n.ref("halberd") }
func (n weaponsNS) Lance() *core.Ref       { return n.ref("lance") }
func (n weaponsNS) Longsword() *core.Ref   { return n.ref("longsword") }
func (n weaponsNS) Maul() *core.Ref        { return n.ref("maul") }
func (n weaponsNS) Morningstar() *core.Ref { return n.ref("morningstar") }
func (n weaponsNS) Pike() *core.Ref        { return n.ref("pike") }
func (n weaponsNS) Rapier() *core.Ref      { return n.ref("rapier") }
func (n weaponsNS) Scimitar() *core.Ref    { return n.ref("scimitar") }
func (n weaponsNS) Shortsword() *core.Ref  { return n.ref("shortsword") }
func (n weaponsNS) Trident() *core.Ref     { return n.ref("trident") }
func (n weaponsNS) WarPick() *core.Ref     { return n.ref("war-pick") }
func (n weaponsNS) Warhammer() *core.Ref   { return n.ref("warhammer") }
func (n weaponsNS) Whip() *core.Ref        { return n.ref("whip") }

// Martial Ranged Weapons
func (n weaponsNS) Blowgun() *core.Ref       { return n.ref("blowgun") }
func (n weaponsNS) HandCrossbow() *core.Ref  { return n.ref("hand-crossbow") }
func (n weaponsNS) HeavyCrossbow() *core.Ref { return n.ref("heavy-crossbow") }
func (n weaponsNS) Longbow() *core.Ref       { return n.ref("longbow") }
func (n weaponsNS) Net() *core.Ref           { return n.ref("net") }

// Category placeholders
func (n weaponsNS) AnySimpleWeapon() *core.Ref  { return n.ref("simple-weapon") }
func (n weaponsNS) AnyMartialWeapon() *core.Ref { return n.ref("martial-weapon") }
func (n weaponsNS) AnyWeapon() *core.Ref        { return n.ref("any-weapon") }
