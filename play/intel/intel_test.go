// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

const (
	testObserver = core.EntityID("alice")
	testSubject  = intel.Subject("s")
)

type IntelSuite struct {
	suite.Suite
	intel *intel.Intel
}

func (s *IntelSuite) SetupTest() {
	var err error
	s.intel, err = intel.NewIntel()
	s.Require().NoError(err)
}

func (s *IntelSuite) holdingOn(observer core.EntityID, subject intel.Subject) intel.Holding {
	h, err := s.intel.On(&intel.OnInput{Observer: observer, Subject: subject})
	s.Require().NoError(err)
	return h
}

func (s *IntelSuite) TestReportLandsHeldIntel() {
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: "alice", Channel: intel.Channel("hearing"), At: 5,
		Reports: []intel.Report{{Subject: "behind-door-3", Payload: []byte("crashing")}},
	})
	s.Require().NoError(err)
	s.Equal([]intel.Report{{Subject: "behind-door-3", Payload: []byte("crashing")}}, out.FirstContact)
	s.Empty(out.Updated)

	h := s.holdingOn("alice", "behind-door-3")
	s.Equal([]byte("crashing"), h.Payload)
	s.Equal(intel.Channel("hearing"), h.Channel)
	s.Equal(uint64(5), h.At)
	s.Nil(h.CurrentVia)
	s.Equal(intel.Held, h.Status)
}

func (s *IntelSuite) TestReportOverwritesLastWins() {
	s.overwriteTest("v1", 5, "hearing", "v2", 9, "rumor")
}

func (s *IntelSuite) overwriteTest(p1 string, at1 uint64, ch1 string, p2 string, at2 uint64, ch2 string) {
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: testObserver, Channel: intel.Channel(ch1), At: at1,
		Reports: []intel.Report{{Subject: testSubject, Payload: []byte(p1)}},
	})
	s.Require().NoError(err)
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: testObserver, Channel: intel.Channel(ch2), At: at2,
		Reports: []intel.Report{{Subject: testSubject, Payload: []byte(p2)}},
	})
	s.Require().NoError(err)
	s.Empty(out.FirstContact)
	s.Equal([]intel.Subject{testSubject}, out.Updated)
	h := s.holdingOn(testObserver, testSubject)
	s.Equal([]byte(p2), h.Payload)
	s.Equal(intel.Channel(ch2), h.Channel) // provenance follows latest testimony
	s.Equal(at2, h.At)
}

func (s *IntelSuite) TestReportDedupeAndValidationOrder() {
	// dedupe-first, last wins, survivor at last occurrence's position
	out, err := s.intel.Report(&intel.ReportInput{Observer: "a", Channel: "c", Reports: []intel.Report{
		{Subject: "x", Payload: []byte("old")}, {Subject: "y", Payload: []byte("y")},
		{Subject: "x", Payload: []byte("new")},
	}})
	s.Require().NoError(err)
	s.Equal([]intel.Report{{Subject: "y", Payload: []byte("y")}, {Subject: "x", Payload: []byte("new")}}, out.FirstContact)
	// validation order: nil → observer → channel → subjects; first failure wins; R5 no mutation
	_, err = s.intel.Report(nil)
	s.Require().ErrorIs(err, intel.ErrNilInput)
	_, err = s.intel.Report(&intel.ReportInput{Observer: "", Channel: "", Reports: []intel.Report{{Subject: ""}}})
	s.Require().ErrorIs(err, intel.ErrNoObserver)
	_, err = s.intel.Report(&intel.ReportInput{Observer: "a", Channel: "", Reports: []intel.Report{{Subject: ""}}})
	s.Require().ErrorIs(err, intel.ErrNoChannel)
	_, err = s.intel.Report(&intel.ReportInput{Observer: "a", Channel: "c", Reports: []intel.Report{{Subject: ""}}})
	s.Require().ErrorIs(err, intel.ErrNoSubject)
	held, err := s.intel.HeldBy(&intel.HeldByInput{Observer: "a"})
	s.Require().NoError(err)
	s.Len(held, 2, "failed calls changed nothing (R5)")
}

func (s *IntelSuite) TestReportAtBlindness() {
	// Later testimony carrying a SMALLER At still wins
	s.overwriteTest("v1", 9, "hearing", "v2", 3, "rumor")
}

func TestIntelSuite(t *testing.T) {
	suite.Run(t, new(IntelSuite))
}
