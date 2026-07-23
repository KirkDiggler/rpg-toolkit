package encounter_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

const (
	obstacleRevealCarolPlayerID core.PlayerID = "carol"
	obstacleRevealCarolEntityID core.EntityID = "char-carol"
)

// ObstacleRevealSuite verifies that static obstacles use the sticky explored
// map, rather than the moving-entity current-line-of-sight lifecycle.
type ObstacleRevealSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
}

func TestObstacleRevealSuite(t *testing.T) {
	suite.Run(t, new(ObstacleRevealSuite))
}

func (s *ObstacleRevealSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-obstacle-reveal", s.broker)
	s.Require().NoError(s.enc.InitRoom(20, 20, environments.PatternEmpty))
}

func (s *ObstacleRevealSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

func (s *ObstacleRevealSuite) subscribe(playerID core.PlayerID) *encounter.Subscription {
	s.T().Helper()
	sub, err := s.broker.Subscribe("enc-obstacle-reveal", playerID)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = sub.Close() })
	return sub
}

func obstacleAppearances(evts []events.EncounterEvent) []*events.EntityAppearedEvent {
	var appeared []*events.EntityAppearedEvent
	for _, evt := range evts {
		if e, ok := evt.(*events.EntityAppearedEvent); ok {
			appeared = append(appeared, e)
		}
	}
	return appeared
}

// TestMove_RevealsStaticObstacleOnce verifies the move path emits the existing
// singular event exactly once when an obstacle position enters the mover's
// newly revealed hex delta. Returning to an already explored hex cannot emit a
// duplicate or a disappearance.
func (s *ObstacleRevealSuite) TestMove_RevealsStaticObstacleOnce() {
	obstaclePos := lineHex(3)
	s.Require().NoError(s.enc.AddObstacle("pillar-1", "dnd5e:obstacles:pillar", obstaclePos, false, false))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: lineHex(0), SightRange: 2,
	}))
	aliceSub := s.subscribe(alicePlayerID)

	s.Require().NoError(s.enc.Move(alicePlayerID, []core.Hex{lineHex(1)}))
	s.Require().NoError(s.enc.Move(alicePlayerID, []core.Hex{lineHex(0)}))

	observedEvents := collectEventsTyped(aliceSub, 300*time.Millisecond)
	appeared := obstacleAppearances(observedEvents)
	s.Require().Len(appeared, 1)
	s.Equal(core.EntityID("pillar-1"), appeared[0].Entity)
	s.Equal(obstaclePos, appeared[0].Position)
	s.Equal(map[core.PlayerID]struct{}{alicePlayerID: {}}, appeared[0].PerPlayer)
	for _, evt := range observedEvents {
		if disappeared, ok := evt.(*events.EntityDisappearedEvent); ok {
			s.NotEqual(core.EntityID("pillar-1"), disappeared.Entity)
		}
	}
}

// TestMove_HiddenObstacleDoesNotAppear verifies that an obstacle outside a
// move's newly revealed delta is never leaked through EntityAppearedEvent.
func (s *ObstacleRevealSuite) TestMove_HiddenObstacleDoesNotAppear() {
	s.Require().NoError(s.enc.AddObstacle("hidden-1", "dnd5e:obstacles:statue", lineHex(8), false, false))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: lineHex(0), SightRange: 2,
	}))
	aliceSub := s.subscribe(alicePlayerID)

	s.Require().NoError(s.enc.Move(alicePlayerID, []core.Hex{lineHex(1)}))

	for _, appeared := range obstacleAppearances(collectEventsTyped(aliceSub, 300*time.Millisecond)) {
		s.NotEqual(core.EntityID("hidden-1"), appeared.Entity)
	}
}

// TestMove_MultipleStaticObstaclesUseStableIDOrder verifies event order does
// not depend on the persisted obstacle insertion order.
func (s *ObstacleRevealSuite) TestMove_MultipleStaticObstaclesUseStableIDOrder() {
	s.Require().NoError(s.enc.AddObstacle("z-statue", "dnd5e:obstacles:statue", core.Hex{Q: 1, R: -2, S: 1}, false, false))
	s.Require().NoError(s.enc.AddObstacle("a-pillar", "dnd5e:obstacles:pillar", lineHex(2), false, false))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: lineHex(0), SightRange: 1,
	}))
	aliceSub := s.subscribe(alicePlayerID)

	s.Require().NoError(s.enc.Move(alicePlayerID, []core.Hex{lineHex(1)}))

	appeared := obstacleAppearances(collectEventsTyped(aliceSub, 300*time.Millisecond))
	s.Require().Len(appeared, 2)
	s.Equal(core.EntityID("a-pillar"), appeared[0].Entity)
	s.Equal(core.EntityID("z-statue"), appeared[1].Entity)
}

// TestOpenDoor_RevealsStaticObstacleToOnlyNewlyRevealedViewers verifies a door
// opening groups all and only viewers whose reveal delta contains the obstacle.
func (s *ObstacleRevealSuite) TestOpenDoor_RevealsStaticObstacleToOnlyNewlyRevealedViewers() {
	s.Require().NoError(s.enc.AddDoor("door-1", lineHex(3), false))
	s.Require().NoError(s.enc.AddObstacle("altar-1", "dnd5e:obstacles:altar", lineHex(6), false, false))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: lineHex(0), SightRange: 8,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID, Position: lineHex(1), SightRange: 8,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID:   obstacleRevealCarolPlayerID,
		EntityID:   obstacleRevealCarolEntityID,
		Position:   core.Hex{Q: 15, R: -15, S: 0},
		SightRange: 2,
	}))
	aliceSub := s.subscribe(alicePlayerID)
	bobSub := s.subscribe(bobPlayerID)
	carolSub := s.subscribe(obstacleRevealCarolPlayerID)

	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, "door-1"))

	aliceAppeared := obstacleAppearances(collectEventsTyped(aliceSub, 300*time.Millisecond))
	bobAppeared := obstacleAppearances(collectEventsTyped(bobSub, 300*time.Millisecond))
	s.Require().Len(aliceAppeared, 1)
	s.Require().Len(bobAppeared, 1)
	wantAudience := map[core.PlayerID]struct{}{alicePlayerID: {}, bobPlayerID: {}}
	s.Equal(wantAudience, aliceAppeared[0].PerPlayer)
	s.Equal(wantAudience, bobAppeared[0].PerPlayer)
	s.Empty(obstacleAppearances(collectEventsTyped(carolSub, 300*time.Millisecond)))
}
