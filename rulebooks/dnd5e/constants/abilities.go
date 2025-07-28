package constants

// Ability represents a D&D 5e ability score
type Ability string

// Ability score constants
const (
	STR Ability = "str"
	DEX Ability = "dex"
	CON Ability = "con"
	INT Ability = "int"
	WIS Ability = "wis"
	CHA Ability = "cha"
)

// Display returns the human-readable name of the ability
func (a Ability) Display() string {
	switch a {
	case STR:
		return "Strength"
	case DEX:
		return "Dexterity"
	case CON:
		return "Constitution"
	case INT:
		return "Intelligence"
	case WIS:
		return "Wisdom"
	case CHA:
		return "Charisma"
	default:
		return string(a)
	}
}

// AllAbilities returns all ability scores in standard order
func AllAbilities() []Ability {
	return []Ability{STR, DEX, CON, INT, WIS, CHA}
}
