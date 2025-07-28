package shared

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"

// Re-export ability constants from the constants package
// These now use the new short format (str, dex, etc.)
const (
	AbilityStrength     = string(constants.STR) // "str"
	AbilityDexterity    = string(constants.DEX) // "dex"
	AbilityConstitution = string(constants.CON) // "con"
	AbilityIntelligence = string(constants.INT) // "int"
	AbilityWisdom       = string(constants.WIS) // "wis"
	AbilityCharisma     = string(constants.CHA) // "cha"
)
