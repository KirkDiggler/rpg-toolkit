package classes

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// EquipmentItem represents an item with quantity
type EquipmentItem struct {
	ID       shared.EquipmentID `json:"id"`       // Equipment ID
	Quantity int                `json:"quantity"` // How many (default 1)
}

// AutomaticGrants represents what a class automatically provides (not player choices)
type AutomaticGrants struct {
	// Core mechanics
	HitDice int // The die size (6, 8, 10, 12)

	// Proficiencies that ALL members of this class get
	SavingThrows        []abilities.Ability    // Two saving throw proficiencies
	ArmorProficiencies  []proficiencies.Armor  // e.g., "light", "medium", "heavy", "shields"
	WeaponProficiencies []proficiencies.Weapon // e.g., "simple", "martial"
	ToolProficiencies   []proficiencies.Tool   // Some classes get tools (e.g., thieves' tools for rogue)

	// Starting equipment that ALL members of this class get automatically
	StartingEquipment []EquipmentItem // e.g., Monk gets 10 darts, Rogue gets leather armor + 2 daggers

	// Note: Skill proficiencies are NOT here because they're choices, not grants
	// They belong in Requirements, not Grants
}

// GetAutomaticGrants returns what a class automatically grants at level 1
// Returns nil if the classID is invalid
func GetAutomaticGrants(classID Class) *AutomaticGrants {
	switch classID {
	case Barbarian:
		return &AutomaticGrants{
			HitDice:      12,
			SavingThrows: []abilities.Ability{abilities.STR, abilities.CON},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorHeavy,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial},
		}

	case Bard:
		return &AutomaticGrants{
			HitDice:            8,
			SavingThrows:       []abilities.Ability{abilities.DEX, abilities.CHA},
			ArmorProficiencies: []proficiencies.Armor{proficiencies.ArmorLight},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponHandCrossbow,
				proficiencies.WeaponLongsword,
				proficiencies.WeaponRapier,
				proficiencies.WeaponShortsword,
			},
			// Bards get 3 musical instruments of choice - that's in Requirements, not here
		}

	case Cleric:
		return &AutomaticGrants{
			HitDice:      8,
			SavingThrows: []abilities.Ability{abilities.WIS, abilities.CHA},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple},
		}

	case Druid:
		return &AutomaticGrants{
			HitDice:      8,
			SavingThrows: []abilities.Ability{abilities.INT, abilities.WIS},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorShields,
			}, // Note: non-metal restriction is a rule, not a proficiency
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponClub,
				proficiencies.WeaponDagger,
				proficiencies.WeaponDart,
				proficiencies.WeaponJavelin,
				proficiencies.WeaponMace,
				proficiencies.WeaponQuarterstaff,
				proficiencies.WeaponScimitar,
				proficiencies.WeaponSickle,
				proficiencies.WeaponSling,
				proficiencies.WeaponSpear,
			},
			ToolProficiencies: []proficiencies.Tool{proficiencies.ToolHerbalism},
		}

	case Fighter:
		return &AutomaticGrants{
			HitDice:      10,
			SavingThrows: []abilities.Ability{abilities.STR, abilities.CON},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorHeavy,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
		}

	case Monk:
		return &AutomaticGrants{
			HitDice:      8,
			SavingThrows: []abilities.Ability{abilities.STR, abilities.DEX},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponShortsword,
			},
			StartingEquipment: []EquipmentItem{
				{ID: "dart", Quantity: 10}, // Monks always start with 10 darts
			},
		}

	case Paladin:
		return &AutomaticGrants{
			HitDice:      10,
			SavingThrows: []abilities.Ability{abilities.WIS, abilities.CHA},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorHeavy,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
		}

	case Ranger:
		return &AutomaticGrants{
			HitDice:      10,
			SavingThrows: []abilities.Ability{abilities.STR, abilities.DEX},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
		}

	case Rogue:
		return &AutomaticGrants{
			HitDice:            8,
			SavingThrows:       []abilities.Ability{abilities.DEX, abilities.INT},
			ArmorProficiencies: []proficiencies.Armor{proficiencies.ArmorLight},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponHandCrossbow,
				proficiencies.WeaponLongsword,
				proficiencies.WeaponRapier,
				proficiencies.WeaponShortsword,
			},
			ToolProficiencies: []proficiencies.Tool{proficiencies.ToolThieves},
			StartingEquipment: []EquipmentItem{
				{ID: "leather", Quantity: 1},       // Leather armor
				{ID: "dagger", Quantity: 2},        // Two daggers
				{ID: "thieves-tools", Quantity: 1}, // Thieves' tools
			},
		}

	case Sorcerer:
		return &AutomaticGrants{
			HitDice:      6,
			SavingThrows: []abilities.Ability{abilities.CON, abilities.CHA},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponDagger,
				proficiencies.WeaponDart,
				proficiencies.WeaponSling,
				proficiencies.WeaponQuarterstaff,
				proficiencies.WeaponLightCrossbow,
			},
		}

	case Warlock:
		return &AutomaticGrants{
			HitDice:             8,
			SavingThrows:        []abilities.Ability{abilities.WIS, abilities.CHA},
			ArmorProficiencies:  []proficiencies.Armor{proficiencies.ArmorLight},
			WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple},
		}

	case Wizard:
		return &AutomaticGrants{
			HitDice:      6,
			SavingThrows: []abilities.Ability{abilities.INT, abilities.WIS},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponDagger,
				proficiencies.WeaponDart,
				proficiencies.WeaponSling,
				proficiencies.WeaponQuarterstaff,
				proficiencies.WeaponLightCrossbow,
			},
			StartingEquipment: []EquipmentItem{
				{ID: items.Spellbook, Quantity: 1}, // Wizards always start with a spellbook
			},
		}

	default:
		return nil
	}
}

// GetHitDice is a convenience function to quickly get a class's hit die
// Returns 0 if the classID is invalid
func GetHitDice(classID Class) int {
	grants := GetAutomaticGrants(classID)
	if grants == nil {
		return 0
	}
	return grants.HitDice
}

// GetSavingThrows is a convenience function to get saving throw proficiencies
// Returns nil if the classID is invalid
func GetSavingThrows(classID Class) []abilities.Ability {
	grants := GetAutomaticGrants(classID)
	if grants == nil {
		return nil
	}
	return grants.SavingThrows
}

// GetSubclassGrants returns the complete grants for a subclass (base + subclass specific)
// Returns nil if the subclassID is invalid
func GetSubclassGrants(subclassID Subclass) *AutomaticGrants {
	// Get base class grants
	baseGrants := GetAutomaticGrants(SubclassParent(subclassID))
	if baseGrants == nil {
		return nil
	}

	// Copy base grants to avoid modifying the original
	grants := &AutomaticGrants{
		HitDice:             baseGrants.HitDice,
		SavingThrows:        baseGrants.SavingThrows,
		ArmorProficiencies:  make([]proficiencies.Armor, len(baseGrants.ArmorProficiencies)),
		WeaponProficiencies: make([]proficiencies.Weapon, len(baseGrants.WeaponProficiencies)),
		ToolProficiencies:   make([]proficiencies.Tool, len(baseGrants.ToolProficiencies)),
	}
	copy(grants.ArmorProficiencies, baseGrants.ArmorProficiencies)
	copy(grants.WeaponProficiencies, baseGrants.WeaponProficiencies)
	copy(grants.ToolProficiencies, baseGrants.ToolProficiencies)

	// Add subclass-specific grants
	switch subclassID {
	// Cleric subclasses
	case LifeDomain:
		// Life Domain gets heavy armor
		grants.ArmorProficiencies = append(grants.ArmorProficiencies, proficiencies.ArmorHeavy)

	case KnowledgeDomain:
		// Knowledge Domain gets expertise in 2 INT skills (handled through features)
		// Languages are handled through requirements/choices
		// No additional proficiencies

	case NatureDomain:
		// Nature Domain gets heavy armor
		grants.ArmorProficiencies = append(grants.ArmorProficiencies, proficiencies.ArmorHeavy)

	case TempestDomain:
		// Tempest Domain gets martial weapons and heavy armor
		grants.WeaponProficiencies = append(grants.WeaponProficiencies, proficiencies.WeaponMartial)
		grants.ArmorProficiencies = append(grants.ArmorProficiencies, proficiencies.ArmorHeavy)

	case WarDomain:
		// War Domain gets martial weapons and heavy armor
		grants.WeaponProficiencies = append(grants.WeaponProficiencies, proficiencies.WeaponMartial)
		grants.ArmorProficiencies = append(grants.ArmorProficiencies, proficiencies.ArmorHeavy)

	// Sorcerer subclasses
	case DraconicBloodline:
		// Draconic Bloodline gets +1 HP per level (handled elsewhere)
		// Base AC becomes 13 + Dex (handled as a feature, not a grant)

	// Most subclasses don't add proficiencies
	default:
		// No additional grants
	}

	return grants
}
