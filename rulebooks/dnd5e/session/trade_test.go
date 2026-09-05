// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TradeTestSuite covers Trade: the verb that actually moves an item, over
// the same reach/visibility check Interact already performs (rpg-project#369,
// design.md at rpg-project#370). Fixture setup mirrors InteractTestSuite and
// InteractInventoryTestSuite exactly — same hexWorld, same alice-at-(0,0)
// placement, same npcs.NewMerchant(nil) demo stock (1 longsword, 1 longbow,
// unlimited arrows).
type TradeTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestTradeSuite(t *testing.T) { suite.Run(t, new(TradeTestSuite)) }

func (s *TradeTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

// placeVendor places npcs.NewMerchant(nil)'s demo stock (1 longsword, 1
// longbow — both limited — plus unlimited arrows) at the given position.
func (s *TradeTestSuite) placeVendor(at spatial.Position) {
	vendor, err := npcs.NewMerchant(nil)
	s.Require().NoError(err)

	_, err = s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: at, NPC: vendor.NPC().ToData(),
	})
	s.Require().NoError(err)
}

// fund sets alice's wallet directly, bypassing Trade — the fixture
// character starts broke (Wallet's zero value), and Wave 4 requires a real
// balance for any test that expects a purchase to actually succeed.
func (s *TradeTestSuite) fund(amount currency.Money) {
	s.characters.byID["alice"].Wallet = amount
}

// own gives alice quantity units of id directly, bypassing Trade — the
// fixture character starts with no inventory, and Sell requires the actor
// to already own what it's selling.
func (s *TradeTestSuite) own(itemType shared.EquipmentType, id string, quantity int) {
	s.characters.byID["alice"].Inventory = append(s.characters.byID["alice"].Inventory,
		character.InventoryItemData{Type: itemType, ID: id, Quantity: quantity})
}

// placeVendorWithWallet is placeVendor plus a set (enforced) vendor Wallet —
// design.md §3's "not a decorative field" bar requires exercising this
// branch directly, since npcs.NewMerchant's demo stock has none.
func (s *TradeTestSuite) placeVendorWithWallet(at spatial.Position, wallet currency.Money) {
	vendor, err := npcs.NewVendor(npcs.VendorConfig{
		NPC: npc.Config{
			Ref: refs.NPCs.Merchant(), DisplayName: "Wealth-Limited Merchant",
			Capabilities: []npc.Capability{npc.CapabilityVendor},
		},
		Inventory: npcs.VendorInventoryConfig{
			Entries: []npcs.StockEntryData{{
				Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longbow),
				Availability: npcs.Availability{Mode: npcs.StockModeLimited, Quantity: 1},
			}},
		},
	})
	s.Require().NoError(err)
	data := vendor.NPC().ToData()

	var inventory npcs.VendorInventoryData
	s.Require().NoError(json.Unmarshal(data.Inventory, &inventory))
	inventory.Wallet = &wallet
	marshaled, err := json.Marshal(inventory)
	s.Require().NoError(err)
	data.Inventory = marshaled

	_, err = s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: at, NPC: data,
	})
	s.Require().NoError(err)
}

// TestTradeBuysTheListedItem is the headline: the longsword lands in
// alice's saved character data over the real verb, one unit, the vendor's
// stock reflects the decrement in the same response, and alice's wallet is
// charged exactly the server-computed price.
func (s *TradeTestSuite) TestTradeBuysTheListedItem() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	price := currency.FromGold(15) // weapons.Longsword's real Cost
	s.fund(price)

	out, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: price},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().NoError(err)
	s.NotZero(out.Seq)

	// The vendor's displayed stock reflects the decrement in this response —
	// no second Interact round trip needed. An exhausted limited row is
	// REMOVED (npcs.DecrementVendorStock's fix, #1508) rather than shown at
	// zero, since a Limited row at zero is not a value this vendor's own
	// stock validation accepts.
	for _, entry := range out.Descriptor.Inventory {
		s.NotEqual(string(weapons.Longsword), entry.ID, "the now-exhausted row must be gone, not shown at zero")
	}

	// Persisted: fakeCharacters.SaveCharacter already landed the real write
	// saveCharacterRecord makes, not an in-memory-only mutation.
	saved := s.characters.byID["alice"]
	s.Require().Len(saved.Inventory, 1)
	s.Equal(string(weapons.Longsword), saved.Inventory[0].ID)
	s.Equal(shared.EquipmentTypeWeapon, saved.Inventory[0].Type)
	s.Equal(1, saved.Inventory[0].Quantity)
	s.Equal(currency.Money{}, saved.Wallet, "the exact price was charged, leaving nothing behind")
}

// TestTradeQuantityAboveOneIsSupported pins design.md's 2026-09-04 amendment:
// Quantity is not capped at 1 — a single Receive line can name any amount up
// to what the vendor holds — and that the price scales with quantity
// (unitPrice * Quantity), not a flat per-line charge.
func (s *TradeTestSuite) TestTradeQuantityAboveOneIsSupported() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	price := currency.FromGold(500) // ammunition.Arrows20's "1 gp" * 500
	s.fund(price)

	out, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: price},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeAmmunition, ID: string(ammunition.Arrows20), Quantity: 500},
		}},
	})
	s.Require().NoError(err)

	saved := s.characters.byID["alice"]
	s.Require().Len(saved.Inventory, 1)
	s.Equal(500, saved.Inventory[0].Quantity)
	s.Equal(currency.Money{}, saved.Wallet)

	// Unlimited stock stays unlimited — no quantity displayed.
	for _, entry := range out.Descriptor.Inventory {
		if entry.ID == string(ammunition.Arrows20) {
			s.Equal(npcs.StockModeUnlimited, entry.Mode)
		}
	}
}

// TestWrongPriceIsRefused pins Wave 4's whole point: the server never
// trusts an offered amount, only checks it. An amount that would actually
// be affordable is still refused if it does not exactly match the real
// price.
func (s *TradeTestSuite) TestWrongPriceIsRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	s.fund(currency.FromGold(1000)) // plenty — affordability is not the question here

	for _, tc := range []struct {
		name  string
		offer currency.Money
	}{
		{"too little", currency.FromCopper(1)},
		{"too much", currency.FromGold(16)},
		{"zero", currency.Money{}},
	} {
		s.Run(tc.name, func() {
			_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
				Session: "sess", Actor: "alice", Target: "vendor",
				Give: session.TradeOffer{Currency: tc.offer},
				Receive: session.TradeOffer{Items: []session.TradeItem{
					{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
				}},
			})
			s.Require().Error(err)
			s.ErrorIs(err, session.ErrWrongPrice)
		})
	}
}

// TestInsufficientFundsIsRefused: the offered amount is correct — it
// exactly matches the real price — but alice's wallet cannot cover it.
// Distinct from TestWrongPriceIsRefused's refusal, and pinned separately so
// a host can tell a player "that's not the price" from "you can't afford
// that."
func (s *TradeTestSuite) TestInsufficientFundsIsRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	price := currency.FromGold(15)
	s.fund(currency.FromCopper(price.Copper - 1)) // one copper short

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: price},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrInsufficientFunds)

	// Refused atomically: nothing landed, nothing was charged. The vendor's
	// stock was already decremented IN MEMORY by this point in the
	// pipeline (DecrementVendorStock runs before the wallet check) — this
	// pins that an abandoned write scope never reaches commit, the same
	// "rejected mid-pipeline costs nothing durable" guarantee
	// TestBuyingPastAvailableQuantityIsOutOfStock already proves for
	// ErrOutOfStock, now checked for a failure one step later.
	s.Empty(s.characters.byID["alice"].Inventory)
	s.Equal(currency.FromCopper(price.Copper-1), s.characters.byID["alice"].Wallet)

	interacted, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().NoError(err)
	found := false
	for _, entry := range interacted.Descriptor.Inventory {
		if entry.ID == string(weapons.Longsword) {
			found = true
			s.Equal(1, entry.Quantity, "the in-memory decrement must not have been persisted")
		}
	}
	s.True(found, "the longsword row must still be there, not consumed by the failed purchase")
}

// TestBarterIsRefused pins that both sides naming items (rpg-toolkit#1537
// gave Give.Items a legal meaning — sell — but never together with
// Receive.Items) is still refused, now under ErrInvalidTradeOffer rather
// than the retired ErrGiveNotSupported.
func (s *TradeTestSuite) TestBarterIsRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Dagger), Quantity: 1},
		}},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrInvalidTradeOffer)
}

func (s *TradeTestSuite) TestReceiveShapeViolationsAreRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	s.Run("both sides empty", func() {
		_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
			Session: "sess", Actor: "alice", Target: "vendor",
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrInvalidTradeOffer)
	})

	s.Run("two distinct items", func() {
		_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
			Session: "sess", Actor: "alice", Target: "vendor",
			Receive: session.TradeOffer{Items: []session.TradeItem{
				{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
				{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longbow), Quantity: 1},
			}},
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrInvalidTradeOffer)
	})

	s.Run("nonpositive quantity", func() {
		_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
			Session: "sess", Actor: "alice", Target: "vendor",
			Receive: session.TradeOffer{Items: []session.TradeItem{
				{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 0},
			}},
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrInvalidTradeOffer)
	})
}

// TestBuyingPastAvailableQuantityIsOutOfStock covers both the direct
// over-ask and the already-exhausted row a first purchase leaves behind.
// Every offer here names the correct price for its own quantity — the
// point of this test is the stock check, not the price check, so price
// must not be what's under test rejects any of these.
func (s *TradeTestSuite) TestBuyingPastAvailableQuantityIsOutOfStock() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	unit := currency.FromGold(15) // weapons.Longsword's real Cost
	s.fund(unit)                  // enough for exactly one — never enough for two

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: currency.FromCopper(unit.Copper * 2)},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrOutOfStock)

	_, err = s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: unit},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().NoError(err, "the rejected over-ask must not have partially decremented the row")

	_, err = s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: unit},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().Error(err, "a row already exhausted by the first purchase is out of stock too")
	s.ErrorIs(err, session.ErrOutOfStock)
}

func (s *TradeTestSuite) TestNonVendorTargetIsRefused() {
	built, err := npc.New(npc.Config{Ref: refs.NPCs.Vendor(), DisplayName: "Villager"})
	s.Require().NoError(err)
	_, err = s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "villager", Position: spatial.Position{X: 1, Y: 0}, NPC: built.ToData(),
	})
	s.Require().NoError(err)

	_, err = s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "villager",
		Give: session.TradeOffer{Currency: currency.FromGold(15)},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotAVendor)
}

// TestOutOfRangeRefusalPropagates pins that Trade reuses encounter.Interact's
// own reach check rather than re-implementing it — the same proof
// InteractTestSuite.TestOutOfRangeRefusalPropagates makes for Interact. The
// price offered is correct so that the price check does not shadow the
// reach check this test is actually about.
func (s *TradeTestSuite) TestOutOfRangeRefusalPropagates() {
	s.placeVendor(spatial.Position{X: 4, Y: 4})

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: currency.FromGold(15)},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrOutOfRange)
}

func (s *TradeTestSuite) TestNilInputRejected() {
	_, err := s.mgr.Trade(context.Background(), nil)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNilInput)
}

// TestTradeSellsAnOwnedItem is Sell's headline (rpg-toolkit#1537): alice
// sells a longsword she owns to a vendor that already stocks longswords —
// the item leaves her inventory, her wallet is credited exactly the price,
// and the vendor's EXISTING row is incremented (not retagged PlayerSold,
// since it was already authored stock).
func (s *TradeTestSuite) TestTradeSellsAnOwnedItem() {
	s.placeVendor(spatial.Position{X: 1, Y: 0}) // demo stock already has 1 longsword
	s.own(shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)
	price := currency.FromGold(15)

	out, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
		Receive: session.TradeOffer{Currency: price},
	})
	s.Require().NoError(err)
	s.NotZero(out.Seq)

	saved := s.characters.byID["alice"]
	s.Empty(saved.Inventory, "the sold item must leave alice's inventory")
	s.Equal(price, saved.Wallet, "credited exactly the server-computed price")

	found := false
	for _, entry := range out.Descriptor.Inventory {
		if entry.ID == string(weapons.Longsword) {
			found = true
			s.Equal(2, entry.Quantity, "the vendor's existing row is incremented, not replaced")
			s.False(entry.PlayerSold, "an already-authored row must not be retagged")
		}
	}
	s.True(found)
}

// TestSellAppendsANewPlayerSoldRow covers the other branch of
// AddToVendorStock: an item the vendor never stocked becomes a brand-new
// row, tagged PlayerSold, and immediately buyable back through the
// ordinary buy path — design.md §1's "buyback is free" claim, exercised
// end to end rather than just at the npcs-package level.
func (s *TradeTestSuite) TestSellAppendsANewPlayerSoldRowAndItIsBuyableBack() {
	s.placeVendor(spatial.Position{X: 1, Y: 0}) // no dagger in the demo stock
	s.own(shared.EquipmentTypeWeapon, string(weapons.Dagger), 1)
	price, err := currencyPriceOf(weapons.Dagger)
	s.Require().NoError(err)

	out, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Dagger), Quantity: 1},
		}},
		Receive: session.TradeOffer{Currency: price},
	})
	s.Require().NoError(err)

	found := false
	for _, entry := range out.Descriptor.Inventory {
		if entry.ID == string(weapons.Dagger) {
			found = true
			s.Equal(1, entry.Quantity)
			s.True(entry.PlayerSold)
		}
	}
	s.True(found)

	// Buyback: the same item, bought back through the ordinary buy path,
	// no special-casing.
	s.fund(price)
	_, err = s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: price},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Dagger), Quantity: 1},
		}},
	})
	s.Require().NoError(err)
	s.Require().Len(s.characters.byID["alice"].Inventory, 1)
	s.Equal(string(weapons.Dagger), s.characters.byID["alice"].Inventory[0].ID)
}

func (s *TradeTestSuite) TestSellWrongPriceIsRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	s.own(shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
		Receive: session.TradeOffer{Currency: currency.FromGold(16)}, // real price is 15gp
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrWrongPrice)

	// Refused atomically: the item never left alice's inventory.
	s.Require().Len(s.characters.byID["alice"].Inventory, 1)
}

func (s *TradeTestSuite) TestSellNotOwnedIsRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	// alice owns nothing.

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
		Receive: session.TradeOffer{Currency: currency.FromGold(15)},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotInInventory)
}

func (s *TradeTestSuite) TestSellInsufficientQuantityOwnedIsRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	s.own(shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2},
		}},
		Receive: session.TradeOffer{Currency: currency.FromGold(30)},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotInInventory)

	// Refused atomically: alice's one longsword is still there.
	s.Require().Len(s.characters.byID["alice"].Inventory, 1)
	s.Equal(1, s.characters.byID["alice"].Inventory[0].Quantity)
}

// TestSellVendorInsufficientFundsIsRefused is design.md §3's real
// enforcement case: a vendor with a set, exhausted Wallet correctly
// refuses a sell it can't afford, even though every vendor
// npcs.NewMerchant(nil) authors is nil (unlimited, never reachable here).
func (s *TradeTestSuite) TestSellVendorInsufficientFundsIsRefused() {
	s.placeVendorWithWallet(spatial.Position{X: 1, Y: 0}, currency.FromGold(10))
	s.own(shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
		Receive: session.TradeOffer{Currency: currency.FromGold(15)}, // exceeds the vendor's 10gp
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrInsufficientFunds)

	// Refused atomically: alice still has the item and no payout landed.
	s.Require().Len(s.characters.byID["alice"].Inventory, 1)
	s.Equal(currency.Money{}, s.characters.byID["alice"].Wallet)
}

func (s *TradeTestSuite) TestSellShapeViolationsAreRefused() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	s.own(shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)
	s.own(shared.EquipmentTypeWeapon, string(weapons.Longbow), 1)

	s.Run("two distinct items", func() {
		_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
			Session: "sess", Actor: "alice", Target: "vendor",
			Give: session.TradeOffer{Items: []session.TradeItem{
				{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
				{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longbow), Quantity: 1},
			}},
			Receive: session.TradeOffer{Currency: currency.FromGold(45)},
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrInvalidTradeOffer)
	})

	s.Run("give currency must be empty on a sell", func() {
		_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
			Session: "sess", Actor: "alice", Target: "vendor",
			Give: session.TradeOffer{
				Items:    []session.TradeItem{{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1}},
				Currency: currency.FromCopper(1),
			},
			Receive: session.TradeOffer{Currency: currency.FromGold(15)},
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrInvalidTradeOffer)
	})
}

// TestBuyReceiveCurrencyMustBeEmpty is the buy-side mirror: naming a
// Receive.Currency on a buy makes no sense (buy's payment is Give.Currency)
// and is refused as a shape defect.
func (s *TradeTestSuite) TestBuyReceiveCurrencyMustBeEmpty() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	s.fund(currency.FromGold(15))

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: currency.FromGold(15)},
		Receive: session.TradeOffer{
			Items:    []session.TradeItem{{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1}},
			Currency: currency.FromCopper(1),
		},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrInvalidTradeOffer)
}

// currencyPriceOf resolves a weapon's real catalog price via the same
// ParseCost path equipment.PriceOf uses, so a test can assert against it
// without hand-transcribing the number a second time.
func currencyPriceOf(id weapons.WeaponID) (currency.Money, error) {
	w, ok := weapons.All[id]
	if !ok {
		return currency.Money{}, fmt.Errorf("no such weapon %q", id)
	}
	return currency.ParseCost(w.Cost)
}
