package npcs_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// DebitVendorWalletSuite covers Sell's other vendor-side primitive: a
// vendor's own buying power, checked before a sale completes. Both the nil
// (unlimited) and set (enforced) branches are real and tested — design.md
// §3's own bar: "not a decorative field."
type DebitVendorWalletSuite struct {
	suite.Suite
}

func TestDebitVendorWalletSuite(t *testing.T) {
	suite.Run(t, new(DebitVendorWalletSuite))
}

func (s *DebitVendorWalletSuite) merchantData(wallet *currency.Money) *npc.Data {
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
			},
		},
	})
	s.Require().NoError(err)
	data := vendor.NPC().ToData()

	if wallet != nil {
		var inventory npcs.VendorInventoryData
		s.Require().NoError(json.Unmarshal(data.Inventory, &inventory))
		inventory.Wallet = wallet
		marshaled, err := json.Marshal(inventory)
		s.Require().NoError(err)
		data.Inventory = marshaled
	}

	return data
}

func (s *DebitVendorWalletSuite) walletOf(data *npc.Data) *currency.Money {
	var inventory npcs.VendorInventoryData
	s.Require().NoError(json.Unmarshal(data.Inventory, &inventory))
	return inventory.Wallet
}

// TestNilWalletIsUnlimitedAndNeverChecked is the default every currently
// authored vendor gets: no Wallet at all, so any payout succeeds and
// nothing is written back.
func (s *DebitVendorWalletSuite) TestNilWalletIsUnlimitedAndNeverChecked() {
	data := s.merchantData(nil)

	err := npcs.DebitVendorWallet(data, currency.FromGold(1_000_000))
	s.Require().NoError(err)
	s.Nil(s.walletOf(data), "a nil wallet stays nil -- unlimited, not silently created")
}

func (s *DebitVendorWalletSuite) TestSetWalletIsDebitedExactly() {
	data := s.merchantData(ptr(currency.FromGold(100)))

	err := npcs.DebitVendorWallet(data, currency.FromGold(15))
	s.Require().NoError(err)
	s.Equal(currency.FromGold(85), *s.walletOf(data))
}

// TestSetWalletExhaustedRefusesThePayout is the real enforcement case
// design.md §3 insists on: a vendor with a set, exhausted purse correctly
// refuses a sell it can't afford, even though every currently authored
// vendor is nil.
func (s *DebitVendorWalletSuite) TestSetWalletExhaustedRefusesThePayout() {
	data := s.merchantData(ptr(currency.FromGold(10)))

	err := npcs.DebitVendorWallet(data, currency.FromGold(15))
	s.Require().ErrorIs(err, currency.ErrInsufficientFunds)

	// Refused atomically: the wallet is untouched by the failed attempt.
	s.Equal(currency.FromGold(10), *s.walletOf(data))
}

func (s *DebitVendorWalletSuite) TestExactBalanceReachesZero() {
	data := s.merchantData(ptr(currency.FromGold(15)))

	err := npcs.DebitVendorWallet(data, currency.FromGold(15))
	s.Require().NoError(err)
	s.Equal(currency.Money{}, *s.walletOf(data))
}

func (s *DebitVendorWalletSuite) TestNilDataIsRefused() {
	err := npcs.DebitVendorWallet(nil, currency.FromGold(1))
	s.Require().ErrorIs(err, npcs.ErrNoVendorNPC)
}

func (s *DebitVendorWalletSuite) TestNoInventoryIsRefused() {
	data := &npc.Data{
		Ref: refs.NPCs.Merchant(), DisplayName: "No Stock",
		Capabilities: []npc.Capability{npc.CapabilityVendor},
	}
	err := npcs.DebitVendorWallet(data, currency.FromGold(1))
	s.Require().ErrorIs(err, npcs.ErrNoInventory)
}

func ptr[T any](v T) *T { return &v }
