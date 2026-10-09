// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// AcceptanceWorldNPCSuite is #1404's own acceptance scene (design.md/plan.md
// Task 8): a player, a monster, and a vendor-profile world NPC on one map.
// The player interacts with the vendor and sees the vendor capability, a
// fight forms against the monster without the vendor ever entering it, and
// the vendor remains queryable afterward. No stock, quote, purchase, or
// inventory mutation appears here — that is #1275's work, not this one's.
type AcceptanceWorldNPCSuite struct {
	suite.Suite
}

func TestAcceptanceWorldNPCSuite(t *testing.T) { suite.Run(t, new(AcceptanceWorldNPCSuite)) }

func (s *AcceptanceWorldNPCSuite) TestVendorSurvivesAFightItNeverJoins() {
	alice := armedFighter("alice")
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(alice)

	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice:       &sequenceDice{rolls: []int{0, 0}}, // two initiative rolls, order unasserted here
		TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	// The skeleton is on the board from the start, in the room behind a shut
	// door alice stands at: nothing to fight until she opens it.
	sc := scene{
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 0, 0, 6, 6),
				rectRegion("crypt", 6, 0, 6, 6),
			},
			Walls: hexSeamWalls(6, 6, 0),
			Doors: []encounter.DoorInput{{
				ID:    "crypt-door",
				Edges: []encounter.DoorEdge{{From: hexCell(5, 0), To: hexCell(6, 0)}},
				State: encounter.DoorIsClosed(),
			}},
		},
		Party:    []sceneSeat{seatAt(alice.ID, 5, 0)},
		Monsters: []dungeonspec.MonsterPlacement{monsterAt("skeleton", refs.Monsters.Skeleton().String(), 8, 0)},
	}

	ctx := context.Background()
	launched := launchScene(s.T(), mgr, sc)
	s.Require().Empty(launched.Formed, "the curtain keeps the skeleton out of sight at launch")

	// The vendor arrives first, in plain free-roam — nothing to fight yet.
	placed, err := mgr.PlaceNPC(ctx, &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: spatial.Position{X: 4, Y: 0}, NPC: merchantData(),
	})
	s.Require().NoError(err)
	s.Nil(placed.Formed, "a world NPC arriving must never start a fight")

	// The player walks up and interacts — sees the vendor capability,
	// nothing about stock, price, or a shop.
	interacted, err := mgr.Interact(ctx, &session.InteractInput{Session: "sess", Actor: "alice", Target: "vendor"})
	s.Require().NoError(err)
	s.Contains(interacted.Descriptor.Capabilities, npc.CapabilityVendor)
	s.Equal(npc.CombatPolicyNonCombatant, interacted.Descriptor.CombatPolicy)

	// Now alice opens the door — this IS the contact: a fight forms the
	// instant the skeleton is in her sight, with the vendor on the map.
	opened, err := mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: "crypt-door"})
	s.Require().NoError(err)
	s.Require().NotNil(opened.Formed, "opening onto the skeleton in plain sight must start a fight")
	s.ElementsMatch([]string{"alice", "skeleton"}, opened.Formed.Order,
		"the vendor must never be named in the fight's initiative order")

	// The vendor is still queryable, mid-fight, from outside it.
	afterFight, err := mgr.Interact(ctx, &session.InteractInput{Session: "sess", Actor: "alice", Target: "vendor"})
	s.Require().NoError(err)
	s.Equal(interacted.Descriptor, afterFight.Descriptor)

	// And never an attack candidate for either side.
	afford, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, decl := range afford.Declarations {
		if decl.Verb != session.VerbAttack {
			continue
		}
		for _, c := range decl.Candidates {
			s.NotEqual("vendor", c.Member, "the vendor must never be offered as an attack candidate")
		}
	}
}
