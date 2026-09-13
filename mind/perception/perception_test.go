// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// reachAll is a Reach that connects every observer to every subject on
// every channel.
type reachAll struct{}

func (reachAll) Reaches(_ perception.Channel, _, _ core.EntityID) bool { return true }

// reachNone is a Reach that connects nothing to anything.
type reachNone struct{}

func (reachNone) Reaches(_ perception.Channel, _, _ core.EntityID) bool { return false }

// reachPairs is a Reach that connects only the observer/subject pairs it
// names, on any channel. Absent pairs default to false.
type reachPairs map[core.EntityID]map[core.EntityID]bool

func (r reachPairs) Reaches(_ perception.Channel, observer, subject core.EntityID) bool {
	return r[observer][subject]
}

type PerceptionSuite struct {
	suite.Suite
	p *perception.Perception
}

func (s *PerceptionSuite) SetupTest() {
	var err error
	s.p, err = perception.New()
	s.Require().NoError(err)
}

// observe runs one sight Pass against the suite's Perception.
func (s *PerceptionSuite) observe(
	at uint64, presences []perception.Presence, observers []core.EntityID, reach perception.Reach,
) (map[core.EntityID]*perception.Delta, error) {
	return s.p.Observe(perception.Pass{
		At:        at,
		Channel:   perception.Sight,
		Presences: presences,
		Observers: observers,
		Reach:     reach,
	})
}

// holdingOn is a convenience for the common case: every test below that
// checks a holding directly does so from alice's point of view.
func (s *PerceptionSuite) holdingOn(subject core.EntityID) perception.Holding {
	h, err := s.p.On("alice", subject)
	s.Require().NoError(err)
	return h
}

func TestPerceptionSuite(t *testing.T) {
	suite.Run(t, new(PerceptionSuite))
}

// Case 1: one observer, one reachable presence → FirstContact carries the payload.
func (s *PerceptionSuite) TestFirstContactCarriesPayload() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	payload := []byte("wounded")

	deltas, err := s.observe(1, []perception.Presence{{ID: goblin, Payload: payload}},
		[]core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	s.Equal([]perception.Presence{{ID: goblin, Payload: payload}}, deltas[alice].FirstContact)
	s.Empty(deltas[alice].Refreshed)
	s.Empty(deltas[alice].Changed)
	s.Empty(deltas[alice].Faded)
	s.Empty(deltas[alice].Reacquired)
}

// Case 2: same presence next pass, same payload → Refreshed contains it,
// Changed does not.
func (s *PerceptionSuite) TestRefreshedWithoutChangeIsNotChanged() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	payload := []byte("wounded")

	_, err := s.observe(1, []perception.Presence{{ID: goblin, Payload: payload}}, []core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	deltas, err := s.observe(2, []perception.Presence{{ID: goblin, Payload: payload}},
		[]core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	s.Empty(deltas[alice].FirstContact)
	s.Equal([]core.EntityID{goblin}, deltas[alice].Refreshed)
	s.Empty(deltas[alice].Changed, "identical payload is a refresh, not a change")
}

// Case 3: same presence, different payload → Refreshed and Changed.
func (s *PerceptionSuite) TestChangedPayloadRefreshesAndChanges() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)

	_, err := s.observe(1, []perception.Presence{{ID: goblin, Payload: []byte("wounded")}},
		[]core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	deltas, err := s.observe(2, []perception.Presence{{ID: goblin, Payload: []byte("fleeing")}},
		[]core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	s.Equal([]core.EntityID{goblin}, deltas[alice].Refreshed)
	s.Equal([]core.EntityID{goblin}, deltas[alice].Changed)
}

// Case 4: presence goes out of reach → Faded; On still returns the holding
// with Current == false and the old payload.
func (s *PerceptionSuite) TestOutOfReachFadesButHoldingSurvives() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	payload := []byte("wounded")
	presences := []perception.Presence{{ID: goblin, Payload: payload}}

	_, err := s.observe(1, presences, []core.EntityID{alice}, reachPairs{alice: {goblin: true}})
	s.Require().NoError(err)

	deltas, err := s.observe(2, presences, []core.EntityID{alice}, reachPairs{})
	s.Require().NoError(err)

	s.Equal([]core.EntityID{goblin}, deltas[alice].Faded)

	h := s.holdingOn(goblin)
	s.False(h.CurrentOn(perception.Sight))
	s.Equal(payload, h.Payload)
}

// Case 5: back in reach, same payload → Reacquired; Observed is still the
// original.
func (s *PerceptionSuite) TestReacquiredKeepsOriginalObserved() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	payload := []byte("wounded")
	presences := []perception.Presence{{ID: goblin, Payload: payload}}

	_, err := s.observe(1, presences, []core.EntityID{alice}, reachPairs{alice: {goblin: true}})
	s.Require().NoError(err)
	_, err = s.observe(2, presences, []core.EntityID{alice}, reachPairs{}) // ghosted
	s.Require().NoError(err)

	deltas, err := s.observe(3, presences, []core.EntityID{alice}, reachPairs{alice: {goblin: true}})
	s.Require().NoError(err)

	s.Equal([]core.EntityID{goblin}, deltas[alice].Reacquired)
	s.Equal([]core.EntityID{goblin}, deltas[alice].Refreshed, "reacquired refines Refreshed, it doesn't replace it")
	s.Empty(deltas[alice].Changed, "same payload: reacquired, not changed")

	h := s.holdingOn(goblin)
	s.True(h.CurrentOn(perception.Sight))
	s.Equal(uint64(1), h.Observed, "original first-contact timestamp survives reacquisition")
	s.Equal(uint64(3), h.Confirmed)
}

// Case 6: observer is also a presence → it never appears in its own delta.
func (s *PerceptionSuite) TestObserverNeverPerceivesItself() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)

	deltas, err := s.observe(1, []perception.Presence{
		{ID: alice, Payload: []byte("self")},
		{ID: goblin, Payload: []byte("wounded")},
	}, []core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	s.Equal([]perception.Presence{{ID: goblin, Payload: []byte("wounded")}}, deltas[alice].FirstContact)

	_, err = s.p.On(alice, alice)
	s.Require().ErrorIs(err, perception.ErrNotHeld, "alice never held anything about itself, across any pass")
}

// Case 7: observer reaches nothing → complete empty percept — everything it
// held fades. This is the case that matters most: skipping the call for a
// zero-reach observer is how ghosts stay falsely current forever.
func (s *PerceptionSuite) TestObserverReachingNothingFadesEverything() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	presences := []perception.Presence{{ID: goblin, Payload: []byte("wounded")}}

	_, err := s.observe(1, presences, []core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	deltas, err := s.observe(2, presences, []core.EntityID{alice}, reachNone{})
	s.Require().NoError(err)

	s.Contains(deltas, alice)
	s.Equal([]core.EntityID{goblin}, deltas[alice].Faded)

	h := s.holdingOn(goblin)
	s.False(h.CurrentOn(perception.Sight))
}

// Case 8: observer absent from Observers → its holdings are untouched,
// nothing fades.
func (s *PerceptionSuite) TestObserverAbsentFromObserversIsUntouched() {
	const (
		alice  = core.EntityID("alice")
		bob    = core.EntityID("bob")
		goblin = core.EntityID("goblin-1")
	)
	presences := []perception.Presence{{ID: goblin, Payload: []byte("wounded")}}

	_, err := s.observe(1, presences, []core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	deltas, err := s.observe(2, presences, []core.EntityID{bob}, reachAll{})
	s.Require().NoError(err)

	s.NotContains(deltas, alice, "alice was not in Observers this pass")

	h := s.holdingOn(goblin)
	s.True(h.CurrentOn(perception.Sight), "untouched: still current from pass 1")
	s.Equal(uint64(1), h.Confirmed, "untouched: pass 2 never re-confirmed alice's holding")
}

// Case 9: two observers, Reach true for one only → their knowledge differs;
// neither can read the other's.
func (s *PerceptionSuite) TestTwoObserversDoNotShareKnowledge() {
	const (
		alice  = core.EntityID("alice")
		bob    = core.EntityID("bob")
		goblin = core.EntityID("goblin-1")
	)

	deltas, err := s.observe(1, []perception.Presence{{ID: goblin, Payload: []byte("wounded")}},
		[]core.EntityID{alice, bob}, reachPairs{alice: {goblin: true}})
	s.Require().NoError(err)

	s.Equal([]perception.Presence{{ID: goblin, Payload: []byte("wounded")}}, deltas[alice].FirstContact)
	s.Contains(deltas, bob)
	s.Empty(deltas[bob].FirstContact)

	_, err = s.p.On(alice, goblin)
	s.Require().NoError(err)
	_, err = s.p.On(bob, goblin)
	s.Require().ErrorIs(err, perception.ErrNotHeld,
		"bob never perceived the goblin; alice's knowledge did not leak to him")
}

// Case 10: payload is bytes that are not valid anything → round-trips
// intact — proves nothing decodes it.
func (s *PerceptionSuite) TestOpaquePayloadRoundTripsIntact() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	// Invalid UTF-8, invalid JSON, invalid everything: just bytes.
	payload := []byte{0xff, 0x00, 0xde, 0xad, 0xbe, 0xef, 0x00, 0xff}

	deltas, err := s.observe(1, []perception.Presence{{ID: goblin, Payload: payload}},
		[]core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)
	s.Equal(payload, deltas[alice].FirstContact[0].Payload)

	h := s.holdingOn(goblin)
	s.Equal(payload, h.Payload)
}

// Case 11: presences supplied unsorted, twice → both passes produce
// identical deltas.
func (s *PerceptionSuite) TestUnsortedPresencesAreDeterministic() {
	const alice = core.EntityID("alice")
	unsorted := []perception.Presence{
		{ID: core.EntityID("zeta"), Payload: []byte("z")},
		{ID: core.EntityID("alpha"), Payload: []byte("a")},
		{ID: core.EntityID("mike"), Payload: []byte("m")},
	}
	wantOrder := []perception.Presence{
		{ID: core.EntityID("alpha"), Payload: []byte("a")},
		{ID: core.EntityID("mike"), Payload: []byte("m")},
		{ID: core.EntityID("zeta"), Payload: []byte("z")},
	}

	first, err := perception.New()
	s.Require().NoError(err)
	deltasA, err := first.Observe(perception.Pass{
		At: 1, Channel: perception.Sight, Presences: unsorted, Observers: []core.EntityID{alice}, Reach: reachAll{},
	})
	s.Require().NoError(err)

	second, err := perception.New()
	s.Require().NoError(err)
	deltasB, err := second.Observe(perception.Pass{
		At: 1, Channel: perception.Sight, Presences: unsorted, Observers: []core.EntityID{alice}, Reach: reachAll{},
	})
	s.Require().NoError(err)

	s.Equal(wantOrder, deltasA[alice].FirstContact)
	s.Equal(deltasA[alice], deltasB[alice], "two identical passes, presences supplied unsorted, must match exactly")
}

// Case 12: nil Reach, empty channel, empty ids, duplicate ids → the
// matching sentinel error, and nothing written.
func (s *PerceptionSuite) TestValidationOrderAndNothingWritten() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	payload := []byte("wounded")

	// Reach nil beats every other violation, even layered on top of an
	// empty channel, an empty presence ID, and an empty observer ID.
	_, err := s.p.Observe(perception.Pass{
		Channel:   "",
		Presences: []perception.Presence{{ID: ""}},
		Observers: []core.EntityID{alice, ""},
		Reach:     nil,
	})
	s.Require().ErrorIs(err, perception.ErrNoReach)

	// Empty channel beats the remaining violations, once Reach is supplied.
	_, err = s.p.Observe(perception.Pass{
		Channel:   "",
		Presences: []perception.Presence{{ID: ""}},
		Observers: []core.EntityID{alice, ""},
		Reach:     reachAll{},
	})
	s.Require().ErrorIs(err, perception.ErrNoChannel)

	// An empty presence ID beats an empty observer ID.
	_, err = s.p.Observe(perception.Pass{
		Channel:   perception.Sight,
		Presences: []perception.Presence{{ID: ""}, {ID: goblin, Payload: payload}},
		Observers: []core.EntityID{alice, ""},
		Reach:     reachAll{},
	})
	s.Require().ErrorIs(err, perception.ErrNoSubject)

	// An empty observer ID, alone, with everything else valid.
	_, err = s.p.Observe(perception.Pass{
		Channel:   perception.Sight,
		Presences: []perception.Presence{{ID: goblin, Payload: payload}},
		Observers: []core.EntityID{alice, ""},
		Reach:     reachAll{},
	})
	s.Require().ErrorIs(err, perception.ErrNoObserver)

	// A duplicate Presence ID beats a duplicate Observer, and both duplicate
	// checks run only once every empty-ID check has cleared the whole Pass —
	// so a duplicate presence layered on top of the same empty-observer slot
	// used above still resolves to ErrNoObserver, not ErrDuplicateSubject.
	_, err = s.p.Observe(perception.Pass{
		Channel: perception.Sight,
		Presences: []perception.Presence{
			{ID: goblin, Payload: payload},
			{ID: goblin, Payload: []byte("different")},
		},
		Observers: []core.EntityID{alice, ""},
		Reach:     reachAll{},
	})
	s.Require().ErrorIs(err, perception.ErrNoObserver)

	// The duplicate presence, alone, with every observer valid.
	_, err = s.p.Observe(perception.Pass{
		Channel: perception.Sight,
		Presences: []perception.Presence{
			{ID: goblin, Payload: payload},
			{ID: goblin, Payload: []byte("different")},
		},
		Observers: []core.EntityID{alice},
		Reach:     reachAll{},
	})
	s.Require().ErrorIs(err, perception.ErrDuplicateSubject)

	// A duplicate Observer, alone, with everything else valid.
	_, err = s.p.Observe(perception.Pass{
		Channel:   perception.Sight,
		Presences: []perception.Presence{{ID: goblin, Payload: payload}},
		Observers: []core.EntityID{alice, alice},
		Reach:     reachAll{},
	})
	s.Require().ErrorIs(err, perception.ErrDuplicateObserver)

	// None of the six failed calls wrote anything, even though alice was a
	// valid observer in every one of them.
	held, err := s.p.Held(alice)
	s.Require().NoError(err)
	s.Empty(held)
}

// Case 13: ToData → Load → holdings, both stamps, and Current all survive.
func (s *PerceptionSuite) TestToDataLoadRoundTrip() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
		spider = core.EntityID("spider-1")
	)

	_, err := s.observe(1, []perception.Presence{
		{ID: goblin, Payload: []byte("wounded")},
		{ID: spider, Payload: []byte("skittering")},
	}, []core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	// Pass 2: spider drops out of reach (ghosts), goblin stays current.
	_, err = s.observe(2, []perception.Presence{{ID: goblin, Payload: []byte("wounded")}},
		[]core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	beforeGoblin := s.holdingOn(goblin)
	beforeSpider := s.holdingOn(spider)
	s.True(beforeGoblin.CurrentOn(perception.Sight))
	s.False(beforeSpider.CurrentOn(perception.Sight))

	loaded, err := perception.Load(s.p.ToData())
	s.Require().NoError(err)

	afterGoblin, err := loaded.On(alice, goblin)
	s.Require().NoError(err)
	afterSpider, err := loaded.On(alice, spider)
	s.Require().NoError(err)

	s.Equal(beforeGoblin, afterGoblin)
	s.Equal(beforeSpider, afterSpider)

	held, err := loaded.Held(alice)
	s.Require().NoError(err)
	s.Len(held, 2)
}

// Regression (review of PR #1686, Important #1): rule 7 used to derive
// Changed by comparing a holding's Observed to the pass's At, which was only
// correct while At strictly increased. Three passes sharing the same At and
// an identical payload used to report Changed on the second and third pass
// anyway. Consuming intel v0.4.0's own Changed field — the comparison the
// store already makes at landing — makes that impossible by construction.
func (s *PerceptionSuite) TestSameAtNeverForcesChanged() {
	const (
		alice  = core.EntityID("alice")
		goblin = core.EntityID("goblin-1")
	)
	payload := []byte("wounded")
	presences := []perception.Presence{{ID: goblin, Payload: payload}}

	_, err := s.observe(5, presences, []core.EntityID{alice}, reachAll{})
	s.Require().NoError(err)

	for i := 0; i < 2; i++ {
		deltas, err := s.observe(5, presences, []core.EntityID{alice}, reachAll{})
		s.Require().NoError(err)
		s.Equal([]core.EntityID{goblin}, deltas[alice].Refreshed)
		s.Empty(deltas[alice].Changed, "identical payload at a repeated At must never read as changed")
	}
}

// Load rejects whatever intel.LoadIntel rejects, wrapped. A nil inner map
// for a named observer is unreachable state intel.LoadIntel itself refuses
// to construct; perception.Data.Intel is intel.Data verbatim (the charter's
// documented persistence exception), so building one directly here is
// exercising the public shape, not reaching past it.
func (s *PerceptionSuite) TestLoadRejectsInvalidData() {
	_, err := perception.Load(perception.Data{})
	s.Require().NoError(err, "a zero Data is the idle state, not an error")

	bad := perception.Data{
		Intel: intel.Data{
			Holdings: map[core.EntityID]map[intel.Subject]intel.HoldingData{
				"alice": nil,
			},
		},
	}
	_, err = perception.Load(bad)
	s.Require().Error(err)
}

// Held and On validate their own empty-ID arguments, independent of any
// Pass.
func (s *PerceptionSuite) TestHeldAndOnValidateEmptyIDs() {
	_, err := s.p.Held("")
	s.Require().ErrorIs(err, perception.ErrNoObserver)

	_, err = s.p.On("", "goblin-1")
	s.Require().ErrorIs(err, perception.ErrNoObserver)

	_, err = s.p.On("alice", "")
	s.Require().ErrorIs(err, perception.ErrNoSubject)
}

// --- Report: discrete testimony -------------------------------------------

// deeds and hearing are test channels. Only Sight is predeclared; the
// vocabulary is open, which is the point of these.
const (
	deeds   = perception.Channel("deeds")
	hearing = perception.Channel("hearing")
)

// report lands discrete testimony on alice, the observer every holding
// assertion in this file is written from, on the deeds channel. Tests that
// need another channel call Report directly.
func (s *PerceptionSuite) report(at uint64, reports []perception.Presence) *perception.ReportOutput {
	out, err := s.p.Report(perception.ReportInput{
		Observer: "alice", Channel: deeds, Reports: reports, At: at,
	})
	s.Require().NoError(err)
	return out
}

// A reported subject is held and sustained by nothing. This is the whole
// difference from Observe: being told something is not perceiving it, so
// there is no channel delivering it and CurrentOn is false everywhere —
// including on the very channel that reported it.
func (s *PerceptionSuite) TestReportLandsHeldAndSustainsNothing() {
	out := s.report(1, []perception.Presence{{ID: "heal-1", Payload: []byte("cleric healed knight")}})

	s.Require().Len(out.FirstContact, 1)
	s.Equal(core.EntityID("heal-1"), out.FirstContact[0].ID)
	s.Empty(out.Updated, "a brand new subject is first contact, not an update")
	s.Empty(out.Changed, "first contact is never Changed")

	h := s.holdingOn("heal-1")
	s.Equal([]byte("cleric healed knight"), h.Payload)
	s.Equal(deeds, h.Channel, "provenance is the channel that reported it")
	s.Empty(h.CurrentVia, "discrete testimony sustains nothing")
	s.False(h.CurrentOn(deeds), "not even the reporting channel is delivering it")
	s.False(h.CurrentOn(perception.Sight))
	s.Equal(uint64(1), h.Observed)
	s.Equal(uint64(1), h.Confirmed)
}

// A later complete sight pass must not retire a reported holding. A Pass is
// a complete statement about ONE channel, and a deed was never on it — so
// omission from the sight percept says nothing about the deed. Without this,
// every deed would arrive and then immediately announce itself as Faded.
func (s *PerceptionSuite) TestReportedSubjectIsNotFadedByALaterPass() {
	s.report(1, []perception.Presence{{ID: "heal-1", Payload: []byte("a heal happened")}})

	deltas, err := s.observe(2, []perception.Presence{{ID: "goblin", Payload: []byte("here")}},
		[]core.EntityID{"alice"}, reachAll{})
	s.Require().NoError(err)

	s.NotContains(deltas["alice"].Faded, core.EntityID("heal-1"),
		"a sight pass cannot retire testimony that was never sight")
	s.Equal([]byte("a heal happened"), s.holdingOn("heal-1").Payload, "and it is still held")
}

// The defect a Current bool could not express, and the reason v0.2.0 exists.
//
// Alice currently SEES the goblin. A deed is then reported about that same
// goblin on another channel. intel moves Channel to the reporting channel
// and deliberately leaves CurrentVia alone — a rumour is not a sighting — so
// provenance and currency now disagree. A consumer holding only a bool plus
// Channel would read this as "current via deeds": current on a channel that
// delivers nothing. CurrentVia says what is actually true.
func (s *PerceptionSuite) TestReportMovesProvenanceWithoutSustainingItsChannel() {
	_, err := s.observe(1, []perception.Presence{{ID: "goblin", Payload: []byte("standing")}},
		[]core.EntityID{"alice"}, reachAll{})
	s.Require().NoError(err)
	s.Require().True(s.holdingOn("goblin").CurrentOn(perception.Sight))

	s.report(2, []perception.Presence{{ID: "goblin", Payload: []byte("it killed your brother")}})

	h := s.holdingOn("goblin")
	s.Equal(deeds, h.Channel, "provenance moved to the latest landing")
	s.Equal([]perception.Channel{perception.Sight}, h.CurrentVia, "but sight alone is still delivering")
	s.False(h.CurrentOn(deeds), "the reporting channel sustains nothing, and the pair now disagree")
	s.True(h.CurrentOn(perception.Sight))
}

// One payload per (observer, subject): a report about a subject already held
// on another channel OVERWRITES rather than merges, and is not refused. The
// store has no second slot, and deciding that these are one thing was the
// caller's doing the moment it reused the id (R11). Stated as a test because
// it is the cost of R11 going unheeded, and silent costs should be visible.
func (s *PerceptionSuite) TestReportOverwritesAnUnqualifiedSubject() {
	_, err := s.observe(1, []perception.Presence{{ID: "goblin", Payload: []byte("standing")}},
		[]core.EntityID{"alice"}, reachAll{})
	s.Require().NoError(err)

	out := s.report(2, []perception.Presence{{ID: "goblin", Payload: []byte("a deed")}})

	s.Equal([]core.EntityID{"goblin"}, out.Updated)
	s.Equal([]core.EntityID{"goblin"}, out.Changed)
	s.Equal([]byte("a deed"), s.holdingOn("goblin").Payload, "the sighting payload is gone, not kept beside it")
}

// R11 in practice: qualify the id by channel and the store cannot merge
// them. Two holdings, each sustained by its own channel, both payloads
// intact — the shape every caller should be writing.
func (s *PerceptionSuite) TestQualifiedIDsKeepBothChannelsIntact() {
	_, err := s.observe(1, []perception.Presence{{ID: "goblin", Payload: []byte("standing")}},
		[]core.EntityID{"alice"}, reachAll{})
	s.Require().NoError(err)

	s.report(2, []perception.Presence{{ID: "deeds|goblin", Payload: []byte("a deed")}})

	sight := s.holdingOn("goblin")
	s.Equal([]byte("standing"), sight.Payload)
	s.True(sight.CurrentOn(perception.Sight))

	deed := s.holdingOn("deeds|goblin")
	s.Equal([]byte("a deed"), deed.Payload)
	s.Empty(deed.CurrentVia)
}

// Two channels genuinely sustaining one subject: CurrentVia carries both,
// sorted, and CurrentOn answers each. This is what the bool could not say,
// and what hearing needs. The surviving payload is the last channel to land
// — the same one-payload rule as above, here between two live channels.
func (s *PerceptionSuite) TestTwoChannelsSustainOneSubject() {
	_, err := s.observe(1, []perception.Presence{{ID: "goblin", Payload: []byte("seen")}},
		[]core.EntityID{"alice"}, reachAll{})
	s.Require().NoError(err)

	_, err = s.p.Observe(perception.Pass{
		At: 2, Channel: hearing,
		Presences: []perception.Presence{{ID: "goblin", Payload: []byte("heard")}},
		Observers: []core.EntityID{"alice"}, Reach: reachAll{},
	})
	s.Require().NoError(err)

	h := s.holdingOn("goblin")
	s.Equal([]perception.Channel{hearing, perception.Sight}, h.CurrentVia, "sorted, both sustaining")
	s.True(h.CurrentOn(perception.Sight))
	s.True(h.CurrentOn(hearing))
	s.False(h.CurrentOn("tremorsense"), "a channel nobody ran is not delivering anything")
}

// Losing one of two sustaining channels is not a fade. Alice stops hearing
// the goblin but still sees it: the hearing pass retires only its own
// channel, and the subject is still current on sight.
func (s *PerceptionSuite) TestLosingOneChannelOfTwoIsNotAFade() {
	_, err := s.observe(1, []perception.Presence{{ID: "goblin", Payload: []byte("seen")}},
		[]core.EntityID{"alice"}, reachAll{})
	s.Require().NoError(err)
	_, err = s.p.Observe(perception.Pass{
		At: 2, Channel: hearing,
		Presences: []perception.Presence{{ID: "goblin", Payload: []byte("heard")}},
		Observers: []core.EntityID{"alice"}, Reach: reachAll{},
	})
	s.Require().NoError(err)

	deltas, err := s.p.Observe(perception.Pass{
		At: 3, Channel: hearing,
		Presences: []perception.Presence{{ID: "goblin", Payload: []byte("heard")}},
		Observers: []core.EntityID{"alice"}, Reach: reachNone{},
	})
	s.Require().NoError(err)

	s.Empty(deltas["alice"].Faded, "still sustained by sight, so nothing faded")
	h := s.holdingOn("goblin")
	s.Equal([]perception.Channel{perception.Sight}, h.CurrentVia)
	s.False(h.CurrentOn(hearing))
	s.True(h.CurrentOn(perception.Sight))
}

// Report's sentinels, each from a call that actually returns it, in the
// order R8 requires: channel, then subjects, then observer. Every violation
// is present in the first input, so the assertions prove precedence rather
// than merely that each check exists.
func (s *PerceptionSuite) TestReportValidationOrderAndNothingWritten() {
	empty := []perception.Presence{{ID: "", Payload: []byte("x")}}

	_, err := s.p.Report(perception.ReportInput{Observer: "", Channel: "", Reports: empty})
	s.Require().ErrorIs(err, perception.ErrNoChannel, "channel outranks both")

	_, err = s.p.Report(perception.ReportInput{Observer: "", Channel: deeds, Reports: empty})
	s.Require().ErrorIs(err, perception.ErrNoSubject, "an empty subject outranks an empty observer")

	_, err = s.p.Report(perception.ReportInput{
		Observer: "", Channel: deeds, Reports: []perception.Presence{{ID: "heal-1"}},
	})
	s.Require().ErrorIs(err, perception.ErrNoObserver)

	held, err := s.p.Held("alice")
	s.Require().NoError(err)
	s.Empty(held, "no rejected report wrote anything")
}

// Updated and Changed refine rather than partition, exactly as Delta's do: a
// re-report with identical content updates without changing, and moves only
// Confirmed. A report with new content moves both stamps.
func (s *PerceptionSuite) TestReportUpdatedRefinesIntoChanged() {
	s.report(1, []perception.Presence{{ID: "heal-1", Payload: []byte("same")}})

	out := s.report(2, []perception.Presence{{ID: "heal-1", Payload: []byte("same")}})
	s.Equal([]core.EntityID{"heal-1"}, out.Updated)
	s.Empty(out.Changed, "identical content is confirmed, not changed")
	h := s.holdingOn("heal-1")
	s.Equal(uint64(1), h.Observed, "unchanged content leaves Observed where it was")
	s.Equal(uint64(2), h.Confirmed)

	out = s.report(3, []perception.Presence{{ID: "heal-1", Payload: []byte("different")}})
	s.Equal([]core.EntityID{"heal-1"}, out.Updated)
	s.Equal([]core.EntityID{"heal-1"}, out.Changed, "Changed refines Updated, never partitions it")
	h = s.holdingOn("heal-1")
	s.Equal(uint64(3), h.Observed, "new content is a new thing: Observed moves")
	s.Equal(uint64(3), h.Confirmed)
}

// A Pass rejects a repeated subject; a Report does not, and the difference
// is not an oversight. The Pass rejection exists because sorting makes
// last-wins dedupe depend on an unstable sort. Report does not sort, so
// last-wins is already deterministic and there is nothing to protect.
func (s *PerceptionSuite) TestReportDedupesLastWinsRatherThanRejecting() {
	out := s.report(1, []perception.Presence{
		{ID: "heal-1", Payload: []byte("first")},
		{ID: "heal-1", Payload: []byte("last")},
	})

	s.Require().Len(out.FirstContact, 1, "one subject, however many times it was named")
	s.Equal([]byte("last"), s.holdingOn("heal-1").Payload)
}
