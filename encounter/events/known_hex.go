package events

import "github.com/KirkDiggler/rpg-toolkit/encounter/core"

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
	// Facing is a hex-direction index 0-5.
	Facing int
}
