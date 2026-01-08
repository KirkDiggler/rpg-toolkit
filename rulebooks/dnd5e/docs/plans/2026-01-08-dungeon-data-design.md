# DungeonData Design

**Date:** 2026-01-08
**Status:** Approved
**Issue:** #540 (continued)

## Overview

Move D&D 5e dungeon data structures and runtime logic from `rpg-api` to `rpg-toolkit/rulebooks/dnd5e/dungeon`. The toolkit owns the game logic; the API persists and orchestrates.

## Goals

1. **Full round-trip persistence** - Save/load dungeons with complete fidelity
2. **Runtime state management** - Track exploration, doors, metrics during gameplay
3. **Compose EnvironmentData** - Leverage spatial infrastructure from `tools/environments`
4. **Clean API migration** - Same method signatures where possible

## Package Location

```
rulebooks/dnd5e/dungeon/
├── dungeon.go          # Dungeon runtime struct + methods
├── dungeon_data.go     # DungeonData, RoomData, persistence structs
├── dungeon_test.go     # Tests for runtime logic
├── types.go            # DungeonState, RoomType, MonsterRole, etc.
└── go.mod              # Depends on tools/environments, core
```

## Data Structures

### DungeonData (Persistence)

Composes `EnvironmentData` and adds D&D 5e-specific fields:

```go
type DungeonData struct {
    // Environment contains zones, passages, entities, walls
    Environment environments.EnvironmentData `json:"environment"`

    // D&D 5e specific
    StartRoomID string  `json:"start_room_id"`
    BossRoomID  string  `json:"boss_room_id"`
    Seed        int64   `json:"seed"` // For reproducible generation

    // Room details indexed by zone ID
    Rooms map[string]RoomData `json:"rooms"`

    // Exploration state
    State         DungeonState    `json:"state"`
    CurrentRoomID string          `json:"current_room_id"`
    RevealedRooms map[string]bool `json:"revealed_rooms"`
    OpenDoors     map[string]bool `json:"open_doors"`

    // Metrics
    RoomsCleared   int `json:"rooms_cleared"`
    MonstersKilled int `json:"monsters_killed"`

    // Timestamps
    CreatedAt   time.Time  `json:"created_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}
```

### RoomData

D&D-specific room content, linked to zones via matching ID:

```go
type RoomData struct {
    // Type categorizes the room (entrance, chamber, boss, etc.)
    Type RoomType `json:"type"`

    // Encounter defines monsters in this room (nil if cleared/empty)
    Encounter *EncounterData `json:"encounter,omitempty"`

    // Features contains obstacles and terrain
    Features FeatureData `json:"features"`

    // SpawnZones defines where players/monsters can spawn
    SpawnZones []SpawnZoneData `json:"spawn_zones,omitempty"`
}
```

### EncounterData

Monster composition for a room:

```go
type EncounterData struct {
    Monsters []MonsterPlacementData `json:"monsters"`
    TotalCR  float64                `json:"total_cr"`
}

type MonsterPlacementData struct {
    ID        string      `json:"id"`
    MonsterID string      `json:"monster_id"` // References rulebook monster
    Role      MonsterRole `json:"role"`
    CR        float64     `json:"cr"`
    // Position stored in EnvironmentData.Entities, linked by ID
}
```

### FeatureData

Room obstacles and terrain:

```go
type FeatureData struct {
    Obstacles []ObstacleData     `json:"obstacles,omitempty"`
    Terrain   []TerrainPatchData `json:"terrain,omitempty"`
}

type ObstacleData struct {
    ID                string       `json:"id"`
    Type              ObstacleType `json:"type"`
    BlocksMovement    bool         `json:"blocks_movement"`
    BlocksLineOfSight bool         `json:"blocks_los"`
    // Position stored in EnvironmentData.Entities, linked by ID
}

type TerrainPatchData struct {
    ID           string      `json:"id"`
    Type         TerrainType `json:"type"`
    MovementCost float64     `json:"movement_cost"`
    // Bounds stored as zone or entity positions in EnvironmentData
}

type SpawnZoneData struct {
    ID       string   `json:"id"`
    Type     ZoneType `json:"type"`
    Capacity int      `json:"capacity"`
    // Bounds stored in EnvironmentData
}
```

## Dungeon Runtime

Wraps `DungeonData` with exploration logic:

```go
type Dungeon struct {
    data *DungeonData

    // Cached lookups (built on load)
    roomsByID       map[string]*RoomData
    connectionsByID map[string]*environments.PassageData
}

// Construction
func New(data *DungeonData) *Dungeon

// Identity
func (d *Dungeon) ID() string
func (d *Dungeon) State() DungeonState
func (d *Dungeon) StartRoom() string
func (d *Dungeon) BossRoom() string

// Room queries
func (d *Dungeon) Room(roomID string) *RoomData
func (d *Dungeon) CurrentRoom() *RoomData
func (d *Dungeon) RoomRevealed(roomID string) bool

// Door queries
func (d *Dungeon) Doors() []*environments.PassageData
func (d *Dungeon) DoorsFromRoom(roomID string) []*environments.PassageData
func (d *Dungeon) VisibleDoors() []*environments.PassageData
func (d *Dungeon) DoorOpen(connectionID string) bool

// State mutations
func (d *Dungeon) RevealRoom(roomID string)
func (d *Dungeon) OpenDoor(connectionID string)
func (d *Dungeon) SetCurrentRoom(roomID string)
func (d *Dungeon) IncrementRoomsCleared()
func (d *Dungeon) IncrementMonstersKilled(count int)
func (d *Dungeon) MarkVictory()
func (d *Dungeon) MarkFailed()
func (d *Dungeon) MarkAbandoned()

// Persistence
func (d *Dungeon) ToData() *DungeonData
```

## Typed Enums

### DungeonState (int - for switch statements)

```go
type DungeonState int

const (
    StateActive DungeonState = iota
    StateVictorious
    StateFailed
    StateAbandoned
)
```

### RoomType (typed string - serializes cleanly)

```go
type RoomType string

const (
    RoomTypeEntrance  RoomType = "entrance"
    RoomTypeChamber   RoomType = "chamber"
    RoomTypeCorridor  RoomType = "corridor"
    RoomTypeBoss      RoomType = "boss"
    RoomTypeTreasure  RoomType = "treasure"
    RoomTypeTrap      RoomType = "trap"
)
```

### MonsterRole (typed string)

```go
type MonsterRole string

const (
    RoleMelee   MonsterRole = "melee"
    RoleRanged  MonsterRole = "ranged"
    RoleSupport MonsterRole = "support"
    RoleBoss    MonsterRole = "boss"
)
```

### Other Types (typed strings)

- `ConnectionType` - door, stairs, passage
- `Direction` - north, south, east, west, up, down
- `ObstacleType` - pillar, sarcophagus, altar, boulder, etc.
- `TerrainType` - difficult, hazardous, water, lava, ice
- `ZoneType` - player_spawn, monster_spawn, boss, entrance, exit

## What Stays in the API

Generation-specific types that are inputs to the generator, not persisted state:

- `RoomSize` - small, medium, large
- `LayoutType` - linear, branching, hub
- `ShapeStyle` - structured, organic, mixed
- `PatternType` - empty, sparse, cover_clusters, etc.
- `DensityRange` - min/max obstacle density
- `Theme` and theme tables

## Dependencies

```
rulebooks/dnd5e/dungeon
├── imports: tools/environments (EnvironmentData, PassageData)
├── imports: tools/spatial (CubeCoordinate via environments)
└── imports: core (EntityType, Ref)
```

## API Migration Path

1. API generator builds `DungeonData` with `EnvironmentData` inside
2. API creates `dungeon.Dungeon` via `dungeon.New(data)`
3. API calls toolkit methods for exploration logic
4. API persists via `dungeon.ToData()` when saving
5. API's `entities.Dungeon` becomes a thin wrapper or is removed

## Key Design Decisions

1. **Composition over embedding** - `Environment` is a named field for clear JSON
2. **Rooms map keyed by zone ID** - Links game content to spatial zones
3. **Monster positions in EnvironmentData.Entities** - Spatial data stays spatial
4. **Caches built on load** - Performance without persistence complexity
5. **Same method signatures** - Minimal API code changes during migration
6. **Go naming conventions** - `VisibleDoors()` not `GetVisibleDoors()`
