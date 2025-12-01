package races

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Grant represents what a race provides at character creation.
// For intrinsic racial properties (speed, size, ability scores), see Data.
type Grant struct {
	// Proficiencies
	SkillProficiencies  []skills.Skill
	WeaponProficiencies []proficiencies.Weapon // e.g., Dwarf combat training
	ArmorProficiencies  []proficiencies.Armor  // e.g., Mountain Dwarf
	ToolProficiencies   []proficiencies.Tool   // e.g., Dwarf artisan tools

	// Languages
	Languages []languages.Language

	// Future: conditions and features when racial traits are implemented
	// Conditions []ConditionRef  // e.g., Darkvision, poison resistance
	// Features   []FeatureRef    // e.g., Breath Weapon, Relentless Endurance
}

// GetGrants returns what a race grants at character creation (not choices).
func GetGrants(race Race) *Grant {
	switch race {
	case Dragonborn:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Draconic,
			},
		}

	case Dwarf, HillDwarf, MountainDwarf:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Dwarvish,
			},
		}

	case Elf, HighElf, WoodElf:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Elvish,
			},
		}

	case Gnome, ForestGnome, RockGnome:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Gnomish,
			},
		}

	case HalfElf:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Elvish,
				// Note: Half-Elf gets one additional language of choice - that's handled in choices
			},
		}

	case Halfling, LightfootHalfling, StoutHalfling:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Halfling,
			},
		}

	case HalfOrc:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Intimidation, // Menacing trait
			},
			Languages: []languages.Language{
				languages.Common,
				languages.Orc,
			},
		}

	case Human:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				// Note: Human gets one additional language of choice - that's handled in choices
			},
		}

	case Tiefling:
		return &Grant{
			Languages: []languages.Language{
				languages.Common,
				languages.Infernal,
			},
		}

	default:
		// Unknown race or one without grants
		return nil
	}
}
