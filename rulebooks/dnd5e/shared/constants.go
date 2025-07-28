package shared

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"

// Re-export ability constants for backward compatibility
// These map to the lowercase values for compatibility with existing code
const (
	AbilityStrength     = "strength"
	AbilityDexterity    = "dexterity"
	AbilityConstitution = "constitution"
	AbilityIntelligence = "intelligence"
	AbilityWisdom       = "wisdom"
	AbilityCharisma     = "charisma"
)

// AbilityToConstant converts legacy ability strings to typed constants
func AbilityToConstant(ability string) constants.Ability {
	switch ability {
	case AbilityStrength, "str":
		return constants.STR
	case AbilityDexterity, "dex":
		return constants.DEX
	case AbilityConstitution, "con":
		return constants.CON
	case AbilityIntelligence, "int":
		return constants.INT
	case AbilityWisdom, "wis":
		return constants.WIS
	case AbilityCharisma, "cha":
		return constants.CHA
	default:
		return constants.Ability(ability)
	}
}
