package races

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// AutomaticGrants represents what a race automatically provides
type AutomaticGrants struct {
	Skills    []skills.Skill
	Languages []languages.Language
	// TODO: Add weapon proficiencies, armor proficiencies, etc.
}

// GetAutomaticGrants returns what a race automatically grants (not choices)
func GetAutomaticGrants(race Race) AutomaticGrants {
	switch race {
	case Dragonborn:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				languages.Draconic,
			},
		}

	case Dwarf, HillDwarf, MountainDwarf:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				languages.Dwarvish,
			},
		}

	case Elf, HighElf, WoodElf:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				languages.Elvish,
			},
		}

	case Gnome, ForestGnome, RockGnome:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				languages.Gnomish,
			},
		}

	case HalfElf:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				languages.Elvish,
				// Note: Half-Elf gets one additional language of choice - that's handled in choices
			},
		}

	case Halfling, LightfootHalfling, StoutHalfling:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				languages.Halfling,
			},
		}

	case HalfOrc:
		return AutomaticGrants{
			Skills: []skills.Skill{
				skills.Intimidation, // Menacing trait
			},
			Languages: []languages.Language{
				languages.Common,
				languages.Orc,
			},
		}

	case Human:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				// Note: Human gets one additional language of choice - that's handled in choices
			},
		}

	case Tiefling:
		return AutomaticGrants{
			Languages: []languages.Language{
				languages.Common,
				languages.Infernal,
			},
		}

	default:
		// Unknown race or one without automatic grants
		return AutomaticGrants{}
	}
}
