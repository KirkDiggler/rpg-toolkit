// Package system provides ref builders and constants for D&D 5e system concepts.
package system

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref creates a system reference from an index
func Ref(index string) *core.Ref {
	return core.MustNewRef(core.RefInput{
		Module: "dnd5e",
		Type:   "system",
		Value:  index,
	})
}

// System event and source refs as constants
var (
	// Character lifecycle
	Creation = Ref("creation")
	LevelUp  = Ref("level_up")
	Rest     = Ref("rest")
	Death    = Ref("death")

	// Combat phases
	Initiative  = Ref("initiative")
	TurnStart   = Ref("turn_start")
	TurnEnd     = Ref("turn_end")
	RoundStart  = Ref("round_start")
	RoundEnd    = Ref("round_end")
	CombatStart = Ref("combat_start")
	CombatEnd   = Ref("combat_end")

	// Rest types
	ShortRest = Ref("short_rest")
	LongRest  = Ref("long_rest")

	// Damage types
	DamageAcid        = Ref("damage_acid")
	DamageBludgeoning = Ref("damage_bludgeoning")
	DamageCold        = Ref("damage_cold")
	DamageFire        = Ref("damage_fire")
	DamageForce       = Ref("damage_force")
	DamageLightning   = Ref("damage_lightning")
	DamageNecrotic    = Ref("damage_necrotic")
	DamagePiercing    = Ref("damage_piercing")
	DamagePoison      = Ref("damage_poison")
	DamagePsychic     = Ref("damage_psychic")
	DamageRadiant     = Ref("damage_radiant")
	DamageSlashing    = Ref("damage_slashing")
	DamageThunder     = Ref("damage_thunder")

	// Sources
	Player      = Ref("player")
	Environment = Ref("environment")
	Magic       = Ref("magic")
	Item        = Ref("item")
)