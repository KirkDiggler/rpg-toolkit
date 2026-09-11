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
	case classes.Barbarian:
		return getBarbarianRequirements()
	case classes.Wizard:
		return getWizardRequirements()
	case classes.Rogue:
		return getRogueRequirements()
	case classes.Cleric:
		return getClericRequirements()
	case classes.Bard:
		return getBardRequirements()
	case classes.Druid:
		return getDruidRequirements()
	case classes.Monk:
		return getMonkRequirements()
	case classes.Paladin:
		return getPaladinRequirements()
	case classes.Ranger:
		return getRangerRequirements()
	case classes.Sorcerer:
		return getSorcererRequirements()
	case classes.Warlock:
		return getWarlockRequirements()
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
	}
}

func getBarbarianRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      getSkillChoiceID(classes.Barbarian),
			Count:   classes.ClassData[classes.Barbarian].SkillCount,
			Options: classes.ClassData[classes.Barbarian].SkillList,
			Label:   fmt.Sprintf("Choose %d skills", classes.ClassData[classes.Barbarian].SkillCount),
		},
		Equipment: enrichEquipmentRequirements(getBarbarianEquipmentRequirements()),
		// No subclass at level 1 (Path chosen at level 3)
		// No spells or cantrips
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
		Equipment: enrichEquipmentRequirements(getWizardEquipmentRequirements()),
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
		Equipment: enrichEquipmentRequirements(getRogueEquipmentRequirements()),
		Expertise: &ExpertiseRequirement{
			ID:    RogueExpertise1,
			Count: 2,
			Label: "Choose 2 skills or thieves' tools for expertise",
		},
	}
}

// getBardRequirements returns requirements for Bard class
func getBardRequirements() *Requirements {
	return &Requirements{
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
		// Gated to the cantrips this build can cast. Unsupported catalog
		// entries are not offered as choices that produce nothing.
		Cantrips: &CantripRequirement{
			ID:      BardCantrips1,
			Count:   2,
			Options: spells.Castable(spells.BardCantrips),
			Label:   "Choose 2 cantrips",
		},
		// The supported acquisition catalog is intentionally narrower than
		// the factual four-known-spells class progression. These are the
		// levelled spells this build can carry forward to casting: every
		// option here compiles to a cast profile, because an option that
		// produced nothing would be a choice with nothing behind it.
		//
		// Count stays at one. Which spells are OFFERED is a fact about what
		// this build can do; how many a level-1 bard picks is a fact about the
		// class, and the second is not free to move because the first did.
		Spellbook: &SpellbookRequirement{
			ID:         BardSpells1,
			Count:      1,
			SpellLevel: 1,
			Options:    []spells.Spell{spells.Bane, spells.Thunderwave},
			Label:      "Choose 1 supported 1st-level spell",
		},

		// Bards get expertise at level 3, not level 1
	}
}

// getDruidRequirements returns requirements for Druid class
func getDruidRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      DruidSkills,
			Count:   2,
			Options: classes.ClassData[classes.Druid].SkillList,
			Label:   "Choose 2 skills",
		},
		Equipment: enrichEquipmentRequirements(getDruidEquipmentRequirements()),
		Cantrips: &CantripRequirement{
			ID:    DruidCantrips1,
			Count: 2,
			Options: []spells.Spell{
				// Damage cantrips
				spells.Frostbite,
				spells.PrimalSavagery,
				spells.Thornwhip,
				spells.CreateBonfire,
				spells.Infestation,
				// Utility cantrips
				spells.Druidcraft,
				spells.Guidance,
				spells.MagicStone,
				spells.MoldEarth,
				spells.Resistance,
				spells.ShapeWater,
			},
			Label: "Choose 2 cantrips",
		},
		// Note: Druids prepare spells, they don't have a spellbook
		// They prepare spells = Wisdom modifier + druid level (minimum 1)
	}
}

// getMonkRequirements returns requirements for Monk class
func getMonkRequirements() *Requirements {
	return &Requirements{
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
	}
}

// getPaladinRequirements returns requirements for Paladin class
func getPaladinRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      PaladinSkills,
			Count:   2,
			Options: classes.ClassData[classes.Paladin].SkillList,
			Label:   "Choose 2 skills",
		},
		Equipment: enrichEquipmentRequirements(getPaladinEquipmentRequirements()),
		// Note: Paladins get spells at level 2, not level 1
		// Fighting style comes at level 2 for Paladins
	}
}

// getRangerRequirements returns requirements for Ranger class
func getRangerRequirements() *Requirements {
	return &Requirements{
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
	}
}

// getSorcererRequirements returns requirements for Sorcerer class
func getSorcererRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      SorcererSkills,
			Count:   2,
			Options: classes.ClassData[classes.Sorcerer].SkillList,
			Label:   "Choose 2 skills",
		},
		Equipment: enrichEquipmentRequirements(getSorcererEquipmentRequirements()),
		Cantrips: &CantripRequirement{
			ID:    SorcererCantrips1,
			Count: 4,
			Options: []spells.Spell{
				// Damage cantrips
				spells.FireBolt,
				spells.RayOfFrost,
				spells.ShockingGrasp,
				spells.AcidSplash,
				spells.PoisonSpray,
				spells.ChillTouch,
				spells.BoomingBlade,
				spells.GreenFlameBlade,
				spells.SwordBurst,
				// Utility cantrips
				spells.MageHand,
				spells.MinorIllusion,
				spells.Prestidigitation,
				spells.Light,
				spells.DancingLights,
				spells.Friends,
				spells.Mending,
				spells.Message,
				spells.TrueStrike,
				spells.ControlFlames,
				spells.CreateBonfire,
			},
			Label: "Choose 4 cantrips",
		},
		Spellbook: &SpellbookRequirement{
			ID:         SorcererSpells1,
			Count:      2,
			SpellLevel: 1,
			Options: []spells.Spell{
				// Level 1 Sorcerer spells
				spells.MagicMissile,
				spells.BurningHands,
				spells.ChromaticOrb,
				spells.Shield,
				spells.Sleep,
				spells.CharmPerson,
				spells.DisguiseSelf,
				spells.ExpeditiousRetreat,
				spells.FalseLife,
				spells.FogCloud,
				spells.RayOfSickness,
				spells.Thunderwave,
				spells.WitchBolt,
				spells.ColorSpray,
				spells.FeatherFall,
			},
			Label: "Choose 2 1st-level spells",
		},
	}
}

// getWarlockRequirements returns requirements for Warlock class
func getWarlockRequirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			ID:      WarlockSkills,
			Count:   2,
			Options: classes.ClassData[classes.Warlock].SkillList,
			Label:   "Choose 2 skills",
		},
		Equipment: enrichEquipmentRequirements(getWarlockEquipmentRequirements()),
		Cantrips: &CantripRequirement{
			ID:    WarlockCantrips1,
			Count: 2,
			Options: []spells.Spell{
				// Damage cantrips
				spells.EldritchBlast, // signature warlock cantrip
				spells.ChillTouch,
				spells.PoisonSpray,
				spells.SacredFlame,
				spells.TollTheDead,
				// Utility cantrips
				spells.MageHand,
				spells.MinorIllusion,
				spells.Prestidigitation,
				spells.Friends,
				spells.BladeWard,
				spells.CreateBonfire,
				spells.Infestation,
			},
			Label: "Choose 2 cantrips",
		},
		Spellbook: &SpellbookRequirement{
			ID:         WarlockSpells1,
			Count:      2,
			SpellLevel: 1,
			Options: []spells.Spell{
				// Level 1 Warlock spells
				spells.ArmsOfHadar,
				spells.CharmPerson,
				spells.ComprehendLanguages,
				spells.ExpeditiousRetreat,
				spells.HellishRebuke,
				spells.Hex,
				spells.ProtectionEvil,
				spells.UnseenServant,
			},
			Label: "Choose 2 1st-level spells",
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
		Equipment: enrichEquipmentRequirements(getClericEquipmentRequirements()),
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
	reqs.Equipment = enrichEquipmentRequirements(reqs.Equipment)

	return reqs
}
