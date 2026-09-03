// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// placenpc_movement_test.go covers PlaceNPC's wiring of npc.Data.MovementPolicy
// through to encounter's BlocksMovement (rpg-toolkit#1434). Every claim here is
// proven end to end — a real second arrival attempted on the placed NPC's
// cell — rather than inferred from "place() was called with the right bool",
// matching the standard #1434's encounter-side PR already set.
//
// hexWorld (PlaceNPCTestSuite's fixture, via SetupTest) already pre-places
// "alice" at hexCell(0, 0) — so these tests place the NPC at hexCell(2, 2)
// (clear floor, away from alice and the corridor's occluding prop at (1,1))
// and attempt the second arrival with "bob", who is registered as a
// character (testCharacters) but never pre-placed. Trying to re-arrive as
// "alice" would hit encounter's own "already a member" rejection — a
// duplicate-join refusal that also routes through ErrNoMember/"no such
// member" via translate(), and would silently make every one of these tests
// pass or fail for the wrong reason.
//
// Methods on PlaceNPCTestSuite (placenpc_test.go); Go test suites collect
// methods by type regardless of which file declares them.

func merchantDataWithMovement(policy npc.MovementPolicy) *npc.Data {
	built, err := npc.New(npc.Config{
		Ref:            refs.NPCs.Merchant(),
		DisplayName:    "Demo Merchant",
		Capabilities:   []npc.Capability{npc.CapabilityVendor},
		MovementPolicy: policy,
	})
	if err != nil {
		panic(err) // fixture construction; a broken fixture is a test bug
	}
	return built.ToData()
}

func (s *PlaceNPCTestSuite) joinBob(at spatial.Position) (*session.JoinOutput, error) {
	return s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob", Position: at,
	})
}

// TestPlaceNPCWithBlockingMovementPolicyRefusesALaterArrivalOnItsCell proves
// the wiring end to end: a real Join attempt onto the NPC's cell fails, not
// merely "place() received true".
func (s *PlaceNPCTestSuite) TestPlaceNPCWithBlockingMovementPolicyRefusesALaterArrivalOnItsCell() {
	at := hexCell(2, 2)
	_, err := s.place("vendor-1", merchantDataWithMovement(npc.MovementPolicyBlocking), at)
	s.Require().NoError(err)

	_, err = s.joinBob(at)
	s.Require().Error(err, "a blocking world NPC must refuse a later arrival on its own cell")
	s.ErrorIs(err, session.ErrBadPosition)
}

// TestPlaceNPCWithPassableMovementPolicyAllowsALaterArrivalOnItsCell is the
// other direction of the same proof.
func (s *PlaceNPCTestSuite) TestPlaceNPCWithPassableMovementPolicyAllowsALaterArrivalOnItsCell() {
	at := hexCell(2, 2)
	_, err := s.place("vendor-1", merchantDataWithMovement(npc.MovementPolicyPassable), at)
	s.Require().NoError(err)

	_, err = s.joinBob(at)
	s.Require().NoError(err, "a passable world NPC must genuinely allow co-location, not merely fail to refuse it for some other reason")
}

// TestDemoMerchantDefaultsToBlockingBecauseNPCNewAlreadyDoes is the test that
// matters most for #1434's goal: merchantData() never sets MovementPolicy,
// so this proves the default falls out of npc.New's own defaultMovementPolicy
// (shipped before this issue existed), not new logic written here to force it.
func (s *PlaceNPCTestSuite) TestDemoMerchantDefaultsToBlockingBecauseNPCNewAlreadyDoes() {
	data := merchantData()
	s.Require().Equal(npc.MovementPolicyBlocking, data.MovementPolicy,
		"npc.New's own default must already be Blocking — this test does not set it")

	at := hexCell(2, 2)
	_, err := s.place("vendor-1", data, at)
	s.Require().NoError(err)

	_, err = s.joinBob(at)
	s.Require().Error(err, "the demo merchant, placed with its default MovementPolicy, must block")
}

// TestPlaceNPCRejectsAMalformedMovementPolicy pins ErrBadNPC: PlaceNPC takes
// already-built content directly rather than resolving it through npc.New,
// so a caller who constructs npc.Data by hand (bypassing npc.New's own
// validation) can still reach this seam with an empty or unrecognized
// MovementPolicy. This must fail closed, not panic or silently default.
func (s *PlaceNPCTestSuite) TestPlaceNPCRejectsAMalformedMovementPolicy() {
	data := merchantData()
	data.MovementPolicy = "sideways" // never a value npc.New would have allowed through

	_, err := s.place("vendor-1", data, hexCell(2, 2))
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadNPC)
}

// TestJoinStillDoesNotBlockAfterMovementPolicyWiring is the regression pin:
// players never blocked movement before this wiring existed, and this PR's
// change to place()'s shared signature must not have changed that — proven
// by a real arrival attempt onto alice's own cell (hexWorld's fixture
// pre-places her at hexCell(0, 0)), not by inspecting the false literal at
// Join's own call site.
func (s *PlaceNPCTestSuite) TestJoinStillDoesNotBlockAfterMovementPolicyWiring() {
	_, err := s.joinBob(hexCell(0, 0))
	s.Require().NoError(err, "a player must still allow co-location after place()'s new parameter — unchanged behavior")
}
