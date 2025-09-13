// Package choices provides character creation choice requirements and validation
package choices

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Requirements represents what choices need to be made
type Requirements struct {
	// Skills that need to be chosen
	Skills           *SkillRequirement   `json:"skills,omitempty"`
	AdditionalSkills []*SkillRequirement `json:"additional_skills,omitempty"` // For subclass-granted skills

	// Equipment choices
	Equipment []*EquipmentRequirement `json:"equipment,omitempty"`

	// Equipment category choices (e.g., "choose 2 martial weapons")
	EquipmentCategories []*EquipmentCategoryRequirement `json:"equipment_categories,omitempty"`

	// Proficiency choices
	Languages []*LanguageRequirement `json:"languages,omitempty"` // Changed to array for multiple language choices
	Tools     *ToolRequirement       `json:"tools,omitempty"`

	// Class-specific choices
	FightingStyle *FightingStyleRequirement `json:"fighting_style,omitempty"`
	Expertise     *ExpertiseRequirement     `json:"expertise,omitempty"`

	// Subclass choice (required at specific levels)
	Subclass *SubclassRequirement `json:"subclass,omitempty"`

	// Spell choices
	Cantrips  *CantripRequirement   `json:"cantrips,omitempty"`
	Spellbook *SpellbookRequirement `json:"spellbook,omitempty"`
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
	ID              OptionID                  `json:"id"`               // Unique identifier for this option
	Items           []EquipmentItem           `json:"items"`            // What you get if you choose this
	Label           string                    `json:"label"`            // e.g., "Chain mail"
	CategoryChoices []EquipmentCategoryChoice `json:"category_choices"` // Category-based selections this option grants
}

// EquipmentItem represents an item in an equipment option
type EquipmentItem struct {
	ID       shared.EquipmentID `json:"id"`       // Equipment ID
	Quantity int                `json:"quantity"` // How many (default 1)
}

// EquipmentCategoryChoice represents a category-based equipment selection within an option
type EquipmentCategoryChoice struct {
	Choose     int                        `json:"choose"`     // How many to choose (e.g., 1 or 2)
	Type       shared.EquipmentType       `json:"type"`       // Equipment type (weapon, armor, etc.)
	Categories []shared.EquipmentCategory `json:"categories"` // Categories to choose from
	Label      string                     `json:"label"`      // e.g., "Choose a martial weapon"
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

// CantripRequirement defines cantrip choice requirements
type CantripRequirement struct {
	ID      ChoiceID       `json:"id"`      // Unique identifier
	Count   int            `json:"count"`   // How many cantrips to choose
	Options []spells.Spell `json:"options"` // Available cantrips
	Label   string         `json:"label"`   // e.g., "Choose 3 cantrips"
}

// SpellbookRequirement defines spellbook choice requirements
type SpellbookRequirement struct {
	ID         ChoiceID       `json:"id"`          // Unique identifier
	Count      int            `json:"count"`       // How many spells to choose
	SpellLevel int            `json:"spell_level"` // Level of spells to choose (1 for 1st level)
	Options    []spells.Spell `json:"options"`     // Available spells
	Label      string         `json:"label"`       // e.g., "Choose 6 1st-level spells for your spellbook"
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
	// Get class-specific requirements
	switch classID {
	case classes.Fighter:
		return getFighterRequirements()
	case classes.Wizard:
		return getWizardRequirements()
	case classes.Rogue:
		return getRogueRequirements()
	case classes.Cleric:
		return getClericRequirements()
	default:
		// For unimplemented classes, return basic skill requirements from class data
		classData := classes.ClassData[classID]
		if classData == nil {
			return &Requirements{}
		}

		reqs := &Requirements{}
		if classData.SkillCount > 0 && len(classData.SkillList) > 0 {
			reqs.Skills = &SkillRequirement{
				ID:      getSkillChoiceID(classID),
				Count:   classData.SkillCount,
				Options: classData.SkillList,
				Label:   fmt.Sprintf("Choose %d skills", classData.SkillCount),
			}
		}
		return reqs
	}
}

func getFighterRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      getSkillChoiceID(classes.Fighter),
			Count:   classes.ClassData[classes.Fighter].SkillCount,
			Options: classes.ClassData[classes.Fighter].SkillList,
			Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Fighter].SkillCount),
		},
		Equipment: getFighterEquipmentRequirements(),
		FightingStyle: &FightingStyleRequirement{
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
		},
	}
}

func getWizardRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      getSkillChoiceID(classes.Wizard),
			Count:   classes.ClassData[classes.Wizard].SkillCount,
			Options: classes.ClassData[classes.Wizard].SkillList,
			Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Wizard].SkillCount),
		},
		Equipment: getWizardEquipmentRequirements(),
		Cantrips: &CantripRequirement{
			ID:    WizardCantrips1,
			Count: 3,
			Options: []spells.Spell{
				// Damage cantrips
				spells.FireBolt,
				spells.RayOfFrost,
				spells.ShockingGrasp,
				spells.AcidSplash,
				spells.PoisonSpray,
				spells.ChillTouch,
				// Utility cantrips
				spells.MageHand,
				spells.MinorIllusion,
				spells.Prestidigitation,
				spells.Light,
			},
			Label: "Choose 3 cantrips",
		},
		Spellbook: &SpellbookRequirement{
			ID:         WizardSpells1,
			Count:      6,
			SpellLevel: 1,
			Options: []spells.Spell{
				// Damage spells
				spells.MagicMissile,
				spells.BurningHands,
				spells.ChromaticOrb,
				spells.Thunderwave,
				spells.IceKnife,
				spells.WitchBolt,
				// Utility spells
				spells.Shield,
				spells.Sleep,
				spells.CharmPerson,
				spells.DetectMagic,
				spells.Identify,
				// Note: This is not the complete wizard spell list
				// In a real implementation, we'd have a comprehensive list
			},
			Label: "Choose 6 1st-level spells for your spellbook",
		},
	}
}

func getRogueRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      getSkillChoiceID(classes.Rogue),
			Count:   classes.ClassData[classes.Rogue].SkillCount,
			Options: classes.ClassData[classes.Rogue].SkillList,
			Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Rogue].SkillCount),
		},
		Equipment: getRogueEquipmentRequirements(),
		Expertise: &ExpertiseRequirement{
			ID:    RogueExpertise1,
			Count: 2,
			Label: "Choose 2 skills or thieves' tools for expertise",
		},
	}
}

func getClericRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      getSkillChoiceID(classes.Cleric),
			Count:   classes.ClassData[classes.Cleric].SkillCount,
			Options: classes.ClassData[classes.Cleric].SkillList,
			Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Cleric].SkillCount),
		},
		Equipment: getClericEquipmentRequirements(),
		Cantrips: &CantripRequirement{
			ID:    ClericCantrips1,
			Count: 3,
			Options: []spells.Spell{
				// Damage cantrips
				spells.SacredFlame,
				spells.TollTheDead,
				spells.WordOfRadiance,
				// Utility cantrips
				spells.Guidance,
				spells.Light,
				spells.Resistance,
				spells.SpareTheDying,
				spells.Thaumaturgy,
			},
			Label: "Choose 3 cantrips",
		},
		// Note: Clerics prepare spells, they don't have a spellbook
		// Domain spells are automatically prepared and don't count against the limit
	}
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
			Languages: []*LanguageRequirement{
				{
					ID:      HalfElfLanguage,
					Count:   1,
					Options: nil, // Any language
					Label:   "Choose 1 language",
				},
			},
		}
	case races.Human:
		return &Requirements{
			Languages: []*LanguageRequirement{
				{
					ID:      HumanLanguage,
					Count:   1,
					Options: nil, // Any language
					Label:   "Choose 1 language",
				},
			},
		}
	case races.HighElf:
		return &Requirements{
			Languages: []*LanguageRequirement{
				{
					ID:      HighElfLanguage,
					Count:   1,
					Options: nil, // Any language
					Label:   "Choose 1 language",
				},
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
					ID:    FighterArmorChainMail,
					Label: "Chain mail",
					Items: []EquipmentItem{
						{ID: armor.ChainMail, Quantity: 1},
					},
				},
				{
					ID:    FighterArmorLeather,
					Label: "Leather armor, longbow, and 20 arrows",
					Items: []EquipmentItem{
						{ID: armor.Leather, Quantity: 1},
						{ID: weapons.Longbow, Quantity: 1},
						{ID: weapons.Arrows20, Quantity: 1},
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
					ID:    FighterWeaponMartialShield,
					Label: "A martial weapon and a shield",
					Items: []EquipmentItem{
						{ID: armor.Shield, Quantity: 1},
					},
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose: 1,
							Type:   shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{
								weapons.CategoryMartialMelee,
								weapons.CategoryMartialRanged,
							},
							Label: "Choose a martial weapon",
						},
					},
				},
				{
					ID:    FighterWeaponTwoMartial,
					Label: "Two martial weapons",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose: 2,
							Type:   shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{
								weapons.CategoryMartialMelee,
								weapons.CategoryMartialRanged,
							},
							Label: "Choose two martial weapons",
						},
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
					ID:    FighterRangedCrossbow,
					Label: "A light crossbow and 20 bolts",
					Items: []EquipmentItem{
						{ID: weapons.LightCrossbow, Quantity: 1},
						{ID: weapons.Bolts20, Quantity: 1},
					},
				},
				{
					ID:    FighterRangedHandaxes,
					Label: "Two handaxes",
					Items: []EquipmentItem{
						{ID: weapons.Handaxe, Quantity: 2},
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
					ID:    FighterPackDungeoneer,
					Label: "Dungeoneer's pack",
					Items: []EquipmentItem{
						{ID: packs.DungeoneerPack, Quantity: 1},
					},
				},
				{
					ID:    FighterPackExplorer,
					Label: "Explorer's pack",
					Items: []EquipmentItem{
						{ID: packs.ExplorerPack, Quantity: 1},
					},
				},
			},
			Label: "Choose your equipment pack",
		},
	}
}

func getRogueEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     RogueWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    RogueWeaponRapier,
					Label: "Rapier",
					Items: []EquipmentItem{
						{ID: weapons.Rapier, Quantity: 1},
					},
				},
				{
					ID:    RogueWeaponShortsword,
					Label: "Shortsword",
					Items: []EquipmentItem{
						{ID: weapons.Shortsword, Quantity: 1},
					},
				},
			},
			Label: "Choose your primary weapon",
		},
		{
			ID:     RogueWeaponsSecondary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    RogueSecondaryShortbow,
					Label: "Shortbow and quiver of 20 arrows",
					Items: []EquipmentItem{
						{ID: weapons.Shortbow, Quantity: 1},
						{ID: weapons.Arrows20, Quantity: 1},
					},
				},
				{
					ID:    RogueSecondaryShortsword,
					Label: "Shortsword",
					Items: []EquipmentItem{
						{ID: weapons.Shortsword, Quantity: 1},
					},
				},
			},
			Label: "Choose your secondary weapon",
		},
		{
			ID:     RoguePack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    RoguePackBurglar,
					Label: "Burglar's pack",
					Items: []EquipmentItem{
						{ID: packs.BurglarPack, Quantity: 1},
					},
				},
				{
					ID:    RoguePackDungeoneer,
					Label: "Dungeoneer's pack",
					Items: []EquipmentItem{
						{ID: packs.DungeoneerPack, Quantity: 1},
					},
				},
				{
					ID:    RoguePackExplorer,
					Label: "Explorer's pack",
					Items: []EquipmentItem{
						{ID: packs.ExplorerPack, Quantity: 1},
					},
				},
			},
			Label: "Choose your equipment pack",
		},
		// Rogues automatically get leather armor, two daggers, and thieves' tools
		// These would be granted automatically, not choices
	}
}

func getWizardEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     WizardWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    WizardWeaponQuarterstaff,
					Label: "Quarterstaff",
					Items: []EquipmentItem{
						{ID: weapons.Quarterstaff, Quantity: 1},
					},
				},
				{
					ID:    WizardWeaponDagger,
					Label: "Dagger",
					Items: []EquipmentItem{
						{ID: weapons.Dagger, Quantity: 1},
					},
				},
			},
			Label: "Choose your weapon",
		},
		{
			ID:     WizardFocus,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    WizardFocusComponent,
					Label: "Component pouch",
					Items: []EquipmentItem{
						{ID: items.ComponentPouch, Quantity: 1},
					},
				},
				{
					ID:    WizardFocusStaff,
					Label: "Arcane focus",
					Items: []EquipmentItem{
						{ID: items.ArcaneFocus, Quantity: 1},
					},
				},
			},
			Label: "Choose your spellcasting focus",
		},
		{
			ID:     WizardPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    WizardPackScholar,
					Label: "Scholar's pack",
					Items: []EquipmentItem{
						{ID: packs.ScholarPack, Quantity: 1},
					},
				},
				{
					ID:    WizardPackExplorer,
					Label: "Explorer's pack",
					Items: []EquipmentItem{
						{ID: packs.ExplorerPack, Quantity: 1},
					},
				},
			},
			Label: "Choose your equipment pack",
		},
	}
}

func getClericEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     ClericWeapons,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    ClericWeaponMace,
					Label: "Mace",
					Items: []EquipmentItem{
						{ID: weapons.Mace, Quantity: 1},
					},
				},
				{
					ID:    ClericWeaponWarhammer,
					Label: "Warhammer (if proficient)",
					Items: []EquipmentItem{
						{ID: weapons.Warhammer, Quantity: 1},
					},
					// Note: War Domain and some others grant martial weapon proficiency
				},
			},
			Label: "Choose your weapon",
		},
		{
			ID:     ClericArmor,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    ClericArmorScale,
					Label: "Scale mail",
					Items: []EquipmentItem{
						{ID: armor.ScaleMail, Quantity: 1},
					},
				},
				{
					ID:    ClericArmorLeather,
					Label: "Leather armor",
					Items: []EquipmentItem{
						{ID: armor.Leather, Quantity: 1},
					},
				},
				{
					ID:    ClericArmorChainMail,
					Label: "Chain mail (if proficient)",
					Items: []EquipmentItem{
						{ID: armor.ChainMail, Quantity: 1},
					},
					// Note: Life, Nature, Tempest, and War domains grant heavy armor proficiency
				},
			},
			Label: "Choose your armor",
		},
		{
			ID:     ClericSecondaryWeapon,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    ClericSecondaryShortbow,
					Label: "Light crossbow and 20 bolts",
					Items: []EquipmentItem{
						{ID: weapons.LightCrossbow, Quantity: 1},
						{ID: weapons.Bolts20, Quantity: 1},
					},
				},
				{
					ID:    ClericSecondarySimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose: 1,
							Type:   shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{
								weapons.CategorySimpleMelee,
								weapons.CategorySimpleRanged,
							},
							Label: "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your secondary weapon",
		},
		{
			ID:     ClericPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    ClericPackPriest,
					Label: "Priest's pack",
					Items: []EquipmentItem{
						{ID: packs.PriestPack, Quantity: 1},
					},
				},
				{
					ID:    ClericPackExplorer,
					Label: "Explorer's pack",
					Items: []EquipmentItem{
						{ID: packs.ExplorerPack, Quantity: 1},
					},
				},
			},
			Label: "Choose your equipment pack",
		},
		{
			ID:     ClericHolySymbol,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    ClericHolyAmulet,
					Label: "Holy symbol",
					Items: []EquipmentItem{
						{ID: items.HolySymbol, Quantity: 1},
					},
				},
			},
			Label: "Choose your holy symbol",
		},
	}
}

// GetClassRequirementsWithSubclass returns requirements modified by the chosen subclass
func GetClassRequirementsWithSubclass(class classes.Class, level int, subclass classes.Subclass) *Requirements {
	// Start with base requirements for the level
	reqs := GetClassRequirementsAtLevel(class, level)
	if reqs == nil {
		return nil
	}

	// Get subclass modifications
	mods := GetSubclassModifications(subclass)

	// Apply the modifications
	ApplySubclassModifications(reqs, mods)

	return reqs
}
