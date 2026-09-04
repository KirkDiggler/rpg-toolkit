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

// DecrementVendorStockSuite covers the runtime mutation Trade needs to
// actually spend a unit of a placed vendor's stock, as opposed to
// VendorInventoryFromNPCData's read-only resolve (vendor_test.go,
// interact.go's own consumer).
type DecrementVendorStockSuite struct {
	suite.Suite
}

func TestDecrementVendorStockSuite(t *testing.T) {
	suite.Run(t, new(DecrementVendorStockSuite))
}

// merchantData builds an *npc.Data with a two-quantity limited longsword row
// and an unlimited arrows row, so a suite can decrement partway, decrement to
// exact zero, and prove unlimited stays untouched, from one fixture.
func (s *DecrementVendorStockSuite) merchantData() *npc.Data {
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
						Mode: npcs.StockModeLimited, Quantity: 2,
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

func (s *DecrementVendorStockSuite) stockOf(data *npc.Data, itemID string) npcs.Availability {
	var inventory npcs.VendorInventoryData
	s.Require().NoError(json.Unmarshal(data.Inventory, &inventory))
	for _, entry := range inventory.Entries {
		if entry.ID == itemID {
			return entry.Availability
		}
	}
	s.Failf("no such stock entry", "item %q", itemID)
	return npcs.Availability{}
}

func (s *DecrementVendorStockSuite) TestPartialDecrementLeavesRemainder() {
	data := s.merchantData()

	err := npcs.DecrementVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)
	s.Require().NoError(err)

	avail := s.stockOf(data, string(weapons.Longsword))
	s.Equal(npcs.StockModeLimited, avail.Mode)
	s.Equal(1, avail.Quantity)
}

func (s *DecrementVendorStockSuite) TestExactDecrementReachesZero() {
	data := s.merchantData()

	err := npcs.DecrementVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), 2)
	s.Require().NoError(err)

	avail := s.stockOf(data, string(weapons.Longsword))
	s.Equal(0, avail.Quantity)
}

func (s *DecrementVendorStockSuite) TestInsufficientQuantityIsRefused() {
	data := s.merchantData()

	err := npcs.DecrementVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), 3)
	s.Require().ErrorIs(err, npcs.ErrOutOfStock)

	// Refused atomically: the row is untouched by the failed attempt.
	avail := s.stockOf(data, string(weapons.Longsword))
	s.Equal(2, avail.Quantity)
}

func (s *DecrementVendorStockSuite) TestUnstockedItemIsOutOfStock() {
	data := s.merchantData()

	err := npcs.DecrementVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Greatsword), 1)
	s.Require().ErrorIs(err, npcs.ErrOutOfStock)
}

func (s *DecrementVendorStockSuite) TestUnlimitedRowIsUntouchedAndStillSucceeds() {
	data := s.merchantData()

	err := npcs.DecrementVendorStock(data, shared.EquipmentTypeAmmunition, string(ammunition.Arrows20), 500)
	s.Require().NoError(err)

	avail := s.stockOf(data, string(ammunition.Arrows20))
	s.Equal(npcs.StockModeUnlimited, avail.Mode)
	s.Equal(0, avail.Quantity, "unlimited rows never carry a quantity")
}

// TestNonpositiveQuantityIsRefused pins the guard against decrementing by a
// zero or negative amount — a negative quantity here would otherwise ADD
// stock back rather than spend it, which is the same "reject, don't
// silently reinterpret" rule AddInventoryItem applies on the buyer's side.
func (s *DecrementVendorStockSuite) TestNonpositiveQuantityIsRefused() {
	data := s.merchantData()

	for _, quantity := range []int{0, -1} {
		err := npcs.DecrementVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), quantity)
		s.Require().Error(err)
	}

	// Untouched by either rejected attempt.
	avail := s.stockOf(data, string(weapons.Longsword))
	s.Equal(2, avail.Quantity)
}

func (s *DecrementVendorStockSuite) TestNilDataIsRefused() {
	err := npcs.DecrementVendorStock(nil, shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)
	s.Require().ErrorIs(err, npcs.ErrNoVendorNPC)
}

func (s *DecrementVendorStockSuite) TestNoInventoryIsRefused() {
	data := &npc.Data{
		Ref: refs.NPCs.Merchant(), DisplayName: "No Stock",
		Capabilities: []npc.Capability{npc.CapabilityVendor},
	}
	err := npcs.DecrementVendorStock(data, shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)
	s.Require().ErrorIs(err, npcs.ErrNoInventory)
}
