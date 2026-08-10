// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel_test

import (
	"encoding/json"
	"errors"
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
	const alice = core.EntityID("alice")
	// Pin: mutating returned FirstContact payload must not corrupt stored payload
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: alice, Channel: "hearing", At: 5,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("original")}},
	})
	s.Require().NoError(err)
	s.Len(out.FirstContact, 1)

	// Mutate the returned FirstContact payload
	out.FirstContact[0].Payload[0] = 'X'

	// Internal storage must be unchanged
	h := s.holdingOn(alice, "s")
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

// QueriesSuite tests the read-side queries: HeldBy and On.
type QueriesSuite struct {
	suite.Suite
	intel *intel.Intel
}

func (s *QueriesSuite) SetupTest() {
	var err error
	s.intel, err = intel.NewIntel()
	s.Require().NoError(err)
}

// TestHeldByMultiHoldingSortedBySubject tests that HeldBy returns multiple
// holdings sorted by Subject with derived statuses.
func (s *QueriesSuite) TestHeldByMultiHoldingSortedBySubject() {
	const obs = core.EntityID("alice")
	// Report three subjects in non-alphabetical order
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "hearing", At: 1,
		Reports: []intel.Report{
			{Subject: "zebra", Payload: []byte("z")},
			{Subject: "apple", Payload: []byte("a")},
			{Subject: "moon", Payload: []byte("m")},
		},
	})
	s.Require().NoError(err)

	// Query all holdings
	holdings, err := s.intel.HeldBy(&intel.HeldByInput{Observer: obs})
	s.Require().NoError(err)
	s.Len(holdings, 3)

	// Verify sorted by Subject
	s.Equal(intel.Subject("apple"), holdings[0].Subject)
	s.Equal(intel.Subject("moon"), holdings[1].Subject)
	s.Equal(intel.Subject("zebra"), holdings[2].Subject)

	// Verify payloads
	s.Equal([]byte("a"), holdings[0].Payload)
	s.Equal([]byte("m"), holdings[1].Payload)
	s.Equal([]byte("z"), holdings[2].Payload)

	// Verify derived statuses (Report → Held, no sustaining channels)
	s.Equal(intel.Held, holdings[0].Status)
	s.Equal(intel.Held, holdings[1].Status)
	s.Equal(intel.Held, holdings[2].Status)
}

// TestHeldByUnknownObserverEmptyNil tests that an unknown observer
// returns empty slice, nil error (not a guessable zero value).
func (s *QueriesSuite) TestHeldByUnknownObserverEmptyNil() {
	holdings, err := s.intel.HeldBy(&intel.HeldByInput{Observer: "ghost-obs"})
	s.Require().NoError(err)
	s.Empty(holdings)
	s.NotNil(holdings, "must be empty slice, not nil")
}

// TestHeldByNilInputReturnsErrNilInput verifies guard clause.
func (s *QueriesSuite) TestHeldByNilInputReturnsErrNilInput() {
	holdings, err := s.intel.HeldBy(nil)
	s.Require().ErrorIs(err, intel.ErrNilInput)
	s.Nil(holdings)
}

// TestHeldByEmptyObserverReturnsErrNoObserver verifies field validation.
func (s *QueriesSuite) TestHeldByEmptyObserverReturnsErrNoObserver() {
	holdings, err := s.intel.HeldBy(&intel.HeldByInput{Observer: ""})
	s.Require().ErrorIs(err, intel.ErrNoObserver)
	s.Nil(holdings)
}

// TestOnFound returns the holding when observer and subject are held.
func (s *QueriesSuite) TestOnFound() {
	const (
		obs     = core.EntityID("bob")
		subject = intel.Subject("target")
		payload = "payload data"
		channel = intel.Channel("sight")
		at      = uint64(42)
	)
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: channel, At: at,
		Reports: []intel.Report{{Subject: subject, Payload: []byte(payload)}},
	})
	s.Require().NoError(err)

	holding, err := s.intel.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	s.Equal(subject, holding.Subject)
	s.Equal([]byte(payload), holding.Payload)
	s.Equal(channel, holding.Channel)
	s.Equal(at, holding.At)
	s.Equal(intel.Held, holding.Status)
}

// TestOnUnknownSubjectReturnsErrNotHeld when subject never held.
func (s *QueriesSuite) TestOnUnknownSubjectReturnsErrNotHeld() {
	const obs = core.EntityID("charlie")
	// Report something, but not the subject we query
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "ch", At: 1,
		Reports: []intel.Report{{Subject: "other", Payload: nil}},
	})
	s.Require().NoError(err)

	holding, err := s.intel.On(&intel.OnInput{Observer: obs, Subject: "never-held"})
	s.Require().ErrorIs(err, intel.ErrNotHeld)
	s.Equal(intel.Holding{}, holding, "must be zero value, not guessable")
}

// TestOnUnknownObserverReturnsErrNotHeld when observer has no holdings.
func (s *QueriesSuite) TestOnUnknownObserverReturnsErrNotHeld() {
	holding, err := s.intel.On(&intel.OnInput{Observer: "unknown", Subject: "any"})
	s.Require().ErrorIs(err, intel.ErrNotHeld)
	s.Equal(intel.Holding{}, holding)
}

// TestOnNilInputReturnsErrNilInput verifies guard clause.
func (s *QueriesSuite) TestOnNilInputReturnsErrNilInput() {
	holding, err := s.intel.On(nil)
	s.Require().ErrorIs(err, intel.ErrNilInput)
	s.Equal(intel.Holding{}, holding)
}

// TestOnEmptyObserverReturnsErrNoObserver verifies field validation order.
func (s *QueriesSuite) TestOnEmptyObserverReturnsErrNoObserver() {
	holding, err := s.intel.On(&intel.OnInput{Observer: "", Subject: "x"})
	s.Require().ErrorIs(err, intel.ErrNoObserver)
	s.Equal(intel.Holding{}, holding)
}

// TestOnEmptySubjectReturnsErrNoSubject verifies field validation order.
func (s *QueriesSuite) TestOnEmptySubjectReturnsErrNoSubject() {
	const alice = core.EntityID("alice")
	holding, err := s.intel.On(&intel.OnInput{Observer: alice, Subject: ""})
	s.Require().ErrorIs(err, intel.ErrNoSubject)
	s.Equal(intel.Holding{}, holding)
}

// TestOnMultiDefectOrderingObserverFirst pins that when both Observer and
// Subject are empty, ErrNoObserver is returned (Observer checked first).
func (s *QueriesSuite) TestOnMultiDefectOrderingObserverFirst() {
	holding, err := s.intel.On(&intel.OnInput{Observer: "", Subject: ""})
	s.Require().ErrorIs(err, intel.ErrNoObserver)
	s.Equal(intel.Holding{}, holding)
}

// TestHeldByCopyOutPayload verifies that mutating a returned holding's
// payload does not corrupt internal state.
func (s *QueriesSuite) TestHeldByCopyOutPayload() {
	const obs = core.EntityID("alice")
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "c", At: 1,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("original")}},
	})
	s.Require().NoError(err)

	// Get holdings, mutate the payload
	holdings, err := s.intel.HeldBy(&intel.HeldByInput{Observer: obs})
	s.Require().NoError(err)
	s.Len(holdings, 1)
	holdings[0].Payload[0] = 'X'

	// Query again: internal state unchanged
	holdings2, err := s.intel.HeldBy(&intel.HeldByInput{Observer: obs})
	s.Require().NoError(err)
	s.Equal([]byte("original"), holdings2[0].Payload)
}

// TestOnCopyOutPayload verifies that mutating a returned holding's
// payload from On does not corrupt internal state.
func (s *QueriesSuite) TestOnCopyOutPayload() {
	const obs = core.EntityID("bob")
	const subject = intel.Subject("target")
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "c", At: 1,
		Reports: []intel.Report{{Subject: subject, Payload: []byte("original")}},
	})
	s.Require().NoError(err)

	// Get holding, mutate payload
	holding, err := s.intel.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	holding.Payload[0] = 'Y'

	// Query again: internal state unchanged
	holding2, err := s.intel.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	s.Equal([]byte("original"), holding2.Payload)
}

// TestOnCopyOutCurrentVia verifies that mutating a returned holding's
// CurrentVia slice does not corrupt internal state.
func (s *QueriesSuite) TestOnCopyOutCurrentVia() {
	const obs = core.EntityID("alice")
	const subject = intel.Subject("target")
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: intel.Sight, At: 1,
		Percept: []intel.Report{{Subject: subject, Payload: []byte("sight")}},
	})
	s.Require().NoError(err)

	// Get holding, mutate CurrentVia slice
	holding, err := s.intel.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	s.Len(holding.CurrentVia, 1)
	holding.CurrentVia[0] = "modified"

	// Query again: internal state unchanged
	holding2, err := s.intel.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	s.Equal([]intel.Channel{intel.Sight}, holding2.CurrentVia)
}

// TestHeldByCopyOutCurrentVia verifies that mutating a returned holding's
// CurrentVia slice from HeldBy does not corrupt internal state.
func (s *QueriesSuite) TestHeldByCopyOutCurrentVia() {
	const obs = core.EntityID("charlie")
	const subject = intel.Subject("target")
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: intel.Sight, At: 1,
		Percept: []intel.Report{{Subject: subject, Payload: []byte("sight")}},
	})
	s.Require().NoError(err)

	// Get holdings, find the target, mutate its CurrentVia slice element
	holdings, err := s.intel.HeldBy(&intel.HeldByInput{Observer: obs})
	s.Require().NoError(err)
	s.Len(holdings, 1)
	holdings[0].CurrentVia[0] = "modified"

	// Query again: internal state unchanged
	holdings2, err := s.intel.HeldBy(&intel.HeldByInput{Observer: obs})
	s.Require().NoError(err)
	s.Equal([]intel.Channel{intel.Sight}, holdings2[0].CurrentVia)
}

// TestHeldByCopyOutFromInputMutation verifies that if a caller mutates
// a Report input slice AFTER calling Report, HeldBy sees unchanged state.
func (s *QueriesSuite) TestHeldByCopyOutFromInputMutation() {
	const obs = core.EntityID("alice")
	reports := []intel.Report{
		{Subject: "s1", Payload: []byte("p1")},
		{Subject: "s2", Payload: []byte("p2")},
	}
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "c", At: 1, Reports: reports,
	})
	s.Require().NoError(err)

	// Mutate the input slice
	reports[0].Payload[0] = 'X'
	reports[0].Subject = "mutated"

	// Query: should see original state
	holdings, err := s.intel.HeldBy(&intel.HeldByInput{Observer: obs})
	s.Require().NoError(err)
	// holdings should be sorted by subject, so s1 comes first, s2 second
	var s1Holding intel.Holding
	for _, h := range holdings {
		if h.Subject == "s1" {
			s1Holding = h
			break
		}
	}
	s.Equal([]byte("p1"), s1Holding.Payload)
}

// TestDeciderContract is the normative integration test: an NPC decider
// consults HeldBy(itself) and nothing else. This documents the core contract
// and verifies that HeldBy provides all necessary information for decision-making.
func (s *QueriesSuite) TestDeciderContract() {
	const (
		monsterID  = core.EntityID("goblin-1")
		treasure   = intel.Subject("treasure")
		adventurer = intel.Subject("adventurer-party")
		obstacle   = intel.Subject("stone-door")
	)

	// Setup: monster has various intel from different channels.
	// Sight: active percept of adventurers and stone door
	// Smell: lingering awareness of treasure (sustain faded - remembered goblin intel)
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: monsterID, Channel: intel.Sight, At: 100,
		Percept: []intel.Report{
			{Subject: adventurer, Payload: []byte("group of 4, armed")},
			{Subject: obstacle, Payload: []byte("10ft tall, sealed")},
		},
	})
	s.Require().NoError(err)

	// Report: secondhand intel about treasure (supernatural channel)
	_, err = s.intel.Report(&intel.ReportInput{
		Observer: monsterID, Channel: "whispers", At: 50,
		Reports: []intel.Report{{Subject: treasure, Payload: []byte("gold in chamber")}},
	})
	s.Require().NoError(err)

	// Surveil on smell channel with treasure, making it Current via smell
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: monsterID, Channel: "smell", At: 80,
		Percept: []intel.Report{{Subject: treasure, Payload: []byte("gold scent")}},
	})
	s.Require().NoError(err)
	s.Equal([]intel.Subject{treasure}, out.Refreshed, "treasure refreshed via smell")

	// Fade the smell sense by surveilling with empty percept, so treasure becomes Held
	out, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: monsterID, Channel: "smell", At: 90, Percept: []intel.Report{},
	})
	s.Require().NoError(err)
	s.Equal([]intel.Subject{treasure}, out.Faded, "treasure faded when smell lost it")

	// NPC DECIDER: consult HeldBy(itself) and nothing else
	myIntel, err := s.intel.HeldBy(&intel.HeldByInput{Observer: monsterID})
	s.Require().NoError(err)
	s.Len(myIntel, 3)

	// Verify all holdings are present and correctly ordered (sorted by Subject)
	// Expected order: adventurer, obstacle, treasure
	s.Equal(adventurer, myIntel[0].Subject)
	s.Equal(obstacle, myIntel[1].Subject)
	s.Equal(treasure, myIntel[2].Subject)

	// Verify statuses: adventurer and obstacle are Current (sight sustains them),
	// treasure is Held (no sustaining channel).
	s.Equal(intel.Current, myIntel[0].Status)
	s.Equal(intel.Current, myIntel[1].Status)
	s.Equal(intel.Held, myIntel[2].Status)

	// Verify the monster can act based on this alone:
	// - Attack adventurers (Current, sight confirmed)
	// - Search for treasure despite not seeing it (Held, remembered via whispers)
	// - Try to open door (Current, sight confirmed)
	// The contract: all this decision logic flows from HeldBy(itself) alone.
	_ = myIntel // Decision logic would consume this
}

// PersistenceSuite tests the persistence layer: ToData/LoadIntel.
type PersistenceSuite struct {
	suite.Suite
	intel *intel.Intel
}

func (s *PersistenceSuite) SetupTest() {
	var err error
	s.intel, err = intel.NewIntel()
	s.Require().NoError(err)
}

// TestIdleConventionZeroValue tests that NewIntel().ToData() returns a zero
// Data with nil map (not empty non-nil).
func (s *PersistenceSuite) TestIdleConventionZeroValue() {
	data := s.intel.ToData()
	// Zero value: nil map, not empty non-nil
	s.Nil(data.Holdings, "idle Intel should marshal to nil Holdings map")
}

// TestIdleConventionMarshalEmpty tests that zero Data marshals to "{}".
func (s *PersistenceSuite) TestIdleConventionMarshalEmpty() {
	data := s.intel.ToData()
	b, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Equal([]byte("{}"), b, "idle Data must marshal to empty object")
}

// TestIdleConventionLoadEmpty tests that LoadIntel(Data{}) succeeds
// and the Intel is usable (verbs work).
func (s *PersistenceSuite) TestIdleConventionLoadEmpty() {
	loaded, err := intel.LoadIntel(intel.Data{})
	s.Require().NoError(err)
	// Verify it's usable: HeldBy on empty Intel succeeds
	holdings, err := loaded.HeldBy(&intel.HeldByInput{Observer: "test"})
	s.Require().NoError(err)
	s.Empty(holdings)
}

// TestRoundTripSingleHolding tests round-trip of a single Report holding.
func (s *PersistenceSuite) TestRoundTripSingleHolding() {
	const (
		obs     = core.EntityID("alice")
		subject = intel.Subject("target")
		payload = "test-data"
		channel = intel.Channel("hearing")
		at      = uint64(42)
	)
	// Create a holding
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: channel, At: at,
		Reports: []intel.Report{{Subject: subject, Payload: []byte(payload)}},
	})
	s.Require().NoError(err)

	// Snapshot
	data := s.intel.ToData()

	// Load
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err)

	// Verify the holding is accessible
	h, err := loaded.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	s.Equal([]byte(payload), h.Payload)
	s.Equal(channel, h.Channel)
	s.Equal(at, h.At)
	s.Nil(h.CurrentVia, "Report holding should have nil CurrentVia")
	s.Equal(intel.Held, h.Status)
}

// TestRoundTripMultiChannelCurrent tests round-trip of a Current holding
// with multiple sustaining channels.
func (s *PersistenceSuite) TestRoundTripMultiChannelCurrent() {
	const (
		obs      = core.EntityID("bob")
		subject  = intel.Subject("prey")
		channel1 = intel.Channel("sight")
		channel2 = intel.Channel("hearing")
	)
	// Create Current holding via sight
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: channel1, At: 1,
		Percept: []intel.Report{{Subject: subject, Payload: []byte("v1")}},
	})
	s.Require().NoError(err)

	// Sustain via hearing
	_, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: channel2, At: 2,
		Percept: []intel.Report{{Subject: subject, Payload: []byte("v2")}},
	})
	s.Require().NoError(err)

	// Snapshot
	data := s.intel.ToData()

	// Load
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err)

	// Verify the holding is Current with both channels
	h, err := loaded.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	s.Equal(intel.Current, h.Status)
	s.Len(h.CurrentVia, 2)
	s.ElementsMatch([]intel.Channel{channel1, channel2}, h.CurrentVia)
}

// TestRoundTripPostFadeState tests round-trip preserves faded (Held) state.
func (s *PersistenceSuite) TestRoundTripPostFadeState() {
	const (
		obs     = core.EntityID("charlie")
		subject = intel.Subject("ghost")
		channel = intel.Channel("sight")
	)
	// Create Current holding
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: channel, At: 1,
		Percept: []intel.Report{{Subject: subject, Payload: []byte("saw it")}},
	})
	s.Require().NoError(err)

	// Fade it
	_, err = s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: channel, At: 2, Percept: []intel.Report{},
	})
	s.Require().NoError(err)

	// Snapshot
	data := s.intel.ToData()

	// Load
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err)

	// Verify holding is Held (faded state preserved)
	h, err := loaded.On(&intel.OnInput{Observer: obs, Subject: subject})
	s.Require().NoError(err)
	s.Equal(intel.Held, h.Status)
	s.Nil(h.CurrentVia)
}

// TestRoundTripBehaviorIdentical tests that a Surveil after reload behaves
// identically to the same Surveil on the original Intel.
func (s *PersistenceSuite) TestRoundTripBehaviorIdentical() {
	const (
		obs     = core.EntityID("dave")
		subj1   = intel.Subject("s1")
		subj2   = intel.Subject("s2")
		channel = intel.Channel("sight")
	)
	// Setup: create holdings, then snapshot and load
	_, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: channel, At: 1,
		Percept: []intel.Report{
			{Subject: subj1, Payload: []byte("v1")},
			{Subject: subj2, Payload: []byte("v2")},
		},
	})
	s.Require().NoError(err)

	data := s.intel.ToData()
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err)

	// Apply same Surveil to both: only s1 present (s2 fades)
	out1, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: channel, At: 2,
		Percept: []intel.Report{{Subject: subj1, Payload: []byte("v1-refreshed")}},
	})
	s.Require().NoError(err)

	out2, err := loaded.Surveil(&intel.SurveilInput{
		Observer: obs, Channel: channel, At: 2,
		Percept: []intel.Report{{Subject: subj1, Payload: []byte("v1-refreshed")}},
	})
	s.Require().NoError(err)

	// Deltas must match
	s.Equal(out1.FirstContact, out2.FirstContact, "FirstContact must match")
	s.Equal(out1.Refreshed, out2.Refreshed, "Refreshed must match")
	s.Equal(out1.Faded, out2.Faded, "Faded must match")

	// Final state must match
	h1, _ := s.intel.On(&intel.OnInput{Observer: obs, Subject: subj2})
	h2, _ := loaded.On(&intel.OnInput{Observer: obs, Subject: subj2})
	s.Equal(h1.Status, h2.Status, "final status must match")
}

// TestGoldenJSON tests that a populated holding marshals to exact JSON.
func (s *PersistenceSuite) TestGoldenJSON() {
	const (
		obs     = core.EntityID("alice")
		subject = intel.Subject("behind-door-3")
		payload = "treasure"
		channel = intel.Channel("hearing")
		at      = uint64(5)
	)
	// Create holding
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: channel, At: at,
		Reports: []intel.Report{{Subject: subject, Payload: []byte(payload)}},
	})
	s.Require().NoError(err)

	data := s.intel.ToData()
	b, err := json.Marshal(data)
	s.Require().NoError(err)

	// Unmarshal to verify structure: holdings[alice][behind-door-3] exists
	// with payload (base64), channel, at; current_via omitted when nil
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(b, &unmarshaled)
	s.Require().NoError(err)

	holdings, ok := unmarshaled["holdings"].(map[string]interface{})
	s.Require().True(ok, "holdings key must exist and be a map")

	aliceHoldings, ok := holdings[string(obs)].(map[string]interface{})
	s.Require().True(ok, "alice holdings must exist and be a map")

	doorHolding, ok := aliceHoldings[string(subject)].(map[string]interface{})
	s.Require().True(ok, "behind-door-3 holding must exist and be a map")

	// Verify fields: payload should be a base64-encoded string (json.Marshal encodes []byte as base64)
	payloadStr, ok := doorHolding["payload"].(string)
	s.Require().True(ok, "payload must be a string (base64-encoded)")
	// Verify the base64 string decodes to original payload by round-tripping through json.Marshal
	var decodedPayload []byte
	err = json.Unmarshal([]byte(`"`+payloadStr+`"`), &decodedPayload)
	s.Require().NoError(err)
	s.Equal([]byte(payload), decodedPayload)

	channelStr, ok := doorHolding["channel"].(string)
	s.Require().True(ok, "channel must be a string")
	s.Equal(channel, intel.Channel(channelStr))

	atNum, ok := doorHolding["at"].(float64)
	s.Require().True(ok, "at must be a number")
	s.Equal(at, uint64(atNum))

	// current_via should be omitted (nil)
	_, hasCurrentVia := doorHolding["current_via"]
	s.False(hasCurrentVia, "current_via should be omitted when nil")
}

// TestSnapshotImmutability tests that mutating runtime Intel after ToData
// doesn't affect the snapshot.
func (s *PersistenceSuite) TestSnapshotImmutability() {
	const obs = core.EntityID("eve")
	// Create initial holding
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "c", At: 1,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("original")}},
	})
	s.Require().NoError(err)

	// Snapshot
	data := s.intel.ToData()

	// Mutate the runtime Intel
	_, err = s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "c", At: 2,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("modified")}},
	})
	s.Require().NoError(err)

	// Load snapshot: must have original payload
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err)
	h, err := loaded.On(&intel.OnInput{Observer: obs, Subject: "s"})
	s.Require().NoError(err)
	s.Equal([]byte("original"), h.Payload, "snapshot payload must be unchanged")
}

// TestLoadSnapshotMutationImmunity tests that mutating the caller's Data
// maps after LoadIntel doesn't affect the loaded Intel.
func (s *PersistenceSuite) TestLoadSnapshotMutationImmunity() {
	const obs = core.EntityID("frank")
	// Create holding
	_, err := s.intel.Report(&intel.ReportInput{
		Observer: obs, Channel: "c", At: 1,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("original")}},
	})
	s.Require().NoError(err)

	data := s.intel.ToData()

	// Load first
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err)

	// Mutate the data's maps AFTER LoadIntel (in-place index write, not append)
	observerHoldings := data.Holdings[obs]
	// Modify an existing holding's payload
	for _, h := range observerHoldings {
		h.Payload[0] = 'X'
	}

	// Verify loaded Intel is immune to snapshot mutations
	h, err := loaded.On(&intel.OnInput{Observer: obs, Subject: "s"})
	s.Require().NoError(err)
	s.Equal([]byte("original"), h.Payload, "loaded Intel must be immune to caller mutations after LoadIntel")
}

// TestR9EmptyObserverKey tests that empty observer key is rejected.
func (s *PersistenceSuite) TestR9EmptyObserverKey() {
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			"": {}, // Empty observer key
		},
	}
	_, err := intel.LoadIntel(data)
	s.Require().True(errors.Is(err, intel.ErrInvalidData), "empty observer key must be rejected")
}

// TestR9EmptySubjectKey tests that empty subject key is rejected.
func (s *PersistenceSuite) TestR9EmptySubjectKey() {
	const alice = core.EntityID("alice")
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			alice: {"": {}}, // Empty subject key
		},
	}
	_, err := intel.LoadIntel(data)
	s.Require().True(errors.Is(err, intel.ErrInvalidData), "empty subject key must be rejected")
}

// TestR9DuplicateCurrentVia tests that duplicate channels in CurrentVia are rejected.
func (s *PersistenceSuite) TestR9DuplicateCurrentVia() {
	const (
		alice  = core.EntityID("alice")
		target = intel.Subject("target")
		sight  = intel.Channel("sight")
	)
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			alice: {
				target: {
					Payload:    []byte("data"),
					Channel:    sight,
					At:         1,
					CurrentVia: []intel.Channel{sight, sight}, // Duplicate
				},
			},
		},
	}
	_, err := intel.LoadIntel(data)
	s.Require().True(errors.Is(err, intel.ErrInvalidData), "duplicate CurrentVia channels must be rejected")
}

// TestR9EmptyHoldingChannel tests that empty holding Channel is rejected.
func (s *PersistenceSuite) TestR9EmptyHoldingChannel() {
	const (
		alice  = core.EntityID("alice")
		target = intel.Subject("target")
	)
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			alice: {
				target: {
					Payload: []byte("data"),
					Channel: "", // Empty channel
					At:      1,
				},
			},
		},
	}
	_, err := intel.LoadIntel(data)
	s.Require().True(errors.Is(err, intel.ErrInvalidData), "empty holding Channel must be rejected")
}

// TestR9EmptyChannelInCurrentVia tests that empty channel in CurrentVia is rejected.
func (s *PersistenceSuite) TestR9EmptyChannelInCurrentVia() {
	const (
		alice  = core.EntityID("alice")
		target = intel.Subject("target")
		sight  = intel.Channel("sight")
	)
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			alice: {
				target: {
					Payload:    []byte("data"),
					Channel:    sight,
					At:         1,
					CurrentVia: []intel.Channel{""}, // Empty channel
				},
			},
		},
	}
	_, err := intel.LoadIntel(data)
	s.Require().True(errors.Is(err, intel.ErrInvalidData), "empty channel in CurrentVia must be rejected")
}

// TestR9NilInnerMap tests that nil inner map is rejected.
func (s *PersistenceSuite) TestR9NilInnerMap() {
	const alice = core.EntityID("alice")
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			alice: nil, // Nil inner map
		},
	}
	_, err := intel.LoadIntel(data)
	s.Require().True(errors.Is(err, intel.ErrInvalidData), "nil inner map must be rejected")
}

// TestR9EmptyInnerMap tests that empty (non-nil) inner map is rejected.
func (s *PersistenceSuite) TestR9EmptyInnerMap() {
	const alice = core.EntityID("alice")
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			alice: {}, // Empty non-nil map
		},
	}
	_, err := intel.LoadIntel(data)
	s.Require().True(errors.Is(err, intel.ErrInvalidData), "empty inner map must be rejected")
}

// TestNilPayloadLegal tests that nil payload in HoldingData is legal.
func (s *PersistenceSuite) TestNilPayloadLegal() {
	const (
		alice  = core.EntityID("alice")
		target = intel.Subject("target")
	)
	data := intel.Data{
		Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
			alice: {
				target: {
					Payload: nil, // Nil payload is legal
					Channel: "sight",
					At:      1,
				},
			},
		},
	}
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err, "nil payload must be legal")

	// Verify it loads and is usable
	h, err := loaded.On(&intel.OnInput{Observer: alice, Subject: target})
	s.Require().NoError(err)
	s.Nil(h.Payload, "nil payload should load as nil")
}

func TestPersistenceSuite(t *testing.T) {
	suite.Run(t, new(PersistenceSuite))
}

func TestQueriesSuite(t *testing.T) {
	suite.Run(t, new(QueriesSuite))
}

func TestIntelSuite(t *testing.T) {
	suite.Run(t, new(IntelSuite))
}
