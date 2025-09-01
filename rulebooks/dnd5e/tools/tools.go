// Package tools provides D&D 5e tool definitions and data
package tools

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ToolID represents unique identifier for tools
type ToolID string

// ToolCategory represents tool classification
type ToolCategory string

const (
	// CategoryArtisan represents artisan's tools
	CategoryArtisan ToolCategory = "artisan"
	// CategoryGaming represents gaming sets
	CategoryGaming ToolCategory = "gaming"
	// CategoryMusical represents musical instruments
	CategoryMusical ToolCategory = "musical"
	// CategoryOther represents other tools
	CategoryOther ToolCategory = "other"
)

// Artisan's tools
const (
	AlchemistSupplies  ToolID = "alchemist-supplies"
	BrewerSupplies     ToolID = "brewer-supplies"
	CalligrapherSupplies ToolID = "calligrapher-supplies"
	CarpenterTools     ToolID = "carpenter-tools"
	CartographerTools  ToolID = "cartographer-tools"
	CobblerTools       ToolID = "cobbler-tools"
	CookUtensils       ToolID = "cook-utensils"
	GlassblowerTools   ToolID = "glassblower-tools"
	JewelerTools       ToolID = "jeweler-tools"
	LeatherworkerTools ToolID = "leatherworker-tools"
	MasonTools         ToolID = "mason-tools"
	PainterSupplies    ToolID = "painter-supplies"
	PotterTools        ToolID = "potter-tools"
	SmithTools         ToolID = "smith-tools"
	TinkerTools        ToolID = "tinker-tools"
	WeaverTools        ToolID = "weaver-tools"
	WoodcarverTools    ToolID = "woodcarver-tools"
)

// Gaming sets
const (
	DiceSet          ToolID = "dice-set"
	DragonchessSet   ToolID = "dragonchess-set"
	PlayingCardSet   ToolID = "playing-card-set"
	ThreeDragonAnte  ToolID = "three-dragon-ante"
)

// Musical instruments
const (
	Bagpipes   ToolID = "bagpipes"
	Drum       ToolID = "drum"
	Dulcimer   ToolID = "dulcimer"
	Flute      ToolID = "flute"
	Lute       ToolID = "lute"
	Lyre       ToolID = "lyre"
	Horn       ToolID = "horn"
	PanFlute   ToolID = "pan-flute"
	Shawm      ToolID = "shawm"
	Viol       ToolID = "viol"
)

// Other tools
const (
	DisguiseKit      ToolID = "disguise-kit"
	ForgeryKit       ToolID = "forgery-kit"
	HerbalismKit     ToolID = "herbalism-kit"
	NavigatorTools   ToolID = "navigator-tools"
	PoisonerKit      ToolID = "poisoner-kit"
	ThievesTools     ToolID = "thieves-tools"
	VehiclesLand     ToolID = "vehicles-land"
	VehiclesWater    ToolID = "vehicles-water"
)

// Tool represents a D&D 5e tool or kit
type Tool struct {
	ID          ToolID
	Name        string
	Category    ToolCategory
	Weight      float32 // in pounds
	Cost        string  // e.g., "25 gp"
	Description string
}

// GetID returns the unique identifier for this tool
func (t *Tool) GetID() string {
	return string(t.ID)
}

// GetType returns the equipment type (always TypeTool)
func (t *Tool) GetType() shared.EquipmentType {
	return shared.EquipmentTypeTool
}

// GetName returns the name of the tool
func (t *Tool) GetName() string {
	return t.Name
}

// GetWeight returns the weight in pounds
func (t *Tool) GetWeight() float32 {
	return t.Weight
}

// GetValue returns the value in copper pieces
func (t *Tool) GetValue() int {
	// TODO: Parse cost string (e.g., "25 gp") and convert to copper
	// For now, return a placeholder
	return 0
}

// GetDescription returns a description of the tool
func (t *Tool) GetDescription() string {
	if t.Description != "" {
		return t.Description
	}
	// Return a generic description based on category
	switch t.Category {
	case CategoryArtisan:
		return fmt.Sprintf("Artisan's tools for %s", t.Name)
	case CategoryGaming:
		return "Gaming set for entertainment and gambling"
	case CategoryMusical:
		return "Musical instrument for performance"
	default:
		return fmt.Sprintf("Specialized tools: %s", t.Name)
	}
}

// All tool definitions
var All = map[ToolID]Tool{
	// Artisan's tools
	AlchemistSupplies: {
		ID:       AlchemistSupplies,
		Name:     "Alchemist's Supplies",
		Category: CategoryArtisan,
		Weight:   8,
		Cost:     "50 gp",
		Description: "Tools for creating alchemical substances",
	},
	BrewerSupplies: {
		ID:       BrewerSupplies,
		Name:     "Brewer's Supplies",
		Category: CategoryArtisan,
		Weight:   9,
		Cost:     "20 gp",
		Description: "Tools for brewing beer and other beverages",
	},
	CalligrapherSupplies: {
		ID:       CalligrapherSupplies,
		Name:     "Calligrapher's Supplies",
		Category: CategoryArtisan,
		Weight:   5,
		Cost:     "10 gp",
		Description: "Tools for beautiful writing and illumination",
	},
	CarpenterTools: {
		ID:       CarpenterTools,
		Name:     "Carpenter's Tools",
		Category: CategoryArtisan,
		Weight:   6,
		Cost:     "8 gp",
		Description: "Tools for working with wood",
	},
	CartographerTools: {
		ID:       CartographerTools,
		Name:     "Cartographer's Tools",
		Category: CategoryArtisan,
		Weight:   6,
		Cost:     "15 gp",
		Description: "Tools for making maps",
	},
	CobblerTools: {
		ID:       CobblerTools,
		Name:     "Cobbler's Tools",
		Category: CategoryArtisan,
		Weight:   5,
		Cost:     "5 gp",
		Description: "Tools for making and repairing shoes",
	},
	CookUtensils: {
		ID:       CookUtensils,
		Name:     "Cook's Utensils",
		Category: CategoryArtisan,
		Weight:   8,
		Cost:     "1 gp",
		Description: "Tools for preparing food",
	},
	GlassblowerTools: {
		ID:       GlassblowerTools,
		Name:     "Glassblower's Tools",
		Category: CategoryArtisan,
		Weight:   5,
		Cost:     "30 gp",
		Description: "Tools for shaping glass",
	},
	JewelerTools: {
		ID:       JewelerTools,
		Name:     "Jeweler's Tools",
		Category: CategoryArtisan,
		Weight:   2,
		Cost:     "25 gp",
		Description: "Tools for working with gems and precious metals",
	},
	LeatherworkerTools: {
		ID:       LeatherworkerTools,
		Name:     "Leatherworker's Tools",
		Category: CategoryArtisan,
		Weight:   5,
		Cost:     "5 gp",
		Description: "Tools for working with leather",
	},
	MasonTools: {
		ID:       MasonTools,
		Name:     "Mason's Tools",
		Category: CategoryArtisan,
		Weight:   8,
		Cost:     "10 gp",
		Description: "Tools for working with stone",
	},
	PainterSupplies: {
		ID:       PainterSupplies,
		Name:     "Painter's Supplies",
		Category: CategoryArtisan,
		Weight:   5,
		Cost:     "10 gp",
		Description: "Tools for painting and drawing",
	},
	PotterTools: {
		ID:       PotterTools,
		Name:     "Potter's Tools",
		Category: CategoryArtisan,
		Weight:   3,
		Cost:     "10 gp",
		Description: "Tools for shaping clay",
	},
	SmithTools: {
		ID:       SmithTools,
		Name:     "Smith's Tools",
		Category: CategoryArtisan,
		Weight:   8,
		Cost:     "20 gp",
		Description: "Tools for working with metal",
	},
	TinkerTools: {
		ID:       TinkerTools,
		Name:     "Tinker's Tools",
		Category: CategoryArtisan,
		Weight:   10,
		Cost:     "50 gp",
		Description: "Tools for repairing various objects",
	},
	WeaverTools: {
		ID:       WeaverTools,
		Name:     "Weaver's Tools",
		Category: CategoryArtisan,
		Weight:   5,
		Cost:     "1 gp",
		Description: "Tools for working with cloth",
	},
	WoodcarverTools: {
		ID:       WoodcarverTools,
		Name:     "Woodcarver's Tools",
		Category: CategoryArtisan,
		Weight:   5,
		Cost:     "1 gp",
		Description: "Tools for carving wood",
	},

	// Gaming sets
	DiceSet: {
		ID:       DiceSet,
		Name:     "Dice Set",
		Category: CategoryGaming,
		Weight:   0,
		Cost:     "1 sp",
		Description: "A set of dice for gaming",
	},
	DragonchessSet: {
		ID:       DragonchessSet,
		Name:     "Dragonchess Set",
		Category: CategoryGaming,
		Weight:   0.5,
		Cost:     "1 gp",
		Description: "A three-dimensional chess variant",
	},
	PlayingCardSet: {
		ID:       PlayingCardSet,
		Name:     "Playing Card Set",
		Category: CategoryGaming,
		Weight:   0,
		Cost:     "5 sp",
		Description: "A deck of playing cards",
	},
	ThreeDragonAnte: {
		ID:       ThreeDragonAnte,
		Name:     "Three-Dragon Ante Set",
		Category: CategoryGaming,
		Weight:   0,
		Cost:     "1 gp",
		Description: "A card game popular in taverns",
	},

	// Musical instruments
	Bagpipes: {
		ID:       Bagpipes,
		Name:     "Bagpipes",
		Category: CategoryMusical,
		Weight:   6,
		Cost:     "30 gp",
		Description: "A wind instrument with a distinctive sound",
	},
	Drum: {
		ID:       Drum,
		Name:     "Drum",
		Category: CategoryMusical,
		Weight:   3,
		Cost:     "6 gp",
		Description: "A percussion instrument",
	},
	Dulcimer: {
		ID:       Dulcimer,
		Name:     "Dulcimer",
		Category: CategoryMusical,
		Weight:   10,
		Cost:     "25 gp",
		Description: "A stringed instrument played with hammers",
	},
	Flute: {
		ID:       Flute,
		Name:     "Flute",
		Category: CategoryMusical,
		Weight:   1,
		Cost:     "2 gp",
		Description: "A simple wind instrument",
	},
	Lute: {
		ID:       Lute,
		Name:     "Lute",
		Category: CategoryMusical,
		Weight:   2,
		Cost:     "35 gp",
		Description: "A popular stringed instrument",
	},
	Lyre: {
		ID:       Lyre,
		Name:     "Lyre",
		Category: CategoryMusical,
		Weight:   2,
		Cost:     "30 gp",
		Description: "An ancient stringed instrument",
	},
	Horn: {
		ID:       Horn,
		Name:     "Horn",
		Category: CategoryMusical,
		Weight:   2,
		Cost:     "3 gp",
		Description: "A brass wind instrument",
	},
	PanFlute: {
		ID:       PanFlute,
		Name:     "Pan Flute",
		Category: CategoryMusical,
		Weight:   2,
		Cost:     "12 gp",
		Description: "A multi-piped wind instrument",
	},
	Shawm: {
		ID:       Shawm,
		Name:     "Shawm",
		Category: CategoryMusical,
		Weight:   1,
		Cost:     "2 gp",
		Description: "A double-reed wind instrument",
	},
	Viol: {
		ID:       Viol,
		Name:     "Viol",
		Category: CategoryMusical,
		Weight:   1,
		Cost:     "30 gp",
		Description: "A bowed stringed instrument",
	},

	// Other tools
	DisguiseKit: {
		ID:       DisguiseKit,
		Name:     "Disguise Kit",
		Category: CategoryOther,
		Weight:   3,
		Cost:     "25 gp",
		Description: "Cosmetics, hair dye, and props for disguises",
	},
	ForgeryKit: {
		ID:       ForgeryKit,
		Name:     "Forgery Kit",
		Category: CategoryOther,
		Weight:   5,
		Cost:     "15 gp",
		Description: "Tools for creating false documents",
	},
	HerbalismKit: {
		ID:       HerbalismKit,
		Name:     "Herbalism Kit",
		Category: CategoryOther,
		Weight:   3,
		Cost:     "5 gp",
		Description: "Tools for identifying and applying herbs",
	},
	NavigatorTools: {
		ID:       NavigatorTools,
		Name:     "Navigator's Tools",
		Category: CategoryOther,
		Weight:   2,
		Cost:     "25 gp",
		Description: "Instruments for navigation at sea",
	},
	PoisonerKit: {
		ID:       PoisonerKit,
		Name:     "Poisoner's Kit",
		Category: CategoryOther,
		Weight:   2,
		Cost:     "50 gp",
		Description: "Tools for creating and applying poisons",
	},
	ThievesTools: {
		ID:       ThievesTools,
		Name:     "Thieves' Tools",
		Category: CategoryOther,
		Weight:   1,
		Cost:     "25 gp",
		Description: "Tools for picking locks and disarming traps",
	},
	VehiclesLand: {
		ID:       VehiclesLand,
		Name:     "Land Vehicles",
		Category: CategoryOther,
		Weight:   0,
		Cost:     "0 gp",
		Description: "Proficiency with land-based vehicles",
	},
	VehiclesWater: {
		ID:       VehiclesWater,
		Name:     "Water Vehicles",
		Category: CategoryOther,
		Weight:   0,
		Cost:     "0 gp",
		Description: "Proficiency with water-based vehicles",
	},
}

// GetByID returns a tool by its ID
func GetByID(id ToolID) (Tool, error) {
	tool, ok := All[id]
	if !ok {
		validTools := make([]string, 0, len(All))
		for k := range All {
			validTools = append(validTools, string(k))
		}
		return Tool{}, rpgerr.New(rpgerr.CodeInvalidArgument, "invalid tool",
			rpgerr.WithMeta("provided", string(id)),
			rpgerr.WithMeta("valid_options", validTools))
	}
	return tool, nil
}

// GetByCategory returns all tools in a category
func GetByCategory(cat ToolCategory) []Tool {
	var result []Tool
	for _, t := range All {
		if t.Category == cat {
			result = append(result, t)
		}
	}
	return result
}