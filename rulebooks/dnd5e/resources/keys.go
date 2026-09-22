// Package resources provides D&D 5e resource key constants.
// These keys identify class-specific resources that are stored on characters
// and consumed by features.
package resources

import (
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
)

// Resource key constants for D&D 5e class resources.
// These are used with Character.GetResource() to access resource pools.
const (
	WrathOfTheStorm coreResources.ResourceKey = "wrath_of_the_storm"

	// RageCharges is the barbarian's rage uses per long rest.
	// Maximum depends on barbarian level: 2 at level 1-2, 3 at 3-5, 4 at 6-11, 5 at 12-16, 6 at 17-19.
	// At level 20, rage becomes unlimited.
	// Recovered on long rest.
	// Used by: Rage
	RageCharges coreResources.ResourceKey = "rage_charges"

	// Ki is the monk's resource pool, equal to monk level.
	// Recovered on short or long rest.
	// Used by: Flurry of Blows, Patient Defense, Step of the Wind, etc.
	Ki coreResources.ResourceKey = "ki"

	// HitDice is the character's pool of hit dice for short rest healing.
	// Maximum equals character level (sum of all class levels for multiclass).
	// Die size is determined by class (d6 for wizard, d12 for barbarian, etc.).
	// Recovered on long rest: regain half of maximum (minimum 1).
	// Used by: Short rest healing
	HitDice coreResources.ResourceKey = "hit_dice"

	// SecondWind is the fighter's Second Wind feature pool, owned privately by
	// the SecondWind feature object rather than by Character.resources. The
	// feature reports it through its non-mutating Status surface so a projection
	// never has to serialize the feature's persistence JSON to read uses.
	// Recovered on short rest. Used by: Second Wind.
	SecondWind coreResources.ResourceKey = "second_wind"

	// ActionSurge is the fighter's Action Surge feature pool, owned privately by
	// the ActionSurge feature object rather than by Character.resources. The
	// feature reports it through its non-mutating Status surface so a projection
	// never has to serialize the feature's persistence JSON to read uses.
	// Recovered on short rest. Used by: Action Surge.
	ActionSurge coreResources.ResourceKey = "action_surge"

	// Inspiration is the bard's Bardic Inspiration uses, equal to their
	// Charisma modifier and never fewer than one. Recovered on long rest.
	// Used by: Bardic Inspiration.
	Inspiration coreResources.ResourceKey = "inspiration"

	// SpellSlotLevel1 is the pool for 1st-level spell slots. A class's slots
	// at any level come from its classes.SpellProgression; the pool is sized
	// from that table and recovers on a long rest.
	SpellSlotLevel1 coreResources.ResourceKey = "spell_slot_level_1"

	// SpellSlotLevel2 is the pool for 2nd-level spell slots.
	SpellSlotLevel2 coreResources.ResourceKey = "spell_slot_level_2"

	// SpellSlotLevel3 is the pool for 3rd-level spell slots.
	SpellSlotLevel3 coreResources.ResourceKey = "spell_slot_level_3"

	// SpellSlotLevel4 is the pool for 4th-level spell slots.
	SpellSlotLevel4 coreResources.ResourceKey = "spell_slot_level_4"

	// SpellSlotLevel5 is the pool for 5th-level spell slots.
	SpellSlotLevel5 coreResources.ResourceKey = "spell_slot_level_5"

	// SpellSlotLevel6 is the pool for 6th-level spell slots.
	SpellSlotLevel6 coreResources.ResourceKey = "spell_slot_level_6"

	// SpellSlotLevel7 is the pool for 7th-level spell slots.
	SpellSlotLevel7 coreResources.ResourceKey = "spell_slot_level_7"

	// SpellSlotLevel8 is the pool for 8th-level spell slots.
	SpellSlotLevel8 coreResources.ResourceKey = "spell_slot_level_8"

	// SpellSlotLevel9 is the pool for 9th-level spell slots.
	SpellSlotLevel9 coreResources.ResourceKey = "spell_slot_level_9"
)

// MaxSpellLevel is the highest spell level the 2014 class tables reach, and so
// the highest slot pool that exists.
const MaxSpellLevel = 9

// spellSlotKeys is the slot pool for each spell level, indexed by spell level.
// Index 0 is unused: a cantrip costs no slot.
var spellSlotKeys = [MaxSpellLevel + 1]coreResources.ResourceKey{
	1: SpellSlotLevel1,
	2: SpellSlotLevel2,
	3: SpellSlotLevel3,
	4: SpellSlotLevel4,
	5: SpellSlotLevel5,
	6: SpellSlotLevel6,
	7: SpellSlotLevel7,
	8: SpellSlotLevel8,
	9: SpellSlotLevel9,
}

// SpellSlotLevel returns the pool key for a spell level, and whether that spell
// level has one.
//
// A composed key would be one line shorter and would also mint
// "spell_slot_level_0" and "spell_slot_level_12" on request — keys nothing
// grants, nothing recovers and DisplayName does not know. The closed table is
// what makes an out-of-range spell level a refusal instead of a resource.
func SpellSlotLevel(spellLevel int) (coreResources.ResourceKey, bool) {
	if spellLevel < 1 || spellLevel > MaxSpellLevel {
		return "", false
	}
	return spellSlotKeys[spellLevel], true
}

// DisplayName returns the rulebook-owned display name for a resource key and
// whether that key belongs to the closed owner-private status catalog. Unknown
// keys return ("", false); they never fall back to raw persistence bytes and
// therefore cannot become valid-looking status rows by accident.
func DisplayName(key coreResources.ResourceKey) (string, bool) {
	switch key {
	case WrathOfTheStorm:
		return "Wrath of the Storm", true
	case RageCharges:
		return "Rage", true
	case Ki:
		return "Ki", true
	case HitDice:
		return "Hit Dice", true
	case SecondWind:
		return "Second Wind", true
	case ActionSurge:
		return "Action Surge", true
	case Inspiration:
		return "Bardic Inspiration", true
	case SpellSlotLevel1:
		return "1st-level Spell Slots", true
	case SpellSlotLevel2:
		return "2nd-level Spell Slots", true
	case SpellSlotLevel3:
		return "3rd-level Spell Slots", true
	case SpellSlotLevel4:
		return "4th-level Spell Slots", true
	case SpellSlotLevel5:
		return "5th-level Spell Slots", true
	case SpellSlotLevel6:
		return "6th-level Spell Slots", true
	case SpellSlotLevel7:
		return "7th-level Spell Slots", true
	case SpellSlotLevel8:
		return "8th-level Spell Slots", true
	case SpellSlotLevel9:
		return "9th-level Spell Slots", true
	default:
		return "", false
	}
}
