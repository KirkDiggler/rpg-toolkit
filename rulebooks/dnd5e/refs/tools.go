//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Tool singletons - unexported for controlled access via methods
var (
	// Artisan's Tools
	toolAlchemistSupplies    = &core.Ref{Module: Module, Type: TypeTools, ID: "alchemist-supplies"}
	toolBrewerSupplies       = &core.Ref{Module: Module, Type: TypeTools, ID: "brewer-supplies"}
	toolCalligrapherSupplies = &core.Ref{Module: Module, Type: TypeTools, ID: "calligrapher-supplies"}
	toolCarpenterTools       = &core.Ref{Module: Module, Type: TypeTools, ID: "carpenter-tools"}
	toolCartographerTools    = &core.Ref{Module: Module, Type: TypeTools, ID: "cartographer-tools"}
	toolCobblerTools         = &core.Ref{Module: Module, Type: TypeTools, ID: "cobbler-tools"}
	toolCookUtensils         = &core.Ref{Module: Module, Type: TypeTools, ID: "cook-utensils"}
	toolGlassblowerTools     = &core.Ref{Module: Module, Type: TypeTools, ID: "glassblower-tools"}
	toolJewelerTools         = &core.Ref{Module: Module, Type: TypeTools, ID: "jeweler-tools"}
	toolLeatherworkerTools   = &core.Ref{Module: Module, Type: TypeTools, ID: "leatherworker-tools"}
	toolMasonTools           = &core.Ref{Module: Module, Type: TypeTools, ID: "mason-tools"}
	toolPainterSupplies      = &core.Ref{Module: Module, Type: TypeTools, ID: "painter-supplies"}
	toolPotterTools          = &core.Ref{Module: Module, Type: TypeTools, ID: "potter-tools"}
	toolSmithTools           = &core.Ref{Module: Module, Type: TypeTools, ID: "smith-tools"}
	toolTinkerTools          = &core.Ref{Module: Module, Type: TypeTools, ID: "tinker-tools"}
	toolWeaverTools          = &core.Ref{Module: Module, Type: TypeTools, ID: "weaver-tools"}
	toolWoodcarverTools      = &core.Ref{Module: Module, Type: TypeTools, ID: "woodcarver-tools"}

	// Gaming Sets
	toolDiceSet         = &core.Ref{Module: Module, Type: TypeTools, ID: "dice-set"}
	toolDragonchessSet  = &core.Ref{Module: Module, Type: TypeTools, ID: "dragonchess-set"}
	toolPlayingCardSet  = &core.Ref{Module: Module, Type: TypeTools, ID: "playing-card-set"}
	toolThreeDragonAnte = &core.Ref{Module: Module, Type: TypeTools, ID: "three-dragon-ante"}

	// Musical Instruments
	toolBagpipes = &core.Ref{Module: Module, Type: TypeTools, ID: "bagpipes"}
	toolDrum     = &core.Ref{Module: Module, Type: TypeTools, ID: "drum"}
	toolDulcimer = &core.Ref{Module: Module, Type: TypeTools, ID: "dulcimer"}
	toolFlute    = &core.Ref{Module: Module, Type: TypeTools, ID: "flute"}
	toolLute     = &core.Ref{Module: Module, Type: TypeTools, ID: "lute"}
	toolLyre     = &core.Ref{Module: Module, Type: TypeTools, ID: "lyre"}
	toolHorn     = &core.Ref{Module: Module, Type: TypeTools, ID: "horn"}
	toolPanFlute = &core.Ref{Module: Module, Type: TypeTools, ID: "pan-flute"}
	toolShawm    = &core.Ref{Module: Module, Type: TypeTools, ID: "shawm"}
	toolViol     = &core.Ref{Module: Module, Type: TypeTools, ID: "viol"}

	// Other Tools
	toolDisguiseKit    = &core.Ref{Module: Module, Type: TypeTools, ID: "disguise-kit"}
	toolForgeryKit     = &core.Ref{Module: Module, Type: TypeTools, ID: "forgery-kit"}
	toolHerbalismKit   = &core.Ref{Module: Module, Type: TypeTools, ID: "herbalism-kit"}
	toolNavigatorTools = &core.Ref{Module: Module, Type: TypeTools, ID: "navigator-tools"}
	toolPoisonerKit    = &core.Ref{Module: Module, Type: TypeTools, ID: "poisoner-kit"}
	toolThievesTools   = &core.Ref{Module: Module, Type: TypeTools, ID: "thieves-tools"}
	toolVehiclesLand   = &core.Ref{Module: Module, Type: TypeTools, ID: "vehicles-land"}
	toolVehiclesWater  = &core.Ref{Module: Module, Type: TypeTools, ID: "vehicles-water"}
)

// Tools provides type-safe, discoverable references to D&D 5e tools.
// Use IDE autocomplete: refs.Tools.<tab> to discover available tools.
// Methods return singleton pointers enabling identity comparison (ref == refs.Tools.ThievesTools()).
var Tools = toolsNS{}

type toolsNS struct{}

// Artisan's Tools
func (n toolsNS) AlchemistSupplies() *core.Ref    { return toolAlchemistSupplies }
func (n toolsNS) BrewerSupplies() *core.Ref       { return toolBrewerSupplies }
func (n toolsNS) CalligrapherSupplies() *core.Ref { return toolCalligrapherSupplies }
func (n toolsNS) CarpenterTools() *core.Ref       { return toolCarpenterTools }
func (n toolsNS) CartographerTools() *core.Ref    { return toolCartographerTools }
func (n toolsNS) CobblerTools() *core.Ref         { return toolCobblerTools }
func (n toolsNS) CookUtensils() *core.Ref         { return toolCookUtensils }
func (n toolsNS) GlassblowerTools() *core.Ref     { return toolGlassblowerTools }
func (n toolsNS) JewelerTools() *core.Ref         { return toolJewelerTools }
func (n toolsNS) LeatherworkerTools() *core.Ref   { return toolLeatherworkerTools }
func (n toolsNS) MasonTools() *core.Ref           { return toolMasonTools }
func (n toolsNS) PainterSupplies() *core.Ref      { return toolPainterSupplies }
func (n toolsNS) PotterTools() *core.Ref          { return toolPotterTools }
func (n toolsNS) SmithTools() *core.Ref           { return toolSmithTools }
func (n toolsNS) TinkerTools() *core.Ref          { return toolTinkerTools }
func (n toolsNS) WeaverTools() *core.Ref          { return toolWeaverTools }
func (n toolsNS) WoodcarverTools() *core.Ref      { return toolWoodcarverTools }

// Gaming Sets
func (n toolsNS) DiceSet() *core.Ref         { return toolDiceSet }
func (n toolsNS) DragonchessSet() *core.Ref  { return toolDragonchessSet }
func (n toolsNS) PlayingCardSet() *core.Ref  { return toolPlayingCardSet }
func (n toolsNS) ThreeDragonAnte() *core.Ref { return toolThreeDragonAnte }

// Musical Instruments
func (n toolsNS) Bagpipes() *core.Ref { return toolBagpipes }
func (n toolsNS) Drum() *core.Ref     { return toolDrum }
func (n toolsNS) Dulcimer() *core.Ref { return toolDulcimer }
func (n toolsNS) Flute() *core.Ref    { return toolFlute }
func (n toolsNS) Lute() *core.Ref     { return toolLute }
func (n toolsNS) Lyre() *core.Ref     { return toolLyre }
func (n toolsNS) Horn() *core.Ref     { return toolHorn }
func (n toolsNS) PanFlute() *core.Ref { return toolPanFlute }
func (n toolsNS) Shawm() *core.Ref    { return toolShawm }
func (n toolsNS) Viol() *core.Ref     { return toolViol }

// Other Tools
func (n toolsNS) DisguiseKit() *core.Ref    { return toolDisguiseKit }
func (n toolsNS) ForgeryKit() *core.Ref     { return toolForgeryKit }
func (n toolsNS) HerbalismKit() *core.Ref   { return toolHerbalismKit }
func (n toolsNS) NavigatorTools() *core.Ref { return toolNavigatorTools }
func (n toolsNS) PoisonerKit() *core.Ref    { return toolPoisonerKit }
func (n toolsNS) ThievesTools() *core.Ref   { return toolThievesTools }
func (n toolsNS) VehiclesLand() *core.Ref   { return toolVehiclesLand }
func (n toolsNS) VehiclesWater() *core.Ref  { return toolVehiclesWater }
