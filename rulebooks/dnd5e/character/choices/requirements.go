// Package choices provides character creation choice requirements and validation
package choices

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Requirements represents what choices need to be made
type Requirements struct {
	// Skills that need to be chosen
	Skills *SkillRequirement `json:"skills,omitempty"`

	// Equipment choices
	Equipment []*EquipmentRequirement `json:"equipment,omitempty"`

	// Equipment category choices (e.g., "choose 2 martial weapons")
	EquipmentCategories []*EquipmentCategoryRequirement `json:"equipment_categories,omitempty"`

	// Proficiency choices
	Languages *LanguageRequirement `json:"languages,omitempty"`
	Tools     *ToolRequirement     `json:"tools,omitempty"`

	// Class-specific choices
	FightingStyle *FightingStyleRequirement `json:"fighting_style,omitempty"`
	Expertise     *ExpertiseRequirement     `json:"expertise,omitempty"`

	// Subclass choice (required at specific levels)
	Subclass *SubclassRequirement `json:"subclass,omitempty"`
}

// SkillRequirement defines skill choice requirements
type SkillRequirement struct {
	ID      ChoiceID       `json:"id"` // Unique identifier
	Count   int            `json:"count"`
	Options []skills.Skill `json:"options,omitempty"` // nil means any skill
	Label   string         `json:"label"`             // e.g., "Choose 2 skills"
}

// EquipmentRequirement defines equipment choice requirements
type EquipmentRequirement struct {
	ID      ChoiceID          `json:"id"`     // Unique identifier
	Choose  int               `json:"choose"` // How many options to pick (usually 1)
	Options []EquipmentOption `json:"options"`
	Label   string            `json:"label"` // e.g., "Choose your armor"
}

// EquipmentOption represents one equipment choice option
type EquipmentOption struct {
	ID    string          `json:"id"`    // Unique identifier for this option
	Items []EquipmentItem `json:"items"` // What you get if you choose this
	Label string          `json:"label"` // e.g., "Chain mail"
}

// EquipmentItem represents an item in an equipment option
type EquipmentItem struct {
	ID       shared.EquipmentID `json:"id"`       // Equipment ID
	Quantity int                `json:"quantity"` // How many (default 1)
}

// EquipmentCategoryRequirement defines equipment choices from categories (e.g., "choose 2 martial weapons")
type EquipmentCategoryRequirement struct {
	ID         ChoiceID                   `json:"id"`         // Unique identifier
	Choose     int                        `json:"choose"`     // How many to choose
	Type       shared.EquipmentType       `json:"type"`       // Equipment type (weapon, armor, etc.)
	Categories []shared.EquipmentCategory `json:"categories"` // Categories to choose from
	Label      string                     `json:"label"`      // e.g., "Choose 2 martial weapons"
}

// LanguageRequirement defines language choice requirements
type LanguageRequirement struct {
	ID      ChoiceID             `json:"id"` // Unique identifier
	Count   int                  `json:"count"`
	Options []languages.Language `json:"options,omitempty"` // nil means any language
	Label   string               `json:"label"`
}

// ToolRequirement defines tool proficiency choice requirements
type ToolRequirement struct {
	ID      ChoiceID `json:"id"` // Unique identifier
	Count   int      `json:"count"`
	Options []string `json:"options"` // Tool IDs
	Label   string   `json:"label"`
}

// FightingStyleRequirement defines fighting style choice requirements
type FightingStyleRequirement struct {
	ID      ChoiceID                       `json:"id"`      // Unique identifier
	Options []fightingstyles.FightingStyle `json:"options"` // Fighting style constants
	Label   string                         `json:"label"`
}

// ExpertiseRequirement defines expertise choice requirements
type ExpertiseRequirement struct {
	ID    ChoiceID `json:"id"` // Unique identifier
	Count int      `json:"count"`
	Label string   `json:"label"` // e.g., "Choose 2 skills or thieves' tools for expertise"
}

// SubclassRequirement defines subclass choice requirements
type SubclassRequirement struct {
	ID      ChoiceID           `json:"id"`      // Unique identifier
	Options []classes.Subclass `json:"options"` // Available subclasses
	Label   string             `json:"label"`   // e.g., "Choose your Martial Archetype"
}

// GetClassRequirements returns the requirements for a specific class at level 1
func GetClassRequirements(classID classes.Class) *Requirements {
	return GetClassRequirementsAtLevel(classID, 1)
}

// GetClassRequirementsAtLevel returns the requirements for a specific class at a given level
func GetClassRequirementsAtLevel(classID classes.Class, level int) *Requirements {
	reqs := getBaseClassRequirements(classID)

	// Add subclass requirement if needed at this level
	classData := classes.ClassData[classID]
	if classData != nil && classData.SubclassLevel > 0 && level >= classData.SubclassLevel {
		reqs.Subclass = &SubclassRequirement{
			ID:      ChoiceID(classData.SubclassChoiceID),
			Options: classData.Subclasses,
			Label:   classData.SubclassLabel,
		}
	}

	return reqs
}

// getBaseClassRequirements returns the base requirements for a class (without subclass)
func getBaseClassRequirements(classID classes.Class) *Requirements {
	classData := classes.ClassData[classID]
	if classData == nil {
		return &Requirements{}
	}

	reqs := &Requirements{}

	// Add skill requirements if the class has skill choices
	if classData.SkillCount > 0 && len(classData.SkillList) > 0 {
		reqs.Skills = &SkillRequirement{
			ID:      getSkillChoiceID(classID),
			Count:   classData.SkillCount,
			Options: classData.SkillList,
			Label:   fmt.Sprintf("Choose %d skills", classData.SkillCount),
		}
	}

	// Add class-specific requirements
	switch classID {
	case classes.Fighter:
		reqs.FightingStyle = &FightingStyleRequirement{
			ID: FighterFightingStyle,
			Options: []fightingstyles.FightingStyle{
				fightingstyles.Archery,
				fightingstyles.Defense,
				fightingstyles.Dueling,
				fightingstyles.GreatWeaponFighting,
				fightingstyles.Protection,
				fightingstyles.TwoWeaponFighting,
			},
			Label: "Choose a fighting style",
		}
		reqs.Equipment = getFighterEquipmentRequirements()
		reqs.EquipmentCategories = getFighterEquipmentCategoryRequirements()
	case classes.Rogue:
		reqs.Equipment = getRogueEquipmentRequirements()
		reqs.Expertise = &ExpertiseRequirement{
			ID:    RogueExpertise1,
			Count: 2,
			Label: "Choose 2 skills or thieves' tools for expertise",
		}
	case classes.Wizard:
		reqs.Equipment = getWizardEquipmentRequirements()
	case classes.Cleric:
		reqs.Equipment = getClericEquipmentRequirements()
	default:
		// Add equipment requirements for other classes when implemented
	}

	return reqs
}

// getSkillChoiceID returns the appropriate ChoiceID for a class's skill selection
func getSkillChoiceID(classID classes.Class) ChoiceID {
	switch classID {
	case classes.Fighter:
		return FighterSkills
	case classes.Rogue:
		return RogueSkills
	case classes.Wizard:
		return WizardSkills
	case classes.Cleric:
		return ClericSkills
	case classes.Barbarian:
		return BarbarianSkills
	case classes.Bard:
		return BardSkills
	case classes.Druid:
		return DruidSkills
	case classes.Monk:
		return MonkSkills
	case classes.Paladin:
		return PaladinSkills
	case classes.Ranger:
		return RangerSkills
	case classes.Sorcerer:
		return SorcererSkills
	case classes.Warlock:
		return WarlockSkills
	default:
		return ChoiceID(fmt.Sprintf("%s-skills", strings.ToLower(string(classID))))
	}
}

// GetRaceRequirements returns the requirements for a specific race
func GetRaceRequirements(raceID races.Race) *Requirements {
	switch raceID {
	case races.HalfElf:
		return &Requirements{
			Skills: &SkillRequirement{
				ID:      HalfElfSkills,
				Count:   2,
				Options: nil, // Any skills
				Label:   "Choose 2 skills",
			},
			Languages: &LanguageRequirement{
				ID:      HalfElfLanguage,
				Count:   1,
				Options: nil, // Any language
				Label:   "Choose 1 language",
			},
		}
	case races.Human:
		return &Requirements{
			Languages: &LanguageRequirement{
				ID:      HumanLanguage,
				Count:   1,
				Options: nil, // Any language
				Label:   "Choose 1 language",
			},
		}
	case races.HighElf:
		return &Requirements{
			Languages: &LanguageRequirement{
				ID:      HighElfLanguage,
				Count:   1,
				Options: nil, // Any language
				Label:   "Choose 1 language",
			},
		}
	default:
		return &Requirements{}
	}
}

// Helper functions for equipment requirements
func getFighterEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     FighterArmor,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    "fighter-armor-a",
					Label: "Chain mail",
					Items: []EquipmentItem{
						{ID: "chain-mail", Quantity: 1},
					},
				},
				{
					ID:    "fighter-armor-b",
					Label: "Leather armor, longbow, and 20 arrows",
					Items: []EquipmentItem{
						{ID: "leather", Quantity: 1},
						{ID: "longbow", Quantity: 1},
						{ID: "arrows-20", Quantity: 1},
					},
				},
			},
			Label: "Choose your armor",
		},
		{
			ID:     FighterWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    "fighter-weapon-a",
					Label: "A martial weapon and a shield",
					Items: []EquipmentItem{
						// Martial weapon will be chosen via category requirement
						{ID: "shield", Quantity: 1},
					},
				},
				{
					ID:    "fighter-weapon-b",
					Label: "Two martial weapons",
					Items: []EquipmentItem{
						// Both martial weapons will be chosen via category requirements
					},
				},
			},
			Label: "Choose your primary weapons",
		},
		{
			ID:     FighterWeaponsSecondary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    "fighter-ranged-a",
					Label: "A light crossbow and 20 bolts",
					Items: []EquipmentItem{
						{ID: "light-crossbow", Quantity: 1},
						{ID: "bolts-20", Quantity: 1},
					},
				},
				{
					ID:    "fighter-ranged-b",
					Label: "Two handaxes",
					Items: []EquipmentItem{
						{ID: "handaxe", Quantity: 2},
					},
				},
			},
			Label: "Choose your secondary weapons",
		},
		{
			ID:     FighterPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    "fighter-pack-a",
					Label: "Dungeoneer's pack",
					Items: []EquipmentItem{
						{ID: "dungeoneer-pack", Quantity: 1},
					},
				},
				{
					ID:    "fighter-pack-b",
					Label: "Explorer's pack",
					Items: []EquipmentItem{
						{ID: "explorer-pack", Quantity: 1},
					},
				},
			},
			Label: "Choose your equipment pack",
		},
	}
}

func getRogueEquipmentRequirements() []*EquipmentRequirement {
	// Placeholder for rogue equipment
	return []*EquipmentRequirement{}
}

func getWizardEquipmentRequirements() []*EquipmentRequirement {
	// Placeholder for wizard equipment
	return []*EquipmentRequirement{}
}

func getClericEquipmentRequirements() []*EquipmentRequirement {
	// Placeholder for cleric equipment
	return []*EquipmentRequirement{}
}

// getFighterEquipmentCategoryRequirements returns category-based equipment choices for fighters
func getFighterEquipmentCategoryRequirements() []*EquipmentCategoryRequirement {
	// Fighter can choose martial weapons as part of their primary weapon selection
	// This handles the "any martial weapon" part of the choice
	return []*EquipmentCategoryRequirement{
		{
			ID:     FighterMartialWeapon1,
			Choose: 1,
			Type:   shared.EquipmentTypeWeapon,
			Categories: []shared.EquipmentCategory{
				weapons.CategoryMartialMelee,
				weapons.CategoryMartialRanged,
			},
			Label: "Choose your first martial weapon",
		},
		// Note: Second martial weapon choice would be conditional based on
		// whether they chose option B (two martial weapons)
		// This might need to be handled dynamically based on the primary weapon choice
	}
}
