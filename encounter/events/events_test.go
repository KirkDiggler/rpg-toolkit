package events_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
	"github.com/stretchr/testify/suite"
)

type EventsSuite struct {
	suite.Suite
}

func TestEventsSuite(t *testing.T) {
	suite.Run(t, new(EventsSuite))
}

// Each concrete satisfies the sealed EncounterEvent interface.
func (s *EventsSuite) TestConcretes_SatisfyInterface() {
	var _ events.EncounterEvent = (*events.MoveEvent)(nil)
	var _ events.EncounterEvent = (*events.HexRevealedEvent)(nil)
	var _ events.EncounterEvent = (*events.DoorOpenedEvent)(nil)
}

// MoveEvent.Audience derives from PerPlayer keys; absent players are not in audience.
func (s *EventsSuite) TestMoveEvent_AudienceFromPerPlayer() {
	e := events.NewMoveEvent("enc-1", 7, "bob",
		[]types.Hex{{Q: 0, R: 0, S: 0}},
		map[types.PlayerID]events.MovePlayerSlice{
			"alice": {SeenSegments: []types.Hex{{Q: 0, R: 0, S: 0}}},
			"carol": {SeenSegments: []types.Hex{}},
		},
	)

	s.Equal(types.EncounterID("enc-1"), e.EncounterID())
	s.Equal(uint64(7), e.Sequence())
	s.ElementsMatch(types.AudienceSet{"alice", "carol"}, e.Audience())
}

// MoveEvent JSON round-trip preserves all fields, including unexported encID/seq.
func (s *EventsSuite) TestMoveEvent_JSONRoundTrip() {
	original := events.NewMoveEvent("enc-1", 42, "bob",
		[]types.Hex{{Q: 1, R: -1, S: 0}, {Q: 2, R: -1, S: -1}},
		map[types.PlayerID]events.MovePlayerSlice{
			"alice": {SeenSegments: []types.Hex{{Q: 1, R: -1, S: 0}}},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.MoveEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(types.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(42), decoded.Sequence())
	s.Equal(types.EntityID("bob"), decoded.Mover)
	s.Equal(original.Path, decoded.Path)
	s.Require().Contains(decoded.PerPlayer, types.PlayerID("alice"))
	s.Equal(original.PerPlayer["alice"].SeenSegments, decoded.PerPlayer["alice"].SeenSegments)
}

// HexRevealedEvent JSON round-trip — load-bearing because PerceptionView
// embeds HexSet via this slice and the persistence layer round-trips it.
func (s *EventsSuite) TestHexRevealedEvent_JSONRoundTrip() {
	original := events.NewHexRevealedEvent("enc-1", 8,
		map[types.PlayerID]events.HexRevealedSlice{
			"alice": {Hexes: types.NewHexSet(types.Hex{Q: 1, R: 0, S: -1})},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.HexRevealedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(types.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(8), decoded.Sequence())
	aliceSlice := decoded.PerPlayer["alice"]
	s.True(aliceSlice.Hexes.Has(types.Hex{Q: 1, R: 0, S: -1}))
}

// Type switch returns the concrete type.
func (s *EventsSuite) TestTypeSwitch_RecoversConcrete() {
	evts := []events.EncounterEvent{
		events.NewMoveEvent("enc-1", 1, "bob", nil, nil),
		events.NewHexRevealedEvent("enc-1", 2, nil),
		events.NewDoorOpenedEvent("enc-1", 3, "door-1", "bob", nil),
	}

	var seen []string
	for _, evt := range evts {
		switch evt.(type) {
		case *events.MoveEvent:
			seen = append(seen, "move")
		case *events.HexRevealedEvent:
			seen = append(seen, "revealed")
		case *events.DoorOpenedEvent:
			seen = append(seen, "door")
		default:
			s.FailNow("unhandled event type")
		}
	}
	s.Equal([]string{"move", "revealed", "door"}, seen)
}
