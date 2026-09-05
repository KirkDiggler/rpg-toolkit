package npcs_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// AddToVendorStockSuite covers Sell's vendor-side stock mutation — the
// mirror of DecrementVendorStockSuite.
type AddToVendorStockSuite struct {
	suite.Suite
}

func TestAddToVendorStockSuite(t *testing.T) {
	suite.Run(t, new(AddToVendorStockSuite))
}

// merchantData builds an *npc.Data with a one-quantity limited longsword
// row and an unlimited arrows row — enough to exercise incrementing an
// existing row, appending a brand-new one, and unlimited rows staying
// untouched, from one fixture.
func (s *AddToVendorStockSuite) merchantData() *npc.Data {
	vendor, err := npcs.NewVendor(npcs.VendorConfig{
		NPC: npc.Config{
			Ref:          refs.NPCs.Merchant(),
			DisplayName:  "Test Merchant",
			Capabilities: []npc.Capability{npc.CapabilityVendor},
		},
		Inventory: npcs.VendorInventoryConfig{
			Entries: []npcs.StockEntryData{
				{
					Type: shared.EquipmentTypeWeapon,
					ID:   string(weapons.Longsword),
					Availability: npcs.Availability{
						Mode: npcs.StockModeLimited, Quantity: 1,
					},
				},
				{
					Type: shared.EquipmentTypeAmmunition,
					ID:   string(ammunition.Arrows20),
					Availability: npcs.Availability{
						Mode: npcs.StockModeUnlimited,
					},
				},
			},
		},
	})
	s.Require().NoError(err)
	return vendor.NPC().ToData()
}

func (s *AddToVendorStockSuite) findStockEntry(data *npc.Data, itemID string) (npcs.StockEntryData, bool) {
	var inventory npcs.VendorInventoryData
	s.Require().NoError(json.Unmarshal(data.Inventory, &inventory))
	for _, entry := range inventory.Entries {
		if entry.ID == itemID {
			return entry, true
		}
	}
	return npcs.StockEntryData{}, false
}

func (s *AddToVendorStockSuite) TestIncrementingAnExistingRowLeavesItsTagUntouched() {
	data := s.merchantData()

	err := npcs.AddToVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), 2)
	s.Require().NoError(err)

	entry, ok := s.findStockEntry(data, string(weapons.Longsword))
	s.Require().True(ok)
	s.Equal(npcs.StockModeLimited, entry.Availability.Mode)
	s.Equal(3, entry.Availability.Quantity)
	s.False(entry.PlayerSold, "an authored row must stay authored when incremented, not retagged")
}

// TestAppendingANewRowTagsItPlayerSold pins §2 of the design: an item the
// vendor never stocked becomes a brand-new row, tagged PlayerSold — so it
// is distinguishable from authored stock, and buyable back through the
// ordinary buy path (indistinguishable to DecrementVendorStock/PriceOf).
func (s *AddToVendorStockSuite) TestAppendingANewRowTagsItPlayerSold() {
	data := s.merchantData()

	err := npcs.AddToVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Greatsword), 1)
	s.Require().NoError(err)

	entry, ok := s.findStockEntry(data, string(weapons.Greatsword))
	s.Require().True(ok)
	s.Equal(npcs.StockModeLimited, entry.Availability.Mode)
	s.Equal(1, entry.Availability.Quantity)
	s.True(entry.PlayerSold)

	// The write-back is still valid data a normal resolve can read.
	inventory, ok, err := npcs.VendorInventoryFromNPCData(data)
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Len(inventory.View().Entries, 3)
}

func (s *AddToVendorStockSuite) TestUnlimitedRowIsUntouchedAndStillSucceeds() {
	data := s.merchantData()

	err := npcs.AddToVendorStock(data, shared.EquipmentTypeAmmunition, string(ammunition.Arrows20), 500)
	s.Require().NoError(err)

	entry, ok := s.findStockEntry(data, string(ammunition.Arrows20))
	s.Require().True(ok)
	s.Equal(npcs.StockModeUnlimited, entry.Availability.Mode)
	s.Equal(0, entry.Availability.Quantity, "unlimited rows never carry a quantity")
}

// TestNonpositiveQuantityIsRefused pins the same guard DecrementVendorStock
// applies: a zero or negative amount here would either do nothing
// meaningful or, worse, invert into a decrement by accident.
func (s *AddToVendorStockSuite) TestNonpositiveQuantityIsRefused() {
	data := s.merchantData()

	for _, quantity := range []int{0, -1} {
		err := npcs.AddToVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), quantity)
		s.Require().Error(err)
	}

	entry, ok := s.findStockEntry(data, string(weapons.Longsword))
	s.Require().True(ok)
	s.Equal(1, entry.Availability.Quantity, "untouched by either rejected attempt")
}

func (s *AddToVendorStockSuite) TestNilDataIsRefused() {
	err := npcs.AddToVendorStock(nil, shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)
	s.Require().ErrorIs(err, npcs.ErrNoVendorNPC)
}

func (s *AddToVendorStockSuite) TestNoInventoryIsRefused() {
	data := &npc.Data{
		Ref: refs.NPCs.Merchant(), DisplayName: "No Stock",
		Capabilities: []npc.Capability{npc.CapabilityVendor},
	}
	err := npcs.AddToVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)
	s.Require().ErrorIs(err, npcs.ErrNoInventory)
}
