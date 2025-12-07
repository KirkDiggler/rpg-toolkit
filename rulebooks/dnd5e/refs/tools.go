package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Tools provides type-safe, discoverable references to D&D 5e tools.
// Use IDE autocomplete: refs.Tools.<tab> to discover available tools.
var Tools = toolsNS{}

type toolsNS struct{}

// Artisan's Tools

// AlchemistSupplies returns a reference to Alchemist's Supplies.
func (toolsNS) AlchemistSupplies() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "alchemist-supplies"}
}

// BrewerSupplies returns a reference to Brewer's Supplies.
func (toolsNS) BrewerSupplies() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "brewer-supplies"}
}

// CalligrapherSupplies returns a reference to Calligrapher's Supplies.
func (toolsNS) CalligrapherSupplies() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "calligrapher-supplies"}
}

// CarpenterTools returns a reference to Carpenter's Tools.
func (toolsNS) CarpenterTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "carpenter-tools"}
}

// CartographerTools returns a reference to Cartographer's Tools.
func (toolsNS) CartographerTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "cartographer-tools"}
}

// CobblerTools returns a reference to Cobbler's Tools.
func (toolsNS) CobblerTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "cobbler-tools"}
}

// CookUtensils returns a reference to Cook's Utensils.
func (toolsNS) CookUtensils() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "cook-utensils"}
}

// GlassblowerTools returns a reference to Glassblower's Tools.
func (toolsNS) GlassblowerTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "glassblower-tools"}
}

// JewelerTools returns a reference to Jeweler's Tools.
func (toolsNS) JewelerTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "jeweler-tools"}
}

// LeatherworkerTools returns a reference to Leatherworker's Tools.
func (toolsNS) LeatherworkerTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "leatherworker-tools"}
}

// MasonTools returns a reference to Mason's Tools.
func (toolsNS) MasonTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "mason-tools"}
}

// PainterSupplies returns a reference to Painter's Supplies.
func (toolsNS) PainterSupplies() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "painter-supplies"}
}

// PotterTools returns a reference to Potter's Tools.
func (toolsNS) PotterTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "potter-tools"}
}

// SmithTools returns a reference to Smith's Tools.
func (toolsNS) SmithTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "smith-tools"}
}

// TinkerTools returns a reference to Tinker's Tools.
func (toolsNS) TinkerTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "tinker-tools"}
}

// WeaverTools returns a reference to Weaver's Tools.
func (toolsNS) WeaverTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "weaver-tools"}
}

// WoodcarverTools returns a reference to Woodcarver's Tools.
func (toolsNS) WoodcarverTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "woodcarver-tools"}
}

// Gaming Sets

// DiceSet returns a reference to a Dice Set.
func (toolsNS) DiceSet() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "dice-set"}
}

// DragonchessSet returns a reference to a Dragonchess Set.
func (toolsNS) DragonchessSet() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "dragonchess-set"}
}

// PlayingCardSet returns a reference to a Playing Card Set.
func (toolsNS) PlayingCardSet() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "playing-card-set"}
}

// ThreeDragonAnte returns a reference to a Three-Dragon Ante Set.
func (toolsNS) ThreeDragonAnte() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "three-dragon-ante"}
}

// Musical Instruments

// Bagpipes returns a reference to Bagpipes.
func (toolsNS) Bagpipes() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "bagpipes"}
}

// Drum returns a reference to a Drum.
func (toolsNS) Drum() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "drum"}
}

// Dulcimer returns a reference to a Dulcimer.
func (toolsNS) Dulcimer() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "dulcimer"}
}

// Flute returns a reference to a Flute.
func (toolsNS) Flute() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "flute"}
}

// Lute returns a reference to a Lute.
func (toolsNS) Lute() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "lute"}
}

// Lyre returns a reference to a Lyre.
func (toolsNS) Lyre() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "lyre"}
}

// Horn returns a reference to a Horn.
func (toolsNS) Horn() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "horn"}
}

// PanFlute returns a reference to a Pan Flute.
func (toolsNS) PanFlute() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "pan-flute"}
}

// Shawm returns a reference to a Shawm.
func (toolsNS) Shawm() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "shawm"}
}

// Viol returns a reference to a Viol.
func (toolsNS) Viol() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "viol"}
}

// Other Tools

// DisguiseKit returns a reference to a Disguise Kit.
func (toolsNS) DisguiseKit() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "disguise-kit"}
}

// ForgeryKit returns a reference to a Forgery Kit.
func (toolsNS) ForgeryKit() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "forgery-kit"}
}

// HerbalismKit returns a reference to a Herbalism Kit.
func (toolsNS) HerbalismKit() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "herbalism-kit"}
}

// NavigatorTools returns a reference to Navigator's Tools.
func (toolsNS) NavigatorTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "navigator-tools"}
}

// PoisonerKit returns a reference to a Poisoner's Kit.
func (toolsNS) PoisonerKit() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "poisoner-kit"}
}

// ThievesTools returns a reference to Thieves' Tools.
func (toolsNS) ThievesTools() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "thieves-tools"}
}

// VehiclesLand returns a reference to Land Vehicles proficiency.
func (toolsNS) VehiclesLand() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "vehicles-land"}
}

// VehiclesWater returns a reference to Water Vehicles proficiency.
func (toolsNS) VehiclesWater() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeTools, ID: "vehicles-water"}
}
