// Package abilities provides D&D 5e ability score constants and utilities
package abilities

// Ability represents a D&D 5e ability score
type Ability string

// The six ability scores
const (
	STR Ability = "str"
	DEX Ability = "dex"
	CON Ability = "con"
	INT Ability = "int"
	WIS Ability = "wis"
	CHA Ability = "cha"
)

// All contains all abilities mapped by ID for O(1) lookup
var All = map[string]Ability{
	"str":      STR,
	"dex":      DEX,
	"con":      CON,
	"int":      INT,
	"wis":      WIS,
	"cha":      CHA,
	"strength": STR,     // Allow full names too
	"dexterity": DEX,
	"constitution": CON,
	"intelligence": INT,
	"wisdom": WIS,
	"charisma": CHA,
}

// GetByID returns an ability by its ID (accepts abbreviations and full names)
func GetByID(id string) (Ability, bool) {
	ability, ok := All[id]
	return ability, ok
}

// List returns all abilities in standard order
func List() []Ability {
	return []Ability{STR, DEX, CON, INT, WIS, CHA}
}

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

// Abbreviation returns the three-letter abbreviation
func (a Ability) Abbreviation() string {
	switch a {
	case STR:
		return "STR"
	case DEX:
		return "DEX"
	case CON:
		return "CON"
	case INT:
		return "INT"
	case WIS:
		return "WIS"
	case CHA:
		return "CHA"
	default:
		return string(a)
	}
}