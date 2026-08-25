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
)

// DisplayName returns the rulebook-owned display name for a resource key and
// whether that key belongs to the closed owner-private status catalog. Unknown
// keys return ("", false); they never fall back to raw persistence bytes and
// therefore cannot become valid-looking status rows by accident.
func DisplayName(key coreResources.ResourceKey) (string, bool) {
	switch key {
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
	default:
		return "", false
	}
}
