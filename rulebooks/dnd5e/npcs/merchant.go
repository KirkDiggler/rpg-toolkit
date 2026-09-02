package npcs

import (
	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MerchantConfig is a vendor config for NewMerchant. It carries exactly the
// same required identity, capabilities, policy, and inventory fields
// VendorConfig does — a merchant is a vendor, not a second content shape.
type MerchantConfig = VendorConfig

// NewMerchant builds a D&D merchant vendor.
//
// A non-nil config is validated exactly as strictly as NewVendor validates
// one today: a config with a missing required field (an empty Ref, empty
// inventory) still errors. Nothing here silently fills in a gap left by an
// explicit, incomplete config.
//
// config == nil is a different, explicit request: the caller's
// can't-happen-by-accident signal for "give me the toolkit's own default."
// That default is authored here, in the toolkit, only because no host-side
// NPC-authoring layer exists yet to own it instead (docs/ideas/world-npcs/
// design.md's 2026-09-02 amendment). When one exists, NewMerchant(config)
// with a real config becomes the normal path, and this nil branch stays
// exactly what it always was: a demo convenience, not a stand-in for a
// repository or catalog.
func NewMerchant(config *MerchantConfig) (*Vendor, error) {
	if config == nil {
		return NewVendor(demoMerchantConfig())
	}

	return NewVendor(*config)
}

// demoMerchantConfig is the toolkit's own default merchant: a concrete
// identity, the vendor capability, and a small demo stock list. Built
// through the same NewVendor(config) path a caller-authored merchant is —
// not a second, competing construction route.
//
// This is a bootstrap demo profile, not #1275's real vendor-inventory work —
// it exists so PlaceNPC (session, PR 4) has something concrete to place
// before any host-side NPC-authoring layer does.
func demoMerchantConfig() MerchantConfig {
	return MerchantConfig{
		NPC: npc.Config{
			Ref:         refs.NPCs.Merchant(),
			DisplayName: "Demo Merchant",
			Capabilities: []npc.Capability{
				npc.CapabilityVendor,
			},
		},
		Inventory: VendorInventoryConfig{Entries: demoMerchantStock()},
	}
}

// demoMerchantStock is the demo merchant's stock: a longsword, a longbow,
// and a bundle of arrows — the same items and Availability shape
// vendor_test.go's own "Blacksmith" fixture already uses and already proves
// resolve through the equipment registry.
func demoMerchantStock() []StockEntryData {
	return []StockEntryData{
		{
			Type: shared.EquipmentTypeWeapon,
			ID:   string(weapons.Longsword),
			Availability: Availability{
				Mode:     StockModeLimited,
				Quantity: 1,
			},
		},
		{
			Type: shared.EquipmentTypeWeapon,
			ID:   string(weapons.Longbow),
			Availability: Availability{
				Mode:     StockModeLimited,
				Quantity: 1,
			},
		},
		{
			Type: shared.EquipmentTypeAmmunition,
			ID:   string(ammunition.Arrows20),
			Availability: Availability{
				Mode: StockModeUnlimited,
			},
		},
	}
}
