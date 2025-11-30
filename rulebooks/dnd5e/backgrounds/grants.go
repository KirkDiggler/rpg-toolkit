package backgrounds

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Grant represents what a background provides at character creation.
// For intrinsic background properties (feature description), see Data.
type Grant struct {
	SkillProficiencies []skills.Skill
	ToolProficiencies  []proficiencies.Tool
	Languages          []languages.Language
}

// GetGrants returns what a background automatically grants (not choices)
func GetGrants(bg Background) Grant {
	switch bg {
	case Acolyte:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Insight,
				skills.Religion,
			},
		}

	case Criminal, Spy:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Deception,
				skills.Stealth,
			},
		}

	case Entertainer:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Acrobatics,
				skills.Performance,
			},
		}

	case FolkHero:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.AnimalHandling,
				skills.Survival,
			},
		}

	case GuildArtisan, GuildMerchant:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Insight,
				skills.Persuasion,
			},
		}

	case Hermit:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Medicine,
				skills.Religion,
			},
		}

	case Noble, Knight:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.History,
				skills.Persuasion,
			},
		}

	case Outlander:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Athletics,
				skills.Survival,
			},
		}

	case Sage:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Arcana,
				skills.History,
			},
		}

	case Sailor, Pirate:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Athletics,
				skills.Perception,
			},
		}

	case Soldier:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Athletics,
				skills.Intimidation,
			},
		}

	case Urchin:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.SleightOfHand,
				skills.Stealth,
			},
		}

	case Charlatan:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Deception,
				skills.SleightOfHand,
			},
		}

	default:
		// Unknown background or custom
		return Grant{}
	}
}
