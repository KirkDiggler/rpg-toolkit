package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// KnownHex is one hex's complete authorized knowledge for a viewer, as
// carried by MoveEvent.MoverKnownHexes.
//
// It flatly mirrors encounter/perception.HexObservation (and that package's
// Edge/Placement) rather than importing it directly: encounter/perception
// already imports this events package (perception.ProjectMove builds
// MovePlayerSlice/HexRevealedSlice values), so events importing perception
// back would cycle. This follows the SAME "events owns the minimal wire
// shape; a richer package projects down into it" split
// MovePlayerSlice/HexRevealedSlice already establish for this exact event —
// encounter.go's applyAndPublishMove (the one place with both packages
// in scope) is what builds a KnownHex from a real
// perception.HexObservation, immediately after mutating the mover's view,
// so this is never at risk of the stale-read race MoverKnownHexes exists to
// close (see MoveEvent's own doc).
type KnownHex struct {
	Position core.Hex
	// State mirrors perception.KnowledgeState's int values (0 = unspecified,
	// 1 = visible, 2 = remembered) without importing that package.
	State int
	// Terrain mirrors perception.TerrainKind's int values.
	Terrain  int
	ZoneID   string
	Edges    []KnownHexEdge
	Contents []KnownHexPlacement
}

// KnownHexEdge mirrors encounter/perception.Edge.
type KnownHexEdge struct {
	From core.Hex
	To   core.Hex

	BlocksMovement bool
	BlocksLoS      bool

	// DoorID is the door's entity id, empty when this edge is not a door.
	DoorID     string
	DoorOpen   bool
	DoorLocked bool
}

// KnownHexPlacement mirrors encounter/perception.Placement.
type KnownHexPlacement struct {
	EntityID core.EntityID
	// Facing is optional canonical hex-facing metadata. Pointer presence
	// distinguishes absent from explicit E = 0 through event serialization.
	Facing *uint32
	// Offset is optional presentation-only [x,y,z] metadata in canonical
	// game-world axes. Pointer presence distinguishes absent from zero.
	Offset *core.PlacementOffset
}

type knownHexPlacementWire struct {
	EntityID core.EntityID         `json:"EntityID"`
	Facing   *uint32               `json:"facing,omitempty"`
	Offset   *core.PlacementOffset `json:"offset,omitempty"`
}

// MarshalJSON writes optional facing under a lowercase key so explicit E = 0
// remains present without colliding with legacy known-hex event JSON.
func (p KnownHexPlacement) MarshalJSON() ([]byte, error) {
	return json.Marshal(knownHexPlacementWire(p))
}

// UnmarshalJSON restores current optional facing and ignores the legacy
// uppercase "Facing":0 emitted by the old mandatory field. That zero meant
// absent, not authored E, so accepting it would invent orientation on replay.
func (p *KnownHexPlacement) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	var entityID core.EntityID
	if rawEntityID, ok := fields["EntityID"]; ok {
		if err := json.Unmarshal(rawEntityID, &entityID); err != nil {
			return err
		}
	}
	var facing *uint32
	if rawFacing, ok := fields["facing"]; ok {
		if err := json.Unmarshal(rawFacing, &facing); err != nil {
			return err
		}
	}
	var offset *core.PlacementOffset
	if rawOffset, ok := fields["offset"]; ok {
		if err := json.Unmarshal(rawOffset, &offset); err != nil {
			return err
		}
	}
	p.EntityID = entityID
	p.Facing = facing
	p.Offset = offset
	return nil
}
