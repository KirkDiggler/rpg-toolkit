package backgrounds

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquipmentItem is one item a background grants automatically, by identity
// and quantity — mirrors classes.EquipmentItem's shape exactly (same
// convention: each package defines its own rather than importing a shared
// type).
type EquipmentItem struct {
	ID       shared.EquipmentID
	Quantity int
}

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

	// Equipment lists only the fixed items a background grants
	// automatically — a background's own player-made equipment choice
	// (e.g. Folk Hero's artisan's tools) is a Requirements-driven choice,
	// not part of this fixed list.
	Equipment []EquipmentItem

	// StartingGold is the gold a background grants automatically,
	// alongside its fixed Equipment.
	StartingGold currency.Money

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
			// A prayer book or prayer wheel is also part of Acolyte's PHB
			// equipment list, but has no official individual PHB/SRD price
			// (unlike holy symbol/incense/vestments/clothes below) — not
			// modeled, same as any other unpriced narrative item.
			Equipment: []EquipmentItem{
				{ID: items.HolySymbol, Quantity: 1},
				{ID: items.Incense, Quantity: 1},
				{ID: items.Vestments, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			StartingGold: currency.FromGold(15),
		}

	case Criminal, Spy:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Deception,
				skills.Stealth,
			},
			// Gaming set proficiency is a choice (one type of gaming set),
			// not fixed — previously hardcoded to ToolPlayingCardSet here,
			// which was simply wrong per PHB ("one type of gaming set").
			// See choices.GetBackgroundRequirements' CriminalGamingSet.
			ToolProficiencies: []proficiencies.Tool{
				proficiencies.ToolThieves, // Thieves' tools
			},
			Equipment: []EquipmentItem{
				{ID: items.Crowbar, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1}, // dark, with a hood — same catalog item
			},
			StartingGold: currency.FromGold(15),
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
			Equipment: []EquipmentItem{
				{ID: items.Costume, Quantity: 1},
			},
			StartingGold: currency.FromGold(15),
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
			Equipment: []EquipmentItem{
				{ID: items.Shovel, Quantity: 1},
				{ID: items.IronPot, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			StartingGold: currency.FromGold(10),
		}

	case GuildArtisan, GuildMerchant:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Insight,
				skills.Persuasion,
			},
			// Note: Artisan's tools proficiency is a choice
			// A letter of introduction from the guild is also part of the
			// PHB equipment list, but has no official individual price —
			// not modeled.
			Equipment: []EquipmentItem{
				{ID: items.ClothesTraveler, Quantity: 1},
			},
			StartingGold: currency.FromGold(15),
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
			Equipment: []EquipmentItem{
				{ID: items.CaseMap, Quantity: 1}, // scroll case
				{ID: items.Blanket, Quantity: 1}, // "winter blanket" — same catalog item
				{ID: items.ClothesCommon, Quantity: 1},
				{ID: tools.HerbalismKit, Quantity: 1},
			},
			StartingGold: currency.FromGold(5),
		}

	case Noble, Knight:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.History,
				skills.Persuasion,
			},
			// Note: Gaming set proficiency is a choice
			// A scroll of pedigree is also part of the PHB equipment list,
			// but has no official individual price — not modeled.
			Equipment: []EquipmentItem{
				{ID: items.FineClothes, Quantity: 1},
				{ID: items.SignetRing, Quantity: 1},
			},
			StartingGold: currency.FromGold(25),
		}

	case Outlander:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Athletics,
				skills.Survival,
			},
			// Note: Musical instrument proficiency is a choice
			// A trophy from an animal killed is also part of the PHB
			// equipment list, but is pure narrative flavor with no price —
			// not modeled.
			Equipment: []EquipmentItem{
				{ID: weapons.Quarterstaff, Quantity: 1}, // "a staff"
				{ID: items.HuntingTrap, Quantity: 1},
				{ID: items.ClothesTraveler, Quantity: 1},
			},
			StartingGold: currency.FromGold(10),
		}

	case Sage:
		return &Grant{
			SkillProficiencies: []skills.Skill{
				skills.Arcana,
				skills.History,
			},
			// A letter from a dead colleague is also part of the PHB
			// equipment list, but is pure narrative flavor with no price —
			// not modeled.
			Equipment: []EquipmentItem{
				{ID: items.Ink, Quantity: 1},
				{ID: items.InkPen, Quantity: 1}, // "a quill" — same catalog item
				{ID: items.SmallKnife, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			StartingGold: currency.FromGold(10),
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
			// A lucky charm/trinket is also part of the PHB equipment
			// list, but is pure narrative flavor with no price — not
			// modeled.
			Equipment: []EquipmentItem{
				{ID: weapons.Club, Quantity: 1}, // "a belaying pin"
				{ID: items.SilkRope, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			StartingGold: currency.FromGold(10),
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
			// An insignia of rank and a trophy taken from a fallen enemy
			// are also part of the PHB equipment list, but are pure
			// narrative flavor with no price — not modeled. Bone dice or a
			// deck of cards is a real physical-item choice, wired
			// separately (not a fixed grant).
			Equipment: []EquipmentItem{
				{ID: items.ClothesCommon, Quantity: 1},
			},
			StartingGold: currency.FromGold(10),
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
			// A city map, a pet mouse, and a token to remember your
			// parents by are also part of the PHB equipment list, but are
			// pure narrative flavor with no price — not modeled.
			Equipment: []EquipmentItem{
				{ID: items.SmallKnife, Quantity: 1},
				{ID: items.ClothesCommon, Quantity: 1},
			},
			StartingGold: currency.FromGold(10),
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
			Equipment: []EquipmentItem{
				{ID: items.FineClothes, Quantity: 1},
				{ID: tools.DisguiseKit, Quantity: 1},
			},
			StartingGold: currency.FromGold(15),
		}

	default:
		// Unknown background or custom
		return nil
	}
}
