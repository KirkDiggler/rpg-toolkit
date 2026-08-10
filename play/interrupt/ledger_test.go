// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package interrupt_test

import (
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
		Options:  []interrupt.Option{optShield},
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

	second, err := s.ledger.Pose(&interrupt.PoseInput{Audience: "grunk", Options: []interrupt.Option{"attack"}})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(2), second.Window.ID, "failed Pose must not consume an ID (R5)")
}

func TestLedgerSuite(t *testing.T) {
	suite.Run(t, new(LedgerSuite))
}
