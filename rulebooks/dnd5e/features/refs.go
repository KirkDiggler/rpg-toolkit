// Package features provides ref builders and constants for D&D 5e class features.
package features

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref creates a feature reference from an index
func Ref(index string) *core.Ref {
	return core.MustNewRef(core.RefInput{
		Module: "dnd5e",
		Type:   "feature",
		Value:  index,
	})
}

// Common feature refs as constants
var (
	// Barbarian features
	RageRef           = Ref("rage")
	UnarmoredDefense  = Ref("unarmored_defense")
	RecklessAttack    = Ref("reckless_attack")
	DangerSense       = Ref("danger_sense")
	PrimalPath        = Ref("primal_path")
	ExtraAttack       = Ref("extra_attack")
	FastMovement      = Ref("fast_movement")
	FeralInstinct     = Ref("feral_instinct")
	BrutalCritical    = Ref("brutal_critical")

	// Fighter features
	FightingStyle     = Ref("fighting_style")
	SecondWind        = Ref("second_wind")
	ActionSurge       = Ref("action_surge")
	MartialArchetype  = Ref("martial_archetype")
	Indomitable       = Ref("indomitable")

	// Wizard features
	ArcaneRecovery    = Ref("arcane_recovery")
	ArcaneTradition   = Ref("arcane_tradition")
	SpellMastery      = Ref("spell_mastery")
	SignatureSpells   = Ref("signature_spells")

	// Rogue features
	Expertise         = Ref("expertise")
	SneakAttack       = Ref("sneak_attack")
	ThievesCant       = Ref("thieves_cant")
	CunningAction     = Ref("cunning_action")
	RoguishArchetype  = Ref("roguish_archetype")
	UncannyDodge      = Ref("uncanny_dodge")
	Evasion           = Ref("evasion")
	ReliableTalent    = Ref("reliable_talent")

	// Cleric features
	DivineDomain      = Ref("divine_domain")
	ChannelDivinity   = Ref("channel_divinity")
	TurnUndead        = Ref("turn_undead")
	DestroyUndead     = Ref("destroy_undead")
	DivineIntervention = Ref("divine_intervention")

	// Shared features
	AbilityScoreImprovement = Ref("ability_score_improvement")
	Spellcasting            = Ref("spellcasting")
)