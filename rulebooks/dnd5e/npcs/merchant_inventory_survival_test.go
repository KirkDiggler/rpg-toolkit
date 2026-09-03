// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package npcs_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// merchant_inventory_survival_test.go proves the actual gap #1444 exists to
// close: a vendor's inventory used to have no path into npc.Data at all, so
// anything that only ever copies npc.Data (session.PlaceNPC, in particular)
// silently dropped it. These tests prove the data now lives inside
// npc.NPC's own opaque Inventory bytes, and survives exactly the kind of
// whole-value struct copy session.PlaceNPC already does — not merely that
// Vendor.Inventory() happens to still report the right thing some other way.
type MerchantInventorySurvivalSuite struct {
	suite.Suite
}

func TestMerchantInventorySurvivalSuite(t *testing.T) {
	suite.Run(t, new(MerchantInventorySurvivalSuite))
}

// TestDemoMerchantStillHasItsThreeItems is the regression pin: the demo
// merchant's stock (longsword, longbow, arrows) must still resolve exactly
// as it always has, through the new opaque-field path.
func (s *MerchantInventorySurvivalSuite) TestDemoMerchantStillHasItsThreeItems() {
	vendor, err := npcs.NewMerchant(nil)
	s.Require().NoError(err)

	entries := vendor.Inventory().Entries()
	s.Require().Len(entries, 3)
	s.Equal(string(weapons.Longsword), entries[0].Equipment().EquipmentID())
	s.Equal(string(weapons.Longbow), entries[1].Equipment().EquipmentID())
	s.Equal(string(ammunition.Arrows20), entries[2].Equipment().EquipmentID())
}

// TestInventoryLivesInTheNPCsOwnOpaqueBytes proves the data actually lives
// where #1444 says it should — inside npc.NPC.Inventory() — not merely that
// Vendor.Inventory() happens to still work through some other mechanism.
// Unmarshals the raw bytes directly, bypassing Vendor.Inventory() entirely.
func (s *MerchantInventorySurvivalSuite) TestInventoryLivesInTheNPCsOwnOpaqueBytes() {
	vendor, err := npcs.NewMerchant(nil)
	s.Require().NoError(err)

	raw := vendor.NPC().Inventory()
	s.Require().NotNil(raw, "the underlying npc.NPC must carry the inventory bytes directly")

	var data npcs.VendorInventoryData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Require().Len(data.Entries, 3)
	s.Equal(string(weapons.Longsword), data.Entries[0].ID)
	s.Equal(string(weapons.Longbow), data.Entries[1].ID)
	s.Equal(string(ammunition.Arrows20), data.Entries[2].ID)
}

// TestInventorySurvivesTheExactCopySessionPlaceNPCAlreadyDoes is the actual
// gap this issue closes, proven end to end. session.PlaceNPC (write.go)
// stores a placed NPC's content as `NPC: *in.NPC` — a plain Go struct-value
// copy of *npc.Data. Before rpg-toolkit#1444, that copy carried identity and
// policy but had no field for inventory at all, so a vendor's stock vanished
// at exactly this point. Reproducing that copy here, without importing
// `session` (npcs has no reason to depend on it), proves the fix without
// needing to touch that module.
func (s *MerchantInventorySurvivalSuite) TestInventorySurvivesTheExactCopySessionPlaceNPCAlreadyDoes() {
	vendor, err := npcs.NewMerchant(nil)
	s.Require().NoError(err)

	original := vendor.NPC().ToData()

	// The exact operation write.go's PlaceNPC performs: `NPC: *in.NPC`.
	copied := *original

	reloaded, err := npc.Load(&copied)
	s.Require().NoError(err)

	var data npcs.VendorInventoryData
	s.Require().NoError(json.Unmarshal(reloaded.Inventory(), &data))
	s.Require().Len(data.Entries, 3)
	s.Equal(string(weapons.Longsword), data.Entries[0].ID)
	s.Equal(string(weapons.Longbow), data.Entries[1].ID)
	s.Equal(string(ammunition.Arrows20), data.Entries[2].ID)
}

// TestInventoryReResolvesFreshOnEveryCall pins that Inventory() has no
// cached copy of its own to go stale — each call independently unmarshals
// and resolves from the npc's own bytes, so mutating one call's result
// cannot affect the next (Vendor's own copy-out standard, extended to the
// new field).
func (s *MerchantInventorySurvivalSuite) TestInventoryReResolvesFreshOnEveryCall() {
	vendor, err := npcs.NewMerchant(nil)
	s.Require().NoError(err)

	first := vendor.Inventory()
	firstEntries := first.Entries()
	firstEntries[0] = firstEntries[1]

	second := vendor.Inventory()
	s.Equal(string(weapons.Longsword), second.Entries()[0].Equipment().EquipmentID(),
		"mutating one call's returned entries must not affect a later call")
}
