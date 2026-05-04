# Support CubeCoordinate in EntityPlacement

**Date:** 2025-12-16
**Issue:** https://github.com/KirkDiggler/rpg-toolkit/issues/469

## Problem

`EntityPlacement` only stores `Position` (float64 X, Y), but hex grids should use cube coordinates (int X, Y, Z). The rpg-api currently converts on-the-fly between offset and cube coordinates, creating a conversion layer that violates our "pick ONE way to represent data" principle.

## Decision

Use **separate structs** rather than adding an optional field to the existing struct:

- `EntityPlacement` - uses `Position` for square/gridless grids
- `EntityCubePlacement` - uses `CubeCoordinate` for hex grids

This approach was chosen because:
1. Within a single room, ALL entities use the same coordinate system (determined by grid type)
2. Type system enforces correct usage
3. No ambiguity about which field to read
4. Cleaner semantics

## Design

### EntityCubePlacement Struct

```go
// EntityCubePlacement represents an entity's placement using cube coordinates (for hex grids)
type EntityCubePlacement struct {
    EntityID          string         `json:"entity_id"`
    EntityType        string         `json:"entity_type"`
    CubePosition      CubeCoordinate `json:"cube_position"`
    Size              int            `json:"size,omitempty"`
    BlocksMovement    bool           `json:"blocks_movement"`
    BlocksLineOfSight bool           `json:"blocks_line_of_sight"`
}
```

### RoomData Changes

```go
type RoomData struct {
    ID           string                         `json:"id"`
    Type         string                         `json:"type"`
    Width        int                            `json:"width"`
    Height       int                            `json:"height"`
    GridType     string                         `json:"grid_type"`
    HexFlatTop   bool                           `json:"hex_flat_top,omitempty"`
    Entities     map[string]EntityPlacement     `json:"entities,omitempty"`      // square/gridless
    CubeEntities map[string]EntityCubePlacement `json:"cube_entities,omitempty"` // hex
}
```

### Data Flow

**ToData() - Serialization:**
- Hex grids: Convert internal offset positions to cube coordinates, store in `CubeEntities`
- Square/gridless: Store offset positions in `Entities` (unchanged)

**LoadRoomFromContext() - Deserialization:**
- Hex grids: Read from `CubeEntities`, convert cube to offset for internal storage
- Square/gridless: Read from `Entities` (unchanged)

### Internal Storage

The room still stores positions internally as offset coordinates (since the grid does the math). The conversion to/from cube coordinates happens at the persistence boundary:

```
Runtime (offset) → ToData() → Serialized (cube for hex)
Serialized (cube for hex) → LoadRoomFromContext() → Runtime (offset)
```

## Testing

1. EntityCubePlacement struct - JSON serialization round-trip
2. RoomData with CubeEntities - Verify hex rooms serialize to `cube_entities`
3. ToData() for hex rooms - Entities get cube coordinates
4. LoadRoomFromContext() for hex rooms - Cube coordinates load correctly
5. Round-trip - Save hex room → load hex room → positions match

Existing tests for square/gridless should continue to pass unchanged.

## Impact on rpg-api

After this change, rpg-api's converters can:
- Read `CubeEntities` directly for hex grids (no conversion needed)
- The `OffsetCoordinateToCubeWithOrientation` call in `convertPositionToProto` becomes unnecessary for hex grids
