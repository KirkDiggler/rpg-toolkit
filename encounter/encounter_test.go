package encounter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/stretchr/testify/suite"
)

type EncounterSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestEncounterSuite(t *testing.T) {
	suite.Run(t, new(EncounterSuite))
}

func (s *EncounterSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *EncounterSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

func (s *EncounterSuite) TestAddPlayer_PopulatesView() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID:   "alice",
		EntityID:   "char-alice",
		Position:   core.Hex{Q: 0, R: 0, S: 0},
		SightRange: 3,
	}))

	snap := e.SnapshotFor("alice")
	s.Equal(core.PlayerID("alice"), snap.PlayerID)
	s.Equal(core.Hex{}, snap.Position)
	s.True(snap.RevealedHexes.Has(core.Hex{}))
}

func (s *EncounterSuite) TestAddPlayer_RejectsDuplicate() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	input := encounter.PlayerInput{PlayerID: "alice", EntityID: "char-1", SightRange: 3}
	s.Require().NoError(e.AddPlayer(input))
	s.Error(e.AddPlayer(input))
}

// ToData / LoadFromData round-trips through JSON cleanly. This test is
// load-bearing because earlier iterations of this design failed JSON
// round-trip for HexSet (struct map keys) — the types subpackage's
// MarshalJSON now fixes that and this test guards against regression.
func (s *EncounterSuite) TestRoundTrip_ToDataLoadFromData() {
	e1 := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e1.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-1",
		Position: core.Hex{Q: 1, R: -1, S: 0}, SightRange: 5,
	}))
	s.Require().NoError(e1.AddDoor("door-1", core.Hex{Q: 2, R: 0, S: -2}, false))

	payload, err := json.Marshal(e1.ToData())
	s.Require().NoError(err)

	var data encounter.Data
	s.Require().NoError(json.Unmarshal(payload, &data))

	e2, err := encounter.LoadFromData(context.Background(), &data, s.broker)
	s.Require().NoError(err)

	s.Equal(core.EncounterID("enc-1"), e2.ID())
	snap := e2.SnapshotFor("alice")
	s.Equal(core.Hex{Q: 1, R: -1, S: 0}, snap.Position)
	s.True(snap.RevealedHexes.Has(core.Hex{Q: 1, R: -1, S: 0}),
		"RevealedHexes must round-trip — guards against HexSet JSON regression")
}

func (s *EncounterSuite) TestSnapshotFor_UnknownPlayer() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	snap := e.SnapshotFor("nobody")
	s.Equal(encounter.Snapshot{}, snap)
}

// Move publishes MoveEvent. Mover and viewers in range get a slice; viewers
// out of range are absent from PerPlayer.
func (s *EncounterSuite) TestMove_PublishesMoveEvent() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 5,
	}))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 50, R: -25, S: -25}, SightRange: 5,
	}))

	aliceSub, _ := s.broker.Subscribe("enc-1", "alice")
	bobSub, _ := s.broker.Subscribe("enc-1", "bob")

	path := []core.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	s.Require().NoError(e.Move("alice", path))

	// Alice (mover) gets MoveEvent.
	s.assertReceivesType(aliceSub, "*events.MoveEvent")
	// Bob (out of range) gets nothing.
	s.assertNoReceive(bobSub)
}

// Move publishes HexRevealedEvent when the mover's vision grew. This test
// guards against a regression where the delta was computed AFTER applying
// reveal — making the delta always empty.
func (s *EncounterSuite) TestMove_PublishesHexRevealedEventForMover() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 2,
	}))
	aliceSub, _ := s.broker.Subscribe("enc-1", "alice")

	path := []core.Hex{{Q: 1, R: 0, S: -1}}
	s.Require().NoError(e.Move("alice", path))

	seen := collectTypes(aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.MoveEvent")
	s.Contains(seen, "*events.HexRevealedEvent")
}

// rpg-toolkit#862 (companion fix for KirkDiggler/rpg-api#737): the mover's
// own MoveEvent.MoverKnownHexes must place them at the NEW hex, and must
// NOT still report the just-vacated origin hex as VISIBLE with them on it.
// This is the toolkit-side half of a real bug: a downstream consumer
// (rpg-api's stream translator) that instead re-derives this via its own
// out-of-band repository read raced against the SAME move's not-yet-executed
// persist, and read pre-move truth — the origin
// hex, still VISIBLE, still listing the mover. MoverKnownHexes exists so a
// consumer never needs that separate, racy read: this asserts the event's
// OWN payload is correct at the moment of publish.
func (s *EncounterSuite) TestMove_MoverKnownHexesReflectsPostMoveTruth() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	aliceSub, _ := s.broker.Subscribe("enc-1", "alice")

	// Straight-line move of hex-distance 3 — strictly farther than
	// SightRange 2, so the origin hex (0,0,0) unambiguously drops out of
	// sight this move, the same shape as "stepping behind a pillar" or
	// backing out of a room far enough to lose the doorway.
	path := []core.Hex{
		{Q: 1, R: -1, S: 0},
		{Q: 2, R: -2, S: 0},
		{Q: 3, R: -3, S: 0},
	}
	s.Require().NoError(e.Move("alice", path))

	var moveEvt *events.MoveEvent
	select {
	case evt, ok := <-aliceSub.Events():
		s.Require().True(ok)
		var isMove bool
		moveEvt, isMove = evt.(*events.MoveEvent)
		s.Require().True(isMove, "expected *events.MoveEvent, got %T", evt)
	case <-time.After(time.Second):
		s.FailNow("did not receive MoveEvent in 1s")
	}

	byPos := make(map[core.Hex]events.KnownHex, len(moveEvt.MoverKnownHexes))
	for _, kh := range moveEvt.MoverKnownHexes {
		byPos[kh.Position] = kh
	}

	// This is the actual bug: rpg-api's own out-of-band re-read reported the
	// origin hex as STILL VISIBLE with alice on it, because it raced ahead
	// of the persist for this exact move. State REMEMBERED (not VISIBLE) is
	// the one assertion that pins the fix — a remembered record's Contents
	// legitimately still names alice (she WAS there the last time this hex
	// was actually observed as visible, which for the starting hex is true
	// by construction); "nothing is ever deleted" from a memory without a
	// fresh VISIBLE re-observation is the documented HexRecord contract,
	// not a bug this test should assert against.
	origin, ok := byPos[core.Hex{Q: 0, R: 0, S: 0}]
	s.Require().True(ok, "origin hex must still be present (remembered), not dropped")
	s.Equal(int(perception.KnowledgeStateRemembered), origin.State,
		"origin hex must be REMEMBERED, not still VISIBLE, now that alice has moved out of range")

	dest, ok := byPos[core.Hex{Q: 3, R: -3, S: 0}]
	s.Require().True(ok, "destination hex must be present")
	s.Equal(int(perception.KnowledgeStateVisible), dest.State, "destination hex must be VISIBLE")
	foundMoverAtDest := false
	for _, c := range dest.Contents {
		if c.EntityID == "char-alice" {
			foundMoverAtDest = true
		}
	}
	s.True(foundMoverAtDest, "destination hex's contents must place alice there")
}

func (s *EncounterSuite) TestMove_Validations() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-1", SightRange: 3,
	}))

	s.Error(e.Move("alice", nil), "empty path should error")
	s.Error(e.Move("nobody", []core.Hex{{}}), "unknown player should error")
}

func (s *EncounterSuite) TestOpenDoor_PublishesEvents() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 4,
	}))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 50, R: -25, S: -25}, SightRange: 4,
	}))
	// rpg-toolkit#864: OpenDoor requires adjacency — door is 1 hex from
	// alice (was 2; unrelated to what this test actually proves: event
	// publishing + bob's out-of-range non-delivery).
	s.Require().NoError(e.AddDoor("door-1", core.Hex{Q: 1, R: 0, S: -1}, false))

	aliceSub, _ := s.broker.Subscribe("enc-1", "alice")
	bobSub, _ := s.broker.Subscribe("enc-1", "bob")

	s.Require().NoError(e.OpenDoor("alice", "door-1"))

	seenAlice := collectTypes(aliceSub, 500*time.Millisecond)
	s.Contains(seenAlice, "*events.DoorOpenedEvent")

	seenBob := collectTypes(bobSub, 100*time.Millisecond)
	s.Empty(seenBob, "bob out of range should receive nothing")
}

func (s *EncounterSuite) TestOpenDoor_Validations() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-1", SightRange: 3,
	}))
	s.Require().NoError(e.AddDoor("door-1", core.Hex{}, false))

	s.Error(e.OpenDoor("nobody", "door-1"))
	s.Error(e.OpenDoor("alice", "nonexistent"))
	s.Require().NoError(e.OpenDoor("alice", "door-1"))
	s.Error(e.OpenDoor("alice", "door-1"), "second open should error")
}

// Helpers shared with later EncounterSuite tests (OpenDoor in Task 8) and
// integration tests in Task 9.
func (s *EncounterSuite) assertReceivesType(sub *encounter.Subscription, want string) {
	s.T().Helper()
	select {
	case evt, ok := <-sub.Events():
		s.Require().True(ok)
		s.Equal(want, fmt.Sprintf("%T", evt))
	case <-time.After(time.Second):
		s.FailNow("did not receive event in 1s")
	}
}

func (s *EncounterSuite) assertNoReceive(sub *encounter.Subscription) {
	s.T().Helper()
	select {
	case evt, ok := <-sub.Events():
		if ok {
			s.FailNowf("unexpected event", "got %T", evt)
		}
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func collectTypes(sub *encounter.Subscription, timeout time.Duration) []string {
	var out []string
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return out
			}
			out = append(out, fmt.Sprintf("%T", evt))
		case <-deadline:
			return out
		}
	}
}
