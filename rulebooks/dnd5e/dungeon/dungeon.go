package dungeon

import (
	"errors"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// Validation errors
var (
	// ErrNilData is returned when LoadFromData receives nil data
	ErrNilData = errors.New("dungeon data is nil")
	// ErrEmptyID is returned when the dungeon has no ID
	ErrEmptyID = errors.New("dungeon ID is required")
	// ErrNoStartRoom is returned when no start room is specified
	ErrNoStartRoom = errors.New("start room ID is required")
)

// Dungeon is the runtime representation with exploration logic.
// It wraps DungeonData and provides methods for gameplay.
type Dungeon struct {
	data *DungeonData

	// Cached lookups (built on load)
	passagesByID map[string]environments.PassageData
}

// LoadFromDataInput contains parameters for loading a dungeon from persisted data.
type LoadFromDataInput struct {
	// Data is the persisted dungeon data to load
	Data *DungeonData
}

// LoadFromDataOutput contains the result of loading a dungeon.
type LoadFromDataOutput struct {
	// Dungeon is the loaded runtime dungeon
	Dungeon *Dungeon
}

// LoadFromData creates a runtime Dungeon from persisted DungeonData.
// Returns an error if the data is invalid (nil, missing ID, etc.).
func LoadFromData(input *LoadFromDataInput) (*LoadFromDataOutput, error) {
	if input == nil || input.Data == nil {
		return nil, ErrNilData
	}

	data := input.Data

	// Validate required fields
	if data.Environment.ID == "" {
		return nil, ErrEmptyID
	}
	if data.StartRoomID == "" {
		return nil, ErrNoStartRoom
	}

	d := &Dungeon{
		data:         data,
		passagesByID: make(map[string]environments.PassageData),
	}
	d.buildCaches()

	return &LoadFromDataOutput{Dungeon: d}, nil
}

// buildCaches populates lookup maps for efficient queries.
func (d *Dungeon) buildCaches() {
	for _, p := range d.data.Environment.Passages {
		d.passagesByID[p.ID] = p
	}
}

// ID returns the dungeon's unique identifier.
func (d *Dungeon) ID() string {
	return d.data.Environment.ID
}

// State returns the current dungeon lifecycle state.
func (d *Dungeon) State() DungeonState {
	return d.data.State
}

// StartRoom returns the ID of the entrance room.
func (d *Dungeon) StartRoom() string {
	return d.data.StartRoomID
}

// BossRoom returns the ID of the boss room.
func (d *Dungeon) BossRoom() string {
	return d.data.BossRoomID
}

// Seed returns the random seed used for generation.
func (d *Dungeon) Seed() int64 {
	return d.data.Seed
}

// Room returns the room data for the given ID, or nil if not found.
// The returned pointer references the actual data - modifications persist.
func (d *Dungeon) Room(roomID string) *RoomData {
	if d.data.Rooms == nil {
		return nil
	}
	return d.data.Rooms[roomID]
}

// CurrentRoom returns the room players are currently in.
func (d *Dungeon) CurrentRoom() *RoomData {
	return d.Room(d.data.CurrentRoomID)
}

// CurrentRoomID returns the ID of the current room.
func (d *Dungeon) CurrentRoomID() string {
	return d.data.CurrentRoomID
}

// RoomRevealed returns whether a room has been revealed to players.
func (d *Dungeon) RoomRevealed(roomID string) bool {
	if d.data.RevealedRooms == nil {
		return false
	}
	return d.data.RevealedRooms[roomID]
}

// DoorOpen returns whether a door/connection has been opened.
func (d *Dungeon) DoorOpen(connectionID string) bool {
	if d.data.OpenDoors == nil {
		return false
	}
	return d.data.OpenDoors[connectionID]
}

// IsBossRoom returns true if the given room ID is the boss room.
func (d *Dungeon) IsBossRoom(roomID string) bool {
	return d.data.BossRoomID != "" && d.data.BossRoomID == roomID
}

// Doors returns all passages in the dungeon.
func (d *Dungeon) Doors() []environments.PassageData {
	return d.data.Environment.Passages
}

// DoorsFromRoom returns all connections from the specified room.
func (d *Dungeon) DoorsFromRoom(roomID string) []environments.PassageData {
	var result []environments.PassageData
	for _, p := range d.data.Environment.Passages {
		if p.FromZoneID == roomID || p.ToZoneID == roomID {
			result = append(result, p)
		}
	}
	return result
}

// VisibleDoors returns connections from the current room that lead to unrevealed rooms.
func (d *Dungeon) VisibleDoors() []environments.PassageData {
	connections := d.DoorsFromRoom(d.data.CurrentRoomID)
	var visible []environments.PassageData
	for _, conn := range connections {
		// Door is visible if the room it leads to hasn't been revealed yet
		targetRoomID := conn.ToZoneID
		if conn.ToZoneID == d.data.CurrentRoomID {
			targetRoomID = conn.FromZoneID
		}
		if !d.RoomRevealed(targetRoomID) {
			visible = append(visible, conn)
		}
	}
	return visible
}

// RevealRoom marks a room as revealed.
func (d *Dungeon) RevealRoom(roomID string) {
	if d.data.RevealedRooms == nil {
		d.data.RevealedRooms = make(map[string]bool)
	}
	d.data.RevealedRooms[roomID] = true
}

// OpenDoor marks a connection as open.
func (d *Dungeon) OpenDoor(connectionID string) {
	if d.data.OpenDoors == nil {
		d.data.OpenDoors = make(map[string]bool)
	}
	d.data.OpenDoors[connectionID] = true
}

// SetCurrentRoom changes the current room.
func (d *Dungeon) SetCurrentRoom(roomID string) {
	d.data.CurrentRoomID = roomID
}

// IncrementRoomsCleared increases the rooms cleared counter.
func (d *Dungeon) IncrementRoomsCleared() {
	d.data.RoomsCleared++
}

// IncrementMonstersKilled increases the monsters killed counter.
func (d *Dungeon) IncrementMonstersKilled(count int) {
	d.data.MonstersKilled += count
}

// RoomsCleared returns the number of rooms cleared.
func (d *Dungeon) RoomsCleared() int {
	return d.data.RoomsCleared
}

// MonstersKilled returns the number of monsters killed.
func (d *Dungeon) MonstersKilled() int {
	return d.data.MonstersKilled
}

// MarkVictory sets the dungeon state to victorious and records completion time.
func (d *Dungeon) MarkVictory() {
	d.data.State = StateVictorious
	now := time.Now()
	d.data.CompletedAt = &now
}

// MarkFailed sets the dungeon state to failed and records completion time.
func (d *Dungeon) MarkFailed() {
	d.data.State = StateFailed
	now := time.Now()
	d.data.CompletedAt = &now
}

// MarkAbandoned sets the dungeon state to abandoned and records completion time.
func (d *Dungeon) MarkAbandoned() {
	d.data.State = StateAbandoned
	now := time.Now()
	d.data.CompletedAt = &now
}

// ToData returns the underlying data for persistence.
func (d *Dungeon) ToData() *DungeonData {
	return d.data
}

// CreatedAt returns when the dungeon was created.
func (d *Dungeon) CreatedAt() time.Time {
	return d.data.CreatedAt
}

// CompletedAt returns when the dungeon was completed, or nil if still active.
func (d *Dungeon) CompletedAt() *time.Time {
	return d.data.CompletedAt
}

// Environment returns the underlying environment data.
func (d *Dungeon) Environment() *environments.EnvironmentData {
	return &d.data.Environment
}

// Rooms returns all room data.
// The returned map references the actual data - modifications persist.
func (d *Dungeon) Rooms() map[string]*RoomData {
	return d.data.Rooms
}

// RoomIDs returns the IDs of all rooms in the dungeon.
func (d *Dungeon) RoomIDs() []string {
	ids := make([]string, 0, len(d.data.Rooms))
	for id := range d.data.Rooms {
		ids = append(ids, id)
	}
	return ids
}
