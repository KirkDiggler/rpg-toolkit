package encounter_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
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
	e := encounter.New("enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID:   "alice",
		EntityID:   "char-alice",
		Position:   types.Hex{Q: 0, R: 0, S: 0},
		SightRange: 3,
	}))

	snap := e.SnapshotFor("alice")
	s.Equal(types.PlayerID("alice"), snap.PlayerID)
	s.Equal(types.Hex{}, snap.Position)
	s.True(snap.RevealedHexes.Has(types.Hex{}))
}

func (s *EncounterSuite) TestAddPlayer_RejectsDuplicate() {
	e := encounter.New("enc-1", s.broker)
	input := encounter.PlayerInput{PlayerID: "alice", EntityID: "char-1", SightRange: 3}
	s.Require().NoError(e.AddPlayer(input))
	s.Error(e.AddPlayer(input))
}

// ToData / LoadFromData round-trips through JSON cleanly. This test is
// load-bearing because earlier iterations of this design failed JSON
// round-trip for HexSet (struct map keys) — the types subpackage's
// MarshalJSON now fixes that and this test guards against regression.
func (s *EncounterSuite) TestRoundTrip_ToDataLoadFromData() {
	e1 := encounter.New("enc-1", s.broker)
	s.Require().NoError(e1.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-1",
		Position: types.Hex{Q: 1, R: -1, S: 0}, SightRange: 5,
	}))
	e1.AddDoor("door-1", types.Hex{Q: 2, R: 0, S: -2}, false)

	payload, err := json.Marshal(e1.ToData())
	s.Require().NoError(err)

	var data encounter.Data
	s.Require().NoError(json.Unmarshal(payload, &data))

	e2, err := encounter.LoadFromData(&data, s.broker)
	s.Require().NoError(err)

	s.Equal(types.EncounterID("enc-1"), e2.ID())
	snap := e2.SnapshotFor("alice")
	s.Equal(types.Hex{Q: 1, R: -1, S: 0}, snap.Position)
	s.True(snap.RevealedHexes.Has(types.Hex{Q: 1, R: -1, S: 0}),
		"RevealedHexes must round-trip — guards against HexSet JSON regression")
}

func (s *EncounterSuite) TestSnapshotFor_UnknownPlayer() {
	e := encounter.New("enc-1", s.broker)
	snap := e.SnapshotFor("nobody")
	s.Equal(encounter.Snapshot{}, snap)
}

// Move publishes MoveEvent. Mover and viewers in range get a slice; viewers
// out of range are absent from PerPlayer.
func (s *EncounterSuite) TestMove_PublishesMoveEvent() {
	e := encounter.New("enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: types.Hex{}, SightRange: 5,
	}))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: types.Hex{Q: 50, R: -25, S: -25}, SightRange: 5,
	}))

	aliceSub, _ := s.broker.Subscribe("enc-1", "alice")
	bobSub, _ := s.broker.Subscribe("enc-1", "bob")

	path := []types.Hex{
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
	e := encounter.New("enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: types.Hex{}, SightRange: 2,
	}))
	aliceSub, _ := s.broker.Subscribe("enc-1", "alice")

	path := []types.Hex{{Q: 1, R: 0, S: -1}}
	s.Require().NoError(e.Move("alice", path))

	seen := collectTypes(aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.MoveEvent")
	s.Contains(seen, "*events.HexRevealedEvent")
}

func (s *EncounterSuite) TestMove_Validations() {
	e := encounter.New("enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-1", SightRange: 3,
	}))

	s.Error(e.Move("alice", nil), "empty path should error")
	s.Error(e.Move("nobody", []types.Hex{{}}), "unknown player should error")
}

func (s *EncounterSuite) TestOpenDoor_PublishesEvents() {
	e := encounter.New("enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: types.Hex{}, SightRange: 4,
	}))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: types.Hex{Q: 50, R: -25, S: -25}, SightRange: 4,
	}))
	e.AddDoor("door-1", types.Hex{Q: 2, R: 0, S: -2}, false)

	aliceSub, _ := s.broker.Subscribe("enc-1", "alice")
	bobSub, _ := s.broker.Subscribe("enc-1", "bob")

	s.Require().NoError(e.OpenDoor("alice", "door-1"))

	seenAlice := collectTypes(aliceSub, 500*time.Millisecond)
	s.Contains(seenAlice, "*events.DoorOpenedEvent")

	seenBob := collectTypes(bobSub, 100*time.Millisecond)
	s.Empty(seenBob, "bob out of range should receive nothing")
}

func (s *EncounterSuite) TestOpenDoor_Validations() {
	e := encounter.New("enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-1", SightRange: 3,
	}))
	e.AddDoor("door-1", types.Hex{}, false)

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
