package classes

// ClassRequirements defines what choices a class requires at each level
type ClassRequirements struct {
	ClassName string
	Level1    LevelRequirements
	// Future: Level2, Level3, etc.
}

// LevelRequirements defines required choices for a specific level
type LevelRequirements struct {
	Skills      *ChoiceRequirement
	Cantrips    *ChoiceRequirement
	Spells      *ChoiceRequirement
	Equipment   *ChoiceRequirement
	Instruments *ChoiceRequirement
	Tools       *ChoiceRequirement
	Expertise   *ChoiceRequirement
	// Add more as needed
}

// ChoiceRequirement defines the requirements for a specific choice type
type ChoiceRequirement struct {
	Required      bool
	Count         int
	AllowedValues []string // If restricted to specific options
	AllowAny      bool     // If true, any value is allowed (like Bard skills)
}

// GetRequirements returns the requirements for a specific class
func GetRequirements(classID Class) *ClassRequirements {
	switch classID {
	case Bard:
		return &BardRequirements
	case Fighter:
		return &FighterRequirements
	case Rogue:
		return &RogueRequirements
	// Add more as implemented
	default:
		return nil
	}
}

// BardRequirements defines what a Bard needs at each level
var BardRequirements = ClassRequirements{
	ClassName: "Bard",
	Level1: LevelRequirements{
		Skills: &ChoiceRequirement{
			Required: true,
			Count:    3,
			AllowAny: true, // Bard can choose ANY skills
		},
		Cantrips: &ChoiceRequirement{
			Required: true,
			Count:    2,
		},
		Spells: &ChoiceRequirement{
			Required: true,
			Count:    4,
		},
		Equipment: &ChoiceRequirement{
			Required: true,
			Count:    1, // One equipment pack choice
		},
		Instruments: &ChoiceRequirement{
			Required: true,
			Count:    3, // This is now data, not code!
		},
	},
}

// FighterRequirements defines what a Fighter needs at each level
var FighterRequirements = ClassRequirements{
	ClassName: "Fighter",
	Level1: LevelRequirements{
		Skills: &ChoiceRequirement{
			Required: true,
			Count:    2,
			AllowedValues: []string{
				"acrobatics", "animal-handling", "athletics", "history",
				"insight", "intimidation", "perception", "survival",
			},
		},
		Equipment: &ChoiceRequirement{
			Required: true,
			Count:    1,
		},
		// Fighting style is handled separately as it's a class feature
	},
}

// RogueRequirements defines what a Rogue needs at each level
var RogueRequirements = ClassRequirements{
	ClassName: "Rogue",
	Level1: LevelRequirements{
		Skills: &ChoiceRequirement{
			Required: true,
			Count:    4,
			AllowedValues: []string{
				"acrobatics", "athletics", "deception", "insight", "intimidation",
				"investigation", "perception", "performance", "persuasion",
				"sleight-of-hand", "stealth",
			},
		},
		Expertise: &ChoiceRequirement{
			Required: true,
			Count:    2,
		},
		Equipment: &ChoiceRequirement{
			Required: true,
			Count:    1,
		},
	},
}
