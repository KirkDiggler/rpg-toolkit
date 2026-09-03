// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Interact: the host seam's half of reaching a placed world NPC
// (rpg-toolkit#1404). encounter.Interact (PR2) answers only identity,
// adjacency and visibility; this suite is about the layer ABOVE that —
// assembling the WorldNPCDescriptor from session's own store, and proving
// encounter's own refusals propagate rather than get re-implemented here.
type InteractTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestInteractSuite(t *testing.T) { suite.Run(t, new(InteractTestSuite)) }

// SetupTest reuses hexWorld's fixture: alice, a player, already placed at
// (0,0) in the corridor region, sight unconditional (encEveryoneSees) — the
// same fixture PlaceNPCTestSuite already proves works for placement.
func (s *InteractTestSuite) SetupTest() {
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

func (s *InteractTestSuite) placeVendor(at spatial.Position) {
	_, err := s.mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: at, NPC: merchantData(),
	})
	s.Require().NoError(err)
}

// TestAdjacentPlayerGetsADescriptorMatchingWhatWasPlaced is the headline:
// the descriptor is assembled from session's OWN store, not anything
// encounter reports (encounter carries no NPC content at all).
func (s *InteractTestSuite) TestAdjacentPlayerGetsADescriptorMatchingWhatWasPlaced() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	out, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().NoError(err)

	data := merchantData()
	s.Equal("vendor", out.Descriptor.TargetID)
	s.Equal(data.Ref.String(), out.Descriptor.Ref)
	s.Equal(data.DisplayName, out.Descriptor.DisplayName)
	s.Equal(data.Capabilities, out.Descriptor.Capabilities)
	s.Equal(data.CombatPolicy, out.Descriptor.CombatPolicy)
	s.NotZero(out.Seq)
}

// TestOutOfRangeRefusalPropagates proves encounter's own range refusal
// crosses the seam rather than being re-implemented here — a distant vendor
// is exactly the same shape of refusal encounter.Interact already covers
// (PR2's own suite), this just confirms session does not swallow or
// reinterpret it.
func (s *InteractTestSuite) TestOutOfRangeRefusalPropagates() {
	s.placeVendor(spatial.Position{X: 4, Y: 4})

	_, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrOutOfRange)
}

// TestNegativeRangeRefusalPropagates is lesson 4 pinned as a test, not just
// a claim: encounter.Interact already rejects a negative Range as a caller
// defect (wrapping encounter.ErrNoMember, per PR2's own fix). Session must
// forward Range untouched rather than re-validating it — if this passed
// silently instead of refusing, that would mean session started
// re-implementing (or worse, re-normalizing) a rule that already lives one
// layer down.
func (s *InteractTestSuite) TestNegativeRangeRefusalPropagates() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	_, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor", Range: -1,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoMember)
}

func (s *InteractTestSuite) TestNilInputRejected() {
	_, err := s.mgr.Interact(context.Background(), nil)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNilInput)
}

func (s *InteractTestSuite) TestEmptyActorOrTargetRejected() {
	_, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "", Target: "vendor",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoMemberID)
}

func (s *InteractTestSuite) TestNoSuchSessionIsRefused() {
	_, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "no-such-session", Actor: "alice", Target: "vendor",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoSession)
}

func (s *InteractTestSuite) TestClosedEncounterRefusesInteract() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	_, err := s.mgr.End(context.Background(), &session.EndInput{Session: "sess", Ending: "out"})
	s.Require().NoError(err)

	_, err = s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrClosed)
}

// TestDescriptorCapabilitiesSliceIsCopyOut proves mutating a returned
// descriptor cannot mutate session's own stored record.
func (s *InteractTestSuite) TestDescriptorCapabilitiesSliceIsCopyOut() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	out, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Descriptor.Capabilities)

	out.Descriptor.Capabilities[0] = npc.Capability("mutated")

	again, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().NoError(err)
	s.Equal(merchantData().Capabilities, again.Descriptor.Capabilities,
		"the stored record must be unaffected by mutating a prior descriptor")
}

func (s *InteractTestSuite) TestSaveAndDeliveryAreReported() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	out, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().NoError(err)
	s.NotEmpty(out.Saved.Written, "something must be named as saved")
}

// TestMissingWorldNPCRecordFailsClosed exercises the internal-inconsistency
// guard directly rather than only asserting it by reading the code (the
// lesson from verifying PR4's Attack claim): if encounter confirms a
// KindWorld target but session's own store has nothing for it, Interact
// must fail closed with ErrNoSheet, not hand back a zero-value descriptor a
// caller could mistake for a real, empty NPC.
func (s *InteractTestSuite) TestMissingWorldNPCRecordFailsClosed() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})

	stored := s.sessions.byID["sess"]
	stored.WorldNPCs = nil // simulate PlaceNPC's two writes having disagreed

	_, err := s.mgr.Interact(context.Background(), &session.InteractInput{
		Session: "sess", Actor: "alice", Target: "vendor",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoSheet)
}
