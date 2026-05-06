package encounter_test

// visibility_transition_test.go covers the five acceptance-criteria scenarios
// from issue #629:
//   1. Enter LoS   — A starts outside B's view, ends inside → EntityAppearedEvent
//   2. Leave LoS   — A starts inside B's view, ends outside → EntityDisappearedEvent
//   3. Pass through — A starts outside, traverses through, ends outside → BOTH events
//   4. Stays visible — A visible at start and end → NO appear/disappear events
//   5. Negative audience — Viewer C shares no LoS → receives neither MoveEvent nor
//      EntityAppeared/EntityDisappeared
//
// It also covers ToData/LoadFromData round-trips for both new event types
// (mirrors the patterns in encounter/events/events_test.go).

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/stretchr/testify/suite"
)

// typeNameOf returns the fmt %T type name string for an event — e.g.
// "*events.MoveEvent". Mirrors the pattern used in collectTypes.
func typeNameOf(e events.EncounterEvent) string {
	return fmt.Sprintf("%T", e)
}

type VisibilityTransitionSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter

	// alice is the mover in most tests.
	// bob   is close to alice (within sight range of some positions).
	// carol is far away and should never see anything.
	aliceSub *encounter.Subscription
	bobSub   *encounter.Subscription
	carolSub *encounter.Subscription
}

func TestVisibilityTransitionSuite(t *testing.T) {
	suite.Run(t, new(VisibilityTransitionSuite))
}

func (s *VisibilityTransitionSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New("enc-vis", s.broker)

	// carol is far away — she will never see alice in any test.
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "carol", EntityID: "char-carol",
		Position: core.Hex{Q: 100, R: -50, S: -50}, SightRange: 3,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-vis", "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe("enc-vis", "bob")
	s.Require().NoError(err)
	s.carolSub, err = s.broker.Subscribe("enc-vis", "carol")
	s.Require().NoError(err)
}

func (s *VisibilityTransitionSuite) TearDownTest() {
	_ = s.aliceSub.Close()
	_ = s.bobSub.Close()
	_ = s.carolSub.Close()
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// collectEventsTyped collects all events from a subscription within the given
// timeout and returns them typed so callers can inspect both type names and
// concrete event fields.
func collectEventsTyped(sub *encounter.Subscription, timeout time.Duration) []events.EncounterEvent {
	var out []events.EncounterEvent
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return out
			}
			out = append(out, evt)
		case <-deadline:
			return out
		}
	}
}

// assertHasType returns the first event of the given type name, or fails.
func (s *VisibilityTransitionSuite) assertHasType(evts []events.EncounterEvent, typeName string) events.EncounterEvent {
	s.T().Helper()
	for _, e := range evts {
		if typeName == typeNameOf(e) {
			return e
		}
	}
	var names []string
	for _, e := range evts {
		names = append(names, typeNameOf(e))
	}
	s.FailNowf("event not found", "want %s, got %v", typeName, names)
	return nil
}

// assertLacksType fails if any event with the given type name is found.
func (s *VisibilityTransitionSuite) assertLacksType(evts []events.EncounterEvent, typeName string) {
	s.T().Helper()
	for _, e := range evts {
		if typeNameOf(e) == typeName {
			s.FailNowf("unexpected event", "got %s but expected none", typeName)
		}
	}
}

// ─── Case 1: Enter LoS ────────────────────────────────────────────────────────
//
// Alice starts at Q=-10 (far from bob), bob at origin with sightRange=4.
// Alice moves to Q=3 which is inside bob's view.
// Expected: bob receives EntityAppearedEvent with Position == path end.
func (s *VisibilityTransitionSuite) TestEnterLoS_EntityAppearedEventFired() {
	// Alice starts outside bob's view (Q=-10).
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: -10, R: 0, S: 10}, SightRange: 4,
	}))
	// Bob at origin with sight range 4.
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))

	// Alice moves to Q=3 which is within bob's sight range of 4.
	pathEnd := core.Hex{Q: 3, R: 0, S: -3}
	s.Require().NoError(s.enc.Move("alice", []core.Hex{pathEnd}))

	bobEvts := collectEventsTyped(s.bobSub, 500*time.Millisecond)

	evt := s.assertHasType(bobEvts, "*events.EntityAppearedEvent")
	appeared := evt.(*events.EntityAppearedEvent)
	s.Equal(core.EntityID("char-alice"), appeared.Entity)
	s.Equal(pathEnd, appeared.Position, "position should be where mover became visible (path end)")
	s.Contains(appeared.PerPlayer, core.PlayerID("bob"))
}

// ─── Case 2: Leave LoS ────────────────────────────────────────────────────────
//
// Alice starts at Q=2 (within bob's view), moves to Q=10 (outside view).
// Expected: bob receives EntityDisappearedEvent with PerPlayer[bob] == last visible hex.
func (s *VisibilityTransitionSuite) TestLeaveLoS_EntityDisappearedEventFired() {
	// Alice starts inside bob's view (Q=2, bob has sightRange=4).
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 2, R: 0, S: -2}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))

	// Alice moves to Q=10 which is outside bob's sight range of 4.
	// Path crosses from visible to not-visible. We pick a path that stays
	// inside bob's view for the first few hexes, then exits.
	// Q=4 is the boundary of bob's sight range (distance 4 from origin).
	// Q=10 is clearly outside.
	path := []core.Hex{
		{Q: 3, R: 0, S: -3},   // distance 3 from bob — still visible
		{Q: 4, R: 0, S: -4},   // distance 4 from bob — on the edge (still visible under stub LoS)
		{Q: 10, R: 0, S: -10}, // distance 10 — outside
	}
	s.Require().NoError(s.enc.Move("alice", path))

	bobEvts := collectEventsTyped(s.bobSub, 500*time.Millisecond)

	evt := s.assertHasType(bobEvts, "*events.EntityDisappearedEvent")
	disappeared := evt.(*events.EntityDisappearedEvent)
	s.Equal(core.EntityID("char-alice"), disappeared.Entity)
	s.Require().Contains(disappeared.PerPlayer, core.PlayerID("bob"),
		"bob should be in PerPlayer as a viewer who lost sight")

	// The last-known hex for bob must be the last visible hex from bob's SeenSegments.
	// Under the stub LoS, bob's sight range is 4 from origin.
	// path[0]={3,0,-3} distance=3 ✓, path[1]={4,0,-4} distance=4 ✓, path[2]={10,0,-10} ✗
	// So bob's SeenSegments = [{3,0,-3}, {4,0,-4}]. Last visible = {4,0,-4}.
	s.Equal(core.Hex{Q: 4, R: 0, S: -4}, disappeared.PerPlayer[core.PlayerID("bob")],
		"last-known hex must match the last visible hex from bob's SeenSegments")
}

// ─── Case 3: Pass through ─────────────────────────────────────────────────────
//
// Alice starts at Q=-10 (outside bob's view), passes through it (Q=2), then
// exits at Q=10. Expected: bob receives BOTH EntityAppearedEvent AND
// EntityDisappearedEvent with appropriate hexes.
func (s *VisibilityTransitionSuite) TestPassThrough_BothEventsFired() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: -10, R: 0, S: 10}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))

	// Alice's path: starts outside bob's view, enters it, then leaves.
	// Under stub LoS bob sees hexes within distance 4 of origin.
	// path[0]={-10,...} outside, path[1]={-3,...} inside (dist=3), path[2]={4,...} inside (dist=4),
	// path[3]={10,...} outside.
	path := []core.Hex{
		{Q: -10, R: 0, S: 10}, // outside
		{Q: -3, R: 0, S: 3},   // inside (dist 3)
		{Q: 4, R: 0, S: -4},   // inside (dist 4, on edge)
		{Q: 10, R: 0, S: -10}, // outside
	}
	s.Require().NoError(s.enc.Move("alice", path))

	bobEvts := collectEventsTyped(s.bobSub, 500*time.Millisecond)

	// Both events should be present.
	appearedEvt := s.assertHasType(bobEvts, "*events.EntityAppearedEvent")
	disappearedEvt := s.assertHasType(bobEvts, "*events.EntityDisappearedEvent")

	appeared := appearedEvt.(*events.EntityAppearedEvent)
	disappeared := disappearedEvt.(*events.EntityDisappearedEvent)

	s.Equal(core.EntityID("char-alice"), appeared.Entity)
	s.Equal(core.EntityID("char-alice"), disappeared.Entity)

	// Appeared at the first visible hex (SeenSegments[0] = {-3,0,3}).
	s.Equal(core.Hex{Q: -3, R: 0, S: 3}, appeared.Position,
		"appeared position should be first visible hex (SeenSegments[0]) for pass-through")

	// Disappeared at the last visible hex (SeenSegments[len-1] = {4,0,-4}).
	s.Require().Contains(disappeared.PerPlayer, core.PlayerID("bob"))
	s.Equal(core.Hex{Q: 4, R: 0, S: -4}, disappeared.PerPlayer[core.PlayerID("bob")],
		"last-known hex should be last visible hex (SeenSegments[last]) for pass-through")
}

// ─── Case 4: Stays visible ────────────────────────────────────────────────────
//
// Alice starts within bob's view, moves a short distance, stays within view.
// Expected: bob receives MoveEvent but NO EntityAppeared/EntityDisappeared.
func (s *VisibilityTransitionSuite) TestStaysVisible_NoAppearDisappearEvents() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))

	// Alice moves from Q=1 to Q=2, staying well inside bob's view.
	path := []core.Hex{{Q: 2, R: 0, S: -2}}
	s.Require().NoError(s.enc.Move("alice", path))

	bobEvts := collectEventsTyped(s.bobSub, 500*time.Millisecond)

	// Bob should receive MoveEvent but NOT appear/disappear.
	s.assertHasType(bobEvts, "*events.MoveEvent")
	s.assertLacksType(bobEvts, "*events.EntityAppearedEvent")
	s.assertLacksType(bobEvts, "*events.EntityDisappearedEvent")
}

// ─── Case 5: Negative audience ────────────────────────────────────────────────
//
// Carol is at Q=100 (far from everything). Alice moves near bob.
// Expected: carol receives NO events at all.
func (s *VisibilityTransitionSuite) TestNegativeAudience_CarolReceivesNoEvents() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: -10, R: 0, S: 10}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))

	// Alice moves into bob's view; carol is at Q=100 and should receive nothing.
	s.Require().NoError(s.enc.Move("alice", []core.Hex{{Q: 3, R: 0, S: -3}}))

	carolEvts := collectEventsTyped(s.carolSub, 300*time.Millisecond)
	s.Empty(carolEvts, "carol shares no LoS with alice's path and should receive no events")
}

// ─── Round-trip tests ─────────────────────────────────────────────────────────

// TestEntityAppearedEvent_JSONRoundTrip verifies that all fields, including the
// unexported encID and seq, survive a JSON marshal/unmarshal cycle.
func (s *VisibilityTransitionSuite) TestEntityAppearedEvent_JSONRoundTrip() {
	original := events.NewEntityAppearedEvent(
		"enc-1", 11, "char-alice",
		core.Hex{Q: 3, R: 0, S: -3},
		map[core.PlayerID]struct{}{"bob": {}, "carol": {}},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.EntityAppearedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(11), decoded.Sequence())
	s.Equal(core.EntityID("char-alice"), decoded.Entity)
	s.Equal(core.Hex{Q: 3, R: 0, S: -3}, decoded.Position)
	s.Contains(decoded.PerPlayer, core.PlayerID("bob"))
	s.Contains(decoded.PerPlayer, core.PlayerID("carol"))
	s.ElementsMatch(
		events.AudienceSet{"bob", "carol"},
		decoded.Audience(),
	)
}

// TestEntityDisappearedEvent_JSONRoundTrip verifies that all fields survive a
// JSON marshal/unmarshal cycle, including the per-viewer last-known hex map.
func (s *VisibilityTransitionSuite) TestEntityDisappearedEvent_JSONRoundTrip() {
	original := events.NewEntityDisappearedEvent(
		"enc-1", 12, "char-alice",
		map[core.PlayerID]core.Hex{
			"bob":   {Q: 4, R: 0, S: -4},
			"carol": {Q: 2, R: -1, S: -1},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.EntityDisappearedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(12), decoded.Sequence())
	s.Equal(core.EntityID("char-alice"), decoded.Entity)
	s.Require().Contains(decoded.PerPlayer, core.PlayerID("bob"))
	s.Equal(core.Hex{Q: 4, R: 0, S: -4}, decoded.PerPlayer[core.PlayerID("bob")])
	s.Require().Contains(decoded.PerPlayer, core.PlayerID("carol"))
	s.Equal(core.Hex{Q: 2, R: -1, S: -1}, decoded.PerPlayer[core.PlayerID("carol")])
	s.ElementsMatch(
		events.AudienceSet{"bob", "carol"},
		decoded.Audience(),
	)
}

// TestEntityAppearedEvent_SatisfiesInterface verifies the sealed interface constraint.
func (s *VisibilityTransitionSuite) TestEntityAppearedEvent_SatisfiesInterface() {
	var _ events.EncounterEvent = (*events.EntityAppearedEvent)(nil)
}

// TestEntityDisappearedEvent_SatisfiesInterface verifies the sealed interface constraint.
func (s *VisibilityTransitionSuite) TestEntityDisappearedEvent_SatisfiesInterface() {
	var _ events.EncounterEvent = (*events.EntityDisappearedEvent)(nil)
}
