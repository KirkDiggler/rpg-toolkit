//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Backgrounds provides type-safe, discoverable references to D&D 5e backgrounds.
// Use IDE autocomplete: refs.Backgrounds.<tab> to discover available backgrounds.
var Backgrounds = backgroundsNS{ns{TypeBackgrounds}}

type backgroundsNS struct{ ns }

// Base Backgrounds
func (n backgroundsNS) Acolyte() *core.Ref      { return n.ref("acolyte") }
func (n backgroundsNS) Charlatan() *core.Ref    { return n.ref("charlatan") }
func (n backgroundsNS) Criminal() *core.Ref     { return n.ref("criminal") }
func (n backgroundsNS) Entertainer() *core.Ref  { return n.ref("entertainer") }
func (n backgroundsNS) FolkHero() *core.Ref     { return n.ref("folk-hero") }
func (n backgroundsNS) GuildArtisan() *core.Ref { return n.ref("guild-artisan") }
func (n backgroundsNS) Hermit() *core.Ref       { return n.ref("hermit") }
func (n backgroundsNS) Noble() *core.Ref        { return n.ref("noble") }
func (n backgroundsNS) Outlander() *core.Ref    { return n.ref("outlander") }
func (n backgroundsNS) Sage() *core.Ref         { return n.ref("sage") }
func (n backgroundsNS) Sailor() *core.Ref       { return n.ref("sailor") }
func (n backgroundsNS) Soldier() *core.Ref      { return n.ref("soldier") }
func (n backgroundsNS) Urchin() *core.Ref       { return n.ref("urchin") }

// Variants
func (n backgroundsNS) Spy() *core.Ref           { return n.ref("spy") }
func (n backgroundsNS) Pirate() *core.Ref        { return n.ref("pirate") }
func (n backgroundsNS) Knight() *core.Ref        { return n.ref("knight") }
func (n backgroundsNS) GuildMerchant() *core.Ref { return n.ref("guild-merchant") }
