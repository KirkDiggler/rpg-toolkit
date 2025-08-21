package shared

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"

// Size represents creature size categories
type Size string

// Size constants
const (
	SizeTiny       Size = "tiny"
	SizeSmall      Size = "small"
	SizeMedium     Size = "medium"
	SizeLarge      Size = "large"
	SizeHuge       Size = "huge"
	SizeGargantuan Size = "gargantuan"
)

// Display returns the human-readable name of the size
func (s Size) Display() string {
	switch s {
	case SizeTiny:
		return "Tiny"
	case SizeSmall:
		return "Small"
	case SizeMedium:
		return "Medium"
	case SizeLarge:
		return "Large"
	case SizeHuge:
		return "Huge"
	case SizeGargantuan:
		return "Gargantuan"
	default:
		return string(s)
	}
}

// Re-export ability constants from the abilities package
// These now use the new short format (str, dex, etc.)
const (
	AbilityStrength     = string(abilities.STR) // "str"
	AbilityDexterity    = string(abilities.DEX) // "dex"
	AbilityConstitution = string(abilities.CON) // "con"
	AbilityIntelligence = string(abilities.INT) // "int"
	AbilityWisdom       = string(abilities.WIS) // "wis"
	AbilityCharisma     = string(abilities.CHA) // "cha"
)
