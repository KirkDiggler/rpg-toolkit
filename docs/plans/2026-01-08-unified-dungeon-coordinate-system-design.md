# Unified Dungeon Coordinate System Design

## Overview

This design establishes how dungeons are generated, stored, and communicated with absolute coordinates throughout. The goal is to eliminate coordinate conversions in the game server (rpg-api) by ensuring the toolkit produces data ready for use.

## Problem Statement

Current implementation has coordinate conversions scattered across layers:
- `entities/dungeon.go`: `CalculateRoomPositions()`, `ToAbsolute()`, `ToLocal()`
- `orchestrator.go`: Door positioning, monster placement, movement validation

This violates the principle: **"rpg-api stores data, rpg-toolkit handles rules."**

The game server should receive absolute coordinates and pass them through. No conversions.

## Core Principles

1. **Generator outputs absolute coordinates** - Rooms are local during generation, converted to absolute on output
2. **One unified coordinate space** - A dungeon is one continuous map, not separate rooms with transitions
3. **Doors are entities** - Doors have state (open/closed/locked) and are placed at hex positions
4. **Rooms are logical boundaries** - Useful for queries ("what room is this?") and visibility control
5. **Server controls visibility** - Client only receives what it should see

## Architecture

### Toolkit Placement

**`tools/environments/`** - Generic infrastructure:
- Room/Zone graphs
- Connections with positions
- Coordinate systems (local to absolute conversion)
- Walls, doors as geometry
- Progression patterns (start, waypoints, finale)
- Collision detection, pathfinding

**`rulebooks/dnd5e/dungeon/`** - D&D-specific:
- DungeonGenerator (uses environments)
- Theme system (crypt, cave, bandit lair)
- CR-based encounter spawning
- Trap mechanics (DC, saving throws)
- Monster/obstacle type selection

### Data Structures

#### DungeonData (Persistence)

```go
type DungeonData struct {
    ID            string            `json:"id"`
    Seed          int64             `json:"seed"`

    // Metadata
    StartRoomID   string            `json:"start_room_id"`
    BossRoomID    string            `json:"boss_room_id"`
    Theme         string            `json:"theme"`

    // Rooms (logical groupings, all coords absolute)
    Rooms         []RoomData        `json:"rooms"`

    // Connections (for pathfinding, door references)
    Connections   []ConnectionData  `json:"connections"`

    // All entities including doors (absolute coords)
    Entities      []EntityData      `json:"entities"`

    // All walls (absolute coords)
    Walls         []WallData        `json:"walls"`

    // Runtime state
    RevealedRooms []string          `json:"revealed_rooms"`
}
```

#### RoomData

```go
type RoomData struct {
    ID            string            `json:"id"`
    Type          string            `json:"type"`  // "combat", "treasure", "boss"
    Origin        PositionData      `json:"origin"`
    EntityIDs     []string          `json:"entity_ids"`
}
```

#### ConnectionData

```go
type ConnectionData struct {
    ID            string            `json:"id"`
    FromRoomID    string            `json:"from_room_id"`
    ToRoomID      string            `json:"to_room_id"`
    DoorEntityID  string            `json:"door_entity_id"`
}
```

#### EntityData

```go
type EntityData struct {
    ID              string            `json:"id"`
    Type            EntityType        `json:"type"`
    Position        PositionData      `json:"position"`
    Size            EntitySize        `json:"size"`
    BlocksMovement  bool              `json:"blocks_movement"`
    BlocksLoS       bool              `json:"blocks_los"`

    // Type-specific (oneof in proto)
    MonsterType     *MonsterType      `json:"monster_type,omitempty"`
    ObstacleType    *ObstacleType     `json:"obstacle_type,omitempty"`
    DoorType        *DoorType         `json:"door_type,omitempty"`

    // Door-specific state
    Properties      map[string]any    `json:"properties,omitempty"`
}
```

#### WallData

```go
type WallData struct {
    Start           PositionData      `json:"start"`
    End             PositionData      `json:"end"`
    BlocksMovement  bool              `json:"blocks_movement"`
    BlocksLoS       bool              `json:"blocks_los"`
}
```

### Entity Types (Proto Update)

Extend `EntityPlacement` to include doors:

```protobuf
enum EntityType {
  ENTITY_TYPE_UNSPECIFIED = 0;
  ENTITY_TYPE_CHARACTER = 1;
  ENTITY_TYPE_MONSTER = 2;
  ENTITY_TYPE_OBSTACLE = 3;
  ENTITY_TYPE_DOOR = 4;
}

enum DoorType {
  DOOR_TYPE_UNSPECIFIED = 0;
  DOOR_TYPE_WOODEN = 1;
  DOOR_TYPE_IRON = 2;
  DOOR_TYPE_STONE = 3;
  DOOR_TYPE_PORTCULLIS = 4;
  DOOR_TYPE_MAGICAL_BARRIER = 5;
}

// In EntityPlacement oneof
oneof visual_type {
  MonsterType monster_type = 5;
  ObstacleType obstacle_type = 6;
  DoorType door_type = 7;
}
```

### Generation Flow

**Phase 1: Room Generation (local coords)**

Each room is generated independently in its own local coordinate space (origin 0,0,0). This makes sense because:
- Room doesn't know about other rooms yet
- Spawning algorithms work in local space
- Room templates/prefabs are reusable

**Phase 2: Dungeon Assembly (convert to absolute)**

Once all rooms are generated and connections defined:
1. Place first room at origin (0,0,0)
2. BFS through connections
3. For each connection, position next room so doors align
4. Convert all room geometry to absolute coords
5. Create door entities at connection points
6. Return complete dungeon with everything absolute

**Output:** A complete `DungeonData` with all coordinates absolute. The orchestrator receives this and never does coordinate math.

### Orchestrator Interaction

The orchestrator receives finalized data and just passes through:

```go
func (o *Orchestrator) CreateDungeon(ctx context.Context, input *CreateDungeonInput) (*CreateDungeonOutput, error) {
    // 1. Call component to generate (returns absolute coords)
    dungeon := o.dungeonGen.Generate(input.Seed, input.Config)

    // 2. Persist dungeon
    o.dungeonRepo.Save(ctx, dungeon)

    // 3. Return starting room data to client
    startRoom := dungeon.Rooms[dungeon.StartRoomID]
    return &CreateDungeonOutput{
        DungeonID: dungeon.ID,
        Room:      startRoom,
    }
}
```

No `ToAbsolute()`, no `ToLocal()`, no coordinate math.

### Collision Detection

Lives in component/toolkit layer:

```go
func (d *Dungeon) CanMoveTo(entityID string, target CubeCoordinate) (bool, BlockReason) {
    entity := d.GetEntity(entityID)
    from := entity.Position

    // Check wall collision
    for _, wall := range d.AllWalls() {
        if PathCrossesWall(from, target, wall) {
            return false, BlockedByWall
        }
    }

    // Check door collision
    for _, door := range d.GetDoorsOnPath(from, target) {
        if !door.IsOpen() {
            return false, BlockedByDoor
        }
    }

    // Check entity collision
    if occupant := d.GetEntityAt(target); occupant != nil {
        if occupant.BlocksMovement {
            return false, BlockedByEntity
        }
    }

    return true, NotBlocked
}
```

### Door Behavior

- A door occupies a single hex position
- Wall segments extend from the door hex to room edges
- Closed door blocks movement (requires "open door" action)
- Open door is passable like any other hex
- Interior doors are possible (closet, cage within a room)

Door properties use a flexible map:
```go
Properties: map[string]any{
    "open":    false,
    "locked":  true,
    "trap_dc": 15,
}
```

### Client Communication

Server controls what the client sees:

```go
type RoomRevealedEvent struct {
    DungeonID     string
    RoomID        string
    Walls         []WallSegment     // absolute coords
    Entities      []VisibleEntity   // filtered by visibility
    Doors         []DoorInfo        // only visible properties
}
```

- Monsters in unrevealed rooms: not sent
- Trap on a door: not sent until detected
- Secret door: not sent until found

## What Changes

### Moves to `internal/components/dungeon` (portable to toolkit):
- Room generation (local coords during build)
- Dungeon assembly (converts to absolute)
- Collision detection
- Spatial queries
- Door state management

### Stays in orchestrator:
- Load, call component, persist, return
- Event publishing
- Access control

### Gets removed from orchestrator:
- All `ToAbsolute()`, `ToLocal()` calls
- Coordinate conversion logic
- Door position calculations

## Migration Path

1. Prototype in `rpg-api/internal/components/dungeon`
2. Validate design works end-to-end
3. Extract generic parts to `rpg-toolkit/tools/environments`
4. Extract D&D parts to `rpg-toolkit/rulebooks/dnd5e/dungeon`
5. Update rpg-api to import from toolkit

## Related Documents

- [Entity Asset Types Design](../../rpg-api-protos/docs/plans/2026-01-03-entity-asset-types-design.md)
- [Spatial Module CLAUDE.md](../tools/spatial/CLAUDE.md)
