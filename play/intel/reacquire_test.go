// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// ReacquireSuite pins SurveilOutput.Reacquired — the inverse of Faded, and
// the one transition Refreshed cannot express on its own.
//
// Every test here is written as a SEQUENCE of passes, because re-acquisition
// is not a property of a percept: the same percept is a refresh or a return
// depending only on what the holding looked like a moment earlier. A test
// that surveils once can never fail for the right reason.
type ReacquireSuite struct {
	suite.Suite
	intel *intel.Intel
}

func (s *ReacquireSuite) SetupTest() {
	var err error
	s.intel, err = intel.NewIntel()
	s.Require().NoError(err)
}

const (
	watcher = core.EntityID("watcher")
	quarry  = intel.Subject("quarry")
	hearing = intel.Channel("hearing")
)

// see surveils one channel with exactly the subjects named, which is the
// whole of a percept: Surveil derives fading from what is ABSENT, so the
// subjects omitted here are the point of the call as much as the ones given.
func (s *ReacquireSuite) see(ch intel.Channel, at uint64, subjects ...intel.Subject) *intel.SurveilOutput {
	percept := make([]intel.Report, 0, len(subjects))
	for _, subj := range subjects {
		percept = append(percept, intel.Report{Subject: subj, Payload: fmt.Appendf(nil, "at %d", at)})
	}
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: watcher, Channel: ch, At: at, Percept: percept,
	})
	s.Require().NoError(err)
	return out
}

// A subject nobody ever held is FIRST CONTACT and nothing else. There was no
// holding to be dark, so there is nothing to have come back.
func (s *ReacquireSuite) TestFirstContactIsNotAReacquisition() {
	out := s.see(intel.Sight, 1, quarry)

	s.Len(out.FirstContact, 1)
	s.Empty(out.Reacquired, "nothing returned — this is the first time anyone looked")
	s.Empty(out.Refreshed)
}

// THE NOISE TEST, and the reason this field exists. An observer who never
// looks away refreshes on every single pass; if Reacquired fired with it,
// a consumer emitting on Reacquired would emit constantly and the field
// would carry no information at all.
func (s *ReacquireSuite) TestUnbrokenWatchNeverReacquires() {
	s.see(intel.Sight, 1, quarry)

	for at := uint64(2); at <= 5; at++ {
		out := s.see(intel.Sight, at, quarry)
		s.Equal([]intel.Subject{quarry}, out.Refreshed, "still watching, pass %d", at)
		s.Empty(out.Reacquired, "never looked away, pass %d", at)
		s.Empty(out.Faded, "pass %d", at)
	}
}

// THE MOMENT ITSELF: seen, lost, seen again. Reacquired fires on exactly one
// pass — the returning one — and on no other.
func (s *ReacquireSuite) TestAGhostThatComesBackReacquiresOnThatPassAlone() {
	s.see(intel.Sight, 1, quarry)

	gone := s.see(intel.Sight, 2)
	s.Equal([]intel.Subject{quarry}, gone.Faded, "walked out of view")
	s.Empty(gone.Reacquired)

	// Still a ghost, and a second empty percept must not re-fade or
	// re-acquire it: the holding is already dark and nothing changed.
	stillGone := s.see(intel.Sight, 3)
	s.Empty(stillGone.Faded, "already a ghost — fading is a transition, not a state")
	s.Empty(stillGone.Reacquired)

	back := s.see(intel.Sight, 4, quarry)
	s.Equal([]intel.Subject{quarry}, back.Reacquired, "the ghost became real again")
	s.Empty(back.FirstContact, "the holding was never gone, only dark")

	held := s.see(intel.Sight, 5, quarry)
	s.Empty(held.Reacquired, "back for a second pass is not coming back again")
}

// REACQUIRED REFINES REFRESHED — it does not carve subjects out of it.
//
// This is the mutation guard for the compatibility promise in the field's
// doc: every existing caller reads FirstContact ∪ Refreshed as "everything I
// perceive right now", and moving re-acquisitions out of Refreshed would make
// each of those callers silently drop exactly the subject that just came back.
// Remove `out.Refreshed = append(...)` from the ghost branch and this fails.
func (s *ReacquireSuite) TestAReacquiredSubjectIsAlsoRefreshed() {
	s.see(intel.Sight, 1, quarry)
	s.see(intel.Sight, 2)

	back := s.see(intel.Sight, 3, quarry)

	s.Equal([]intel.Subject{quarry}, back.Reacquired)
	s.Equal([]intel.Subject{quarry}, back.Refreshed,
		"a holding already existed, so Refreshed is true of it too")
}

// CHANNELS ARE THE WHOLE OF THE TEST. Sight looks away while hearing keeps
// the subject current, so the holding is never a ghost — and sight coming
// back is a refresh, because nothing had gone.
func (s *ReacquireSuite) TestAnotherChannelHoldingItCurrentMeansNothingReturned() {
	s.see(intel.Sight, 1, quarry)
	s.see(hearing, 1, quarry)

	lost := s.see(intel.Sight, 2)
	s.Empty(lost.Faded, "hearing still has them — losing one channel is not fading")

	back := s.see(intel.Sight, 3, quarry)
	s.Empty(back.Reacquired, "never dark, so nothing came back")
	s.Equal([]intel.Subject{quarry}, back.Refreshed)
}

// ...AND THE SAME SUBJECT DOES RETURN once the last channel lets go. Same
// two channels as above, both dropped, so the distinction above is shown to
// be about currency rather than about there being a second channel at all.
func (s *ReacquireSuite) TestWhenTheLastChannelLetsGoTheReturnIsReal() {
	s.see(intel.Sight, 1, quarry)
	s.see(hearing, 1, quarry)

	s.see(intel.Sight, 2)
	silence := s.see(hearing, 2)
	s.Equal([]intel.Subject{quarry}, silence.Faded, "the last channel let go")

	back := s.see(intel.Sight, 3, quarry)
	s.Equal([]intel.Subject{quarry}, back.Reacquired)
}

// A REPORTED HOLDING IS BORN DARK — Report lands intel as Held with no
// currentVia (a rumor is not a sighting), so the first time sight confirms it
// the holding goes from dark to current and that IS a return. The observer
// knew of them; now they can see them. A consumer emitting on Reacquired
// wants exactly that moment.
func (s *ReacquireSuite) TestRumorConfirmedBySightIsAReacquisition() {
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: watcher, Channel: hearing, At: 1,
		Reports: []intel.Report{{Subject: quarry, Payload: []byte("a scrape of steel")}},
	})
	s.Require().NoError(err)
	s.Require().Nil(s.heldBy(quarry).CurrentVia, "a rumor is held, not current")

	out := s.see(intel.Sight, 2, quarry)
	s.Equal([]intel.Subject{quarry}, out.Reacquired, "the rumor now has a body")
	s.Empty(out.FirstContact, "Report already created the holding")
}

// OBSERVERS DO NOT SHARE TRANSITIONS. One observer's ghost returning says
// nothing about anybody else's, and the holdings are per-observer maps.
func (s *ReacquireSuite) TestReacquisitionIsPerObserver() {
	const other = core.EntityID("other")

	s.see(intel.Sight, 1, quarry)
	s.see(intel.Sight, 2)

	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: other, Channel: intel.Sight, At: 3,
		Percept: []intel.Report{{Subject: quarry, Payload: []byte("x")}},
	})
	s.Require().NoError(err)
	s.Empty(out.Reacquired, "this observer never held them, so nothing returned")
	s.Len(out.FirstContact, 1)
}

// MULTIPLE RETURNS ARRIVE IN PERCEPT ORDER, like FirstContact and Refreshed
// beside them. Faded sorts because it walks a map and has no order of its
// own; this pass walks the deduped percept and already has one.
func (s *ReacquireSuite) TestReacquiredArrivesInPerceptOrder() {
	const (
		alpha = intel.Subject("alpha")
		omega = intel.Subject("omega")
	)
	s.see(intel.Sight, 1, alpha, omega)
	s.see(intel.Sight, 2)

	back := s.see(intel.Sight, 3, omega, alpha)
	s.Equal([]intel.Subject{omega, alpha}, back.Reacquired,
		"percept order, not sorted order — omega was named first")
}

func (s *ReacquireSuite) heldBy(subject intel.Subject) intel.Holding {
	h, err := s.intel.On(&intel.OnInput{Observer: watcher, Subject: subject})
	s.Require().NoError(err)
	return h
}

func TestReacquireSuite(t *testing.T) {
	suite.Run(t, new(ReacquireSuite))
}
