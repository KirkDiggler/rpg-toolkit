// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// UnpackTestSuite covers Unpack: the first session verb with no scope.enc
// call at all (rpg-toolkit#1544) — its target is a pack the actor already
// owns, so there's no reach/visibility check and no story beat.
type UnpackTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestUnpackSuite(t *testing.T) { suite.Run(t, new(UnpackTestSuite)) }

func (s *UnpackTestSuite) SetupTest() {
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

// own gives alice quantity units of id directly, bypassing any verb.
func (s *UnpackTestSuite) own(itemType shared.EquipmentType, id string, quantity int) {
	s.characters.byID["alice"].Inventory = append(s.characters.byID["alice"].Inventory,
		character.InventoryItemData{Type: itemType, ID: id, Quantity: quantity})
}

// TestUnpackDecomposesAnOwnedPack is the headline: the pack leaves alice's
// inventory and every one of its Contents lines lands in its place.
func (s *UnpackTestSuite) TestUnpackDecomposesAnOwnedPack() {
	s.own(shared.EquipmentTypePack, string(packs.ExplorerPack), 1)

	out, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
		Session: "sess", Actor: "alice", ItemID: string(packs.ExplorerPack), Quantity: 1,
	})
	s.Require().NoError(err)
	s.NotNil(out)

	saved := s.characters.byID["alice"]
	byID := make(map[string]character.InventoryItemData, len(saved.Inventory))
	for _, item := range saved.Inventory {
		byID[item.ID] = item
	}

	_, hasPack := byID[string(packs.ExplorerPack)]
	s.False(hasPack, "the pack itself must be gone")

	pack := packs.All[packs.ExplorerPack]
	s.Require().Len(saved.Inventory, len(pack.Contents), "one line per content item, no more, no less")
	for _, content := range pack.Contents {
		got, ok := byID[content.ItemID]
		s.Require().True(ok, "missing content item %q", content.ItemID)
		s.Equal(content.Quantity, got.Quantity, "content item %q quantity", content.ItemID)
	}
}

// TestUnpackQuantityScalesContents pins that Quantity is generic: two pack
// instances at once yields contents at twice their per-pack amount.
func (s *UnpackTestSuite) TestUnpackQuantityScalesContents() {
	s.own(shared.EquipmentTypePack, string(packs.ExplorerPack), 2)

	_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
		Session: "sess", Actor: "alice", ItemID: string(packs.ExplorerPack), Quantity: 2,
	})
	s.Require().NoError(err)

	saved := s.characters.byID["alice"]
	found := false
	for _, item := range saved.Inventory {
		if item.ID == string(items.Torch) {
			found = true
			s.Equal(20, item.Quantity, "Explorer's Pack's 10 torches, times 2 packs")
		}
	}
	s.True(found)
}

func (s *UnpackTestSuite) TestUnpackNotAPackIsRefused() {
	s.own(shared.EquipmentTypeWeapon, string(weapons.Longsword), 1)

	_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
		Session: "sess", Actor: "alice", ItemID: string(weapons.Longsword), Quantity: 1,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotAPack)

	// Refused atomically: nothing was touched.
	s.Require().Len(s.characters.byID["alice"].Inventory, 1)
}

func (s *UnpackTestSuite) TestUnpackNotOwnedIsRefused() {
	// alice owns nothing.
	_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
		Session: "sess", Actor: "alice", ItemID: string(packs.ExplorerPack), Quantity: 1,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotInInventory)
}

func (s *UnpackTestSuite) TestUnpackInsufficientQuantityOwnedIsRefused() {
	s.own(shared.EquipmentTypePack, string(packs.ExplorerPack), 1)

	_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
		Session: "sess", Actor: "alice", ItemID: string(packs.ExplorerPack), Quantity: 2,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotInInventory)

	// Refused atomically: the one pack alice owns is untouched.
	s.Require().Len(s.characters.byID["alice"].Inventory, 1)
	s.Equal(1, s.characters.byID["alice"].Inventory[0].Quantity)
}

func (s *UnpackTestSuite) TestUnpackShapeViolationsAreRefused() {
	s.own(shared.EquipmentTypePack, string(packs.ExplorerPack), 1)

	s.Run("empty item id", func() {
		_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
			Session: "sess", Actor: "alice", Quantity: 1,
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrInvalidUnpackRequest)
	})

	s.Run("nonpositive quantity", func() {
		_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
			Session: "sess", Actor: "alice", ItemID: string(packs.ExplorerPack), Quantity: 0,
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrInvalidUnpackRequest)
	})
}

func (s *UnpackTestSuite) TestUnpackNilInputRejected() {
	_, err := s.mgr.Unpack(context.Background(), nil)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNilInput)
}

func (s *UnpackTestSuite) TestUnpackEmptyActorRejected() {
	_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
		Session: "sess", ItemID: string(packs.ExplorerPack), Quantity: 1,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoMemberID)
}

// TestUnpackedItemIsImmediatelySellable closes the loop design.md's own
// "done when" bar asks for: an unpacked item is not just present in
// inventory, it is a real, ordinary InventoryItemData that Sell's own
// primitive (RemoveInventoryItem) and equipment.PriceOf already handle —
// no special-casing needed for something that came from a pack.
func (s *UnpackTestSuite) TestUnpackedItemIsImmediatelySellable() {
	s.own(shared.EquipmentTypePack, string(packs.ExplorerPack), 1)

	_, err := s.mgr.Unpack(context.Background(), &session.UnpackInput{
		Session: "sess", Actor: "alice", ItemID: string(packs.ExplorerPack), Quantity: 1,
	})
	s.Require().NoError(err)

	vendor, err := npcs.NewMerchant(nil)
	s.Require().NoError(err)
	_, err = s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: spatial.Position{X: 1, Y: 0}, NPC: vendor.NPC().ToData(),
	})
	s.Require().NoError(err)

	price := currency.FromCopper(1) // Torch's real Cost, "1 cp"
	_, err = s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeItem, ID: string(items.Torch), Quantity: 1},
		}},
		Receive: session.TradeOffer{Currency: price},
	})
	s.Require().NoError(err, "an unpacked item must be sellable with no special-casing")
}
