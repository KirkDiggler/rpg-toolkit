package backgrounds

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// AutomaticGrants represents what a background automatically provides
type AutomaticGrants struct {
	Skills []skills.Skill
	// TODO: Add tool proficiencies, languages if any backgrounds grant them automatically
}

// GetAutomaticGrants returns what a background automatically grants (not choices)
func GetAutomaticGrants(bg Background) AutomaticGrants {
	switch bg {
	case Acolyte:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Insight,
				skills.Religion,
			},
		}

	case Criminal, Spy:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Deception,
				skills.Stealth,
			},
		}

	case Entertainer:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Acrobatics,
				skills.Performance,
			},
		}

	case FolkHero:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.AnimalHandling,
				skills.Survival,
			},
		}

	case GuildArtisan, GuildMerchant:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Insight,
				skills.Persuasion,
			},
		}

	case Hermit:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Medicine,
				skills.Religion,
			},
		}

	case Noble, Knight:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.History,
				skills.Persuasion,
			},
		}

	case Outlander:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Survival,
			},
		}

	case Sage:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Arcana,
				skills.History,
			},
		}

	case Sailor, Pirate:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Perception,
			},
		}

	case Soldier:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Intimidation,
			},
		}

	case Urchin:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.SleightOfHand,
				skills.Stealth,
			},
		}

	case Charlatan:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Deception,
				skills.SleightOfHand,
			},
		}

	default:
		// Unknown background or custom
		return AutomaticGrants{}
	}
}
