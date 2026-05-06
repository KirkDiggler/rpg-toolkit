package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
)

// Data is the persisted shape of an Encounter. The orchestrator
// stores this in Redis (or any KV store) and rehydrates the live Encounter
// via LoadFromData.
//
// Slice 1 persists what's needed for Move and OpenDoor: identity, players
// (with position + perception view), doors, and a sequence counter.
// Future slices add: monsters, action economy, turn state, mode, conditions.
type Data struct {
	ID       core.EncounterID              `json:"id"`
	Sequence uint64                        `json:"sequence"`
	Players  map[core.PlayerID]*PlayerData `json:"players"`
	Doors    map[core.EntityID]*DoorData   `json:"doors"`
}

// PlayerData persists a single player seat.
type PlayerData struct {
	ID       core.PlayerID    `json:"id"`
	EntityID core.EntityID    `json:"entity_id"`
	View     *perception.View `json:"view"`
}

// DoorData persists a door entity.
type DoorData struct {
	ID       core.EntityID `json:"id"`
	Position core.Hex      `json:"position"`
	Open     bool          `json:"open"`
}

// NewData constructs an empty Data with a fresh ID.
func NewData(id core.EncounterID) *Data {
	return &Data{
		ID:      id,
		Players: make(map[core.PlayerID]*PlayerData),
		Doors:   make(map[core.EntityID]*DoorData),
	}
}
