// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// PlaceNPC: caller-supplied NPC content, entering the session directly
// rather than resolved from a ref (rpg-toolkit#1404).
//
// The seam's claim here is the OPPOSITE of Spawn's: where Spawn's tests
// assert values that only the catalog constructor could have produced,
// these assert that PlaceNPC stores and returns EXACTLY what the caller
// handed it — nothing resolved, nothing defaulted a second time. The
// default-vs-explicit decision already happened one layer up, in
// npcs.NewMerchant; this seam just carries whatever it was given.
type PlaceNPCTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestPlaceNPCSuite(t *testing.T) { suite.Run(t, new(PlaceNPCTestSuite)) }

func (s *PlaceNPCTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *PlaceNPCTestSuite) SetupSubTest() { s.SetupTest() }

func merchantData() *npc.Data {
	built, err := npc.New(npc.Config{
		Ref:          refs.NPCs.Merchant(),
		DisplayName:  "Demo Merchant",
		Capabilities: []npc.Capability{npc.CapabilityVendor},
	})
	if err != nil {
		panic(err) // fixture construction; a broken fixture is a test bug, not a case to assert on
	}
	return built.ToData()
}

func (s *PlaceNPCTestSuite) place(member string, data *npc.Data, at spatial.Position) (*session.PlaceNPCOutput, error) {
	return s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: member, Position: at, NPC: data,
	})
}

// TestPlaceNPCStoresExactlyWhatTheCallerSupplied is the headline: nothing
// here is resolved or defaulted a second time by this seam.
func (s *PlaceNPCTestSuite) TestPlaceNPCStoresExactlyWhatTheCallerSupplied() {
	data := merchantData()

	out, err := s.place("vendor-1", data, spatial.Position{X: 0, Y: 0})
	s.Require().NoError(err)

	s.Equal("vendor-1", string(out.Member.ID))
	s.Equal(session.KindWorld, out.Member.Kind)
	s.Equal("Demo Merchant", out.Member.Name)

	stored := s.sessions.byID["sess"]
	s.Require().Len(stored.WorldNPCs, 1)
	s.Equal("vendor-1", stored.WorldNPCs[0].MemberID)
	s.Equal(*data, stored.WorldNPCs[0].NPC, "the exact content handed in, not a rebuilt copy")
}

// TestPlaceNPCRejectsANilNPC is the load-bearing rejection: this seam does
// not itself interpret "no content" as "give me the demo" — that decision
// already happened in npcs.NewMerchant, one layer up.
func (s *PlaceNPCTestSuite) TestPlaceNPCRejectsANilNPC() {
	_, err := s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "vendor-1", Position: spatial.Position{X: 0, Y: 0}, NPC: nil,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoRef)
	s.Empty(s.sessions.byID["sess"].WorldNPCs, "a rejected placement stores nothing")
}

// TestPlaceNPCEnforcesTheSamePlacementRulesAsSpawnAndJoin is the shared-path
// pin place() already gives Join and Spawn — PlaceNPC goes through the same
// door, so a bad cell must refuse identically.
func (s *PlaceNPCTestSuite) TestPlaceNPCEnforcesTheSamePlacementRulesAsSpawnAndJoin() {
	nowhere := spatial.Position{X: 900, Y: 900}

	_, err := s.place("vendor-1", merchantData(), nowhere)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition)
	s.Empty(s.sessions.byID["sess"].WorldNPCs, "the rejected placement stored nothing")
}

// TestPlaceNPCNeverFormsAFight pins the structural exclusion this whole
// issue exists for: a world NPC placed in sight of nobody else forms no
// fight, and encounter never even offers the machinery a KindWorld member
// could trigger one through.
func (s *PlaceNPCTestSuite) TestPlaceNPCNeverFormsAFight() {
	out, err := s.place("vendor-1", merchantData(), spatial.Position{X: 0, Y: 0})
	s.Require().NoError(err)
	s.Nil(out.Formed)
}

// TestExistingSpawnedMonsterBehaviorIsUnchanged pins that PlaceNPC and Spawn
// write to two different stores — a world NPC must never land in NPCs
// (which already means monster sheets, N1) and a monster must never land in
// WorldNPCs.
func (s *PlaceNPCTestSuite) TestExistingSpawnedMonsterBehaviorIsUnchanged() {
	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	_, err = s.place("vendor-1", merchantData(), hexCell(3, 0))
	s.Require().NoError(err)

	stored := s.sessions.byID["sess"]
	s.Require().Len(stored.NPCs, 1, "the monster sheet lands in NPCs")
	s.Equal("skel-1", stored.NPCs[0].ID)
	s.Require().Len(stored.WorldNPCs, 1, "the world NPC lands in WorldNPCs")
	s.Equal("vendor-1", stored.WorldNPCs[0].MemberID)
}

// TestPlaceNPCRejectsNilInput and TestPlaceNPCRejectsAnEmptyMember round out
// the input-shape rejection table alongside the nil-NPC and bad-position
// cases above.
func (s *PlaceNPCTestSuite) TestPlaceNPCRejectsNilInput() {
	_, err := s.mgr.PlaceNPC(context.Background(), nil)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNilInput)
}

func (s *PlaceNPCTestSuite) TestPlaceNPCRejectsAnEmptyMember() {
	_, err := s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "", Position: spatial.Position{X: 0, Y: 0}, NPC: merchantData(),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoMemberID)
}
