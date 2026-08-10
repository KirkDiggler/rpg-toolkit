package spatial_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ManagedOrchestratorSuite struct {
	suite.Suite
}

func TestManagedOrchestratorSuite(t *testing.T) {
	suite.Run(t, new(ManagedOrchestratorSuite))
}

func (s *ManagedOrchestratorSuite) TestManagedMembershipWithoutBus() {
	orchestrator, roomA, roomB := newManagedField()
	s.Require().NoError(orchestrator.AddRoom(roomA))
	s.Require().NoError(orchestrator.AddRoom(roomB))
	s.Require().NoError(orchestrator.AddConnection(spatial.CreateDoorConnection(
		"door-ab", "room-a", "room-b", 1,
	)))

	hero := NewMockEntity("hero", "character")
	placed, err := orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-a", Entity: hero, Position: spatial.Position{X: 1, Y: 1},
	})
	s.Require().NoError(err)
	s.Equal(spatial.EntityPlacementDelta{
		EntityID: "hero", RoomID: "room-a", Position: spatial.Position{X: 1, Y: 1},
	}, placed.Delta)
	roomID, ok := orchestrator.GetEntityRoom("hero")
	s.True(ok)
	s.Equal("room-a", roomID)
	s.True(orchestrator.CanMoveEntityBetweenRooms("hero", "room-a", "room-b", "door-ab"))

	moved, err := orchestrator.MoveEntity(&spatial.MoveEntityInput{
		RoomID: "room-a", EntityID: "hero", To: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
	s.Equal(spatial.EntityMovementDelta{
		EntityID: "hero", RoomID: "room-a",
		From: spatial.Position{X: 1, Y: 1}, To: spatial.Position{X: 2, Y: 1},
	}, moved.Delta)

	removed, err := orchestrator.RemoveEntity(&spatial.RemoveEntityInput{
		RoomID: "room-a", EntityID: "hero",
	})
	s.Require().NoError(err)
	s.Equal(spatial.EntityRemovalDelta{
		EntityID: "hero", RoomID: "room-a", Position: spatial.Position{X: 2, Y: 1},
	}, removed.Delta)
	s.Same(hero, removed.Entity)
	_, ok = orchestrator.GetEntityRoom("hero")
	s.False(ok)
	s.False(orchestrator.CanMoveEntityBetweenRooms("hero", "room-a", "room-b", "door-ab"))

	_, err = orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-a", Entity: hero, Position: spatial.Position{X: 3, Y: 1},
	})
	s.Require().NoError(err)
	transitioned, err := orchestrator.TransitionEntity(&spatial.TransitionEntityInput{
		EntityID: "hero", FromRoom: "room-a", ToRoom: "room-b", ConnectionID: "door-ab",
	})
	s.Require().NoError(err)
	s.Equal(spatial.EntityRemovalDelta{
		EntityID: "hero", RoomID: "room-a", Position: spatial.Position{X: 3, Y: 1},
	}, transitioned.Departure)
	s.Same(hero, transitioned.Entity)
	s.Equal(spatial.EntityTransitionDelta{
		EntityID: "hero", FromRoom: "room-a", ToRoom: "room-b",
		ConnectionID: "door-ab", PlacementRequired: true,
	}, transitioned.Transition)
	_, ok = orchestrator.GetEntityRoom("hero")
	s.False(ok, "a logical transition without a position must leave the entity honestly unplaced")
	s.Empty(roomA.GetAllEntities())
	s.Empty(roomB.GetAllEntities())
	s.False(orchestrator.CanMoveEntityBetweenRooms("hero", "room-b", "room-a", "door-ab"))

	_, err = orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-b", Entity: hero, Position: spatial.Position{X: 4, Y: 1},
	})
	s.Require().NoError(err)
	roomID, ok = orchestrator.GetEntityRoom("hero")
	s.True(ok)
	s.Equal("room-b", roomID)
	s.True(orchestrator.CanMoveEntityBetweenRooms("hero", "room-b", "room-a", "door-ab"))
}

func (s *ManagedOrchestratorSuite) TestBusWiringDoesNotAffectManagedMembership() {
	s.Run("bus connected after rooms", func() {
		orchestrator, roomA, roomB := newManagedField()
		s.Require().NoError(orchestrator.AddRoom(roomA))
		s.Require().NoError(orchestrator.AddRoom(roomB))
		orchestrator.ConnectToEventBus(events.NewEventBus())
		_, err := orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
			RoomID: "room-a", Entity: NewMockEntity("late-bus", "character"),
			Position: spatial.Position{X: 1, Y: 1},
		})
		s.Require().NoError(err)
		roomID, ok := orchestrator.GetEntityRoom("late-bus")
		s.True(ok)
		s.Equal("room-a", roomID)
	})

	s.Run("rooms and orchestrator use different buses", func() {
		orchestrator, roomA, roomB := newManagedField()
		roomA.ConnectToEventBus(events.NewEventBus())
		roomB.ConnectToEventBus(events.NewEventBus())
		orchestrator.ConnectToEventBus(events.NewEventBus())
		s.Require().NoError(orchestrator.AddRoom(roomA))
		s.Require().NoError(orchestrator.AddRoom(roomB))
		_, err := orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
			RoomID: "room-b", Entity: NewMockEntity("split-bus", "character"),
			Position: spatial.Position{X: 1, Y: 1},
		})
		s.Require().NoError(err)
		roomID, ok := orchestrator.GetEntityRoom("split-bus")
		s.True(ok)
		s.Equal("room-b", roomID)
	})
}

func (s *ManagedOrchestratorSuite) TestAddRoomIndexesExistingEntities() {
	orchestrator, roomA, _ := newManagedField()
	s.Require().NoError(roomA.PlaceEntity(
		NewMockEntity("already-there", "character"), spatial.Position{X: 1, Y: 1},
	))
	s.Require().NoError(orchestrator.AddRoom(roomA))
	roomID, ok := orchestrator.GetEntityRoom("already-there")
	s.True(ok)
	s.Equal("room-a", roomID)
}

func newManagedField() (*spatial.BasicRoomOrchestrator, *spatial.BasicRoom, *spatial.BasicRoom) {
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID: "field", Type: "orchestrator",
	})
	room := func(id string) *spatial.BasicRoom {
		return spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID: id, Type: "chamber",
			Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
		})
	}
	return orchestrator, room("room-a"), room("room-b")
}

var _ spatial.ManagedRoomMutator = (*spatial.BasicRoomOrchestrator)(nil)
var _ core.Entity = (*MockEntity)(nil)

func (s *ManagedOrchestratorSuite) TestAddRoomRejectsDuplicateEntityWithoutPartialMutation() {
	orchestrator, roomA, roomB := newManagedField()
	s.Require().NoError(roomA.PlaceEntity(
		NewMockEntity("duplicate", "character"), spatial.Position{X: 1, Y: 1},
	))
	s.Require().NoError(roomB.PlaceEntity(
		NewMockEntity("duplicate", "monster"), spatial.Position{X: 2, Y: 2},
	))
	s.Require().NoError(orchestrator.AddRoom(roomA))

	err := orchestrator.AddRoom(roomB)
	s.Require().Error(err)
	_, added := orchestrator.GetRoom("room-b")
	s.False(added, "failed AddRoom must not partially add the room")
	roomID, indexed := orchestrator.GetEntityRoom("duplicate")
	s.True(indexed)
	s.Equal("room-a", roomID)
}

func (s *ManagedOrchestratorSuite) TestManagedEventsRemainObserverOnlyAndCausal() {
	bus := events.NewEventBus()
	orchestrator, roomA, roomB := newManagedField()
	orchestrator.ConnectToEventBus(bus)
	roomA.ConnectToEventBus(bus)
	roomB.ConnectToEventBus(bus)
	s.Require().NoError(orchestrator.AddRoom(roomA))
	s.Require().NoError(orchestrator.AddRoom(roomB))
	s.Require().NoError(orchestrator.AddConnection(spatial.CreateDoorConnection(
		"door-ab", "room-a", "room-b", 1,
	)))

	var order []string
	var placed []spatial.EntityPlacedEvent
	var moved []spatial.EntityMovedEvent
	var removed []spatial.EntityRemovedEvent
	var transitioned []spatial.EntityRoomTransitionEvent
	_, err := spatial.EntityPlacedTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, event spatial.EntityPlacedEvent,
	) error {
		_, _ = orchestrator.GetEntityRoom(event.EntityID) // synchronous re-entrant read must not deadlock
		order = append(order, "placed")
		placed = append(placed, event)
		return nil
	})
	s.Require().NoError(err)
	_, err = spatial.EntityMovedTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, event spatial.EntityMovedEvent,
	) error {
		_, _ = orchestrator.GetEntityRoom(event.EntityID)
		order = append(order, "moved")
		moved = append(moved, event)
		return nil
	})
	s.Require().NoError(err)
	_, err = spatial.EntityRemovedTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, event spatial.EntityRemovedEvent,
	) error {
		_, _ = orchestrator.GetEntityRoom(event.EntityID)
		order = append(order, "removed")
		removed = append(removed, event)
		return nil
	})
	s.Require().NoError(err)
	_, err = spatial.EntityRoomTransitionTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, event spatial.EntityRoomTransitionEvent,
	) error {
		_, _ = orchestrator.GetEntityRoom(event.EntityID)
		order = append(order, "transition")
		transitioned = append(transitioned, event)
		return nil
	})
	s.Require().NoError(err)

	hero := NewMockEntity("observer-hero", "character")
	_, err = orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-a", Entity: hero, Position: spatial.Position{X: 1, Y: 1},
	})
	s.Require().NoError(err)
	_, err = orchestrator.MoveEntity(&spatial.MoveEntityInput{
		RoomID: "room-a", EntityID: "observer-hero", To: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NoError(orchestrator.MoveEntityBetweenRooms(
		"observer-hero", "room-a", "room-b", "door-ab",
	))
	removedHero := NewMockEntity("removed-hero", "character")
	_, err = orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-a", Entity: removedHero, Position: spatial.Position{X: 3, Y: 1},
	})
	s.Require().NoError(err)
	_, err = orchestrator.RemoveEntity(&spatial.RemoveEntityInput{
		RoomID: "room-a", EntityID: "removed-hero",
	})
	s.Require().NoError(err)

	s.Equal([]string{"placed", "moved", "removed", "transition", "placed", "removed"}, order)
	s.Equal([]spatial.EntityPlacedEvent{
		{
			EntityID: "observer-hero", Position: spatial.Position{X: 1, Y: 1},
			RoomID: "room-a", GridType: "square",
		},
		{
			EntityID: "removed-hero", Position: spatial.Position{X: 3, Y: 1},
			RoomID: "room-a", GridType: "square",
		},
	}, placed)
	s.Equal([]spatial.EntityMovedEvent{{
		EntityID:     "observer-hero",
		FromPosition: spatial.Position{X: 1, Y: 1}, ToPosition: spatial.Position{X: 2, Y: 1},
		RoomID: "room-a", MovementType: "normal",
	}}, moved)
	s.Equal([]spatial.EntityRemovedEvent{
		{
			EntityID: "observer-hero", Position: spatial.Position{X: 2, Y: 1},
			RoomID: "room-a", RemovalType: "normal",
		},
		{
			EntityID: "removed-hero", Position: spatial.Position{X: 3, Y: 1},
			RoomID: "room-a", RemovalType: "normal",
		},
	}, removed)
	s.Require().Len(transitioned, 1)
	s.Equal("observer-hero", transitioned[0].EntityID)
	s.Equal("room-a", transitioned[0].FromRoom)
	s.Equal("room-b", transitioned[0].ToRoom)
	s.Equal("connection:door-ab", transitioned[0].Reason)
	s.False(transitioned[0].Timestamp.IsZero())
}

func (s *ManagedOrchestratorSuite) TestObserverFailureAndValidationFailureDoNotControlMutation() {
	bus := events.NewEventBus()
	orchestrator, roomA, roomB := newManagedField()
	orchestrator.ConnectToEventBus(bus)
	roomA.ConnectToEventBus(bus)
	s.Require().NoError(orchestrator.AddRoom(roomA))
	s.Require().NoError(orchestrator.AddRoom(roomB))
	s.Require().NoError(orchestrator.AddConnection(spatial.CreateDoorConnection(
		"door-ab", "room-a", "room-b", 1,
	)))
	var placed, moved, removed, transitioned int
	_, err := spatial.EntityPlacedTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, _ spatial.EntityPlacedEvent,
	) error {
		placed++
		return errors.New("observer failed")
	})
	s.Require().NoError(err)
	_, err = spatial.EntityMovedTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, _ spatial.EntityMovedEvent,
	) error {
		moved++
		return nil
	})
	s.Require().NoError(err)
	_, err = spatial.EntityRemovedTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, _ spatial.EntityRemovedEvent,
	) error {
		removed++
		return nil
	})
	s.Require().NoError(err)
	_, err = spatial.EntityRoomTransitionTopic.On(bus).Subscribe(context.Background(), func(
		_ context.Context, _ spatial.EntityRoomTransitionEvent,
	) error {
		transitioned++
		return nil
	})
	s.Require().NoError(err)

	_, err = orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "missing", Entity: NewMockEntity("rejected", "character"),
		Position: spatial.Position{X: 1, Y: 1},
	})
	s.Require().Error(err)
	s.Zero(placed)

	hero := NewMockEntity("fallible-observer", "character")
	out, err := orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-a", Entity: hero, Position: spatial.Position{X: 1, Y: 1},
	})
	s.Require().NoError(err, "subscriber errors are observer-tail failures, not mutation results")
	s.Equal(core.EntityID("fallible-observer"), out.Delta.EntityID)
	roomID, indexed := orchestrator.GetEntityRoom("fallible-observer")
	s.True(indexed)
	s.Equal("room-a", roomID)

	_, err = orchestrator.MoveEntity(&spatial.MoveEntityInput{
		RoomID: "room-a", EntityID: "fallible-observer", To: spatial.Position{X: 99, Y: 99},
	})
	s.Require().Error(err)
	_, err = orchestrator.RemoveEntity(&spatial.RemoveEntityInput{
		RoomID: "room-b", EntityID: "fallible-observer",
	})
	s.Require().Error(err)
	_, err = orchestrator.TransitionEntity(&spatial.TransitionEntityInput{
		EntityID: "fallible-observer", FromRoom: "room-a", ToRoom: "room-b", ConnectionID: "missing",
	})
	s.Require().Error(err)
	position, exists := roomA.GetEntityPosition("fallible-observer")
	s.True(exists)
	s.Equal(spatial.Position{X: 1, Y: 1}, position)
	s.Equal(1, placed)
	s.Zero(moved)
	s.Zero(removed)
	s.Zero(transitioned)
}

func (s *ManagedOrchestratorSuite) TestEntityNotificationsAreObserverOnlyAndRaceFree() {
	bus := events.NewEventBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:   "field",
		Type: "orchestrator",
	})
	orchestrator.ConnectToEventBus(bus)
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-a",
		Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	s.Require().NoError(orchestrator.AddRoom(room))

	topic := spatial.EntityPlacedTopic.On(bus)
	const workers = 8
	const publications = 500
	var wg sync.WaitGroup
	errCh := make(chan error, workers*publications)
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			defer wg.Done()
			for n := 0; n < publications; n++ {
				entityID := fmt.Sprintf("observer-event-%d-%d", worker, n)
				if err := topic.Publish(context.Background(), spatial.EntityPlacedEvent{
					EntityID: entityID,
					RoomID:   "room-a",
				}); err != nil {
					errCh <- err
				}
				_, _ = orchestrator.GetEntityRoom(entityID)
			}
		}(worker)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		s.Require().NoError(err)
	}

	_, tracked := orchestrator.GetEntityRoom("observer-event-0-0")
	s.False(tracked, "observer notifications must not mutate the orchestrator index")
}
