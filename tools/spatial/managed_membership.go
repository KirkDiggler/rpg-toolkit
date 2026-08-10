package spatial

import (
	"context"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ManagedRoomMutator is the supported mutation seam for entity membership in
// rooms owned by a BasicRoomOrchestrator. Hosts serialize mutating calls; read
// methods remain safe to call concurrently.
type ManagedRoomMutator interface {
	PlaceEntity(in *PlaceEntityInput) (*PlaceEntityOutput, error)
	MoveEntity(in *MoveEntityInput) (*MoveEntityOutput, error)
	RemoveEntity(in *RemoveEntityInput) (*RemoveEntityOutput, error)
	TransitionEntity(in *TransitionEntityInput) (*TransitionEntityOutput, error)
}

// EntityPlacementDelta describes a completed placement in a managed room.
type EntityPlacementDelta struct {
	EntityID core.EntityID
	RoomID   RoomID
	Position Position
}

// EntityMovementDelta describes a completed move within one managed room.
type EntityMovementDelta struct {
	EntityID core.EntityID
	RoomID   RoomID
	From     Position
	To       Position
}

// EntityRemovalDelta describes a completed removal from a managed room.
type EntityRemovalDelta struct {
	EntityID core.EntityID
	RoomID   RoomID
	Position Position
}

// EntityTransitionDelta describes a logical room transition after departure.
// PlacementRequired is true because TransitionEntity does not choose a
// destination position and therefore does not claim destination membership.
type EntityTransitionDelta struct {
	EntityID          core.EntityID
	FromRoom          RoomID
	ToRoom            RoomID
	ConnectionID      ConnectionID
	PlacementRequired bool
}

// PlaceEntityInput names an entity, managed room, and destination position.
type PlaceEntityInput struct {
	RoomID   RoomID
	Entity   core.Entity
	Position Position
}

// PlaceEntityOutput returns the completed spatial placement as a value.
type PlaceEntityOutput struct {
	Delta EntityPlacementDelta
}

// MoveEntityInput names a managed entity and its new in-room position.
type MoveEntityInput struct {
	RoomID   RoomID
	EntityID core.EntityID
	To       Position
}

// MoveEntityOutput returns the completed spatial movement as a value.
type MoveEntityOutput struct {
	Delta EntityMovementDelta
}

// RemoveEntityInput names an entity to remove from a managed room.
type RemoveEntityInput struct {
	RoomID   RoomID
	EntityID core.EntityID
}

// RemoveEntityOutput returns the removed entity and completed spatial delta.
type RemoveEntityOutput struct {
	Entity core.Entity
	Delta  EntityRemovalDelta
}

// TransitionEntityInput names a logical transition through a connection.
type TransitionEntityInput struct {
	EntityID     core.EntityID
	FromRoom     RoomID
	ToRoom       RoomID
	ConnectionID ConnectionID
}

// TransitionEntityOutput returns the removed entity, its departure, and the
// logical transition that a composition must finish with PlaceEntity.
type TransitionEntityOutput struct {
	Entity     core.Entity
	Departure  EntityRemovalDelta
	Transition EntityTransitionDelta
}

// PlaceEntity places an entity in a managed room and synchronously indexes its
// membership. Room publication remains an observer tail and is not consumed by
// the orchestrator.
func (bro *BasicRoomOrchestrator) PlaceEntity(in *PlaceEntityInput) (*PlaceEntityOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("place entity: input cannot be nil")
	}
	if in.Entity == nil {
		return nil, fmt.Errorf("place entity: entity cannot be nil")
	}

	entityID := core.EntityID(in.Entity.GetID())
	bro.mu.RLock()
	room, roomExists := bro.rooms[in.RoomID]
	currentRoom, indexed := bro.entityRooms[EntityID(entityID)]
	bro.mu.RUnlock()
	if !roomExists {
		return nil, fmt.Errorf("place entity: room %s not found", in.RoomID)
	}
	if indexed {
		return nil, fmt.Errorf("place entity: entity %s is already indexed in room %s", entityID, currentRoom)
	}

	if err := room.PlaceEntity(in.Entity, in.Position); err != nil {
		return nil, fmt.Errorf("place entity in room %s: %w", in.RoomID, err)
	}

	bro.mu.Lock()
	bro.entityRooms[EntityID(entityID)] = in.RoomID
	bro.mu.Unlock()

	return &PlaceEntityOutput{Delta: EntityPlacementDelta{
		EntityID: entityID,
		RoomID:   in.RoomID,
		Position: in.Position,
	}}, nil
}

// MoveEntity moves an indexed entity within its managed room and returns the
// completed movement. The index remains unchanged.
func (bro *BasicRoomOrchestrator) MoveEntity(in *MoveEntityInput) (*MoveEntityOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("move entity: input cannot be nil")
	}

	room, err := bro.indexedRoom(in.RoomID, in.EntityID)
	if err != nil {
		return nil, fmt.Errorf("move entity: %w", err)
	}
	from, exists := room.GetEntityPosition(string(in.EntityID))
	if !exists {
		return nil, fmt.Errorf("move entity: entity %s has no position in room %s", in.EntityID, in.RoomID)
	}
	if err := room.MoveEntity(string(in.EntityID), in.To); err != nil {
		return nil, fmt.Errorf("move entity in room %s: %w", in.RoomID, err)
	}

	return &MoveEntityOutput{Delta: EntityMovementDelta{
		EntityID: in.EntityID,
		RoomID:   in.RoomID,
		From:     from,
		To:       in.To,
	}}, nil
}

// RemoveEntity removes an indexed entity from its managed room, synchronously
// clears membership, and returns both the entity and spatial removal.
func (bro *BasicRoomOrchestrator) RemoveEntity(in *RemoveEntityInput) (*RemoveEntityOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("remove entity: input cannot be nil")
	}

	room, err := bro.indexedRoom(in.RoomID, in.EntityID)
	if err != nil {
		return nil, fmt.Errorf("remove entity: %w", err)
	}
	entity, position, err := entityAndPosition(room, in.EntityID)
	if err != nil {
		return nil, fmt.Errorf("remove entity: %w", err)
	}
	if err := room.RemoveEntity(string(in.EntityID)); err != nil {
		return nil, fmt.Errorf("remove entity from room %s: %w", in.RoomID, err)
	}

	bro.mu.Lock()
	delete(bro.entityRooms, EntityID(in.EntityID))
	bro.mu.Unlock()

	return &RemoveEntityOutput{
		Entity: entity,
		Delta: EntityRemovalDelta{
			EntityID: in.EntityID,
			RoomID:   in.RoomID,
			Position: position,
		},
	}, nil
}

// TransitionEntity removes an entity from its source room through a passable
// connection and leaves it deliberately unplaced and unindexed. The returned
// entity and transition value let a composition choose a destination position
// and finish the operation through PlaceEntity.
func (bro *BasicRoomOrchestrator) TransitionEntity(
	in *TransitionEntityInput,
) (*TransitionEntityOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("transition entity: input cannot be nil")
	}

	room, connection, entity, position, ok := bro.transitionSource(
		in.EntityID, in.FromRoom, in.ToRoom, in.ConnectionID,
	)
	if !ok {
		return nil, fmt.Errorf(
			"cannot transition entity %s from %s to %s via %s",
			in.EntityID, in.FromRoom, in.ToRoom, in.ConnectionID,
		)
	}
	if err := room.RemoveEntity(string(in.EntityID)); err != nil {
		return nil, fmt.Errorf("remove entity from source room: %w", err)
	}

	bro.mu.Lock()
	delete(bro.entityRooms, EntityID(in.EntityID))
	topic := bro.entityRoomTransition
	bro.mu.Unlock()

	if topic != nil {
		_ = topic.Publish(context.Background(), EntityRoomTransitionEvent{
			EntityID:  string(in.EntityID),
			FromRoom:  in.FromRoom.String(),
			ToRoom:    in.ToRoom.String(),
			Reason:    fmt.Sprintf("connection:%s", connection.GetID()),
			Timestamp: time.Now(),
		})
	}

	return &TransitionEntityOutput{
		Entity: entity,
		Departure: EntityRemovalDelta{
			EntityID: in.EntityID,
			RoomID:   in.FromRoom,
			Position: position,
		},
		Transition: EntityTransitionDelta{
			EntityID:          in.EntityID,
			FromRoom:          in.FromRoom,
			ToRoom:            in.ToRoom,
			ConnectionID:      in.ConnectionID,
			PlacementRequired: true,
		},
	}, nil
}

func (bro *BasicRoomOrchestrator) indexedRoom(roomID RoomID, entityID core.EntityID) (Room, error) {
	bro.mu.RLock()
	indexedRoom, indexed := bro.entityRooms[EntityID(entityID)]
	room, roomExists := bro.rooms[roomID]
	bro.mu.RUnlock()
	if !roomExists {
		return nil, fmt.Errorf("room %s not found", roomID)
	}
	if !indexed || indexedRoom != roomID {
		return nil, fmt.Errorf("entity %s is not indexed in room %s", entityID, roomID)
	}
	return room, nil
}

func entityAndPosition(room Room, entityID core.EntityID) (core.Entity, Position, error) {
	entities := room.GetAllEntities()
	entity, exists := entities[string(entityID)]
	if !exists {
		return nil, Position{}, fmt.Errorf("entity %s not found in room %s", entityID, room.GetID())
	}
	position, exists := room.GetEntityPosition(string(entityID))
	if !exists {
		return nil, Position{}, fmt.Errorf("entity %s has no position in room %s", entityID, room.GetID())
	}
	return entity, position, nil
}

func (bro *BasicRoomOrchestrator) transitionSource(
	entityID core.EntityID,
	fromRoom RoomID,
	toRoom RoomID,
	connectionID ConnectionID,
) (Room, Connection, core.Entity, Position, bool) {
	bro.mu.RLock()
	indexedRoom, indexed := bro.entityRooms[EntityID(entityID)]
	room, sourceExists := bro.rooms[fromRoom]
	_, destinationExists := bro.rooms[toRoom]
	connection, connectionExists := bro.connections[connectionID]
	bro.mu.RUnlock()

	if !indexed || indexedRoom != fromRoom || !sourceExists || !destinationExists || !connectionExists {
		return nil, nil, nil, Position{}, false
	}
	forward := connection.GetFromRoom() == fromRoom.String() && connection.GetToRoom() == toRoom.String()
	reverse := connection.IsReversible() && connection.GetFromRoom() == toRoom.String() &&
		connection.GetToRoom() == fromRoom.String()
	if !forward && !reverse {
		return nil, nil, nil, Position{}, false
	}
	entity, position, err := entityAndPosition(room, entityID)
	if err != nil || !connection.IsPassable(entity) {
		return nil, nil, nil, Position{}, false
	}
	return room, connection, entity, position, true
}
