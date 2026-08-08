package encounter_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

type MovementOccupancySuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
}

func TestMovementOccupancySuite(t *testing.T) {
	suite.Run(t, new(MovementOccupancySuite))
}

func (s *MovementOccupancySuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-movement-occupancy", s.broker)

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-movement-occupancy", "alice")
	s.Require().NoError(err)
}

func (s *MovementOccupancySuite) TearDownTest() {
	s.Require().NoError(s.aliceSub.Close())
	s.Require().NoError(s.broker.Close())
	s.Require().NoError(s.transport.Close())
}

func (s *MovementOccupancySuite) addPlayer(sightRange int) {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice", Position: core.Hex{}, SightRange: sightRange,
	}))
}

func (s *MovementOccupancySuite) addMonster(id string, position core.Hex) {
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: core.EntityID(id), Position: position, HP: 7, MaxHP: 7,
	}))
}

func (s *MovementOccupancySuite) moveEventFor(mover core.EntityID) *events.MoveEvent {
	for _, event := range collectEventsTyped(s.aliceSub, 500*time.Millisecond) {
		if move, ok := event.(*events.MoveEvent); ok && move.Mover == mover {
			return move
		}
	}
	return nil
}

// TestNPCMove_OccupiedMonsterDestinationStopsAtLegalPrefix reproduces the
// authoritative monster-on-monster overlap: encounter movement owns both
// positions, while the reconstructed spatial room does not contain creatures.
func (s *MovementOccupancySuite) TestNPCMove_OccupiedMonsterDestinationStopsAtLegalPrefix() {
	s.addPlayer(10)
	s.addMonster("goblin-mover", core.Hex{Q: 3, R: 0, S: -3})
	s.addMonster("goblin-occupant", core.Hex{Q: 1, R: 0, S: -1})
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond)

	requested := []core.Hex{{Q: 2, R: 0, S: -2}, {Q: 1, R: 0, S: -1}}
	s.Require().NoError(s.enc.MoveNPCSteps("goblin-mover", requested))

	data := s.enc.ToData()
	s.Equal(core.Hex{Q: 2, R: 0, S: -2}, data.Monsters["goblin-mover"].Position)
	s.Equal(core.Hex{Q: 1, R: 0, S: -1}, data.Monsters["goblin-occupant"].Position)

	move := s.moveEventFor("goblin-mover")
	s.Require().NotNil(move)
	s.Equal([]core.Hex{{Q: 2, R: 0, S: -2}}, move.Path,
		"the authoritative event must carry the actual legal path, not the occupied intent")
	s.T().Logf("occupancy trace: mover=goblin-mover proposed=%v actual=%v final=%v occupant_visible_to_alice=true",
		requested, move.Path, data.Monsters["goblin-mover"].Position)
}

// TestPlayerMove_HiddenOccupiedDestinationStopsWithoutIdentityLeak reproduces
// the stranded-player shape. SightRange zero keeps the occupying monster out of
// Alice's audience; legality still comes from full encounter state.
func (s *MovementOccupancySuite) TestPlayerMove_HiddenOccupiedDestinationStopsWithoutIdentityLeak() {
	s.addPlayer(0)
	hidden := core.Hex{Q: 2, R: 0, S: -2}
	safe := core.Hex{Q: 1, R: 0, S: -1}
	s.addMonster("hidden-goblin", hidden)
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond)

	s.Require().NoError(s.enc.Move("alice", []core.Hex{safe, hidden}))

	data := s.enc.ToData()
	s.Equal(safe, data.Players["alice"].View.Position)
	s.Equal(hidden, data.Monsters["hidden-goblin"].Position)

	emitted := collectEventsTyped(s.aliceSub, 500*time.Millisecond)
	var move *events.MoveEvent
	for _, event := range emitted {
		switch event := event.(type) {
		case *events.MoveEvent:
			if event.Mover == "char-alice" {
				move = event
			}
		case *events.EntityAppearedEvent:
			s.NotEqual(core.EntityID("hidden-goblin"), event.Entity,
				"occupancy refusal must not reveal the hidden occupant")
		}
	}
	s.Require().NotNil(move)
	s.Equal([]core.Hex{safe}, move.Path)
	s.NotContains(move.PerPlayer, core.PlayerID("hidden-goblin"))
	s.T().Logf("occupancy trace: mover=char-alice proposed=%v actual=%v final=%v "+
		"occupant_visible_to_alice=false emitted_events=%d",
		[]core.Hex{safe, hidden}, move.Path, data.Players["alice"].View.Position, len(emitted))
}

// TestPlayerMove_CanPassThroughOccupiedHex pins the requested scope: creature
// occupancy constrains the final space, not every crossed space.
func (s *MovementOccupancySuite) TestPlayerMove_CanPassThroughOccupiedHex() {
	s.addPlayer(10)
	occupied := core.Hex{Q: 1, R: 0, S: -1}
	destination := core.Hex{Q: 2, R: 0, S: -2}
	s.addMonster("goblin", occupied)
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond)

	s.Require().NoError(s.enc.Move("alice", []core.Hex{occupied, destination}))
	s.Equal(destination, s.enc.ToData().Players["alice"].View.Position)

	move := s.moveEventFor("char-alice")
	s.Require().NotNil(move)
	s.Equal([]core.Hex{occupied, destination}, move.Path)
}

// TestPlayerMove_OccupiedFirstDestinationIsViewerSafeNoOp covers the case with
// no legal prefix: no state or event claims movement occurred.
func (s *MovementOccupancySuite) TestPlayerMove_OccupiedFirstDestinationIsViewerSafeNoOp() {
	s.addPlayer(10)
	occupied := core.Hex{Q: 1, R: 0, S: -1}
	s.addMonster("goblin", occupied)
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond)

	s.Require().NoError(s.enc.Move("alice", []core.Hex{occupied}))
	s.Equal(core.Hex{}, s.enc.ToData().Players["alice"].View.Position)
	s.Nil(s.moveEventFor("char-alice"), "a refused move must not emit a MoveEvent")
}
