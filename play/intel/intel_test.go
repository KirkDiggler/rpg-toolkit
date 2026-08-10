// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
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
	const (
		testObserver = core.EntityID("alice")
		testChannel  = intel.Channel("hearing")
		testSubject  = intel.Subject("behind-door-3")
		testPayload  = "crashing"
		testAt       = uint64(5)
	)
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: testObserver, Channel: testChannel, At: testAt,
		Reports: []intel.Report{{Subject: testSubject, Payload: []byte(testPayload)}},
	})
	s.Require().NoError(err)
	s.Equal([]intel.Report{{Subject: testSubject, Payload: []byte(testPayload)}}, out.FirstContact)
	s.Empty(out.Updated)

	h := s.holdingOn(testObserver, testSubject)
	s.Equal([]byte(testPayload), h.Payload)
	s.Equal(testChannel, h.Channel)
	s.Equal(testAt, h.At)
	s.Nil(h.CurrentVia)
	s.Equal(intel.Held, h.Status)
}

func (s *IntelSuite) TestReportOverwritesLastWins() {
	s.overwriteTest("v1", 5, "hearing", "v2", 9, "rumor")
}

func (s *IntelSuite) overwriteTest(p1 string, at1 uint64, ch1 string, p2 string, at2 uint64, ch2 string) {
	const (
		testObserver = core.EntityID("alice")
		testSubject  = intel.Subject("s")
	)
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

func (s *IntelSuite) TestReportCopyOutPayload() {
	// Pin: mutating returned FirstContact payload must not corrupt stored payload
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: "alice", Channel: "hearing", At: 5,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("original")}},
	})
	s.Require().NoError(err)
	s.Len(out.FirstContact, 1)

	// Mutate the returned FirstContact payload
	out.FirstContact[0].Payload[0] = 'X'

	// Internal storage must be unchanged
	h := s.holdingOn("alice", "s")
	s.Equal([]byte("original"), h.Payload, "stored payload must be immune to mutation of FirstContact")
}

func (s *IntelSuite) TestReportEmptyReportsNoPhantomObserver() {
	// Pin: empty/nil reports after dedupe must not create phantom observer state
	const ghostObs = core.EntityID("ghost-obs")
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: ghostObs, Channel: "c", Reports: nil,
	})
	s.Require().NoError(err)
	s.Empty(out.FirstContact)
	s.Empty(out.Updated)

	// Verify no phantom state: HeldBy returns empty
	held, err := s.intel.HeldBy(&intel.HeldByInput{Observer: ghostObs})
	s.Require().NoError(err)
	s.Empty(held, "empty reports must not create phantom observer")

	// Verify subsequent normal flow doesn't linger: Report a real holding
	out2, err := s.intel.Report(&intel.ReportInput{
		Observer: ghostObs, Channel: "c",
		Reports: []intel.Report{{Subject: "real", Payload: []byte("data")}},
	})
	s.Require().NoError(err)
	s.Len(out2.FirstContact, 1)
	held2, err := s.intel.HeldBy(&intel.HeldByInput{Observer: ghostObs})
	s.Require().NoError(err)
	s.Len(held2, 1, "only real report should be held")
}

// --- Surveil Tests ---

func (s *IntelSuite) TestSurveilFirstContactRefreshFade() {
	const (
		testObserver = core.EntityID("alice")
		goblina      = intel.Subject("goblin-A")
		hex1         = intel.Subject("hex-1")
		wounded      = "wounded"
		floor        = "floor"
	)
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: testObserver, Channel: intel.Sight, At: 1,
		Percept: []intel.Report{
			{Subject: goblina, Payload: []byte(wounded)},
			{Subject: hex1, Payload: []byte(floor)},
		},
	})
	s.Require().NoError(err)
	s.Len(out.FirstContact, 2)
	s.Empty(out.Refreshed)
	s.Empty(out.Faded)
	s.Equal(intel.Current, s.holdingOn(testObserver, goblina).Status)
	s.Equal([]intel.Channel{intel.Sight}, s.holdingOn(testObserver, goblina).CurrentVia)

	// next percept: goblin gone, hex remains → ghost goblin fades, hex refreshes
	out, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: testObserver, Channel: intel.Sight, At: 2,
		Percept: []intel.Report{{Subject: hex1, Payload: []byte(floor)}},
	})
	s.Require().NoError(err)
	s.Empty(out.FirstContact)
	s.Equal([]intel.Subject{hex1}, out.Refreshed)
	s.Equal([]intel.Subject{goblina}, out.Faded)
	g := s.holdingOn(testObserver, goblina)
	s.Equal(intel.Held, g.Status)
	s.Equal([]byte(wounded), g.Payload, "the ghost goblin: held at last observation")
	s.Nil(g.CurrentVia)
}

func (s *IntelSuite) TestSurveilPerChannelCurrency() {
	const (
		testObserver = core.EntityID("m")
		preySubject  = intel.Subject("prey")
		near         = "near"
		tremorsense  = intel.Channel("tremorsense")
	)
	for _, ch := range []intel.Channel{intel.Sight, tremorsense} {
		_, err := s.intel.Surveil(&intel.SurveilInput{Observer: testObserver, Channel: ch, At: 1,
			Percept: []intel.Report{{Subject: preySubject, Payload: []byte(near)}}})
		s.Require().NoError(err)
	}
	s.Equal([]intel.Channel{intel.Sight, tremorsense}, s.holdingOn(testObserver, preySubject).CurrentVia)
	// sight loses it; tremorsense still sustains → NOT faded, still Current
	out, err := s.intel.Surveil(&intel.SurveilInput{Observer: testObserver, Channel: intel.Sight, At: 2, Percept: nil})
	s.Require().NoError(err)
	s.Empty(out.Faded)
	s.Equal(intel.Current, s.holdingOn(testObserver, preySubject).Status)
	s.Equal([]intel.Channel{tremorsense}, s.holdingOn(testObserver, preySubject).CurrentVia)
	// tremorsense loses it too → fades now
	out, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: testObserver, Channel: tremorsense, At: 3, Percept: nil})
	s.Require().NoError(err)
	s.Equal([]intel.Subject{preySubject}, out.Faded)
	s.Equal(intel.Held, s.holdingOn(testObserver, preySubject).Status)
}

// TestSurveilEmptyPerceptIsLegalAndFadesAll tests that an empty percept is
// legal and fades all subjects sustained by that channel.
func (s *IntelSuite) TestSurveilEmptyPerceptIsLegalAndFadesAll() {
	const (
		testObserver = core.EntityID("alice")
		goblin1      = intel.Subject("goblin-1")
		goblin2      = intel.Subject("goblin-2")
		far          = "far"
		nearStr      = "near"
	)
	// Set up two subjects
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: testObserver,
		Channel:  intel.Sight,
		At:       1,
		Percept: []intel.Report{
			{Subject: goblin1, Payload: []byte(far)},
			{Subject: goblin2, Payload: []byte(nearStr)},
		},
	})
	s.Require().NoError(err)
	s.Len(out.FirstContact, 2)

	// Empty percept fades both
	out, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: testObserver,
		Channel:  intel.Sight,
		At:       2,
		Percept:  []intel.Report{},
	})
	s.Require().NoError(err)
	s.Empty(out.FirstContact)
	s.Empty(out.Refreshed)
	// Faded must be sorted by Subject
	s.Equal([]intel.Subject{goblin1, goblin2}, out.Faded)

	// Both are now Held with payloads retained
	g1 := s.holdingOn(testObserver, goblin1)
	s.Equal(intel.Held, g1.Status)
	s.Equal([]byte(far), g1.Payload)

	g2 := s.holdingOn(testObserver, goblin2)
	s.Equal(intel.Held, g2.Status)
	s.Equal([]byte(nearStr), g2.Payload)
}

// TestSurveilDoesNotDisturbReportedHoldings tests that Surveil does not
// fade subjects that were held via Report (with nil CurrentVia).
func (s *IntelSuite) TestSurveilDoesNotDisturbReportedHoldings() {
	const (
		testObserver  = core.EntityID("bob")
		rumorChannel  = intel.Channel("rumor")
		rumoredNPC    = intel.Subject("rumored-npc")
		dangerous     = "dangerous"
		visibleGoblin = intel.Subject("visible-goblin")
		attacking     = "attacking"
	)
	// Report a subject (held, no channels sustaining it)
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: testObserver,
		Channel:  rumorChannel,
		At:       1,
		Reports: []intel.Report{
			{Subject: rumoredNPC, Payload: []byte(dangerous)},
		},
	})
	s.Require().NoError(err)

	// Surveil sight percept without that subject
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: testObserver,
		Channel:  intel.Sight,
		At:       2,
		Percept: []intel.Report{
			{Subject: visibleGoblin, Payload: []byte(attacking)},
		},
	})
	s.Require().NoError(err)
	s.Len(out.FirstContact, 1)
	s.Empty(out.Faded) // rumored-npc not faded
	s.Empty(out.Refreshed)

	// rumored-npc is still Held with payload intact
	r := s.holdingOn(testObserver, rumoredNPC)
	s.Equal(intel.Held, r.Status)
	s.Equal([]byte(dangerous), r.Payload)
	s.Nil(r.CurrentVia)
}

// TestReportOverwriteKeepsCurrency tests that Report can overwrite a
// Surveil-held subject without changing CurrentVia or Status.
func (s *IntelSuite) TestReportOverwriteKeepsCurrency() {
	const (
		testObserver = core.EntityID("charlie")
		target       = intel.Subject("target")
		tenMAway     = "10m away"
		castingSpell = "casting a spell"
		rumorCh      = intel.Channel("rumor")
	)
	// Surveil subject "target" via sight
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: testObserver,
		Channel:  intel.Sight,
		At:       1,
		Percept: []intel.Report{
			{Subject: target, Payload: []byte(tenMAway)},
		},
	})
	s.Require().NoError(err)

	h := s.holdingOn(testObserver, target)
	s.Equal(intel.Current, h.Status)
	s.Equal([]intel.Channel{intel.Sight}, h.CurrentVia)
	s.Equal([]byte(tenMAway), h.Payload)

	// Report new intel on same subject via "rumor"
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: testObserver,
		Channel:  rumorCh,
		At:       2,
		Reports: []intel.Report{
			{Subject: target, Payload: []byte(castingSpell)},
		},
	})
	s.Require().NoError(err)
	s.Empty(out.FirstContact)
	s.Equal([]intel.Subject{target}, out.Updated)

	// Payload and Channel changed, but CurrentVia and Status unchanged
	h = s.holdingOn(testObserver, target)
	s.Equal(intel.Current, h.Status)
	s.Equal([]intel.Channel{intel.Sight}, h.CurrentVia) // Unchanged
	s.Equal([]byte(castingSpell), h.Payload)            // Overwritten
	s.Equal(rumorCh, h.Channel)                         // Provenance changed
}

// TestSurveilValidationOrderAndDedupe tests validation order and deduplication.
func (s *IntelSuite) TestSurveilValidationOrderAndDedupe() {
	const (
		dave   = core.EntityID("dave")
		eve    = core.EntityID("eve")
		dupe   = intel.Subject("dupe")
		unique = intel.Subject("unique")
		first  = "first"
		only   = "only"
		last   = "last"
	)
	// nil input
	_, err := s.intel.Surveil(nil)
	s.ErrorIs(err, intel.ErrNilInput)

	// empty observer
	_, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: "",
		Channel:  intel.Sight,
		Percept:  []intel.Report{},
	})
	s.ErrorIs(err, intel.ErrNoObserver)

	// empty channel
	_, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: dave,
		Channel:  "",
		Percept:  []intel.Report{},
	})
	s.ErrorIs(err, intel.ErrNoChannel)

	// empty subject in percept
	_, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: dave,
		Channel:  intel.Sight,
		Percept: []intel.Report{
			{Subject: "", Payload: []byte("bad")},
		},
	})
	s.ErrorIs(err, intel.ErrNoSubject)

	// R5: guards precede all mutation (initial state unchanged after errors)
	members, err := s.intel.HeldBy(&intel.HeldByInput{Observer: dave})
	s.Require().NoError(err)
	s.Empty(members)

	// Dedupe: last entry wins, survivor at last position
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: eve,
		Channel:  intel.Sight,
		At:       1,
		Percept: []intel.Report{
			{Subject: dupe, Payload: []byte(first)},
			{Subject: unique, Payload: []byte(only)},
			{Subject: dupe, Payload: []byte(last)},
		},
	})
	s.Require().NoError(err)
	// FirstContact should have: unique first (at position 1 in original), then dupe (at last position)
	s.Len(out.FirstContact, 2)
	s.Equal(unique, out.FirstContact[0].Subject)
	s.Equal([]byte(only), out.FirstContact[0].Payload)
	s.Equal(dupe, out.FirstContact[1].Subject)
	s.Equal([]byte(last), out.FirstContact[1].Payload)

	// Verify the holding has the last payload
	dupeholder := s.holdingOn(eve, dupe)
	s.Equal([]byte(last), dupeholder.Payload)
}

// TestSurveilFadedSortedAndObserverIsolation tests that Faded is sorted by
// Subject and that observers are isolated.
func (s *IntelSuite) TestSurveilFadedSortedAndObserverIsolation() {
	const (
		alice   = core.EntityID("alice")
		bob     = core.EntityID("bob")
		zebra   = intel.Subject("zebra")
		apple   = intel.Subject("apple")
		moon    = intel.Subject("moon")
		far     = "far"
		nearStr = "near"
		sky     = "sky"
		bobSees = "bob sees"
	)
	// alice perceives three subjects
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: alice,
		Channel:  intel.Sight,
		At:       1,
		Percept: []intel.Report{
			{Subject: zebra, Payload: []byte(far)},
			{Subject: apple, Payload: []byte(nearStr)},
			{Subject: moon, Payload: []byte(sky)},
		},
	})
	s.Require().NoError(err)

	// bob perceives the same three subjects
	_, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: bob,
		Channel:  intel.Sight,
		At:       1,
		Percept: []intel.Report{
			{Subject: zebra, Payload: []byte(bobSees)},
			{Subject: apple, Payload: []byte(bobSees)},
			{Subject: moon, Payload: []byte(bobSees)},
		},
	})
	s.Require().NoError(err)

	// alice loses all three
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: alice,
		Channel:  intel.Sight,
		At:       2,
		Percept:  []intel.Report{},
	})
	s.Require().NoError(err)
	// Faded must be sorted by Subject
	s.Equal([]intel.Subject{apple, moon, zebra}, out.Faded)

	// bob's holdings are untouched; all still Current
	for _, subj := range []intel.Subject{zebra, apple, moon} {
		h, err := s.intel.On(&intel.OnInput{Observer: bob, Subject: subj})
		s.Require().NoError(err)
		s.Equal(intel.Current, h.Status)
	}

	// alice's holdings are Held
	for _, subj := range []intel.Subject{zebra, apple, moon} {
		h, err := s.intel.On(&intel.OnInput{Observer: alice, Subject: subj})
		s.Require().NoError(err)
		s.Equal(intel.Held, h.Status)
	}
}

func TestIntelSuite(t *testing.T) {
	suite.Run(t, new(IntelSuite))
}
