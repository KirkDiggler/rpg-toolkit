// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
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

// TestTradeBuysTheListedItem is the headline: the longsword lands in
// alice's saved character data over the real verb, one unit, and the
// vendor's stock reflects the decrement in the same response.
func (s *TradeTestSuite) TestTradeBuysTheListedItem() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	out, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
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
}

// TestTradeQuantityAboveOneIsSupported pins design.md's 2026-09-04 amendment:
// Quantity is not capped at 1 — a single Receive line can name any amount up
// to what the vendor holds.
func (s *TradeTestSuite) TestTradeQuantityAboveOneIsSupported() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	out, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeAmmunition, ID: string(ammunition.Arrows20), Quantity: 500},
		}},
	})
	s.Require().NoError(err)

	saved := s.characters.byID["alice"]
	s.Require().Len(saved.Inventory, 1)
	s.Equal(500, saved.Inventory[0].Quantity)

	// Unlimited stock stays unlimited — no quantity displayed.
	for _, entry := range out.Descriptor.Inventory {
		if entry.ID == string(ammunition.Arrows20) {
			s.Equal(npcs.StockModeUnlimited, entry.Mode)
		}
	}
}

func (s *TradeTestSuite) TestGiveItemsIsRefused() {
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
	s.ErrorIs(err, session.ErrGiveNotSupported)
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
func (s *TradeTestSuite) TestBuyingPastAvailableQuantityIsOutOfStock() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrOutOfStock)

	_, err = s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().NoError(err, "the rejected over-ask must not have partially decremented the row")

	_, err = s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
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
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotAVendor)
}

// TestOutOfRangeRefusalPropagates pins that Trade reuses encounter.Interact's
// own reach check rather than re-implementing it — the same proof
// InteractTestSuite.TestOutOfRangeRefusalPropagates makes for Interact.
func (s *TradeTestSuite) TestOutOfRangeRefusalPropagates() {
	s.placeVendor(spatial.Position{X: 4, Y: 4})

	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
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
