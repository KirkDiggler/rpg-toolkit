package events_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
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
	var _ events.EncounterEvent = (*events.EntityAppearedEvent)(nil)
	var _ events.EncounterEvent = (*events.EntityDisappearedEvent)(nil)
}

// MoveEvent.Audience derives from PerPlayer keys; absent players are not in audience.
func (s *EventsSuite) TestMoveEvent_AudienceFromPerPlayer() {
	e := events.NewMoveEvent("enc-1", 7, "bob",
		[]core.Hex{{Q: 0, R: 0, S: 0}},
		map[core.PlayerID]events.MovePlayerSlice{
			"alice": {SeenSegments: []core.Hex{{Q: 0, R: 0, S: 0}}},
			"carol": {SeenSegments: []core.Hex{}},
		},
	)

	s.Equal(core.EncounterID("enc-1"), e.EncounterID())
	s.Equal(uint64(7), e.Sequence())
	s.ElementsMatch(events.AudienceSet{"alice", "carol"}, e.Audience())
}

// MoveEvent JSON round-trip preserves all fields, including unexported encID/seq.
func (s *EventsSuite) TestMoveEvent_JSONRoundTrip() {
	original := events.NewMoveEvent("enc-1", 42, "bob",
		[]core.Hex{{Q: 1, R: -1, S: 0}, {Q: 2, R: -1, S: -1}},
		map[core.PlayerID]events.MovePlayerSlice{
			"alice": {SeenSegments: []core.Hex{{Q: 1, R: -1, S: 0}}},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.MoveEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(42), decoded.Sequence())
	s.Equal(core.EntityID("bob"), decoded.Mover)
	s.Equal(original.Path, decoded.Path)
	s.Require().Contains(decoded.PerPlayer, core.PlayerID("alice"))
	s.Equal(original.PerPlayer["alice"].SeenSegments, decoded.PerPlayer["alice"].SeenSegments)
}

// HexRevealedEvent JSON round-trip — load-bearing because PerceptionView
// embeds HexSet via this slice and the persistence layer round-trips it.
func (s *EventsSuite) TestHexRevealedEvent_JSONRoundTrip() {
	original := events.NewHexRevealedEvent("enc-1", 8,
		map[core.PlayerID]events.HexRevealedSlice{
			"alice": {Hexes: core.NewHexSet(core.Hex{Q: 1, R: 0, S: -1})},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.HexRevealedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(8), decoded.Sequence())
	aliceSlice := decoded.PerPlayer["alice"]
	s.True(aliceSlice.Hexes.Has(core.Hex{Q: 1, R: 0, S: -1}))
}

// EntityAppearedEvent JSON round-trip — audience derived from PerPlayer keys.
func (s *EventsSuite) TestEntityAppearedEvent_JSONRoundTrip() {
	original := events.NewEntityAppearedEvent(
		"enc-1", 9, "char-alice",
		core.Hex{Q: 2, R: 0, S: -2},
		map[core.PlayerID]struct{}{"bob": {}, "carol": {}},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.EntityAppearedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(9), decoded.Sequence())
	s.Equal(core.EntityID("char-alice"), decoded.Entity)
	s.Equal(core.Hex{Q: 2, R: 0, S: -2}, decoded.Position)
	s.Contains(decoded.PerPlayer, core.PlayerID("bob"))
	s.Contains(decoded.PerPlayer, core.PlayerID("carol"))
	s.ElementsMatch(events.AudienceSet{"bob", "carol"}, decoded.Audience())
}

// EntityDisappearedEvent JSON round-trip — per-viewer hex map survives encoding.
func (s *EventsSuite) TestEntityDisappearedEvent_JSONRoundTrip() {
	original := events.NewEntityDisappearedEvent(
		"enc-1", 10, "char-alice",
		map[core.PlayerID]core.Hex{
			"bob":   {Q: 3, R: 0, S: -3},
			"carol": {Q: 1, R: 1, S: -2},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.EntityDisappearedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(10), decoded.Sequence())
	s.Equal(core.EntityID("char-alice"), decoded.Entity)
	s.Require().Contains(decoded.PerPlayer, core.PlayerID("bob"))
	s.Equal(core.Hex{Q: 3, R: 0, S: -3}, decoded.PerPlayer[core.PlayerID("bob")])
	s.Require().Contains(decoded.PerPlayer, core.PlayerID("carol"))
	s.Equal(core.Hex{Q: 1, R: 1, S: -2}, decoded.PerPlayer[core.PlayerID("carol")])
	s.ElementsMatch(events.AudienceSet{"bob", "carol"}, decoded.Audience())
}

// Type switch returns the concrete type.
func (s *EventsSuite) TestTypeSwitch_RecoversConcrete() {
	evts := []events.EncounterEvent{
		events.NewMoveEvent("enc-1", 1, "bob", nil, nil),
		events.NewHexRevealedEvent("enc-1", 2, nil),
		events.NewDoorOpenedEvent("enc-1", 3, "door-1", "bob", nil),
		events.NewEntityAppearedEvent("enc-1", 4, "bob", core.Hex{}, nil),
		events.NewEntityDisappearedEvent("enc-1", 5, "bob", nil),
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
		case *events.EntityAppearedEvent:
			seen = append(seen, "appeared")
		case *events.EntityDisappearedEvent:
			seen = append(seen, "disappeared")
		default:
			s.FailNow("unhandled event type")
		}
	}
	s.Equal([]string{"move", "revealed", "door", "appeared", "disappeared"}, seen)
}
