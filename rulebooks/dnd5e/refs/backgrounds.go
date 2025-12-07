package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Backgrounds provides type-safe, discoverable references to D&D 5e backgrounds.
// Use IDE autocomplete: refs.Backgrounds.<tab> to discover available backgrounds.
var Backgrounds = backgroundsNS{}

type backgroundsNS struct{}

// Acolyte returns a reference to the Acolyte background.
func (backgroundsNS) Acolyte() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "acolyte"}
}

// Charlatan returns a reference to the Charlatan background.
func (backgroundsNS) Charlatan() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "charlatan"}
}

// Criminal returns a reference to the Criminal background.
func (backgroundsNS) Criminal() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "criminal"}
}

// Entertainer returns a reference to the Entertainer background.
func (backgroundsNS) Entertainer() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "entertainer"}
}

// FolkHero returns a reference to the Folk Hero background.
func (backgroundsNS) FolkHero() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "folk-hero"}
}

// GuildArtisan returns a reference to the Guild Artisan background.
func (backgroundsNS) GuildArtisan() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "guild-artisan"}
}

// Hermit returns a reference to the Hermit background.
func (backgroundsNS) Hermit() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "hermit"}
}

// Noble returns a reference to the Noble background.
func (backgroundsNS) Noble() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "noble"}
}

// Outlander returns a reference to the Outlander background.
func (backgroundsNS) Outlander() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "outlander"}
}

// Sage returns a reference to the Sage background.
func (backgroundsNS) Sage() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "sage"}
}

// Sailor returns a reference to the Sailor background.
func (backgroundsNS) Sailor() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "sailor"}
}

// Soldier returns a reference to the Soldier background.
func (backgroundsNS) Soldier() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "soldier"}
}

// Urchin returns a reference to the Urchin background.
func (backgroundsNS) Urchin() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "urchin"}
}

// Spy returns a reference to the Spy background variant (Criminal).
func (backgroundsNS) Spy() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "spy"}
}

// Pirate returns a reference to the Pirate background variant (Sailor).
func (backgroundsNS) Pirate() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "pirate"}
}

// Knight returns a reference to the Knight background variant (Noble).
func (backgroundsNS) Knight() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "knight"}
}

// GuildMerchant returns a reference to the Guild Merchant background variant (Guild Artisan).
func (backgroundsNS) GuildMerchant() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeBackgrounds, ID: "guild-merchant"}
}
