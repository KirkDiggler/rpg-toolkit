package constants

// Class represents a D&D 5e character class
type Class string

// Class constants
const (
	ClassBarbarian Class = "barbarian"
	ClassBard      Class = "bard"
	ClassCleric    Class = "cleric"
	ClassDruid     Class = "druid"
	ClassFighter   Class = "fighter"
	ClassMonk      Class = "monk"
	ClassPaladin   Class = "paladin"
	ClassRanger    Class = "ranger"
	ClassRogue     Class = "rogue"
	ClassSorcerer  Class = "sorcerer"
	ClassWarlock   Class = "warlock"
	ClassWizard    Class = "wizard"
)

// Display returns the human-readable name of the class
func (c Class) Display() string {
	switch c {
	case ClassBarbarian:
		return "Barbarian"
	case ClassBard:
		return "Bard"
	case ClassCleric:
		return "Cleric"
	case ClassDruid:
		return "Druid"
	case ClassFighter:
		return "Fighter"
	case ClassMonk:
		return "Monk"
	case ClassPaladin:
		return "Paladin"
	case ClassRanger:
		return "Ranger"
	case ClassRogue:
		return "Rogue"
	case ClassSorcerer:
		return "Sorcerer"
	case ClassWarlock:
		return "Warlock"
	case ClassWizard:
		return "Wizard"
	default:
		return string(c)
	}
}

// HitDice returns the hit dice size for the class
func (c Class) HitDice() int {
	switch c {
	case ClassBarbarian:
		return 12
	case ClassFighter, ClassPaladin, ClassRanger:
		return 10
	case ClassBard, ClassCleric, ClassDruid, ClassMonk, ClassRogue, ClassWarlock:
		return 8
	case ClassSorcerer, ClassWizard:
		return 6
	default:
		return 0
	}
}

// PrimaryStat returns the primary ability score for the class
func (c Class) PrimaryStat() Ability {
	switch c {
	case ClassBarbarian, ClassFighter:
		return STR
	case ClassRogue, ClassRanger, ClassMonk:
		return DEX
	case ClassWizard:
		return INT
	case ClassCleric, ClassDruid:
		return WIS
	case ClassBard, ClassPaladin, ClassSorcerer, ClassWarlock:
		return CHA
	default:
		return ""
	}
}
