//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Weapon singletons - unexported for controlled access via methods
var (
	// Simple Melee Weapons
	weaponClub         = &core.Ref{Module: Module, Type: TypeWeapons, ID: "club"}
	weaponDagger       = &core.Ref{Module: Module, Type: TypeWeapons, ID: "dagger"}
	weaponGreatclub    = &core.Ref{Module: Module, Type: TypeWeapons, ID: "greatclub"}
	weaponHandaxe      = &core.Ref{Module: Module, Type: TypeWeapons, ID: "handaxe"}
	weaponJavelin      = &core.Ref{Module: Module, Type: TypeWeapons, ID: "javelin"}
	weaponLightHammer  = &core.Ref{Module: Module, Type: TypeWeapons, ID: "light-hammer"}
	weaponMace         = &core.Ref{Module: Module, Type: TypeWeapons, ID: "mace"}
	weaponQuarterstaff = &core.Ref{Module: Module, Type: TypeWeapons, ID: "quarterstaff"}
	weaponSickle       = &core.Ref{Module: Module, Type: TypeWeapons, ID: "sickle"}
	weaponSpear        = &core.Ref{Module: Module, Type: TypeWeapons, ID: "spear"}

	// Simple Ranged Weapons
	weaponLightCrossbow = &core.Ref{Module: Module, Type: TypeWeapons, ID: "light-crossbow"}
	weaponDart          = &core.Ref{Module: Module, Type: TypeWeapons, ID: "dart"}
	weaponShortbow      = &core.Ref{Module: Module, Type: TypeWeapons, ID: "shortbow"}
	weaponSling         = &core.Ref{Module: Module, Type: TypeWeapons, ID: "sling"}

	// Martial Melee Weapons
	weaponBattleaxe   = &core.Ref{Module: Module, Type: TypeWeapons, ID: "battleaxe"}
	weaponFlail       = &core.Ref{Module: Module, Type: TypeWeapons, ID: "flail"}
	weaponGlaive      = &core.Ref{Module: Module, Type: TypeWeapons, ID: "glaive"}
	weaponGreataxe    = &core.Ref{Module: Module, Type: TypeWeapons, ID: "greataxe"}
	weaponGreatsword  = &core.Ref{Module: Module, Type: TypeWeapons, ID: "greatsword"}
	weaponHalberd     = &core.Ref{Module: Module, Type: TypeWeapons, ID: "halberd"}
	weaponLance       = &core.Ref{Module: Module, Type: TypeWeapons, ID: "lance"}
	weaponLongsword   = &core.Ref{Module: Module, Type: TypeWeapons, ID: "longsword"}
	weaponMaul        = &core.Ref{Module: Module, Type: TypeWeapons, ID: "maul"}
	weaponMorningstar = &core.Ref{Module: Module, Type: TypeWeapons, ID: "morningstar"}
	weaponPike        = &core.Ref{Module: Module, Type: TypeWeapons, ID: "pike"}
	weaponRapier      = &core.Ref{Module: Module, Type: TypeWeapons, ID: "rapier"}
	weaponScimitar    = &core.Ref{Module: Module, Type: TypeWeapons, ID: "scimitar"}
	weaponShortsword  = &core.Ref{Module: Module, Type: TypeWeapons, ID: "shortsword"}
	weaponTrident     = &core.Ref{Module: Module, Type: TypeWeapons, ID: "trident"}
	weaponWarPick     = &core.Ref{Module: Module, Type: TypeWeapons, ID: "war-pick"}
	weaponWarhammer   = &core.Ref{Module: Module, Type: TypeWeapons, ID: "warhammer"}
	weaponWhip        = &core.Ref{Module: Module, Type: TypeWeapons, ID: "whip"}

	// Martial Ranged Weapons
	weaponBlowgun       = &core.Ref{Module: Module, Type: TypeWeapons, ID: "blowgun"}
	weaponHandCrossbow  = &core.Ref{Module: Module, Type: TypeWeapons, ID: "hand-crossbow"}
	weaponHeavyCrossbow = &core.Ref{Module: Module, Type: TypeWeapons, ID: "heavy-crossbow"}
	weaponLongbow       = &core.Ref{Module: Module, Type: TypeWeapons, ID: "longbow"}
	weaponNet           = &core.Ref{Module: Module, Type: TypeWeapons, ID: "net"}

	// Category placeholders
	weaponAnySimple  = &core.Ref{Module: Module, Type: TypeWeapons, ID: "simple-weapon"}
	weaponAnyMartial = &core.Ref{Module: Module, Type: TypeWeapons, ID: "martial-weapon"}
	weaponAny        = &core.Ref{Module: Module, Type: TypeWeapons, ID: "any-weapon"}

	// Special weapons
	weaponUnarmedStrike = &core.Ref{Module: Module, Type: TypeWeapons, ID: "unarmed-strike"}
)

// Weapons provides type-safe, discoverable references to D&D 5e weapons.
// Use IDE autocomplete: refs.Weapons.<tab> to discover available weapons.
// Methods return singleton pointers enabling identity comparison (ref == refs.Weapons.Longsword()).
var Weapons = weaponsNS{}

type weaponsNS struct{}

// Simple Melee Weapons
func (n weaponsNS) Club() *core.Ref         { return weaponClub }
func (n weaponsNS) Dagger() *core.Ref       { return weaponDagger }
func (n weaponsNS) Greatclub() *core.Ref    { return weaponGreatclub }
func (n weaponsNS) Handaxe() *core.Ref      { return weaponHandaxe }
func (n weaponsNS) Javelin() *core.Ref      { return weaponJavelin }
func (n weaponsNS) LightHammer() *core.Ref  { return weaponLightHammer }
func (n weaponsNS) Mace() *core.Ref         { return weaponMace }
func (n weaponsNS) Quarterstaff() *core.Ref { return weaponQuarterstaff }
func (n weaponsNS) Sickle() *core.Ref       { return weaponSickle }
func (n weaponsNS) Spear() *core.Ref        { return weaponSpear }

// Simple Ranged Weapons
func (n weaponsNS) LightCrossbow() *core.Ref { return weaponLightCrossbow }
func (n weaponsNS) Dart() *core.Ref          { return weaponDart }
func (n weaponsNS) Shortbow() *core.Ref      { return weaponShortbow }
func (n weaponsNS) Sling() *core.Ref         { return weaponSling }

// Martial Melee Weapons
func (n weaponsNS) Battleaxe() *core.Ref   { return weaponBattleaxe }
func (n weaponsNS) Flail() *core.Ref       { return weaponFlail }
func (n weaponsNS) Glaive() *core.Ref      { return weaponGlaive }
func (n weaponsNS) Greataxe() *core.Ref    { return weaponGreataxe }
func (n weaponsNS) Greatsword() *core.Ref  { return weaponGreatsword }
func (n weaponsNS) Halberd() *core.Ref     { return weaponHalberd }
func (n weaponsNS) Lance() *core.Ref       { return weaponLance }
func (n weaponsNS) Longsword() *core.Ref   { return weaponLongsword }
func (n weaponsNS) Maul() *core.Ref        { return weaponMaul }
func (n weaponsNS) Morningstar() *core.Ref { return weaponMorningstar }
func (n weaponsNS) Pike() *core.Ref        { return weaponPike }
func (n weaponsNS) Rapier() *core.Ref      { return weaponRapier }
func (n weaponsNS) Scimitar() *core.Ref    { return weaponScimitar }
func (n weaponsNS) Shortsword() *core.Ref  { return weaponShortsword }
func (n weaponsNS) Trident() *core.Ref     { return weaponTrident }
func (n weaponsNS) WarPick() *core.Ref     { return weaponWarPick }
func (n weaponsNS) Warhammer() *core.Ref   { return weaponWarhammer }
func (n weaponsNS) Whip() *core.Ref        { return weaponWhip }

// Martial Ranged Weapons
func (n weaponsNS) Blowgun() *core.Ref       { return weaponBlowgun }
func (n weaponsNS) HandCrossbow() *core.Ref  { return weaponHandCrossbow }
func (n weaponsNS) HeavyCrossbow() *core.Ref { return weaponHeavyCrossbow }
func (n weaponsNS) Longbow() *core.Ref       { return weaponLongbow }
func (n weaponsNS) Net() *core.Ref           { return weaponNet }

// Category placeholders
func (n weaponsNS) AnySimpleWeapon() *core.Ref  { return weaponAnySimple }
func (n weaponsNS) AnyMartialWeapon() *core.Ref { return weaponAnyMartial }
func (n weaponsNS) AnyWeapon() *core.Ref        { return weaponAny }

// Special weapons
func (n weaponsNS) UnarmedStrike() *core.Ref { return weaponUnarmedStrike }

// weaponByID maps weapon ID strings to singleton refs for O(1) lookup
var weaponByID = map[string]*core.Ref{
	// Simple Melee Weapons
	"club":         weaponClub,
	"dagger":       weaponDagger,
	"greatclub":    weaponGreatclub,
	"handaxe":      weaponHandaxe,
	"javelin":      weaponJavelin,
	"light-hammer": weaponLightHammer,
	"mace":         weaponMace,
	"quarterstaff": weaponQuarterstaff,
	"sickle":       weaponSickle,
	"spear":        weaponSpear,
	// Simple Ranged Weapons
	"light-crossbow": weaponLightCrossbow,
	"dart":           weaponDart,
	"shortbow":       weaponShortbow,
	"sling":          weaponSling,
	// Martial Melee Weapons
	"battleaxe":   weaponBattleaxe,
	"flail":       weaponFlail,
	"glaive":      weaponGlaive,
	"greataxe":    weaponGreataxe,
	"greatsword":  weaponGreatsword,
	"halberd":     weaponHalberd,
	"lance":       weaponLance,
	"longsword":   weaponLongsword,
	"maul":        weaponMaul,
	"morningstar": weaponMorningstar,
	"pike":        weaponPike,
	"rapier":      weaponRapier,
	"scimitar":    weaponScimitar,
	"shortsword":  weaponShortsword,
	"trident":     weaponTrident,
	"war-pick":    weaponWarPick,
	"warhammer":   weaponWarhammer,
	"whip":        weaponWhip,
	// Martial Ranged Weapons
	"blowgun":        weaponBlowgun,
	"hand-crossbow":  weaponHandCrossbow,
	"heavy-crossbow": weaponHeavyCrossbow,
	"longbow":        weaponLongbow,
	"net":            weaponNet,
	// Category placeholders
	"simple-weapon":  weaponAnySimple,
	"martial-weapon": weaponAnyMartial,
	"any-weapon":     weaponAny,
	// Special weapons
	"unarmed-strike": weaponUnarmedStrike,
}

// ByID returns the singleton ref for the given weapon ID, or nil if not found.
// This enables toolkit code to get singleton refs from weapon IDs.
func (n weaponsNS) ByID(id string) *core.Ref {
	return weaponByID[id]
}
