// Package choices provides character creation choice requirements and validation
package choices

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
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
	ID       shared.EquipmentID         `json:"id"`               // Equipment ID
	Quantity int                        `json:"quantity"`         // How many (default 1)
	Detail   *equipment.EquipmentDetail `json:"detail,omitempty"` // Resolved equipment stats
}

// enrichEquipmentRequirements populates the Detail field for each equipment item
// across all options in the given requirements using the equipment registry.
func enrichEquipmentRequirements(reqs []*EquipmentRequirement) []*EquipmentRequirement {
	for _, req := range reqs {
		for i := range req.Options {
			for j := range req.Options[i].Items {
				req.Options[i].Items[j].Detail = equipment.ResolveEquipmentDetail(req.Options[i].Items[j].ID)
			}
			for j := range req.Options[i].CategoryChoices {
				choice := &req.Options[i].CategoryChoices[j]
				eligible, err := EligibleEquipment(choice.Type, choice.Categories)
				if err != nil {
					continue
				}
				choice.Options = make([]EquipmentItem, 0, len(eligible))
				for _, item := range eligible {
					id := shared.EquipmentID(item.EquipmentID())
					choice.Options = append(choice.Options, EquipmentItem{
						ID:       id,
						Quantity: 1,
						Detail:   equipment.ResolveEquipmentDetail(id),
					})
				}
			}
		}
	}
	return reqs
}

// EquipmentCategoryChoice represents a category-based equipment selection within an option
type EquipmentCategoryChoice struct {
	Choose     int                        `json:"choose"`            // How many to choose (e.g., 1 or 2)
	Type       shared.EquipmentType       `json:"type"`              // Equipment type (weapon, armor, etc.)
	Categories []shared.EquipmentCategory `json:"categories"`        // Categories to choose from
	Label      string                     `json:"label"`             // e.g., "Choose a martial weapon"
	Options    []EquipmentItem            `json:"options,omitempty"` // Concrete, validator-eligible choices
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

// GetClassRequirements returns what a class asks for at level 1 — the choices
// character creation puts in front of a player.
//
// This is creation's entry point and its meaning has not changed: level 1's
// authored row, the spell and cantrip questions derived from the class's
// progression table, and the subclass for the one class that picks it at 1.
func GetClassRequirements(classID classes.Class) *Requirements {
	return GetClassRequirementsGainedAtLevel(classID, 1)
}

// classLevelRequirements is every class's authored requirement table.
//
// Rows are built fresh on each call rather than shared from a package
// variable: callers receive a *Requirements and write to it (the subclass
// requirement is folded in that way), and a shared row would let one caller
// edit what the next one reads.
func classLevelRequirements(classID classes.Class) []LevelRequirements {
	switch classID {
	case classes.Fighter:
		return fighterLevelRequirements()
	case classes.Barbarian:
		return barbarianLevelRequirements()
	case classes.Wizard:
		return wizardLevelRequirements()
	case classes.Rogue:
		return rogueLevelRequirements()
	case classes.Cleric:
		return clericLevelRequirements()
	case classes.Bard:
		return bardLevelRequirements()
	case classes.Druid:
		return druidLevelRequirements()
	case classes.Monk:
		return monkLevelRequirements()
	case classes.Paladin:
		return paladinLevelRequirements()
	case classes.Ranger:
		return rangerLevelRequirements()
	case classes.Sorcerer:
		return sorcererLevelRequirements()
	case classes.Warlock:
		return warlockLevelRequirements()
	default:
		// A class with no table of its own is still asked for the skills its
		// class data names, at level 1.
		classData := classes.ClassData[classID]
		if classData == nil || classData.SkillCount <= 0 || len(classData.SkillList) == 0 {
			return nil
		}
		return []LevelRequirements{{
			Level: 1,
			Requirements: Requirements{
				Skills: &SkillRequirement{
					ID:      getSkillChoiceID(classID),
					Count:   classData.SkillCount,
					Options: classData.SkillList,
					Label:   fmt.Sprintf("Choose %d skills", classData.SkillCount),
				},
			},
		}}
	}
}

// fighterLevelRequirements is what a fighter is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func fighterLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      getSkillChoiceID(classes.Fighter),
				Count:   classes.ClassData[classes.Fighter].SkillCount,
				Options: classes.ClassData[classes.Fighter].SkillList,
				Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Fighter].SkillCount),
			},
			Equipment: enrichEquipmentRequirements(getFighterEquipmentRequirements()),
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
		},
	}}
}

// barbarianLevelRequirements is what a barbarian is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func barbarianLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      getSkillChoiceID(classes.Barbarian),
				Count:   classes.ClassData[classes.Barbarian].SkillCount,
				Options: classes.ClassData[classes.Barbarian].SkillList,
				Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Barbarian].SkillCount),
			},
			Equipment: enrichEquipmentRequirements(getBarbarianEquipmentRequirements()),
			// No subclass at level 1 (Path chosen at level 3)
			// No spells or cantrips
		},
	}}
}

// wizardLevelRequirements is what a wizard is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func wizardLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      getSkillChoiceID(classes.Wizard),
				Count:   classes.ClassData[classes.Wizard].SkillCount,
				Options: classes.ClassData[classes.Wizard].SkillList,
				Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Wizard].SkillCount),
			},
			Equipment: enrichEquipmentRequirements(getWizardEquipmentRequirements()),
		},
	}}
}

// rogueLevelRequirements is what a rogue is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func rogueLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      getSkillChoiceID(classes.Rogue),
				Count:   classes.ClassData[classes.Rogue].SkillCount,
				Options: classes.ClassData[classes.Rogue].SkillList,
				Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Rogue].SkillCount),
			},
			Equipment: enrichEquipmentRequirements(getRogueEquipmentRequirements()),
			Expertise: &ExpertiseRequirement{
				ID:    RogueExpertise1,
				Count: 2,
				Label: "Choose 2 skills or thieves' tools for expertise",
			},
		},
	}}
}

// getBardRequirements returns requirements for Bard class
// bardLevelRequirements is what a bard is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func bardLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:    BardSkills,
				Count: 3,
				// ENUMERATED, NOT NIL. A bard chooses any three skills, and this
				// used to say so by leaving Options empty — a sentinel with two
				// readings. Validation reads empty as "no list to check against"
				// and lets anything through; every consumer that BUILDS a choice
				// out of a requirement reads empty as "nothing to offer" and
				// builds none, so the bard was offered no skill choice at all.
				// "Any" is not a concept this stack needs for one class, so the
				// eighteen are written out and both readings agree.
				Options: []skills.Skill{
					skills.Acrobatics,
					skills.AnimalHandling,
					skills.Arcana,
					skills.Athletics,
					skills.Deception,
					skills.History,
					skills.Insight,
					skills.Intimidation,
					skills.Investigation,
					skills.Medicine,
					skills.Nature,
					skills.Perception,
					skills.Performance,
					skills.Persuasion,
					skills.Religion,
					skills.SleightOfHand,
					skills.Stealth,
					skills.Survival,
				},
				Label: "Choose 3 skills",
			},
			Equipment: enrichEquipmentRequirements(getBardEquipmentRequirements()),
			Tools: &ToolRequirement{
				ID:    BardInstruments,
				Count: 3,
				// THE SHARED LIST, not a second copy of it. These ten were written
				// out here as bare strings while musicalInstrumentToolOptions()
				// already built the same ten from the proficiencies constants for
				// the Outlander background — two lists of one thing, and only one
				// of them tied to the constants.
				//
				// The drift is silent in the worst direction. A consumer maps each
				// option to its own vocabulary and DROPS what it cannot map; drop
				// them all and the choice disappears rather than erroring, so a
				// bard would simply have no instruments to pick and nothing would
				// say why. (The monk's requirement still hand-writes the same ten
				// after a list of artisan's tools; it needs the two halves
				// concatenated and is left for whoever splits that.)
				Options: musicalInstrumentToolOptions(),
				Label:   "Choose 3 musical instruments",
			},

			// Expertise is bard level 3 in 2014 and 2 in 2024; we take 2014
			// (design R2.2), which makes it a row of its own alongside the
			// Bard College — and a subclass is the one requirement the engine
			// can pose and cannot receive (rpg-toolkit#1767).
		},
	}}
}

// getDruidRequirements returns requirements for Druid class
// druidLevelRequirements is what a druid is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func druidLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      DruidSkills,
				Count:   2,
				Options: classes.ClassData[classes.Druid].SkillList,
				Label:   "Choose 2 skills",
			},
			Equipment: enrichEquipmentRequirements(getDruidEquipmentRequirements()),
			// Note: Druids prepare spells, they don't have a spellbook
			// They prepare spells = Wisdom modifier + druid level (minimum 1)
		},
	}}
}

// getMonkRequirements returns requirements for Monk class
// monkLevelRequirements is what a monk is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func monkLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      MonkSkills,
				Count:   2,
				Options: classes.ClassData[classes.Monk].SkillList,
				Label:   "Choose 2 skills",
			},
			Equipment: enrichEquipmentRequirements(getMonkEquipmentRequirements()),
			Tools: &ToolRequirement{
				ID:    MonkTools,
				Count: 1,
				Options: []shared.SelectionID{
					// Artisan's tools
					shared.SelectionID("alchemist-supplies"),
					shared.SelectionID("brewer-supplies"),
					shared.SelectionID("calligrapher-supplies"),
					shared.SelectionID("carpenter-tools"),
					shared.SelectionID("cartographer-tools"),
					shared.SelectionID("cobbler-tools"),
					shared.SelectionID("cook-utensils"),
					shared.SelectionID("glassblower-tools"),
					shared.SelectionID("jeweler-tools"),
					shared.SelectionID("leatherworker-tools"),
					shared.SelectionID("mason-tools"),
					shared.SelectionID("painter-supplies"),
					shared.SelectionID("potter-tools"),
					shared.SelectionID("smith-tools"),
					shared.SelectionID("tinker-tools"),
					shared.SelectionID("weaver-tools"),
					shared.SelectionID("woodcarver-tools"),
					shared.SelectionID("disguise-kit"),
					shared.SelectionID("forgery-kit"),
					// Musical instruments
					shared.SelectionID("bagpipes"),
					shared.SelectionID("drum"),
					shared.SelectionID("dulcimer"),
					shared.SelectionID("flute"),
					shared.SelectionID("lute"),
					shared.SelectionID("lyre"),
					shared.SelectionID("horn"),
					shared.SelectionID("pan-flute"),
					shared.SelectionID("shawm"),
					shared.SelectionID("viol"),
				},
				Label: "Choose 1 artisan's tools or musical instrument",
			},
		},
	}}
}

// getPaladinRequirements returns requirements for Paladin class
// paladinLevelRequirements is what a paladin is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func paladinLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      PaladinSkills,
				Count:   2,
				Options: classes.ClassData[classes.Paladin].SkillList,
				Label:   "Choose 2 skills",
			},
			Equipment: enrichEquipmentRequirements(getPaladinEquipmentRequirements()),
			// Note: Paladins get spells at level 2, not level 1
			// Fighting style comes at level 2 for Paladins
		},
	}}
}

// getRangerRequirements returns requirements for Ranger class
// rangerLevelRequirements is what a ranger is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func rangerLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      RangerSkills,
				Count:   3,
				Options: classes.ClassData[classes.Ranger].SkillList,
				Label:   "Choose 3 skills",
			},
			Equipment: enrichEquipmentRequirements(getRangerEquipmentRequirements()),
			FightingStyle: &FightingStyleRequirement{
				ID: RangerFightingStyle,
				Options: []fightingstyles.FightingStyle{
					fightingstyles.Archery,
					fightingstyles.Defense,
					fightingstyles.Dueling,
					fightingstyles.TwoWeaponFighting,
				},
				Label: "Choose a fighting style",
			},
			// Note: Rangers get spells at level 2, not level 1
		},
	}}
}

// getSorcererRequirements returns requirements for Sorcerer class
// sorcererLevelRequirements is what a sorcerer is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func sorcererLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      SorcererSkills,
				Count:   2,
				Options: classes.ClassData[classes.Sorcerer].SkillList,
				Label:   "Choose 2 skills",
			},
			Equipment: enrichEquipmentRequirements(getSorcererEquipmentRequirements()),
		},
	}}
}

// getWarlockRequirements returns requirements for Warlock class
// warlockLevelRequirements is what a warlock is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func warlockLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      WarlockSkills,
				Count:   2,
				Options: classes.ClassData[classes.Warlock].SkillList,
				Label:   "Choose 2 skills",
			},
			Equipment: enrichEquipmentRequirements(getWarlockEquipmentRequirements()),
		},
	}}
}

// clericLevelRequirements is what a cleric is asked at each of its class
// levels. Level 1 is the class as creation has always built it.
func clericLevelRequirements() []LevelRequirements {
	return []LevelRequirements{{
		Level: 1,
		Requirements: Requirements{
			Skills: &SkillRequirement{
				ID:      getSkillChoiceID(classes.Cleric),
				Count:   classes.ClassData[classes.Cleric].SkillCount,
				Options: classes.ClassData[classes.Cleric].SkillList,
				Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Cleric].SkillCount),
			},
			Equipment: enrichEquipmentRequirements(getClericEquipmentRequirements()),
		},
	}}
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
		return ChoiceID(fmt.Sprintf("%s-skills", strings.ToLower(classID)))
	}
}

// GetRaceRequirements returns the requirements for a specific race
func GetRaceRequirements(raceID races.Race) *Requirements {
	switch raceID {
	case races.Dwarf:
		return &Requirements{
			Tools: &ToolRequirement{
				ID:    DwarfToolProficiency,
				Count: 1,
				Options: []shared.SelectionID{
					shared.SelectionID("smith-tools"),
					shared.SelectionID("brewer-supplies"),
					shared.SelectionID("mason-tools"),
				},
				Label: "Choose 1 artisan's tools",
			},
		}
	case races.Elf:
		// Base elf has no choices, but gets Perception proficiency automatically
		return &Requirements{}
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
	case races.Halfling:
		// Base halfling has no choices
		return &Requirements{}
	case races.Human:
		return &Requirements{
			Languages: []*LanguageRequirement{
				{
					ID:    HumanLanguage,
					Count: 1,
					Options: []languages.Language{
						languages.Dwarvish,
						languages.Elvish,
						languages.Giant,
						languages.Gnomish,
						languages.Goblin,
						languages.Halfling,
						languages.Orc,
						languages.Abyssal,
						languages.Celestial,
						languages.Draconic,
						languages.DeepSpeech,
						languages.Infernal,
						languages.Primordial,
						languages.Sylvan,
						languages.Undercommon,
					},
					Label: "Choose 1 language",
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

// GetBackgroundRequirements returns the real player choices a background
// requires — the seven backgrounds flagged by "// Note: ... is a choice"
// comments in backgrounds/grants.go, whose fixed (non-choice) grants are
// already wired via backgrounds.GetGrants (rpg-toolkit#1554).
//
// Outlander, Noble/Knight, and Criminal/Spy are proficiency-only: a Tools
// requirement, same shape as race's Dwarf choice — no physical item
// appears in that background's equipment list for the instrument/gaming
// set.
//
// Entertainer, Folk Hero, and Guild Artisan/Merchant grant one physical
// item (an Equipment requirement, category-based, same shape as Bard's
// BardInstrument) — no separate Tools requirement for these three. The
// tied tool proficiency is derived from the selected item elsewhere
// (character/draft.go's compileProficiencies), not asked as a second
// choice. This is a design default, not a cited 2014 RAW rule: the PHB
// states the equipment grant ("a musical instrument, one of your choice")
// and the tool-proficiency grant ("one type of musical instrument")
// separately, with nothing in the 2014 text linking them — the 2024 PHB
// revision adds an explicit "(same as above)" to the equipment line for
// this exact case, real evidence the 2014 text left it ambiguous rather
// than settled. Deriving one from the other is the sensible reading (why
// carry an instrument you have no training with), and it guarantees the
// two can't disagree, but a future reader should know it's a choice this
// codebase made, not a rule it found.
//
// Soldier is the one case needing both, independently: its physical-item
// choice (bone dice or a deck of cards, 2 fixed options) and its
// proficiency choice (any of 4 gaming-set types) are different selection
// spaces by RAW — you can carry dice but be proficient with a different
// gaming set.
//
// Charlatan is a plain 4-named-option Equipment requirement with no tied
// proficiency (its fixed disguise-kit proficiency is unrelated) — reduced
// to 3 options here: "ten stopped bottles filled with colored liquid" has
// no official PHB/SRD price and no catalog entry, consistent with every
// other unpriced narrative item this wave leaves unmodeled.
func GetBackgroundRequirements(bg backgrounds.Background) *Requirements {
	switch bg {
	case backgrounds.Outlander:
		return &Requirements{
			Tools: &ToolRequirement{
				ID:      OutlanderInstrument,
				Count:   1,
				Options: musicalInstrumentToolOptions(),
				Label:   "Choose a musical instrument proficiency",
			},
		}

	case backgrounds.Noble, backgrounds.Knight:
		return &Requirements{
			Tools: &ToolRequirement{
				ID:      NobleGamingSet,
				Count:   1,
				Options: gamingSetToolOptions(),
				Label:   "Choose a gaming set proficiency",
			},
		}

	case backgrounds.Criminal, backgrounds.Spy:
		return &Requirements{
			Tools: &ToolRequirement{
				ID:      CriminalGamingSet,
				Count:   1,
				Options: gamingSetToolOptions(),
				Label:   "Choose a gaming set proficiency",
			},
		}

	case backgrounds.Entertainer:
		return &Requirements{
			Equipment: enrichEquipmentRequirements([]*EquipmentRequirement{
				musicalInstrumentEquipmentRequirement(EntertainerInstrument, EntertainerInstrumentChoice),
			}),
		}

	case backgrounds.FolkHero:
		return &Requirements{
			Equipment: enrichEquipmentRequirements([]*EquipmentRequirement{
				artisanToolsEquipmentRequirement(FolkHeroArtisanTools, FolkHeroToolsChoice),
			}),
		}

	case backgrounds.GuildArtisan, backgrounds.GuildMerchant:
		return &Requirements{
			Equipment: enrichEquipmentRequirements([]*EquipmentRequirement{
				artisanToolsEquipmentRequirement(GuildArtisanTools, GuildArtisanToolsChoice),
			}),
		}

	case backgrounds.Soldier:
		return &Requirements{
			Equipment: enrichEquipmentRequirements([]*EquipmentRequirement{
				{
					ID:     SoldierGamingSetItem,
					Choose: 1,
					Options: []EquipmentOption{
						{
							ID:    SoldierGamingSetDice,
							Label: "Set of bone dice",
							Items: []EquipmentItem{{ID: tools.DiceSet, Quantity: 1}},
						},
						{
							ID:    SoldierGamingSetCards,
							Label: "Deck of playing cards",
							Items: []EquipmentItem{{ID: tools.PlayingCardSet, Quantity: 1}},
						},
					},
					Label: "Choose bone dice or a deck of cards",
				},
			}),
			Tools: &ToolRequirement{
				ID:      SoldierGamingSetProficiency,
				Count:   1,
				Options: gamingSetToolOptions(),
				Label:   "Choose a gaming set proficiency",
			},
		}

	case backgrounds.Charlatan:
		return &Requirements{
			Equipment: enrichEquipmentRequirements([]*EquipmentRequirement{
				{
					ID:     CharlatanToolsOfTheCon,
					Choose: 1,
					Options: []EquipmentOption{
						{
							ID:    CharlatanConDice,
							Label: "Set of weighted dice",
							Items: []EquipmentItem{{ID: tools.DiceSet, Quantity: 1}},
						},
						{
							ID:    CharlatanConCards,
							Label: "Deck of marked cards",
							Items: []EquipmentItem{{ID: tools.PlayingCardSet, Quantity: 1}},
						},
						{
							ID:    CharlatanConSignetRing,
							Label: "Signet ring of an imaginary duke",
							Items: []EquipmentItem{{ID: items.SignetRing, Quantity: 1}},
						},
					},
					Label: "Choose your tools of the con",
				},
			}),
		}

	default:
		return &Requirements{}
	}
}

// musicalInstrumentToolOptions returns every musical instrument as a
// ToolRequirement option — the proficiency-only shape (Outlander).
func musicalInstrumentToolOptions() []shared.SelectionID {
	return []shared.SelectionID{
		shared.SelectionID(proficiencies.ToolBagpipes),
		shared.SelectionID(proficiencies.ToolDrum),
		shared.SelectionID(proficiencies.ToolDulcimer),
		shared.SelectionID(proficiencies.ToolFlute),
		shared.SelectionID(proficiencies.ToolLute),
		shared.SelectionID(proficiencies.ToolLyre),
		shared.SelectionID(proficiencies.ToolHorn),
		shared.SelectionID(proficiencies.ToolPanFlute),
		shared.SelectionID(proficiencies.ToolShawm),
		shared.SelectionID(proficiencies.ToolViol),
	}
}

// gamingSetToolOptions returns every gaming set type as a ToolRequirement
// option — the proficiency-only shape (Noble/Knight, Criminal/Spy,
// Soldier's proficiency choice).
func gamingSetToolOptions() []shared.SelectionID {
	return []shared.SelectionID{
		shared.SelectionID(proficiencies.ToolDiceSet),
		shared.SelectionID(proficiencies.ToolPlayingCardSet),
		shared.SelectionID(proficiencies.ToolDragonchessSet),
		shared.SelectionID(proficiencies.ToolThreeDragonAnte),
	}
}

// musicalInstrumentEquipmentRequirement builds a single-option, category-
// based Equipment requirement for "a musical instrument, one of your
// choice" — same shape as Bard's BardInstrument, minus the named Lute
// alternative (no PHB text singles out one instrument as the default
// here).
func musicalInstrumentEquipmentRequirement(reqID ChoiceID, optionID OptionID) *EquipmentRequirement {
	return &EquipmentRequirement{
		ID:     reqID,
		Choose: 1,
		Options: []EquipmentOption{
			{
				ID:    optionID,
				Label: "A musical instrument of your choice",
				CategoryChoices: []EquipmentCategoryChoice{
					{
						Choose:     1,
						Type:       shared.EquipmentTypeTool,
						Categories: []shared.EquipmentCategory{equipment.CategoryMusicalInstruments},
						Label:      "Choose a musical instrument",
					},
				},
			},
		},
		Label: "Choose your musical instrument",
	}
}

// artisanToolsEquipmentRequirement builds a single-option, category-based
// Equipment requirement for "a set of artisan's tools, one of your
// choice" (Folk Hero, Guild Artisan/Merchant).
func artisanToolsEquipmentRequirement(reqID ChoiceID, optionID OptionID) *EquipmentRequirement {
	return &EquipmentRequirement{
		ID:     reqID,
		Choose: 1,
		Options: []EquipmentOption{
			{
				ID:    optionID,
				Label: "A set of artisan's tools of your choice",
				CategoryChoices: []EquipmentCategoryChoice{
					{
						Choose:     1,
						Type:       shared.EquipmentTypeTool,
						Categories: []shared.EquipmentCategory{equipment.CategoryArtisanTools},
						Label:      "Choose a set of artisan's tools",
					},
				},
			},
		},
		Label: "Choose your artisan's tools",
	}
}

func getBardEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     BardWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    BardWeaponRapier,
					Items: []EquipmentItem{{ID: shared.EquipmentID("rapier"), Quantity: 1}},
					Label: "Rapier",
				},
				{
					ID:    BardWeaponLongsword,
					Items: []EquipmentItem{{ID: shared.EquipmentID("longsword"), Quantity: 1}},
					Label: "Longsword",
				},
				{
					ID:    BardWeaponSimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       "weapon",
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee, weapons.CategorySimpleRanged},
							Label:      "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your weapon",
		},
		{
			ID:     BardPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    BardPackDiplomat,
					Items: []EquipmentItem{{ID: shared.EquipmentID("diplomat-pack"), Quantity: 1}},
					Label: "Diplomat's pack",
				},
				{
					ID:    BardPackEntertainer,
					Items: []EquipmentItem{{ID: shared.EquipmentID("entertainer-pack"), Quantity: 1}},
					Label: "Entertainer's pack",
				},
			},
			Label: "Choose your equipment pack",
		},
		{
			ID:     BardInstrument,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    BardInstrumentLute,
					Items: []EquipmentItem{{ID: shared.EquipmentID("lute"), Quantity: 1}},
					Label: "Lute",
				},
				{
					ID:    BardInstrumentOther,
					Label: "Any other musical instrument",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       "tool",
							Categories: []shared.EquipmentCategory{equipment.CategoryMusicalInstruments},
							Label:      "Choose a musical instrument",
						},
					},
				},
			},
			Label: "Choose your musical instrument",
		},
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
						{ID: ammunition.Arrows20, Quantity: 1},
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
						{ID: ammunition.Bolts20, Quantity: 1},
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

func getBarbarianEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     BarbarianWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    BarbarianWeaponGreataxe,
					Label: "Greataxe",
					Items: []EquipmentItem{
						{ID: weapons.Greataxe, Quantity: 1},
					},
				},
				{
					ID:    BarbarianWeaponMartial,
					Label: "Any martial melee weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategoryMartialMelee},
						},
					},
				},
			},
			Label: "Choose your primary weapon",
		},
		{
			ID:     BarbarianWeaponsSecondary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    BarbarianSecondaryHandaxes,
					Label: "Two handaxes",
					Items: []EquipmentItem{
						{ID: weapons.Handaxe, Quantity: 2},
					},
				},
				{
					ID:    BarbarianSecondarySimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose: 1,
							Type:   shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{
								weapons.CategorySimpleMelee,
								weapons.CategorySimpleRanged,
							},
						},
					},
				},
			},
			Label: "Choose your secondary weapon",
		},
		{
			ID:     BarbarianPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    BarbarianPackExplorer,
					Label: "Explorer's pack",
					Items: []EquipmentItem{
						{ID: packs.ExplorerPack, Quantity: 1},
					},
				},
			},
			Label: "Choose your equipment pack",
		},
		// Barbarians also get 4 javelins automatically (not a choice)
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
						{ID: ammunition.Arrows20, Quantity: 1},
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
						{ID: ammunition.Bolts20, Quantity: 1},
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

// Equipment requirement helper functions for the remaining classes

func getDruidEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     DruidWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    DruidWeaponShield,
					Items: []EquipmentItem{{ID: armor.Shield, Quantity: 1}},
					Label: "A wooden shield",
				},
				{
					ID:    DruidWeaponSimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee, weapons.CategorySimpleRanged},
							Label:      "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your primary weapon",
		},
		{
			ID:     DruidWeaponsSecondary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    DruidSecondaryScimitar,
					Items: []EquipmentItem{{ID: weapons.Scimitar, Quantity: 1}},
					Label: "Scimitar",
				},
				{
					ID:    DruidSecondaryMelee,
					Label: "Any simple melee weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee},
							Label:      "Choose a simple melee weapon",
						},
					},
				},
			},
			Label: "Choose your secondary weapon",
		},
		{
			ID:     DruidFocus,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    DruidFocusOption,
					Label: "Druidic focus",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       "tool",
							Categories: []shared.EquipmentCategory{equipment.CategoryDruidicFoci},
							Label:      "Choose a druidic focus",
						},
					},
				},
			},
			Label: "Choose your druidic focus",
		},
	}
}

func getMonkEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     MonkWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    MonkWeaponShortsword,
					Items: []EquipmentItem{{ID: weapons.Shortsword, Quantity: 1}},
					Label: "Shortsword",
				},
				{
					// 2014 Monk starting equipment offers "a shortsword or any simple
					// weapon" - this is the general equipment-proficiency choice, not
					// the narrower Martial Arts "monk weapon" set (shortsword + simple
					// MELEE weapons without Heavy/Two-Handed) described in
					// classes.Description(Monk). Do not narrow this to melee-only -
					// see rpg-toolkit#873/#874.
					ID:    MonkWeaponSimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee, weapons.CategorySimpleRanged},
							Label:      "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your weapon",
		},
		{
			ID:     MonkPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    MonkPackDungeoneer,
					Items: []EquipmentItem{{ID: packs.DungeoneerPack, Quantity: 1}},
					Label: "Dungeoneer's pack",
				},
				{
					ID:    MonkPackExplorer,
					Items: []EquipmentItem{{ID: packs.ExplorerPack, Quantity: 1}},
					Label: "Explorer's pack",
				},
			},
			Label: "Choose your equipment pack",
		},
	}
}

func getPaladinEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     PaladinWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    PaladinWeaponMartialShield,
					Items: []EquipmentItem{{ID: armor.Shield, Quantity: 1}},
					Label: "A martial weapon and a shield",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategoryMartialMelee, weapons.CategoryMartialRanged},
							Label:      "Choose a martial weapon",
						},
					},
				},
				{
					ID:    PaladinWeaponTwoMartial,
					Label: "Two martial weapons",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     2,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategoryMartialMelee, weapons.CategoryMartialRanged},
							Label:      "Choose two martial weapons",
						},
					},
				},
			},
			Label: "Choose your weapons",
		},
		{
			ID:     PaladinWeaponsSecondary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    PaladinSecondaryJavelins,
					Items: []EquipmentItem{{ID: weapons.Javelin, Quantity: 5}},
					Label: "Five javelins",
				},
				{
					ID:    PaladinSecondarySimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee, weapons.CategorySimpleRanged},
							Label:      "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your secondary weapons",
		},
		{
			ID:     PaladinPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    PaladinPackPriest,
					Items: []EquipmentItem{{ID: packs.PriestPack, Quantity: 1}},
					Label: "Priest's pack",
				},
				{
					ID:    PaladinPackExplorer,
					Items: []EquipmentItem{{ID: packs.ExplorerPack, Quantity: 1}},
					Label: "Explorer's pack",
				},
			},
			Label: "Choose your equipment pack",
		},
		{
			ID:     PaladinHolySymbol,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    PaladinHolySymbolOption,
					Label: "Holy symbol",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       "tool",
							Categories: []shared.EquipmentCategory{equipment.CategoryHolySymbols},
							Label:      "Choose a holy symbol",
						},
					},
				},
			},
			Label: "Choose your holy symbol",
		},
	}
}

func getRangerEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     RangerArmor,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    RangerArmorScale,
					Items: []EquipmentItem{{ID: armor.ScaleMail, Quantity: 1}},
					Label: "Scale mail",
				},
				{
					ID:    RangerArmorLeather,
					Items: []EquipmentItem{{ID: armor.Leather, Quantity: 1}},
					Label: "Leather armor",
				},
			},
			Label: "Choose your armor",
		},
		{
			ID:     RangerWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    RangerWeaponShortswords,
					Items: []EquipmentItem{{ID: weapons.Shortsword, Quantity: 2}},
					Label: "Two shortswords",
				},
				{
					ID:    RangerWeaponSimpleMelee,
					Label: "Two simple melee weapons",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     2,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee},
							Label:      "Choose two simple melee weapons",
						},
					},
				},
			},
			Label: "Choose your melee weapons",
		},
		{
			ID:     RangerPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    RangerPackDungeoneer,
					Items: []EquipmentItem{{ID: packs.DungeoneerPack, Quantity: 1}},
					Label: "Dungeoneer's pack",
				},
				{
					ID:    RangerPackExplorer,
					Items: []EquipmentItem{{ID: packs.ExplorerPack, Quantity: 1}},
					Label: "Explorer's pack",
				},
			},
			Label: "Choose your equipment pack",
		},
	}
}

func getSorcererEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     SorcererWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID: SorcererWeaponCrossbow,
					Items: []EquipmentItem{
						{ID: weapons.LightCrossbow, Quantity: 1},
						{ID: ammunition.Bolts20, Quantity: 1},
					},
					Label: "Light crossbow and 20 bolts",
				},
				{
					ID:    SorcererWeaponSimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee, weapons.CategorySimpleRanged},
							Label:      "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your weapon",
		},
		{
			ID:     SorcererFocus,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    SorcererFocusComponent,
					Items: []EquipmentItem{{ID: items.ComponentPouch, Quantity: 1}},
					Label: "Component pouch",
				},
				{
					ID:    SorcererFocusArcane,
					Label: "Arcane focus",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       "tool",
							Categories: []shared.EquipmentCategory{equipment.CategoryArcaneFoci},
							Label:      "Choose an arcane focus",
						},
					},
				},
			},
			Label: "Choose your spellcasting focus",
		},
		{
			ID:     SorcererPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    SorcererPackDungeoneer,
					Items: []EquipmentItem{{ID: packs.DungeoneerPack, Quantity: 1}},
					Label: "Dungeoneer's pack",
				},
				{
					ID:    SorcererPackExplorer,
					Items: []EquipmentItem{{ID: packs.ExplorerPack, Quantity: 1}},
					Label: "Explorer's pack",
				},
			},
			Label: "Choose your equipment pack",
		},
	}
}

func getWarlockEquipmentRequirements() []*EquipmentRequirement {
	return []*EquipmentRequirement{
		{
			ID:     WarlockWeaponsPrimary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID: WarlockWeaponCrossbow,
					Items: []EquipmentItem{
						{ID: weapons.LightCrossbow, Quantity: 1},
						{ID: ammunition.Bolts20, Quantity: 1},
					},
					Label: "Light crossbow and 20 bolts",
				},
				{
					ID:    WarlockWeaponSimple,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee, weapons.CategorySimpleRanged},
							Label:      "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your primary weapon",
		},
		{
			ID:     WarlockFocus,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    WarlockFocusComponent,
					Items: []EquipmentItem{{ID: items.ComponentPouch, Quantity: 1}},
					Label: "Component pouch",
				},
				{
					ID:    WarlockFocusArcane,
					Label: "Arcane focus",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       "tool",
							Categories: []shared.EquipmentCategory{equipment.CategoryArcaneFoci},
							Label:      "Choose an arcane focus",
						},
					},
				},
			},
			Label: "Choose your spellcasting focus",
		},
		{
			ID:     WarlockPack,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    WarlockPackScholar,
					Items: []EquipmentItem{{ID: packs.ScholarPack, Quantity: 1}},
					Label: "Scholar's pack",
				},
				{
					ID:    WarlockPackDungeoneer,
					Items: []EquipmentItem{{ID: packs.DungeoneerPack, Quantity: 1}},
					Label: "Dungeoneer's pack",
				},
			},
			Label: "Choose your equipment pack",
		},
		{
			ID:     WarlockWeaponsSecondary,
			Choose: 1,
			Options: []EquipmentOption{
				{
					ID:    WarlockWeaponSecondary,
					Label: "Any simple weapon",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Choose:     1,
							Type:       shared.EquipmentTypeWeapon,
							Categories: []shared.EquipmentCategory{weapons.CategorySimpleMelee, weapons.CategorySimpleRanged},
							Label:      "Choose a simple weapon",
						},
					},
				},
			},
			Label: "Choose your secondary weapon",
		},
	}
}

// GetClassRequirementsWithSubclass returns what a class asks for at a class
// level, with its chosen subclass's modifications applied.
//
// The level now means the level those requirements are GAINED at rather than a
// creation total (design R4.2). Its one caller asks at the level a subclass is
// chosen, which for the only class that chooses one at creation is level 1,
// where the two readings are the same set.
func GetClassRequirementsWithSubclass(class classes.Class, level int, subclass classes.Subclass) *Requirements {
	reqs := GetClassRequirementsGainedAtLevel(class, level)
	if reqs == nil {
		return nil
	}

	// Get subclass modifications
	mods := GetSubclassModifications(subclass)

	// Apply the modifications
	ApplySubclassModifications(reqs, mods)
	if class == classes.Cleric {
		ExcludeGrantedSpellChoices(reqs, subclass, level)
	}
	reqs.Equipment = enrichEquipmentRequirements(reqs.Equipment)

	return reqs
}
