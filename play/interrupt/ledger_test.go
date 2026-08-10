// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package interrupt_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
)

const (
	testAldric  = core.EntityID(testAudience)
	testShield  = interrupt.Option(optShield)
	testDecline = interrupt.Option(optDecline)
)

// Shared test vocabulary (goconst: repeated literals become consts).
const (
	testAudience = "aldric"
	optShield    = "shield"
	optDecline   = "decline"
	optCorrupted = "corrupted"
	optAttack    = "attack"
	testMonster  = "monster"
	testGrunk    = "grunk"
)

type LedgerSuite struct {
	suite.Suite
	ledger *interrupt.Ledger
}

func (s *LedgerSuite) SetupTest() {
	var err error
	s.ledger, err = interrupt.NewLedger()
	s.Require().NoError(err)
}

func (s *LedgerSuite) TestPoseOpensWindow() {
	out, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAldric,
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  []byte("frozen-attack"),
		At:       7,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out.Window.ID, "IDs are monotonic from 1")
	s.Equal(testAldric, out.Window.Audience)
	s.Equal([]interrupt.Option{testShield, testDecline}, out.Window.Options)
	s.Equal([]byte("frozen-attack"), out.Window.Payload)
	s.Equal(uint64(7), out.Window.At)

	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAldric})
	s.Require().NoError(err)
	s.Len(pending, 1)
	s.Equal(out.Window, pending[0])
}

func (s *LedgerSuite) TestPoseValidationOrderAndAtomicity() {
	// validation order: nil → audience → no options → empty option → duplicate
	_, err := s.ledger.Pose(nil)
	s.Require().ErrorIs(err, interrupt.ErrNilInput)
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "", Options: nil})
	s.Require().ErrorIs(err, interrupt.ErrNoAudience)
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: nil})
	s.Require().ErrorIs(err, interrupt.ErrNoOptions)
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: []interrupt.Option{"x", "", "x"}})
	s.Require().ErrorIs(err, interrupt.ErrNoOption, "empty token found before the duplicate")
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: []interrupt.Option{"x", "y", "x"}})
	s.Require().ErrorIs(err, interrupt.ErrDuplicateOption)

	// R5: none of those consumed an ID or opened a window
	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Empty(open)
	out, err := s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: []interrupt.Option{"x"}})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out.Window.ID, "failed poses consume no IDs")
}

func (s *LedgerSuite) TestPoseNilPayloadIsLegal() {
	out, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAldric,
		Options:  []interrupt.Option{testShield},
		Payload:  nil,
		At:       5,
	})
	s.Require().NoError(err)
	s.Nil(out.Window.Payload, "nil payload must be legal and stored as nil")

	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAldric})
	s.Require().NoError(err)
	s.Len(pending, 1)
	s.Nil(pending[0].Payload)
}

func (s *LedgerSuite) TestPoseCopyIn() {
	options := []interrupt.Option{testShield, testDecline}
	payload := []byte("frozen-attack")

	out, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAldric,
		Options:  options,
		Payload:  payload,
		At:       7,
	})
	s.Require().NoError(err)

	// Mutate the caller's slices after the pose
	if len(options) > 0 {
		options[0] = "mutated"
	}
	if len(payload) > 0 {
		payload[0] = 'X'
	}

	// Re-query and verify stored data is unchanged
	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAldric})
	s.Require().NoError(err)
	s.Len(pending, 1)
	s.Equal([]interrupt.Option{testShield, testDecline}, pending[0].Options,
		"stored options must be immune to caller mutation")
	s.Equal([]byte("frozen-attack"), pending[0].Payload,
		"stored payload must be immune to caller mutation")

	// Also verify the returned window from Pose
	s.Equal([]interrupt.Option{testShield, testDecline}, out.Window.Options)
	s.Equal([]byte("frozen-attack"), out.Window.Payload)
}

func (s *LedgerSuite) TestSeveralWindowsOneAudience() {
	out1, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAldric,
		Options:  []interrupt.Option{"option1"},
		Payload:  []byte("payload1"),
		At:       1,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out1.Window.ID)

	out2, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAldric,
		Options:  []interrupt.Option{"option2"},
		Payload:  []byte("payload2"),
		At:       2,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(2), out2.Window.ID)

	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAldric})
	s.Require().NoError(err)
	s.Len(pending, 2)
	s.Equal(interrupt.WindowID(1), pending[0].ID, "windows in pose order")
	s.Equal(interrupt.WindowID(2), pending[1].ID, "windows in pose order")
}

func (s *LedgerSuite) TestAudienceIsolation() {
	_, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{testShield},
		Payload:  []byte("frozen"),
		At:       1,
	})
	s.Require().NoError(err)

	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: "bob"})
	s.Require().NoError(err)
	s.Empty(pending, "bob should see no windows for aldric")

	pendingAldric, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAudience})
	s.Require().NoError(err)
	s.Len(pendingAldric, 1, "aldric should see their own window")
}

func (s *LedgerSuite) TestAtIsOpaque() {
	out1, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: "a",
		Options:  []interrupt.Option{"x"},
		At:       9,
	})
	s.Require().NoError(err)
	s.Equal(uint64(9), out1.Window.At)

	out2, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: "a",
		Options:  []interrupt.Option{"y"},
		At:       2,
	})
	s.Require().NoError(err)
	s.Equal(uint64(2), out2.Window.At)

	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Len(open, 2)
	s.Equal(uint64(9), open[0].At, "At values stored verbatim, no ordering")
	s.Equal(uint64(2), open[1].At)
}

// TestCopyOutImmunity pins the copy-out direction on all three read
// surfaces (AC2): mutating a returned Window must never corrupt the
// ledger's internal state.
func (s *LedgerSuite) TestCopyOutImmunity() {
	posed, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{optShield, optDecline},
		Payload:  []byte("frozen"),
		At:       7,
	})
	s.Require().NoError(err)

	// Surface 1: Pose's returned Window
	posed.Window.Options[0] = optCorrupted
	posed.Window.Payload[0] = 'X'

	// Surface 2: PendingFor
	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAudience})
	s.Require().NoError(err)
	s.Require().Len(pending, 1)
	s.Equal(interrupt.Option(optShield), pending[0].Options[0],
		"mutating Pose's returned Options corrupted the ledger")
	s.Equal([]byte("frozen"), pending[0].Payload,
		"mutating Pose's returned Payload corrupted the ledger")
	pending[0].Options[1] = optCorrupted
	pending[0].Payload[0] = 'Y'

	// Surface 3: Open
	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Require().Len(open, 1)
	s.Equal([]interrupt.Option{optShield, optDecline}, open[0].Options,
		"mutating PendingFor's returned Options corrupted the ledger")
	s.Equal([]byte("frozen"), open[0].Payload,
		"mutating PendingFor's returned Payload corrupted the ledger")
	open[0].Options[0] = optCorrupted
	open[0].Payload[0] = 'Z'

	// Final independent read: everything above was mutation of copies only.
	final, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAudience})
	s.Require().NoError(err)
	s.Require().Len(final, 1)
	s.Equal([]interrupt.Option{optShield, optDecline}, final[0].Options,
		"mutating Open's returned Options corrupted the ledger")
	s.Equal([]byte("frozen"), final[0].Payload,
		"mutating Open's returned Payload corrupted the ledger")
}

// TestPoseAtomicityFromPopulatedState pins R5 from a non-empty ledger:
// a failed Pose leaves existing windows untouched and consumes no ID.
func (s *LedgerSuite) TestPoseAtomicityFromPopulatedState() {
	first, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{optShield, optDecline},
		Payload:  []byte("frozen"),
		At:       7,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), first.Window.ID)

	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: testAudience, Options: []interrupt.Option{"x", "x"}})
	s.Require().ErrorIs(err, interrupt.ErrDuplicateOption)

	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Require().Len(open, 1, "failed Pose must not add a window (R5)")
	s.Equal(first.Window, open[0], "failed Pose must not disturb existing windows (R5)")

	second, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testGrunk),
		Options:  []interrupt.Option{optAttack},
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(2), second.Window.ID, "failed Pose must not consume an ID (R5)")
}

func (s *LedgerSuite) TestAnswerClosesAndReturnsEnvelope() {
	posed, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testAudience),
		Options: []interrupt.Option{
			interrupt.Option(optShield),
			interrupt.Option(optDecline),
		},
		Payload: []byte("frozen"),
		At:      7,
	})
	s.Require().NoError(err)
	out, err := s.ledger.Answer(&interrupt.AnswerInput{
		Window: posed.Window.ID,
		By:     core.EntityID(testAudience),
		Choice: interrupt.Option(optShield),
	})
	s.Require().NoError(err)
	s.Equal(posed.Window, out.Window, "the envelope returns intact — custody proven")
	s.Equal(interrupt.Option(optShield), out.Choice)
	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{
		Audience: core.EntityID(testAudience),
	})
	s.Require().NoError(err)
	s.Empty(pending, "answered windows leave the ledger")
	// one answer per window
	_, err = s.ledger.Answer(&interrupt.AnswerInput{
		Window: posed.Window.ID,
		By:     core.EntityID(testAudience),
		Choice: interrupt.Option(optShield),
	})
	s.Require().ErrorIs(err, interrupt.ErrNotOpen)
}

func (s *LedgerSuite) TestAnswerValidationOrderAndAtomicity() {
	posed, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testAudience),
		Options: []interrupt.Option{
			interrupt.Option(optShield),
			interrupt.Option(optDecline),
		},
		Payload: []byte("frozen"),
	})
	s.Require().NoError(err)
	// shape → existence → authorization → membership; first failure wins
	_, err = s.ledger.Answer(nil)
	s.Require().ErrorIs(err, interrupt.ErrNilInput)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{
		Window: posed.Window.ID,
		By:     "",
		Choice: "",
	})
	s.Require().ErrorIs(err, interrupt.ErrNoAudience)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{
		Window: posed.Window.ID,
		By:     core.EntityID(testAudience),
		Choice: "",
	})
	s.Require().ErrorIs(err, interrupt.ErrNoOption)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{
		Window: 99,
		By:     core.EntityID(testAudience),
		Choice: interrupt.Option(optShield),
	})
	s.Require().ErrorIs(err, interrupt.ErrNotOpen)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{
		Window: posed.Window.ID,
		By:     "fighter",
		Choice: interrupt.Option(optShield),
	})
	s.Require().ErrorIs(err, interrupt.ErrNotAudience)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{
		Window: posed.Window.ID,
		By:     core.EntityID(testAudience),
		Choice: "fireball",
	})
	s.Require().ErrorIs(err, interrupt.ErrNotOffered)
	// R5: every failure left the window open and unchanged
	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{
		Audience: core.EntityID(testAudience),
	})
	s.Require().NoError(err)
	s.Require().Len(pending, 1)
	s.Equal(posed.Window, pending[0])
}

func (s *LedgerSuite) TestAnswerWindowIDZeroIsNotOpen() {
	_, err := s.ledger.Answer(&interrupt.AnswerInput{
		Window: 0,
		By:     core.EntityID(testAudience),
		Choice: interrupt.Option(optShield),
	})
	s.Require().ErrorIs(err, interrupt.ErrNotOpen,
		"Window ID 0 is never-posed, not a shape error")
}

func (s *LedgerSuite) TestAnswerRemovesExactlyOne() {
	// Pose three windows: two for aldric, one for grunk
	out1, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testAudience),
		Options:  []interrupt.Option{"opt1"},
		Payload:  []byte("p1"),
		At:       1,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out1.Window.ID)
	out2, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testGrunk),
		Options:  []interrupt.Option{"opt2"},
		Payload:  []byte("p2"),
		At:       2,
	})
	s.Require().NoError(err)
	out3, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testAudience),
		Options:  []interrupt.Option{"opt3"},
		Payload:  []byte("p3"),
		At:       3,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(3), out3.Window.ID)

	// Answer the middle one (ID 2, grunk's window)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{
		Window: out2.Window.ID,
		By:     core.EntityID(testGrunk),
		Choice: "opt2",
	})
	s.Require().NoError(err)

	// Verify the others remain, in pose order
	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Len(open, 2)
	s.Equal(interrupt.WindowID(1), open[0].ID, "first window remains")
	s.Equal(interrupt.WindowID(3), open[1].ID, "third window remains in pose order")

	// Verify next Pose still gets monotonic ID
	out4, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: "alice",
		Options:  []interrupt.Option{"opt4"},
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(4), out4.Window.ID, "next Pose continues monotonic sequence")
}

// TestAnswerValidationComboAuthorizationBeforeMembership pins the
// order the design calls out by name: wrong audience AND unoffered
// choice in one call — authorization is checked before membership.
func (s *LedgerSuite) TestAnswerValidationComboAuthorizationBeforeMembership() {
	posed, err := s.ledger.Pose(&interrupt.PoseInput{Audience: testAudience,
		Options: []interrupt.Option{optShield, optDecline}, Payload: []byte("frozen")})
	s.Require().NoError(err)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{Window: posed.Window.ID, By: "fighter", Choice: "fireball"})
	s.Require().ErrorIs(err, interrupt.ErrNotAudience,
		"wrong audience + unoffered choice: authorization must win (design: shape, existence, authorization, membership)")
	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testAudience})
	s.Require().NoError(err)
	s.Require().Len(pending, 1, "failed Answer left the window open (R5)")
}

// TestQueryValidation pins the query-side input guards (Task 4).
func (s *LedgerSuite) TestQueryValidation() {
	_, err := s.ledger.PendingFor(nil)
	s.Require().ErrorIs(err, interrupt.ErrNilInput)
	_, err = s.ledger.PendingFor(&interrupt.PendingForInput{Audience: ""})
	s.Require().ErrorIs(err, interrupt.ErrNoAudience)
}

// TestOpenPoseOrderInterleaved pins pose order across audiences: Open
// returns the table's windows in the order they were posed, not
// grouped by audience.
func (s *LedgerSuite) TestOpenPoseOrderInterleaved() {
	for _, aud := range []core.EntityID{testAudience, testGrunk, testAudience} {
		_, err := s.ledger.Pose(&interrupt.PoseInput{Audience: aud, Options: []interrupt.Option{optShield}})
		s.Require().NoError(err)
	}
	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Require().Len(open, 3)
	s.Equal(interrupt.WindowID(1), open[0].ID, "pose order, not audience grouping")
	s.Equal(interrupt.WindowID(2), open[1].ID)
	s.Equal(interrupt.WindowID(3), open[2].ID)
	s.Equal(core.EntityID("grunk"), open[1].Audience)
}

// TestDeciderContract is executable documentation of the design's
// decider contract: a machine decider is shown ONLY its own pending
// windows (PendingFor(itself)) and answers through the ordinary verb —
// indistinguishable from a human. This is how auto-OA and monster
// reactions ride the same machinery as the Shield prompt.
func (s *LedgerSuite) TestDeciderContract() {
	_, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: testMonster, Options: []interrupt.Option{"take-oa", optDecline}})
	s.Require().NoError(err)

	// The decider's entire view: its own pending windows. Not the world,
	// not other audiences' windows.
	view, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: testMonster})
	s.Require().NoError(err)
	s.Require().Len(view, 1)

	// Policy decider: always take the opportunity attack.
	choice := view[0].Options[0]
	out, err := s.ledger.Answer(&interrupt.AnswerInput{Window: view[0].ID, By: testMonster, Choice: choice})
	s.Require().NoError(err)
	s.Equal(interrupt.Option("take-oa"), out.Choice)

	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Empty(open, "posed and answered in one host call — the world never observed a wait")
}

// PersistenceSuite tests persistence via ToData/LoadLedger (Task 5, R8/R9).
type PersistenceSuite struct {
	suite.Suite
}

// TestIdleToDataDeepEqualsZero pins R8 idle convention: NewLedger().ToData()
// deep-equals the zero LedgerData (not a struct with empty slices).
func (s *PersistenceSuite) TestIdleToDataDeepEqualsZero() {
	idle, err := interrupt.NewLedger()
	s.Require().NoError(err)
	data := idle.ToData()
	zero := interrupt.LedgerData{}
	s.Equal(zero, data, "idle ledger ToData must deep-equal zero LedgerData")
}

// TestIdleToDataMarshalToEmptyJSON pins R8: idle marshals to exactly {}.
func (s *PersistenceSuite) TestIdleToDataMarshalToEmptyJSON() {
	idle, err := interrupt.NewLedger()
	s.Require().NoError(err)
	data := idle.ToData()
	bs, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Equal([]byte("{}"), bs, "idle ledger must marshal to exactly {}")
}

// TestIdleLoadLedgerUsable pins idle load must be usable (can Pose, gets ID 1).
func (s *PersistenceSuite) TestIdleLoadLedgerUsable() {
	ledger, err := interrupt.LoadLedger(interrupt.LedgerData{})
	s.Require().NoError(err)
	s.NotNil(ledger)

	// Can Pose and gets ID 1
	out, err := ledger.Pose(&interrupt.PoseInput{
		Audience: "test",
		Options:  []interrupt.Option{"opt"},
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out.Window.ID)
}

// TestRoundTripFresh pins fresh state round-trip.
func (s *PersistenceSuite) TestRoundTripFresh() {
	ledger1, err := interrupt.NewLedger()
	s.Require().NoError(err)

	data := ledger1.ToData()
	ledger2, err := interrupt.LoadLedger(data)
	s.Require().NoError(err)

	// Both should be idle
	open1, err := ledger1.Open()
	s.Require().NoError(err)
	s.Empty(open1)

	open2, err := ledger2.Open()
	s.Require().NoError(err)
	s.Empty(open2)
}

// TestRoundTripOpenWindows pins round-trip with open windows including nil payload.
func (s *PersistenceSuite) TestRoundTripOpenWindows() {
	ledger1, err := interrupt.NewLedger()
	s.Require().NoError(err)

	// Pose window 1: with payload
	out1, err := ledger1.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testAudience),
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  []byte("frozen"),
		At:       7,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out1.Window.ID)

	// Pose window 2: nil payload
	out2, err := ledger1.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testGrunk),
		Options:  []interrupt.Option{optAttack, testDecline},
		Payload:  nil,
		At:       0,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(2), out2.Window.ID)

	// Snapshot and reload
	data := ledger1.ToData()
	ledger2, err := interrupt.LoadLedger(data)
	s.Require().NoError(err)

	// Verify both windows are present and intact
	open, err := ledger2.Open()
	s.Require().NoError(err)
	s.Len(open, 2)
	s.Equal(interrupt.WindowID(1), open[0].ID)
	s.Equal(core.EntityID("aldric"), open[0].Audience)
	s.Equal([]byte("frozen"), open[0].Payload)
	s.Equal(uint64(7), open[0].At)

	s.Equal(interrupt.WindowID(2), open[1].ID)
	s.Equal(core.EntityID("grunk"), open[1].Audience)
	s.Nil(open[1].Payload)
	s.Equal(uint64(0), open[1].At)
}

// TestRoundTripAllAnswered pins all-answered state (NextID > 0, zero windows).
func (s *PersistenceSuite) TestRoundTripAllAnswered() {
	ledger1, err := interrupt.NewLedger()
	s.Require().NoError(err)

	// Pose and answer
	out, err := ledger1.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  []byte("frozen"),
	})
	s.Require().NoError(err)

	_, err = ledger1.Answer(&interrupt.AnswerInput{
		Window: out.Window.ID,
		By:     testAudience,
		Choice: testShield,
	})
	s.Require().NoError(err)

	// Snapshot and reload
	data := ledger1.ToData()
	ledger2, err := interrupt.LoadLedger(data)
	s.Require().NoError(err)

	// No open windows
	open, err := ledger2.Open()
	s.Require().NoError(err)
	s.Empty(open)

	// But next Pose continues the ID sequence
	out2, err := ledger2.Pose(&interrupt.PoseInput{
		Audience: testGrunk,
		Options:  []interrupt.Option{optAttack},
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(2), out2.Window.ID)
}

// TestRoundTripAnswerIntact pins Answer after reload returns same envelope.
func (s *PersistenceSuite) TestRoundTripAnswerIntact() {
	ledger1, err := interrupt.NewLedger()
	s.Require().NoError(err)

	posed, err := ledger1.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{testShield},
		Payload:  []byte("frozen"),
	})
	s.Require().NoError(err)

	// Snapshot before answer
	data := ledger1.ToData()
	ledger2, err := interrupt.LoadLedger(data)
	s.Require().NoError(err)

	// Answer on reloaded ledger
	answer, err := ledger2.Answer(&interrupt.AnswerInput{
		Window: posed.Window.ID,
		By:     testAudience,
		Choice: testShield,
	})
	s.Require().NoError(err)
	s.Equal(posed.Window.ID, answer.Window.ID)
	s.Equal([]byte("frozen"), answer.Window.Payload)
}

// TestGoldenJSONFresh pins fresh state golden JSON.
func (s *PersistenceSuite) TestGoldenJSONFresh() {
	idle, err := interrupt.NewLedger()
	s.Require().NoError(err)
	data := idle.ToData()
	bs, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Equal(string(bs), "{}")
}

// TestGoldenJSONOpen pins open windows golden JSON with exact-string pin.
func (s *PersistenceSuite) TestGoldenJSONOpen() {
	ledger, err := interrupt.NewLedger()
	s.Require().NoError(err)

	// Pose two windows as per design spec
	_, err = ledger.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  []byte("frozen"),
		At:       7,
	})
	s.Require().NoError(err)

	_, err = ledger.Pose(&interrupt.PoseInput{
		Audience: testGrunk,
		Options:  []interrupt.Option{optAttack, testDecline},
		Payload:  nil,
		At:       0,
	})
	s.Require().NoError(err)

	data := ledger.ToData()
	bs, err := json.Marshal(data)
	s.Require().NoError(err)

	expected := `{"next_id":3,"windows":[{"id":1,"audience":"aldric",` +
		`"options":["shield","decline"],"payload":"ZnJvemVu","at":7},` +
		`{"id":2,"audience":"grunk","options":["attack","decline"]}]}`
	s.Equal(expected, string(bs))
}

// TestGoldenJSONAllAnswered pins all-answered state golden JSON.
func (s *PersistenceSuite) TestGoldenJSONAllAnswered() {
	ledger, err := interrupt.NewLedger()
	s.Require().NoError(err)

	out, err := ledger.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  []byte("frozen"),
	})
	s.Require().NoError(err)

	_, err = ledger.Answer(&interrupt.AnswerInput{
		Window: out.Window.ID,
		By:     testAudience,
		Choice: testShield,
	})
	s.Require().NoError(err)

	data := ledger.ToData()
	bs, err := json.Marshal(data)
	s.Require().NoError(err)

	expected := `{"next_id":2}`
	s.Equal(expected, string(bs))
}

// TestToDataSnapshotImmunity pins ToData immunity: mutating returned Data
// must not affect ledger.
func (s *PersistenceSuite) TestToDataSnapshotImmunity() {
	ledger, err := interrupt.NewLedger()
	s.Require().NoError(err)

	_, err = ledger.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  []byte("frozen"),
	})
	s.Require().NoError(err)

	data := ledger.ToData()

	// Mutate returned data's slices
	if len(data.Windows) > 0 {
		data.Windows[0].Payload[0] = 'X'
		if len(data.Windows[0].Options) > 0 {
			data.Windows[0].Options[0] = "corrupted"
		}
	}

	// Re-query: internal state unchanged
	open, err := ledger.Open()
	s.Require().NoError(err)
	s.Len(open, 1)
	s.Equal([]byte("frozen"), open[0].Payload)
	s.Equal([]interrupt.Option{testShield, testDecline}, open[0].Options)
}

// TestLoadLedgerAliasing pins LoadLedger immunity: mutating caller's Data
// after Load must not affect ledger.
func (s *PersistenceSuite) TestLoadLedgerAliasing() {
	ledger1, err := interrupt.NewLedger()
	s.Require().NoError(err)

	_, err = ledger1.Pose(&interrupt.PoseInput{
		Audience: testAudience,
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  []byte("frozen"),
	})
	s.Require().NoError(err)

	data := ledger1.ToData()

	// Load the ledger
	ledger2, err := interrupt.LoadLedger(data)
	s.Require().NoError(err)

	// Mutate the caller's data
	if len(data.Windows) > 0 {
		data.Windows[0].Payload[0] = 'X'
		if len(data.Windows[0].Options) > 0 {
			data.Windows[0].Options[0] = "corrupted"
		}
	}

	// Loaded ledger is unaffected
	open, err := ledger2.Open()
	s.Require().NoError(err)
	s.Len(open, 1)
	s.Equal([]byte("frozen"), open[0].Payload)
	s.Equal([]interrupt.Option{testShield, testDecline}, open[0].Options)
}

// TestR9RejectionNextIDZeroWithWindows rejects R9: NextID == 0 with windows present.
func (s *PersistenceSuite) TestR9RejectionNextIDZeroWithWindows() {
	data := interrupt.LedgerData{
		NextID: 0,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield},
				Payload:  []byte("frozen"),
				At:       0,
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionWindowIDZero rejects R9: window ID of 0.
func (s *PersistenceSuite) TestR9RejectionWindowIDZero() {
	data := interrupt.LedgerData{
		NextID: 1,
		Windows: []interrupt.WindowData{
			{
				ID:       0,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionDescendingIDs rejects R9: window IDs not strictly ascending.
func (s *PersistenceSuite) TestR9RejectionDescendingIDs() {
	data := interrupt.LedgerData{
		NextID: 3,
		Windows: []interrupt.WindowData{
			{
				ID:       2,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield},
			},
			{
				ID:       1,
				Audience: testGrunk,
				Options:  []interrupt.Option{optAttack},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionDuplicateIDs rejects R9: duplicate window IDs.
func (s *PersistenceSuite) TestR9RejectionDuplicateIDs() {
	data := interrupt.LedgerData{
		NextID: 2,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield},
			},
			{
				ID:       1,
				Audience: testGrunk,
				Options:  []interrupt.Option{optAttack},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionIDEqualNextID rejects R9: window ID == NextID.
func (s *PersistenceSuite) TestR9RejectionIDEqualNextID() {
	data := interrupt.LedgerData{
		NextID: 1,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionIDGreaterNextID rejects R9: window ID > NextID.
func (s *PersistenceSuite) TestR9RejectionIDGreaterNextID() {
	data := interrupt.LedgerData{
		NextID: 1,
		Windows: []interrupt.WindowData{
			{
				ID:       2,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionEmptyAudience rejects R9: empty audience.
func (s *PersistenceSuite) TestR9RejectionEmptyAudience() {
	data := interrupt.LedgerData{
		NextID: 2,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: "",
				Options:  []interrupt.Option{testShield},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionNilOptions rejects R9: nil options.
func (s *PersistenceSuite) TestR9RejectionNilOptions() {
	data := interrupt.LedgerData{
		NextID: 2,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: testAudience,
				Options:  nil,
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionEmptyOptionsSlice rejects R9: empty options slice.
func (s *PersistenceSuite) TestR9RejectionEmptyOptionsSlice() {
	data := interrupt.LedgerData{
		NextID: 2,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: testAudience,
				Options:  []interrupt.Option{},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionEmptyOptionToken rejects R9: empty option token.
func (s *PersistenceSuite) TestR9RejectionEmptyOptionToken() {
	data := interrupt.LedgerData{
		NextID: 2,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield, "", "decline"},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9RejectionDuplicateOption rejects R9: duplicate option within a window.
func (s *PersistenceSuite) TestR9RejectionDuplicateOption() {
	data := interrupt.LedgerData{
		NextID: 2,
		Windows: []interrupt.WindowData{
			{
				ID:       1,
				Audience: testAudience,
				Options:  []interrupt.Option{testShield, "decline", "shield"},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

// TestR9AcceptanceNextIDWithZeroWindows pins R9 acceptance: NextID > 0
// with zero windows MUST load (all-answered state).
func (s *PersistenceSuite) TestR9AcceptanceNextIDWithZeroWindows() {
	data := interrupt.LedgerData{
		NextID:  3,
		Windows: []interrupt.WindowData{},
	}
	ledger, err := interrupt.LoadLedger(data)
	s.Require().NoError(err)
	s.NotNil(ledger)

	// Verify next Pose continues ID sequence
	out, err := ledger.Pose(&interrupt.PoseInput{
		Audience: "grunk",
		Options:  []interrupt.Option{"attack"},
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(3), out.Window.ID)
}

// TestR9WraparoundForgerMaxUint64 rejects R9: window with ID MaxUint64 and NextID 0.
func (s *PersistenceSuite) TestR9WraparoundForgerMaxUint64() {
	data := interrupt.LedgerData{
		NextID: 0,
		Windows: []interrupt.WindowData{
			{
				ID:       interrupt.WindowID(math.MaxUint64),
				Audience: testAudience,
				Options:  []interrupt.Option{testShield},
			},
		},
	}
	_, err := interrupt.LoadLedger(data)
	s.Require().Error(err)
	s.ErrorIs(err, interrupt.ErrInvalidData)
}

func TestPersistenceSuite(t *testing.T) {
	suite.Run(t, new(PersistenceSuite))
}

func TestLedgerSuite(t *testing.T) {
	suite.Run(t, new(LedgerSuite))
}
