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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// InteractInventoryTestSuite closes the gap found auditing #1447: Inventory
// was the one npc.Data field that survives PlaceNPC and persistence but was
// silently dropped at the last hop, worldNPCDescriptor. PlaceNPC and Interact
// are separate top-level verb calls (S4: load-act-save, no session process),
// so calling them in sequence against fakeSessions already exercises a real
// json.Marshal/json.Unmarshal round trip (fakeSessions.copyOf, manager_test.go)
// on the full SessionData in between — not just an in-memory struct copy.
type InteractInventoryTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestInteractInventorySuite(t *testing.T) { suite.Run(t, new(InteractInventoryTestSuite)) }

func (s *InteractInventoryTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, TurnDriver: session.Pass{},
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

// TestInteractReturnsVendorInventorySurvivingSaveAndReload is the headline:
// a placed merchant's stock is not just carried in memory, it survives the
// real persistence round trip and comes back out through Interact.
func (s *InteractInventoryTestSuite) TestInteractReturnsVendorInventorySurvivingSaveAndReload() {
	vendor, err := npcs.NewMerchant(nil)
	s.Require().NoError(err)

	_, err = s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: spatial.Position{X: 1, Y: 0},
		NPC: vendor.NPC().ToData(),
	})
	s.Require().NoError(err)

	out, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().NoError(err)

	s.Require().Len(out.Descriptor.Inventory, 3)
	ids := make([]string, len(out.Descriptor.Inventory))
	for i, entry := range out.Descriptor.Inventory {
		ids[i] = entry.ID
	}
	s.ElementsMatch(
		[]string{string(weapons.Longsword), string(weapons.Longbow), string(ammunition.Arrows20)},
		ids,
	)
}

// TestInteractReturnsNoInventoryForNonVendorNPC proves the normal case —
// a world NPC that isn't a vendor — is an empty slice and NOT an error.
func (s *InteractInventoryTestSuite) TestInteractReturnsNoInventoryForNonVendorNPC() {
	built, err := npc.New(npc.Config{
		Ref:         refs.NPCs.Vendor(),
		DisplayName: "Villager",
	})
	s.Require().NoError(err)

	_, err = s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "villager", Position: spatial.Position{X: 1, Y: 0},
		NPC: built.ToData(),
	})
	s.Require().NoError(err)

	out, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "villager",
	})
	s.Require().NoError(err)
	s.Empty(out.Descriptor.Inventory)
}
