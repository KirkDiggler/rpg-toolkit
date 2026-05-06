package encounter_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
	"github.com/stretchr/testify/suite"
)

type IntegrationSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
	bobSub    *encounter.Subscription
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

func (s *IntegrationSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New("enc-walking-skel", s.broker)

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: types.Hex{}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: types.Hex{Q: 2, R: 0, S: -2}, SightRange: 4,
	}))
	s.enc.AddDoor("door-east", types.Hex{Q: 4, R: 0, S: -4}, false)

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-walking-skel", "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe("enc-walking-skel", "bob")
	s.Require().NoError(err)
}

func (s *IntegrationSuite) TearDownTest() {
	_ = s.aliceSub.Close()
	_ = s.bobSub.Close()
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// Alice moves toward Bob. Bob is within sight (distance 2 between her
// destination and his position). Both should receive MoveEvent.
func (s *IntegrationSuite) TestSlice_MoveTowardEachOther() {
	path := []types.Hex{{Q: 1, R: 0, S: -1}}
	s.Require().NoError(s.enc.Move("alice", path))

	aliceEvents := collectTypes(s.aliceSub, 500*time.Millisecond)
	bobEvents := collectTypes(s.bobSub, 500*time.Millisecond)

	s.Contains(aliceEvents, "*events.MoveEvent")
	s.Contains(bobEvents, "*events.MoveEvent",
		"bob within distance 2 should see alice move")
}

// Door opens at distance 4 from alice (sight range 4) and distance 2 from
// bob (sight range 4). Both should see the door event.
func (s *IntegrationSuite) TestSlice_OpenDoor() {
	s.Require().NoError(s.enc.OpenDoor("alice", "door-east"))

	aliceEvents := collectTypes(s.aliceSub, 500*time.Millisecond)
	bobEvents := collectTypes(s.bobSub, 500*time.Millisecond)

	s.Contains(aliceEvents, "*events.DoorOpenedEvent")
	s.Contains(bobEvents, "*events.DoorOpenedEvent")
}

// Persistence round-trip: serialize, "restart," replay another action,
// observe events still flow and prior reveal persisted.
func (s *IntegrationSuite) TestSlice_RoundTripPersistence() {
	s.Require().NoError(s.enc.Move("alice", []types.Hex{{Q: 1, R: 0, S: -1}}))
	drainSub(s.aliceSub, 200*time.Millisecond)
	drainSub(s.bobSub, 200*time.Millisecond)

	payload, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)

	// Simulate restart.
	_ = s.broker.Close()
	_ = s.transport.Close()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)

	var loaded encounter.Data
	s.Require().NoError(json.Unmarshal(payload, &loaded))
	enc2, err := encounter.LoadFromData(&loaded, s.broker)
	s.Require().NoError(err)

	aliceSub2, err := s.broker.Subscribe("enc-walking-skel", "alice")
	s.Require().NoError(err)
	defer func() { _ = aliceSub2.Close() }()

	s.Require().NoError(enc2.Move("alice", []types.Hex{{Q: 2, R: 0, S: -2}}))

	aliceEvents := collectTypes(aliceSub2, 500*time.Millisecond)
	s.Contains(aliceEvents, "*events.MoveEvent",
		"after reload, encounter publishes events through the new broker")

	snap := enc2.SnapshotFor("alice")
	s.True(snap.RevealedHexes.Has(types.Hex{Q: 1, R: 0, S: -1}),
		"reveal from round-1 move should persist through round-trip")
}

// Sequence numbers monotonic across both events of one action.
func (s *IntegrationSuite) TestSlice_SequenceMonotonic() {
	s.Require().NoError(s.enc.Move("alice", []types.Hex{{Q: 1, R: 0, S: -1}}))

	var seqs []uint64
	timeout := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case evt, ok := <-s.aliceSub.Events():
			if !ok {
				break loop
			}
			seqs = append(seqs, evt.Sequence())
			if len(seqs) >= 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}
	s.Require().Len(seqs, 2, "expected MoveEvent + HexRevealedEvent")
	s.True(seqs[1] > seqs[0], "sequence advances between events")
}

func drainSub(sub *encounter.Subscription, timeout time.Duration) {
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				return
			}
		case <-deadline:
			return
		}
	}
}
