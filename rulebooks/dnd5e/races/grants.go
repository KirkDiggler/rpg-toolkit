package races

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Grant represents what a race provides at character creation.
// For intrinsic racial properties (speed, size, ability scores), see Data.
type Grant struct {
	SkillProficiencies  []skills.Skill
	WeaponProficiencies []proficiencies.Weapon
	ArmorProficiencies  []proficiencies.Armor
	ToolProficiencies   []proficiencies.Tool
	Languages           []languages.Language
}

// GetGrants returns what a race automatically grants (not choices)
func GetGrants(race Race) Grant {
	switch race {
	case Dragonborn:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Draconic,
			},
		}

	case Dwarf, HillDwarf, MountainDwarf:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Dwarvish,
			},
		}

	case Elf, HighElf, WoodElf:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Elvish,
			},
		}

	case Gnome, ForestGnome, RockGnome:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Gnomish,
			},
		}

	case HalfElf:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Elvish,
				// Note: Half-Elf gets one additional language of choice - that's handled in choices
			},
		}

	case Halfling, LightfootHalfling, StoutHalfling:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Halfling,
			},
		}

	case HalfOrc:
		return Grant{
			SkillProficiencies: []skills.Skill{
				skills.Intimidation, // Menacing trait
			},
			Languages: []languages.Language{
				languages.Common,
				languages.Orc,
			},
		}

	case Human:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				// Note: Human gets one additional language of choice - that's handled in choices
			},
		}

	case Tiefling:
		return Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Infernal,
			},
		}

	default:
		// Unknown race or one without automatic grants
		return Grant{}
	}
}
