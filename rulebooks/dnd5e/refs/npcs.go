//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

var (
	npcVendor   = &core.Ref{Module: Module, Type: TypeNPCs, ID: "vendor"}
	npcMerchant = &core.Ref{Module: Module, Type: TypeNPCs, ID: "merchant"}
)

// NPCs provides type-safe, discoverable references to D&D 5e NPC profiles.
// Use IDE autocomplete: refs.NPCs.<tab> to discover available NPC refs.
var NPCs = npcsNS{}

type npcsNS struct{}

// Vendor returns the generic D&D vendor NPC profile ref.
func (n npcsNS) Vendor() *core.Ref { return npcVendor }

// Merchant returns the toolkit's demo merchant NPC profile ref, used as
// NewMerchant's built-in default when no caller-authored config is given.
func (n npcsNS) Merchant() *core.Ref { return npcMerchant }
