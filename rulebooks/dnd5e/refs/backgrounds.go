//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Background singletons - unexported for controlled access via methods
var (
	// Base Backgrounds
	backgroundAcolyte      = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "acolyte"}
	backgroundCharlatan    = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "charlatan"}
	backgroundCriminal     = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "criminal"}
	backgroundEntertainer  = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "entertainer"}
	backgroundFolkHero     = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "folk-hero"}
	backgroundGuildArtisan = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "guild-artisan"}
	backgroundHermit       = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "hermit"}
	backgroundNoble        = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "noble"}
	backgroundOutlander    = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "outlander"}
	backgroundSage         = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "sage"}
	backgroundSailor       = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "sailor"}
	backgroundSoldier      = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "soldier"}
	backgroundUrchin       = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "urchin"}

	// Variants
	backgroundSpy           = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "spy"}
	backgroundPirate        = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "pirate"}
	backgroundKnight        = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "knight"}
	backgroundGuildMerchant = &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "guild-merchant"}
)

// Backgrounds provides type-safe, discoverable references to D&D 5e backgrounds.
// Use IDE autocomplete: refs.Backgrounds.<tab> to discover available backgrounds.
// Methods return singleton pointers enabling identity comparison (ref == refs.Backgrounds.Soldier()).
var Backgrounds = backgroundsNS{}

type backgroundsNS struct{}

// Base Backgrounds
func (n backgroundsNS) Acolyte() *core.Ref      { return backgroundAcolyte }
func (n backgroundsNS) Charlatan() *core.Ref    { return backgroundCharlatan }
func (n backgroundsNS) Criminal() *core.Ref     { return backgroundCriminal }
func (n backgroundsNS) Entertainer() *core.Ref  { return backgroundEntertainer }
func (n backgroundsNS) FolkHero() *core.Ref     { return backgroundFolkHero }
func (n backgroundsNS) GuildArtisan() *core.Ref { return backgroundGuildArtisan }
func (n backgroundsNS) Hermit() *core.Ref       { return backgroundHermit }
func (n backgroundsNS) Noble() *core.Ref        { return backgroundNoble }
func (n backgroundsNS) Outlander() *core.Ref    { return backgroundOutlander }
func (n backgroundsNS) Sage() *core.Ref         { return backgroundSage }
func (n backgroundsNS) Sailor() *core.Ref       { return backgroundSailor }
func (n backgroundsNS) Soldier() *core.Ref      { return backgroundSoldier }
func (n backgroundsNS) Urchin() *core.Ref       { return backgroundUrchin }

// Variants
func (n backgroundsNS) Spy() *core.Ref           { return backgroundSpy }
func (n backgroundsNS) Pirate() *core.Ref        { return backgroundPirate }
func (n backgroundsNS) Knight() *core.Ref        { return backgroundKnight }
func (n backgroundsNS) GuildMerchant() *core.Ref { return backgroundGuildMerchant }
