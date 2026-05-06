package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
)

// Encounter is the transient SDK object for one ongoing encounter.
// Constructed per-call via LoadFromData; mutated by verbs; serialized via
// ToData and saved.
type Encounter struct {
	data   *Data
	broker *Broker
}

// PlayerInput populates a player seat at construction / AddPlayer time.
type PlayerInput struct {
	PlayerID   core.PlayerID
	EntityID   core.EntityID
	Position   core.Hex
	SightRange int
}

// New constructs a fresh encounter with the given ID.
func New(id core.EncounterID, b *Broker) *Encounter {
	return &Encounter{
		data:   NewData(id),
		broker: b,
	}
}

// LoadFromData rehydrates an encounter from persisted state.
func LoadFromData(data *Data, b *Broker) (*Encounter, error) {
	if data == nil {
		return nil, errors.New("nil Data")
	}
	if data.Players == nil {
		data.Players = make(map[core.PlayerID]*PlayerData)
	}
	if data.Doors == nil {
		data.Doors = make(map[core.EntityID]*DoorData)
	}
	return &Encounter{data: data, broker: b}, nil
}

// AddPlayer registers a new player seat with a fresh PerceptionView.
// The player sees their starting position and surrounding hexes immediately.
func (e *Encounter) AddPlayer(input PlayerInput) error {
	if _, exists := e.data.Players[input.PlayerID]; exists {
		return fmt.Errorf("player %q already in encounter", input.PlayerID)
	}
	view := perception.NewView(input.PlayerID, input.Position, input.SightRange)
	view.ApplyReveal(perception.VisibleHexesAt(input.Position, input.SightRange))

	e.data.Players[input.PlayerID] = &PlayerData{
		ID:       input.PlayerID,
		EntityID: input.EntityID,
		View:     view,
	}
	return nil
}

// AddDoor registers a door (slice scope; future slices use a richer entity
// system).
func (e *Encounter) AddDoor(id core.EntityID, position core.Hex, open bool) {
	e.data.Doors[id] = &DoorData{ID: id, Position: position, Open: open}
}

// ID returns the encounter's identifier.
func (e *Encounter) ID() core.EncounterID { return e.data.ID }

// SnapshotFor returns the read-only view a player's gRPC handler ships
// on connect/reconnect.
func (e *Encounter) SnapshotFor(playerID core.PlayerID) Snapshot {
	p, ok := e.data.Players[playerID]
	if !ok || p.View == nil {
		return Snapshot{}
	}
	revealed := make(core.HexSet, len(p.View.RevealedHexes))
	for h := range p.View.RevealedHexes {
		revealed[h] = struct{}{}
	}
	return Snapshot{
		PlayerID:      playerID,
		Position:      p.View.Position,
		RevealedHexes: revealed,
	}
}

// Snapshot is the slice-1 read-only view. Future slices add visible
// entities, turn state, action economy, etc.
type Snapshot struct {
	PlayerID      core.PlayerID
	Position      core.Hex
	RevealedHexes core.HexSet
}

// ToData returns the persisted shape. Caller saves this to the KV store.
func (e *Encounter) ToData() *Data { return e.data }

// nextSeq advances and returns the encounter's monotonic sequence counter.
// Used to stamp events on publish.
func (e *Encounter) nextSeq() uint64 {
	e.data.Sequence++
	return e.data.Sequence
}

// Move applies a move action by playerID along path. Validates, mutates
// player position, and publishes the cause event (MoveEvent) plus a
// HexRevealedEvent for any viewer whose vision grew.
//
// Slice scope: no action economy, no turn-order enforcement, no
// path-contiguity validation beyond non-empty.
func (e *Encounter) Move(playerID core.PlayerID, path []core.Hex) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}
	p, ok := e.data.Players[playerID]
	if !ok {
		return fmt.Errorf("player %q not in encounter", playerID)
	}

	// 1. Compute the mover's reveal delta BEFORE mutating position/view.
	//    The delta = (visible-from-new-position) MINUS (already-revealed).
	//    Critical: if we apply the reveal first, the diff is always empty.
	end := path[len(path)-1]
	newVisible := perception.VisibleHexesAt(end, p.View.SightRange)
	moverNewHexes := diffHexes(p.View.RevealedHexes, newVisible)

	// 2. Mutate state: position, then apply the reveal delta we just computed.
	p.View.Position = end
	p.View.ApplyReveal(moverNewHexes)

	// 3. Per-player projection.
	movePerPlayer := make(map[core.PlayerID]events.MovePlayerSlice)
	revealPerPlayer := make(map[core.PlayerID]events.HexRevealedSlice)

	// The mover always sees their own move; their reveal is the delta we
	// just computed.
	movePerPlayer[playerID] = events.MovePlayerSlice{
		SeenSegments: append([]core.Hex(nil), path...),
	}
	if len(moverNewHexes) > 0 {
		revealPerPlayer[playerID] = events.HexRevealedSlice{Hexes: moverNewHexes}
	}

	// Other players: project the move from their current view.
	for otherID, other := range e.data.Players {
		if otherID == playerID {
			continue
		}
		moveSlice, revealSlice := perception.ProjectMove(p.EntityID, path, other.View)
		if moveSlice != nil {
			movePerPlayer[otherID] = *moveSlice
		}
		if revealSlice != nil {
			if revealSlice.Hexes != nil {
				other.View.ApplyReveal(revealSlice.Hexes)
			}
			revealPerPlayer[otherID] = *revealSlice
		}
	}

	// 4. Publish — cause event always; effect event only when someone's
	//    vision changed. The two events get sequential sequence numbers.
	if err := e.broker.Publish(events.NewMoveEvent(
		e.data.ID, e.nextSeq(), p.EntityID, path, movePerPlayer,
	)); err != nil {
		return fmt.Errorf("publish move: %w", err)
	}
	if len(revealPerPlayer) > 0 {
		if err := e.broker.Publish(events.NewHexRevealedEvent(
			e.data.ID, e.nextSeq(), revealPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish reveal: %w", err)
		}
	}
	return nil
}

// OpenDoor applies an open-door action. Marks the door open and publishes
// the cause event (DoorOpenedEvent) plus a HexRevealedEvent for any viewer
// whose vision grew.
func (e *Encounter) OpenDoor(playerID core.PlayerID, doorID core.EntityID) error {
	p, ok := e.data.Players[playerID]
	if !ok {
		return fmt.Errorf("player %q not in encounter", playerID)
	}
	door, ok := e.data.Doors[doorID]
	if !ok {
		return fmt.Errorf("door %q not in encounter", doorID)
	}
	if door.Open {
		return fmt.Errorf("door %q already open", doorID)
	}

	door.Open = true

	doorPerPlayer := make(map[core.PlayerID]events.DoorOpenedPlayerSlice)
	revealPerPlayer := make(map[core.PlayerID]events.HexRevealedSlice)

	for viewerID, viewer := range e.data.Players {
		doorSlice, revealSlice := perception.ProjectDoorOpen(
			doorID, door.Position, p.EntityID, viewer.View,
		)
		if doorSlice != nil {
			doorPerPlayer[viewerID] = *doorSlice
		}
		if revealSlice != nil {
			if revealSlice.Hexes != nil {
				viewer.View.ApplyReveal(revealSlice.Hexes)
			}
			revealPerPlayer[viewerID] = *revealSlice
		}
	}

	if err := e.broker.Publish(events.NewDoorOpenedEvent(
		e.data.ID, e.nextSeq(), doorID, p.EntityID, doorPerPlayer,
	)); err != nil {
		return fmt.Errorf("publish door: %w", err)
	}
	if len(revealPerPlayer) > 0 {
		if err := e.broker.Publish(events.NewHexRevealedEvent(
			e.data.ID, e.nextSeq(), revealPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish reveal: %w", err)
		}
	}
	return nil
}

// diffHexes returns hexes in candidate that are not already in current.
func diffHexes(current, candidate core.HexSet) core.HexSet {
	out := make(core.HexSet)
	for h := range candidate {
		if !current.Has(h) {
			out[h] = struct{}{}
		}
	}
	return out
}
