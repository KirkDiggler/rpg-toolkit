// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
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
	s.False(h.Current)
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
	s.True(h.Current)
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
	s.Require().Error(err, "alice never held anything about itself, across any pass")
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
	s.False(h.Current)
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
	s.True(h.Current, "untouched: still current from pass 1")
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
	s.Require().Error(err, "bob never perceived the goblin; alice's knowledge did not leak to him")
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

// Case 12: nil Reach, empty channel, empty ids → the matching sentinel
// error, and nothing written.
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

	// None of the four failed calls wrote anything, even though alice was a
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
	s.True(beforeGoblin.Current)
	s.False(beforeSpider.Current)

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
