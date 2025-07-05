// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

// ConditionType represents a specific type of condition.
type ConditionType string

// Standard D&D 5e conditions
const (
	ConditionBlinded       ConditionType = "blinded"
	ConditionCharmed       ConditionType = "charmed"
	ConditionDeafened      ConditionType = "deafened"
	ConditionExhaustion    ConditionType = "exhaustion"
	ConditionFrightened    ConditionType = "frightened"
	ConditionGrappled      ConditionType = "grappled"
	ConditionIncapacitated ConditionType = "incapacitated"
	ConditionInvisible     ConditionType = "invisible"
	ConditionParalyzed     ConditionType = "paralyzed"
	ConditionPetrified     ConditionType = "petrified"
	ConditionPoisoned      ConditionType = "poisoned"
	ConditionProne         ConditionType = "prone"
	ConditionRestrained    ConditionType = "restrained"
	ConditionStunned       ConditionType = "stunned"
	ConditionUnconscious   ConditionType = "unconscious"
)

// EffectType represents a type of mechanical effect.
type EffectType string

// Mechanical effect types
const (
	EffectAdvantage      EffectType = "advantage"
	EffectDisadvantage   EffectType = "disadvantage"
	EffectAutoFail       EffectType = "auto_fail"
	EffectAutoSucceed    EffectType = "auto_succeed"
	EffectImmunity       EffectType = "immunity"
	EffectSpeedReduction EffectType = "speed_reduction"
	EffectSpeedZero      EffectType = "speed_zero"
	EffectIncapacitated  EffectType = "incapacitated"
	EffectNoReactions    EffectType = "no_reactions"
	EffectVulnerability  EffectType = "vulnerability"
	EffectResistance     EffectType = "resistance"
	EffectCantSpeak      EffectType = "cant_speak"
	EffectCantHear       EffectType = "cant_hear"
	EffectCantSee        EffectType = "cant_see"
	EffectDropItems      EffectType = "drop_items"
	EffectMaxHPReduction EffectType = "max_hp_reduction"
)

// EffectTarget represents what the effect applies to.
type EffectTarget string

// Effect targets
const (
	TargetAttackRolls    EffectTarget = "attack_rolls"
	TargetSavingThrows   EffectTarget = "saving_throws"
	TargetAbilityChecks  EffectTarget = "ability_checks"
	TargetDexSaves       EffectTarget = "dex_saves"
	TargetStrSaves       EffectTarget = "str_saves"
	TargetPerception     EffectTarget = "perception"
	TargetMovement       EffectTarget = "movement"
	TargetActions        EffectTarget = "actions"
	TargetReactions      EffectTarget = "reactions"
	TargetDamage         EffectTarget = "damage"
	TargetSight          EffectTarget = "sight"
	TargetHearing        EffectTarget = "hearing"
	TargetSpeech         EffectTarget = "speech"
	TargetAttacksAgainst EffectTarget = "attacks_against"
	TargetAllSaves       EffectTarget = "all_saves"
	TargetAllChecks      EffectTarget = "all_checks"
)

// ConditionEffect represents a mechanical effect of a condition.
type ConditionEffect struct {
	Type     EffectType
	Target   EffectTarget
	Value    interface{} // Could be bool, int, string, etc.
	Metadata map[string]interface{}
}

// ConditionDefinition defines the properties and effects of a condition type.
type ConditionDefinition struct {
	Type        ConditionType
	Name        string
	Description string
	Effects     []ConditionEffect
	Immunities  []ConditionType // Conditions this prevents
	Includes    []ConditionType // Other conditions this automatically includes
	Suppresses  []ConditionType // Weaker conditions this overrides
}

// conditionDefinitions holds all standard condition definitions
var conditionDefinitions = map[ConditionType]*ConditionDefinition{
	ConditionBlinded: {
		Type:        ConditionBlinded,
		Name:        "Blinded",
		Description: "A blinded creature can't see and automatically fails any ability check that requires sight. Attack rolls against the creature have advantage, and the creature's attack rolls have disadvantage.",
		Effects: []ConditionEffect{
			{Type: EffectDisadvantage, Target: TargetAttackRolls},
			{Type: EffectAdvantage, Target: TargetAttacksAgainst},
			{Type: EffectAutoFail, Target: TargetSight},
			{Type: EffectCantSee, Target: TargetSight},
		},
	},
	ConditionCharmed: {
		Type:        ConditionCharmed,
		Name:        "Charmed",
		Description: "A charmed creature can't attack the charmer or target the charmer with harmful abilities or magical effects. The charmer has advantage on any ability check to interact socially with the creature.",
		Effects:     []ConditionEffect{
			// Effects are context-dependent and handled by specific implementations
		},
	},
	ConditionDeafened: {
		Type:        ConditionDeafened,
		Name:        "Deafened",
		Description: "A deafened creature can't hear and automatically fails any ability check that requires hearing.",
		Effects: []ConditionEffect{
			{Type: EffectAutoFail, Target: TargetHearing},
			{Type: EffectCantHear, Target: TargetHearing},
		},
	},
	ConditionExhaustion: {
		Type:        ConditionExhaustion,
		Name:        "Exhaustion",
		Description: "Exhaustion is measured in six levels. An effect can give a creature one or more levels of exhaustion, as specified in the effect's description.",
		Effects:     []ConditionEffect{
			// Level-based effects handled separately
		},
	},
	ConditionFrightened: {
		Type:        ConditionFrightened,
		Name:        "Frightened",
		Description: "A frightened creature has disadvantage on ability checks and attack rolls while the source of its fear is within line of sight. The creature can't willingly move closer to the source of its fear.",
		Effects: []ConditionEffect{
			{Type: EffectDisadvantage, Target: TargetAbilityChecks},
			{Type: EffectDisadvantage, Target: TargetAttackRolls},
			// Movement restriction is context-dependent
		},
	},
	ConditionGrappled: {
		Type:        ConditionGrappled,
		Name:        "Grappled",
		Description: "A grappled creature's speed becomes 0, and it can't benefit from any bonus to its speed. The condition ends if the grappler is incapacitated or if an effect removes the grappled creature from the reach of the grappler.",
		Effects: []ConditionEffect{
			{Type: EffectSpeedZero, Target: TargetMovement},
		},
	},
	ConditionIncapacitated: {
		Type:        ConditionIncapacitated,
		Name:        "Incapacitated",
		Description: "An incapacitated creature can't take actions or reactions.",
		Effects: []ConditionEffect{
			{Type: EffectIncapacitated, Target: TargetActions},
			{Type: EffectNoReactions, Target: TargetReactions},
		},
	},
	ConditionInvisible: {
		Type:        ConditionInvisible,
		Name:        "Invisible",
		Description: "An invisible creature is impossible to see without the aid of magic or a special sense. For the purpose of hiding, the creature is heavily obscured. The creature's location can be detected by any noise it makes or any tracks it leaves. Attack rolls against the creature have disadvantage, and the creature's attack rolls have advantage.",
		Effects: []ConditionEffect{
			{Type: EffectAdvantage, Target: TargetAttackRolls},
			{Type: EffectDisadvantage, Target: TargetAttacksAgainst},
		},
	},
	ConditionParalyzed: {
		Type:        ConditionParalyzed,
		Name:        "Paralyzed",
		Description: "A paralyzed creature is incapacitated and can't move or speak. The creature automatically fails Strength and Dexterity saving throws. Attack rolls against the creature have advantage. Any attack that hits the creature is a critical hit if the attacker is within 5 feet of the creature.",
		Effects: []ConditionEffect{
			{Type: EffectSpeedZero, Target: TargetMovement},
			{Type: EffectCantSpeak, Target: TargetSpeech},
			{Type: EffectAutoFail, Target: TargetStrSaves},
			{Type: EffectAutoFail, Target: TargetDexSaves},
			{Type: EffectAdvantage, Target: TargetAttacksAgainst},
			// Critical hit on melee is context-dependent
		},
		Includes: []ConditionType{ConditionIncapacitated},
	},
	ConditionPetrified: {
		Type:        ConditionPetrified,
		Name:        "Petrified",
		Description: "A petrified creature is transformed, along with any nonmagical object it is wearing or carrying, into a solid inanimate substance. Its weight increases by a factor of ten, and it ceases aging. The creature is incapacitated, can't move or speak, and is unaware of its surroundings. Attack rolls against the creature have advantage. The creature automatically fails Strength and Dexterity saving throws. The creature has resistance to all damage. The creature is immune to poison and disease.",
		Effects: []ConditionEffect{
			{Type: EffectSpeedZero, Target: TargetMovement},
			{Type: EffectCantSpeak, Target: TargetSpeech},
			{Type: EffectAutoFail, Target: TargetStrSaves},
			{Type: EffectAutoFail, Target: TargetDexSaves},
			{Type: EffectAdvantage, Target: TargetAttacksAgainst},
			{Type: EffectResistance, Target: TargetDamage, Value: "all"},
			{Type: EffectImmunity, Target: TargetDamage, Value: "poison"},
		},
		Includes:   []ConditionType{ConditionIncapacitated},
		Immunities: []ConditionType{ConditionPoisoned},
	},
	ConditionPoisoned: {
		Type:        ConditionPoisoned,
		Name:        "Poisoned",
		Description: "A poisoned creature has disadvantage on attack rolls and ability checks.",
		Effects: []ConditionEffect{
			{Type: EffectDisadvantage, Target: TargetAttackRolls},
			{Type: EffectDisadvantage, Target: TargetAbilityChecks},
		},
	},
	ConditionProne: {
		Type:        ConditionProne,
		Name:        "Prone",
		Description: "A prone creature's only movement option is to crawl, unless it stands up and thereby ends the condition. The creature has disadvantage on attack rolls. An attack roll against the creature has advantage if the attacker is within 5 feet of the creature. Otherwise, the attack roll has disadvantage.",
		Effects: []ConditionEffect{
			{Type: EffectDisadvantage, Target: TargetAttackRolls},
			// Advantage/disadvantage on attacks against is context-dependent (range)
		},
	},
	ConditionRestrained: {
		Type:        ConditionRestrained,
		Name:        "Restrained",
		Description: "A restrained creature's speed becomes 0, and it can't benefit from any bonus to its speed. Attack rolls against the creature have advantage, and the creature's attack rolls have disadvantage. The creature has disadvantage on Dexterity saving throws.",
		Effects: []ConditionEffect{
			{Type: EffectSpeedZero, Target: TargetMovement},
			{Type: EffectDisadvantage, Target: TargetAttackRolls},
			{Type: EffectAdvantage, Target: TargetAttacksAgainst},
			{Type: EffectDisadvantage, Target: TargetDexSaves},
		},
	},
	ConditionStunned: {
		Type:        ConditionStunned,
		Name:        "Stunned",
		Description: "A stunned creature is incapacitated, can't move, and can speak only falteringly. The creature automatically fails Strength and Dexterity saving throws. Attack rolls against the creature have advantage.",
		Effects: []ConditionEffect{
			{Type: EffectSpeedZero, Target: TargetMovement},
			{Type: EffectAutoFail, Target: TargetStrSaves},
			{Type: EffectAutoFail, Target: TargetDexSaves},
			{Type: EffectAdvantage, Target: TargetAttacksAgainst},
		},
		Includes: []ConditionType{ConditionIncapacitated},
	},
	ConditionUnconscious: {
		Type:        ConditionUnconscious,
		Name:        "Unconscious",
		Description: "An unconscious creature is incapacitated, can't move or speak, and is unaware of its surroundings. The creature drops whatever it's holding and falls prone. The creature automatically fails Strength and Dexterity saving throws. Attack rolls against the creature have advantage. Any attack that hits the creature is a critical hit if the attacker is within 5 feet of the creature.",
		Effects: []ConditionEffect{
			{Type: EffectSpeedZero, Target: TargetMovement},
			{Type: EffectCantSpeak, Target: TargetSpeech},
			{Type: EffectDropItems, Target: TargetActions},
			{Type: EffectAutoFail, Target: TargetStrSaves},
			{Type: EffectAutoFail, Target: TargetDexSaves},
			{Type: EffectAdvantage, Target: TargetAttacksAgainst},
		},
		Includes: []ConditionType{ConditionIncapacitated, ConditionProne},
	},
}

// GetConditionDefinition returns the definition for a condition type.
func GetConditionDefinition(condType ConditionType) (*ConditionDefinition, bool) {
	def, exists := conditionDefinitions[condType]
	return def, exists
}

// ExhaustionLevel represents a level of exhaustion (1-6).
type ExhaustionLevel int

// Exhaustion level effects
const (
	ExhaustionLevel1 ExhaustionLevel = 1 // Disadvantage on ability checks
	ExhaustionLevel2 ExhaustionLevel = 2 // Speed halved
	ExhaustionLevel3 ExhaustionLevel = 3 // Disadvantage on attack rolls and saving throws
	ExhaustionLevel4 ExhaustionLevel = 4 // Hit point maximum halved
	ExhaustionLevel5 ExhaustionLevel = 5 // Speed reduced to 0
	ExhaustionLevel6 ExhaustionLevel = 6 // Death
)

// GetExhaustionEffects returns the effects for a given exhaustion level.
func GetExhaustionEffects(level ExhaustionLevel) []ConditionEffect {
	effects := []ConditionEffect{}

	// Exhaustion effects are cumulative
	if level >= ExhaustionLevel1 {
		effects = append(effects, ConditionEffect{
			Type:   EffectDisadvantage,
			Target: TargetAbilityChecks,
		})
	}

	if level >= ExhaustionLevel2 {
		effects = append(effects, ConditionEffect{
			Type:   EffectSpeedReduction,
			Target: TargetMovement,
			Value:  0.5, // Speed halved
		})
	}

	if level >= ExhaustionLevel3 {
		effects = append(effects,
			ConditionEffect{
				Type:   EffectDisadvantage,
				Target: TargetAttackRolls,
			},
			ConditionEffect{
				Type:   EffectDisadvantage,
				Target: TargetSavingThrows,
			},
		)
	}

	if level >= ExhaustionLevel4 {
		effects = append(effects, ConditionEffect{
			Type:   EffectMaxHPReduction,
			Target: TargetDamage,
			Value:  0.5, // HP max halved
		})
	}

	if level >= ExhaustionLevel5 {
		// Override speed reduction with speed zero
		for i, effect := range effects {
			if effect.Type == EffectSpeedReduction {
				effects[i] = ConditionEffect{
					Type:   EffectSpeedZero,
					Target: TargetMovement,
				}
				break
			}
		}
	}

	// Level 6 is death, handled separately

	return effects
}
