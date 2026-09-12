// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// ChangedSuite pins SurveilOutput.Changed and ReportOutput.Changed — the
// bytes.Equal comparison Surveil and Report already make, to decide whether
// Observed moves, reported instead of thrown away.
//
// The property every test here defends is that Changed answers exactly one
// question — did the held payload differ from the one that just landed —
// and nothing else: not whether At moved, not whether the subject is new,
// not whether it just returned from being a ghost. A version that derived
// Changed from observed == in.At instead of from the bytes would pass most
// of these by accident; TestTwoPassesAtTheSameAtDoNotChange is the one that
// catches it.
type ChangedSuite struct {
	suite.Suite
	intel *intel.Intel
}

func (s *ChangedSuite) SetupTest() {
	var err error
	s.intel, err = intel.NewIntel()
	s.Require().NoError(err)
}

const (
	witness = core.EntityID("witness")
	mark    = intel.Subject("mark")
)

// surveil percepts exactly the subject/payload pairs given, on the sight
// channel. Subjects omitted from a later call are the point of that call as
// much as the ones given — Surveil derives fading from absence.
func (s *ChangedSuite) surveil(at uint64, percept ...intel.Report) *intel.SurveilOutput {
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: witness, Channel: intel.Sight, At: at, Percept: percept,
	})
	s.Require().NoError(err)
	return out
}

// report lands one piece of discrete testimony for mark, on a non-sight
// channel, so it is unambiguous which verb produced a given assertion.
func (s *ChangedSuite) report(at uint64, payload []byte) *intel.ReportOutput {
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: witness, Channel: "whispers", At: at,
		Reports: []intel.Report{{Subject: mark, Payload: payload}},
	})
	s.Require().NoError(err)
	return out
}

// Case 1: first contact is never a change — nothing was held to differ from.
func (s *ChangedSuite) TestFirstContactIsNeverChanged() {
	out := s.surveil(1, intel.Report{Subject: mark, Payload: []byte("v1")})

	s.Len(out.FirstContact, 1)
	s.Empty(out.Changed, "nothing was held before this pass")
}

// Case 2: the same payload landing again confirms but does not change.
func (s *ChangedSuite) TestSamePayloadRefreshesWithoutChanging() {
	s.surveil(1, intel.Report{Subject: mark, Payload: []byte("v1")})

	out := s.surveil(2, intel.Report{Subject: mark, Payload: []byte("v1")})
	s.Equal([]intel.Subject{mark}, out.Refreshed)
	s.Empty(out.Changed, "identical bytes: nothing changed")
}

// Case 3: a different payload is both a refresh and a change — Changed
// REFINES Refreshed, it does not replace it. Removing `out.Refreshed =
// append(...)` from this branch would leave this assertion true but the
// compatibility promise broken; TestAReacquiredSubjectIsAlsoRefreshed in
// reacquire_test.go is the sibling mutation guard for that promise.
func (s *ChangedSuite) TestDifferentPayloadIsRefreshedAndChanged() {
	s.surveil(1, intel.Report{Subject: mark, Payload: []byte("v1")})

	out := s.surveil(2, intel.Report{Subject: mark, Payload: []byte("v2")})
	s.Equal([]intel.Subject{mark}, out.Refreshed)
	s.Equal([]intel.Subject{mark}, out.Changed)
}

// Case 4: a ghost that returns with the exact content it left with is
// re-acquired, but that is a currency transition, not a content one.
func (s *ChangedSuite) TestReacquiredWithSamePayloadIsNotChanged() {
	s.surveil(1, intel.Report{Subject: mark, Payload: []byte("v1")})
	s.surveil(2) // empty percept: fades

	out := s.surveil(3, intel.Report{Subject: mark, Payload: []byte("v1")})
	s.Equal([]intel.Subject{mark}, out.Reacquired)
	s.Empty(out.Changed, "same content — only currency returned")
}

// Case 5: the same return, but the content is different this time — both
// transitions fire together, because both are true of this pass.
func (s *ChangedSuite) TestReacquiredWithDifferentPayloadIsChanged() {
	s.surveil(1, intel.Report{Subject: mark, Payload: []byte("v1")})
	s.surveil(2) // empty percept: fades

	out := s.surveil(3, intel.Report{Subject: mark, Payload: []byte("v2")})
	s.Equal([]intel.Subject{mark}, out.Reacquired)
	s.Equal([]intel.Subject{mark}, out.Changed)
}

// Case 6: Report obeys the identical rule — the same payload updates
// without changing.
func (s *ChangedSuite) TestReportSamePayloadUpdatesWithoutChanging() {
	s.report(1, []byte("v1"))

	out := s.report(2, []byte("v1"))
	s.Equal([]intel.Subject{mark}, out.Updated)
	s.Empty(out.Changed)
}

// Case 7: Report with a different payload is updated and changed.
func (s *ChangedSuite) TestReportDifferentPayloadIsUpdatedAndChanged() {
	s.report(1, []byte("v1"))

	out := s.report(2, []byte("v2"))
	s.Equal([]intel.Subject{mark}, out.Updated)
	s.Equal([]intel.Subject{mark}, out.Changed)
}

// Case 8: nil and empty payloads are the same identical-nothing, so landing
// one over the other is not a change — the same rule that governs Observed.
func (s *ChangedSuite) TestNilThenEmptyPayloadIsNotChanged() {
	s.surveil(1, intel.Report{Subject: mark, Payload: nil})

	out := s.surveil(2, intel.Report{Subject: mark, Payload: []byte{}})
	s.Empty(out.Changed, "nil and empty are the same legal empty payload")
}

// Case 9: THE WHOLE POINT OF THE ISSUE. Two passes land at the identical At
// with identical bytes; the second must not report Changed. At never moves
// between these two calls, so a version that derived Changed from
// observed == in.At (instead of from the payload comparison Surveil already
// makes) cannot tell these passes apart from a real change and would fail
// this the other way — reporting Changed here just as it did in case 3.
func (s *ChangedSuite) TestTwoPassesAtTheSameAtDoNotChange() {
	s.surveil(5, intel.Report{Subject: mark, Payload: []byte("v1")})

	out := s.surveil(5, intel.Report{Subject: mark, Payload: []byte("v1")})
	s.Equal([]intel.Subject{mark}, out.Refreshed)
	s.Empty(out.Changed, "same At, same bytes: the payload never differed")
}

// Case 10: several changed subjects in one percept arrive in PERCEPT ORDER,
// like FirstContact, Refreshed and Reacquired beside them — not sorted
// order. Faded is the only list that sorts, because it walks a map and has
// no order of its own; this pass walks the deduped percept and already has
// one.
func (s *ChangedSuite) TestChangedArrivesInPerceptOrder() {
	const (
		alpha = intel.Subject("alpha")
		omega = intel.Subject("omega")
	)
	s.surveil(1,
		intel.Report{Subject: alpha, Payload: []byte("a1")},
		intel.Report{Subject: omega, Payload: []byte("o1")},
	)

	out := s.surveil(2,
		intel.Report{Subject: omega, Payload: []byte("o2")},
		intel.Report{Subject: alpha, Payload: []byte("a2")},
	)
	s.Equal([]intel.Subject{omega, alpha}, out.Changed,
		"percept order, not sorted order — omega was named first")
}

func TestChangedSuite(t *testing.T) {
	suite.Run(t, new(ChangedSuite))
}
