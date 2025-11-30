package backgrounds

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Grant represents what a background provides at character creation.
// For intrinsic background properties (feature description, choice counts), see Data.
//
// Grant = "what you get" (proficiencies, languages granted automatically)
// Data = "what you are" (intrinsic properties like feature descriptions, choice counts)
type Grant struct {
	// Proficiencies
	SkillProficiencies []skills.Skill
	ToolProficiencies  []proficiencies.Tool // Some backgrounds grant specific tools

	// Languages - if any backgrounds grant specific languages automatically
	Languages []languages.Language

	// Future: features when background features are implemented
	// Features []FeatureRef
}

// GetGrants returns what a background automatically grants (not choices).
// Returns nil for unknown backgrounds.
func GetGrants(bg Background) *Grant {
	switch bg {
	case Acolyte:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Insight,
				skills.Religion,
			},
		}

	case Criminal, Spy:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Deception,
				skills.Stealth,
			},
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolPlayingCardSet, // Gaming set proficiency
				proficiencies.ToolThieves,        // Thieves' tools
			},
		}

	case Entertainer:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Acrobatics,
				skills.Performance,
			},
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolDisguiseKit,
			},
			// Note: Musical instrument proficiency is a choice, not automatic
		}

	case FolkHero:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.AnimalHandling,
				skills.Survival,
			},
			// Note: Artisan's tools proficiency is a choice
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolVehicleLand,
			},
		}

	case GuildArtisan, GuildMerchant:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Insight,
				skills.Persuasion,
			},
			// Note: Artisan's tools proficiency is a choice
		}

	case Hermit:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Medicine,
				skills.Religion,
			},
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolHerbalism,
			},
		}

	case Noble, Knight:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.History,
				skills.Persuasion,
			},
			// Note: Gaming set proficiency is a choice
		}

	case Outlander:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Athletics,
				skills.Survival,
			},
			// Note: Musical instrument proficiency is a choice
		}

	case Sage:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Arcana,
				skills.History,
			},
		}

	case Sailor, Pirate:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Athletics,
				skills.Perception,
			},
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolNavigator,
				proficiencies.ToolVehicleWater,
			},
		}

	case Soldier:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Athletics,
				skills.Intimidation,
			},
			// Note: Gaming set proficiency is a choice
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolVehicleLand,
			},
		}

	case Urchin:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.SleightOfHand,
				skills.Stealth,
			},
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolDisguiseKit,
				proficiencies.ToolThieves,
			},
		}

	case Charlatan:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Deception,
				skills.SleightOfHand,
			},
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolDisguiseKit,
				proficiencies.ToolForgeryKit,
			},
		}

	default:
		// Unknown background or custom
		return nil
	}
}
