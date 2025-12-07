//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Tools provides type-safe, discoverable references to D&D 5e tools.
// Use IDE autocomplete: refs.Tools.<tab> to discover available tools.
var Tools = toolsNS{ns{TypeTools}}

type toolsNS struct{ ns }

// Artisan's Tools
func (n toolsNS) AlchemistSupplies() *core.Ref    { return n.ref("alchemist-supplies") }
func (n toolsNS) BrewerSupplies() *core.Ref       { return n.ref("brewer-supplies") }
func (n toolsNS) CalligrapherSupplies() *core.Ref { return n.ref("calligrapher-supplies") }
func (n toolsNS) CarpenterTools() *core.Ref       { return n.ref("carpenter-tools") }
func (n toolsNS) CartographerTools() *core.Ref    { return n.ref("cartographer-tools") }
func (n toolsNS) CobblerTools() *core.Ref         { return n.ref("cobbler-tools") }
func (n toolsNS) CookUtensils() *core.Ref         { return n.ref("cook-utensils") }
func (n toolsNS) GlassblowerTools() *core.Ref     { return n.ref("glassblower-tools") }
func (n toolsNS) JewelerTools() *core.Ref         { return n.ref("jeweler-tools") }
func (n toolsNS) LeatherworkerTools() *core.Ref   { return n.ref("leatherworker-tools") }
func (n toolsNS) MasonTools() *core.Ref           { return n.ref("mason-tools") }
func (n toolsNS) PainterSupplies() *core.Ref      { return n.ref("painter-supplies") }
func (n toolsNS) PotterTools() *core.Ref          { return n.ref("potter-tools") }
func (n toolsNS) SmithTools() *core.Ref           { return n.ref("smith-tools") }
func (n toolsNS) TinkerTools() *core.Ref          { return n.ref("tinker-tools") }
func (n toolsNS) WeaverTools() *core.Ref          { return n.ref("weaver-tools") }
func (n toolsNS) WoodcarverTools() *core.Ref      { return n.ref("woodcarver-tools") }

// Gaming Sets
func (n toolsNS) DiceSet() *core.Ref         { return n.ref("dice-set") }
func (n toolsNS) DragonchessSet() *core.Ref  { return n.ref("dragonchess-set") }
func (n toolsNS) PlayingCardSet() *core.Ref  { return n.ref("playing-card-set") }
func (n toolsNS) ThreeDragonAnte() *core.Ref { return n.ref("three-dragon-ante") }

// Musical Instruments
func (n toolsNS) Bagpipes() *core.Ref { return n.ref("bagpipes") }
func (n toolsNS) Drum() *core.Ref     { return n.ref("drum") }
func (n toolsNS) Dulcimer() *core.Ref { return n.ref("dulcimer") }
func (n toolsNS) Flute() *core.Ref    { return n.ref("flute") }
func (n toolsNS) Lute() *core.Ref     { return n.ref("lute") }
func (n toolsNS) Lyre() *core.Ref     { return n.ref("lyre") }
func (n toolsNS) Horn() *core.Ref     { return n.ref("horn") }
func (n toolsNS) PanFlute() *core.Ref { return n.ref("pan-flute") }
func (n toolsNS) Shawm() *core.Ref    { return n.ref("shawm") }
func (n toolsNS) Viol() *core.Ref     { return n.ref("viol") }

// Other Tools
func (n toolsNS) DisguiseKit() *core.Ref    { return n.ref("disguise-kit") }
func (n toolsNS) ForgeryKit() *core.Ref     { return n.ref("forgery-kit") }
func (n toolsNS) HerbalismKit() *core.Ref   { return n.ref("herbalism-kit") }
func (n toolsNS) NavigatorTools() *core.Ref { return n.ref("navigator-tools") }
func (n toolsNS) PoisonerKit() *core.Ref    { return n.ref("poisoner-kit") }
func (n toolsNS) ThievesTools() *core.Ref   { return n.ref("thieves-tools") }
func (n toolsNS) VehiclesLand() *core.Ref   { return n.ref("vehicles-land") }
func (n toolsNS) VehiclesWater() *core.Ref  { return n.ref("vehicles-water") }
