package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
)

// EncounterData is the persisted shape of an Encounter. The orchestrator
// stores this in Redis (or any KV store) and rehydrates the live Encounter
// via LoadFromData.
//
// Slice 1 persists what's needed for Move and OpenDoor: identity, players
// (with position + perception view), doors, and a sequence counter.
// Future slices add: monsters, action economy, turn state, mode, conditions.
type EncounterData struct {
	ID       types.EncounterID              `json:"id"`
	Sequence uint64                         `json:"sequence"`
	Players  map[types.PlayerID]*PlayerData `json:"players"`
	Doors    map[types.EntityID]*DoorData   `json:"doors"`
}

// PlayerData persists a single player seat.
type PlayerData struct {
	ID       types.PlayerID             `json:"id"`
	EntityID types.EntityID             `json:"entity_id"`
	View     *perception.PerceptionView `json:"view"`
}

// DoorData persists a door entity.
type DoorData struct {
	ID       types.EntityID `json:"id"`
	Position types.Hex      `json:"position"`
	Open     bool           `json:"open"`
}

// NewEncounterData constructs an empty EncounterData with a fresh ID.
func NewEncounterData(id types.EncounterID) *EncounterData {
	return &EncounterData{
		ID:      id,
		Players: make(map[types.PlayerID]*PlayerData),
		Doors:   make(map[types.EntityID]*DoorData),
	}
}
