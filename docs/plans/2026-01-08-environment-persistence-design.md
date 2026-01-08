# Environment Persistence Design

**Issue:** #540
**Date:** 2026-01-08
**Scope:** tools/environments module only

## Goal

Define a persistence format for `BasicEnvironment` that achieves **full round-trip fidelity**. `ToData()` followed by `LoadFromData()` must produce a functionally equivalent environment.

## Design Principles

1. **Full fidelity** - No data loss during serialization
2. **Typed strings** - Discoverable constants, not magic strings
3. **Absolute coordinates** - All positions in environment-absolute space
4. **Generic infrastructure** - Game-specific data lives in rulebooks, not here
5. **Deterministic output** - Same environment produces identical JSON

## Typed String Pattern

The environments package defines typed string types. Rulebooks define constants using these types for IDE discoverability.

```go
// Defined in tools/environments
type EnvironmentType string
type PropertyKey string
type GridShapeValue string
type HexOrientationValue string

// Constants for GridShapeValue (maps to spatial.GridShape)
const (
    GridShapeHex      GridShapeValue = "hex"
    GridShapeSquare   GridShapeValue = "square"
    GridShapeGridless GridShapeValue = "gridless"
)

// Constants for HexOrientationValue (maps to spatial.HexOrientation)
const (
    HexOrientationPointy HexOrientationValue = "pointy"
    HexOrientationFlat   HexOrientationValue = "flat"
)
```

Rulebooks define their own constants:
```go
// Defined in rulebooks/dnd5e/dungeon
const (
    EnvironmentTypeDungeon environments.EnvironmentType = "dungeon"
    EnvironmentTypeCave    environments.EnvironmentType = "cave"

    PropertyOpen   environments.PropertyKey = "open"
    PropertyLocked environments.PropertyKey = "locked"
    PropertyDC     environments.PropertyKey = "dc"
)
```

## Data Structures

### EnvironmentData

Top-level persistence structure:

```go
type EnvironmentData struct {
    ID       string                `json:"id"`
    Type     EnvironmentType       `json:"type"`
    Theme    string                `json:"theme,omitempty"`
    Metadata EnvironmentMetadata   `json:"metadata"`
    Zones    []ZoneData            `json:"zones"`
    Passages []PassageData         `json:"passages"`
    Entities []PlacedEntityData    `json:"entities"`
    Walls    []WallSegmentData     `json:"walls"`
}
```

### ZoneData

Represents a room/zone with its grid configuration:

```go
type ZoneData struct {
    ID          string              `json:"id"`
    Type        string              `json:"type"`
    Origin      spatial.CubeCoordinate `json:"origin"`
    Width       int                 `json:"width"`
    Height      int                 `json:"height"`
    GridShape   GridShapeValue      `json:"grid_shape"`
    Orientation HexOrientationValue `json:"orientation,omitempty"`
    EntityIDs   []string            `json:"entity_ids,omitempty"`
}
```

### PassageData

Abstract connection between zones:

```go
type PassageData struct {
    ID                  string `json:"id"`
    FromZoneID          string `json:"from_zone_id"`
    ToZoneID            string `json:"to_zone_id"`
    ControllingEntityID string `json:"controlling_entity_id,omitempty"`
    Bidirectional       bool   `json:"bidirectional"`
}
```

### PlacedEntityData

Entity with absolute position:

```go
type PlacedEntityData struct {
    ID             string                 `json:"id"`
    Type           string                 `json:"type"`
    Position       spatial.CubeCoordinate `json:"position"`
    Size           int                    `json:"size"`
    BlocksMovement bool                   `json:"blocks_movement"`
    BlocksLoS      bool                   `json:"blocks_los"`
    ZoneID         string                 `json:"zone_id"`
    Subtype        string                 `json:"subtype,omitempty"`
    Properties     map[PropertyKey]any    `json:"properties,omitempty"`
}
```

### WallSegmentData

Wall segment in absolute coordinates:

```go
type WallSegmentData struct {
    Start          spatial.CubeCoordinate `json:"start"`
    End            spatial.CubeCoordinate `json:"end"`
    BlocksMovement bool                   `json:"blocks_movement"`
    BlocksLoS      bool                   `json:"blocks_los"`
}
```

## Conversion Functions

### ToData()

Converts `BasicEnvironment` to `EnvironmentData`:

1. Copy ID, Type, Theme, Metadata directly
2. For each room in orchestrator:
   - Get grid shape and convert to GridShapeValue
   - Get orientation if hex grid
   - Get dimensions
   - Create ZoneData with origin from roomPositions
3. For each connection:
   - Create PassageData
4. For each entity in each room:
   - Convert local position to absolute using room origin
   - Extract Placeable properties if available
   - Extract Subtype and Properties if entity supports them
5. Sort all slices by ID for deterministic output

### LoadFromData()

Reconstructs `BasicEnvironment` from `EnvironmentData`:

1. Validate required fields (ID must not be empty)
2. Create orchestrator
3. For each zone:
   - Validate GridShape
   - Create appropriate grid type (hex/square/gridless)
   - Create room with grid
   - Track room position
4. For each passage:
   - Create connection
5. For each entity:
   - Convert absolute position to local using zone origin
   - Create entity from persisted data
   - Place in room (collect errors, don't silently skip)
6. Create BasicEnvironment with all data
7. Return environment and any placement errors

## Error Handling

- `LoadFromData` returns errors for:
  - Empty ID
  - Unknown GridShape
  - Failed room creation
  - Failed connection creation
- Entity placement failures are collected and returned as warnings, not fatal errors

## Safety Requirements

- Nil grid checks before accessing grid methods
- Validate cube coordinates (x + y + z = 0)
- Deterministic slice ordering (sort by ID)

## Out of Scope

- Game-specific entity properties (handled by rulebook layer)
- Seed-based regeneration (seed is for reference only)
- Wall segment generation (walls are entities in current implementation)

## Implementation Order

1. Define typed string types and constants
2. Update data structs with new fields
3. Implement ToData() with proper conversion and sorting
4. Implement LoadFromData() with validation and error collection
5. Write comprehensive tests for round-trip fidelity
