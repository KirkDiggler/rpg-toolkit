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
	var _ events.EncounterEvent = (*events.AttackResolvedEvent)(nil)
	var _ events.EncounterEvent = (*events.DamageDealtEvent)(nil)
	var _ events.EncounterEvent = (*events.ConditionAppliedEvent)(nil)
	var _ events.EncounterEvent = (*events.ModeChangedEvent)(nil)
	var _ events.EncounterEvent = (*events.TurnStartedEvent)(nil)
	var _ events.EncounterEvent = (*events.TurnEndedEvent)(nil)
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

// AttackResolvedEvent JSON round-trip and audience-from-perplayer.
func (s *EventsSuite) TestAttackResolvedEvent_JSONRoundTrip() {
	original := events.NewAttackResolvedEvent(
		"enc-1", 11, "char-alice", "goblin-1",
		true, false, 17, 4, 15,
		map[core.PlayerID]events.AttackResolvedSlice{
			"alice": {Visible: true},
			"bob":   {Visible: false},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.AttackResolvedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(11), decoded.Sequence())
	s.Equal(core.EntityID("char-alice"), decoded.AttackerID)
	s.Equal(core.EntityID("goblin-1"), decoded.TargetID)
	s.True(decoded.Hit)
	s.False(decoded.Critical)
	s.Equal(17, decoded.AttackRoll)
	s.Equal(4, decoded.AttackBonus)
	s.Equal(15, decoded.TargetAC)
	s.True(decoded.PerPlayer["alice"].Visible)
	s.False(decoded.PerPlayer["bob"].Visible)
	s.ElementsMatch(events.AudienceSet{"alice", "bob"}, decoded.Audience())
}

// DamageDealtEvent JSON round-trip.
func (s *EventsSuite) TestDamageDealtEvent_JSONRoundTrip() {
	original := events.NewDamageDealtEvent(
		"enc-1", 12, "goblin-1", "char-alice",
		5, "slashing", 2, 7,
		map[core.PlayerID]events.DamageDealtSlice{
			"alice": {Visible: true},
		},
	)
	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.DamageDealtEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(uint64(12), decoded.Sequence())
	s.Equal(core.EntityID("goblin-1"), decoded.TargetID)
	s.Equal(core.EntityID("char-alice"), decoded.SourceID)
	s.Equal(5, decoded.Amount)
	s.Equal("slashing", decoded.DamageType)
	s.Equal(2, decoded.HPAfter)
	s.Equal(7, decoded.MaxHP)
	s.True(decoded.PerPlayer["alice"].Visible)
}

// ConditionAppliedEvent JSON round-trip.
func (s *EventsSuite) TestConditionAppliedEvent_JSONRoundTrip() {
	original := events.NewConditionAppliedEvent(
		"enc-1", 13, "char-alice", "goblin-1",
		"prone", 1,
		map[core.PlayerID]events.ConditionAppliedSlice{
			"alice": {Visible: true},
		},
	)
	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.ConditionAppliedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal("prone", decoded.ConditionRef)
	s.Equal(1, decoded.DurationRounds)
	s.True(decoded.PerPlayer["alice"].Visible)
}

// ModeChangedEvent JSON round-trip.
func (s *EventsSuite) TestModeChangedEvent_JSONRoundTrip() {
	original := events.NewModeChangedEvent(
		"enc-1", 14, core.ModeFreeRoam, core.ModeTurnBased, "ambush",
		map[core.PlayerID]events.ModeChangedSlice{
			"alice": {Visible: true},
			"bob":   {Visible: true},
		},
	)
	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.ModeChangedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.ModeFreeRoam, decoded.From)
	s.Equal(core.ModeTurnBased, decoded.To)
	s.Equal("ambush", decoded.Reason)
	s.Len(decoded.PerPlayer, 2)
}

// TurnStartedEvent / TurnEndedEvent JSON round-trip.
func (s *EventsSuite) TestTurnEvents_JSONRoundTrip() {
	started := events.NewTurnStartedEvent(
		"enc-1", 15, "char-alice", 1,
		map[core.PlayerID]events.TurnStartedSlice{"alice": {Visible: true}},
	)
	payload, err := json.Marshal(started)
	s.Require().NoError(err)
	var decodedStarted events.TurnStartedEvent
	s.Require().NoError(json.Unmarshal(payload, &decodedStarted))
	s.Equal(core.EntityID("char-alice"), decodedStarted.ActorID)
	s.Equal(1, decodedStarted.Round)

	ended := events.NewTurnEndedEvent(
		"enc-1", 16, "char-alice",
		map[core.PlayerID]events.TurnEndedSlice{"alice": {Visible: true}},
	)
	payload, err = json.Marshal(ended)
	s.Require().NoError(err)
	var decodedEnded events.TurnEndedEvent
	s.Require().NoError(json.Unmarshal(payload, &decodedEnded))
	s.Equal(core.EntityID("char-alice"), decodedEnded.ActorID)
}
