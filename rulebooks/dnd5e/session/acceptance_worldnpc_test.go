// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
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

	mgr, err := session.NewManager(&session.Config{
		Dice:       &sequenceDice{rolls: []int{0, 0}}, // two initiative rolls, order unasserted here
		TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: encounter.MemberID(alice.ID), Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	s.Require().NoError(err)

	// The vendor arrives first, in plain free-roam — nothing to fight yet.
	placed, err := mgr.PlaceNPC(ctx, &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: spatial.Position{X: 2, Y: 1}, NPC: merchantData(),
	})
	s.Require().NoError(err)
	s.Nil(placed.Formed, "a world NPC arriving must never start a fight")

	// The player walks up and interacts — sees the vendor capability,
	// nothing about stock, price, or a shop.
	interacted, err := mgr.Interact(ctx, &session.InteractInput{Session: "sess", Actor: "alice", Target: "vendor"})
	s.Require().NoError(err)
	s.Contains(interacted.Descriptor.Capabilities, npc.CapabilityVendor)
	s.Equal(npc.CombatPolicyNonCombatant, interacted.Descriptor.CombatPolicy)

	// Now a monster arrives — under this fixture's unconditional sight, this
	// IS the contact: a fight forms the instant it is in the map, the same
	// shape aFight's own scene proves elsewhere in this package.
	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 5, Y: 5},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "arriving in plain sight of alice must start a fight")
	s.ElementsMatch([]string{"alice", "skeleton"}, spawned.Formed.Order,
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
