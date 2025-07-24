// Package effects provides D&D 5e spell and ability effects
package effects

// EffectType categorizes different effects
type EffectType string

const (
	// EffectBless represents the Bless spell effect (+1d4 to attacks and saves)
	EffectBless EffectType = "bless"
	// EffectShield represents Shield spell (+5 AC for 1 round)
	EffectShield EffectType = "shield"
	// EffectMageArmor represents Mage Armor spell (13 + Dex AC)
	EffectMageArmor EffectType = "mage_armor"
	// EffectHaste represents Haste spell (+2 AC, double speed, additional action)
	EffectHaste EffectType = "haste"
	// EffectSlow represents Slow spell (-2 AC, halved speed, limited actions)
	EffectSlow EffectType = "slow"
	// EffectBarkskin represents Barkskin spell (minimum 16 AC)
	EffectBarkskin EffectType = "barkskin"

	// EffectRage represents the Barbarian rage effect
	EffectRage EffectType = "rage"
	// EffectRecklessAttack represents Barbarian reckless attack (advantage on attacks)
	EffectRecklessAttack EffectType = "reckless_attack"
	// EffectDivineSmite represents Paladin divine smite (extra radiant damage)
	EffectDivineSmite EffectType = "divine_smite"
	// EffectSneakAttack represents Rogue sneak attack (extra damage dice)
	EffectSneakAttack EffectType = "sneak_attack"

	// EffectMagicWeapon represents magic weapon bonuses
	EffectMagicWeapon EffectType = "magic_weapon"
	// EffectMagicArmor represents magical armor enhancement bonuses
	EffectMagicArmor EffectType = "magic_armor"
)

// Effect represents a temporary effect on a character
type Effect struct {
	Type       EffectType `json:"type"`
	Source     string     `json:"source"`      // Who/what created this effect
	SourceType string     `json:"source_type"` // "spell", "item", "feature"
	Duration   string     `json:"duration,omitempty"`

	// Common modifiers (simplified for persistence)
	AttackBonus string `json:"attack_bonus,omitempty"` // "+1", "+1d4"
	DamageBonus string `json:"damage_bonus,omitempty"` // "+2", "+1d6"
	ACBonus     int    `json:"ac_bonus,omitempty"`
	SaveBonus   string `json:"save_bonus,omitempty"` // "+1", "+1d4"

	// Other properties
	Concentration bool `json:"concentration,omitempty"`
	Data          any  `json:"data,omitempty"` // Effect-specific data
}

// NewBlessEffect creates a Bless spell effect
func NewBlessEffect(source string) Effect {
	return Effect{
		Type:          EffectBless,
		Source:        source,
		SourceType:    "spell",
		AttackBonus:   "+1d4",
		SaveBonus:     "+1d4",
		Duration:      "1_minute",
		Concentration: true,
	}
}

// NewShieldEffect creates a Shield spell effect (+5 AC)
func NewShieldEffect(source string) Effect {
	return Effect{
		Type:       EffectShield,
		Source:     source,
		SourceType: "spell",
		ACBonus:    5,
		Duration:   "1_round",
	}
}

// NewRageEffect creates a Barbarian rage effect
func NewRageEffect(source string) Effect {
	return Effect{
		Type:        EffectRage,
		Source:      source,
		SourceType:  "feature",
		DamageBonus: "+2", // Varies by barbarian level
		Duration:    "1_minute",
		Data: map[string]any{
			"resistance": []string{"bludgeoning", "piercing", "slashing"},
			"advantage":  []string{"strength_checks", "strength_saves"},
		},
	}
}
