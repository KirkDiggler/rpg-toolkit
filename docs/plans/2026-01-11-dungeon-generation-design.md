# Dungeon Generation Design

**Date:** 2026-01-11
**Status:** Draft
**Issue:** #564

## Summary

Move dungeon generation logic from rpg-api (prototype) into rpg-toolkit's dungeon module. The API layer configures generation via simple inputs; the toolkit handles all rules logic (CR budgeting, monster selection, room population).

## Architecture

```
rpg-api (configuration layer)
  - "I want a crypt dungeon, 5 rooms, CR 3"
  ↓ calls
rpg-toolkit/rulebook/dungeon (generation layer)
  - Theme application, CR budgeting, monster selection
  - Uses environments + spatial + selectables + spawn underneath
  ↓ returns
*Dungeon (runtime object)
  - API calls ToData() for persistence
```

**Principle:** API is "dumb" (stores/retrieves), Toolkit is "smart" (calculates/decides).

## Public API

### Generator

```go
// Generator orchestrates dungeon creation
type Generator struct {
    // internal components
}

// NewGenerator creates a generator with default or custom components
func NewGenerator(config *GeneratorConfig) *Generator

// Generate creates a complete dungeon from configuration
func (g *Generator) Generate(ctx context.Context, input *GenerateInput) (*GenerateOutput, error)
```

### Input/Output Types

```go
// GenerateInput configures dungeon generation
type GenerateInput struct {
    // Required
    Theme     Theme   // Theme config (toolkit-provided or custom)
    TargetCR  float64 // Target challenge rating for the dungeon
    RoomCount int     // Number of rooms to generate

    // Optional (sensible defaults)
    Seed      int64      // 0 = random
    Layout    LayoutType // linear, branching, hub, organic (default: auto)
    PartySize int        // For CR scaling (default: 4)
}

// GenerateOutput is the result of generation
type GenerateOutput struct {
    Dungeon *Dungeon // Runtime object, ready for gameplay
    Seed    int64    // Actual seed used (for reproducibility)
}
```

### Usage Pattern

```go
// Generate
generator := dungeon.NewGenerator(nil)
output, err := generator.Generate(ctx, &dungeon.GenerateInput{
    Theme:     dungeon.ThemeCrypt,
    TargetCR:  3.0,
    RoomCount: 5,
})

// Use runtime object
dungeon := output.Dungeon
for _, roomID := range dungeon.RoomIDs() {
    room := dungeon.Room(roomID)
    // ...
}

// Persist
data := dungeon.ToData()  // *DungeonData for storage

// Restore
dungeon, err := dungeon.LoadFromData(&LoadFromDataInput{Data: data})
```

## Theme Structure

Themes are configuration, not behavior. The generator reads theme config to drive generation.

```go
type Theme struct {
    ID   string
    Name string

    // Monster selection - weighted tables using selectables
    MonsterPool selectables.SelectionTable[MonsterRef]
    BossPool    selectables.SelectionTable[MonsterRef]

    // Room generation parameters - reuse environments package
    RoomTables *environments.RoomTables

    // Visual flavor (typed for proto enum mapping)
    WallMaterial  WallMaterial
    ObstacleTypes []ObstacleType
    TerrainTypes  []TerrainType
}

// MonsterRef identifies a monster in the rulebook
type MonsterRef struct {
    Ref  *core.Ref   // e.g., refs.Monsters.Skeleton()
    CR   float64     // Challenge rating for budget calculations
    Role MonsterRole // melee, ranged, support, boss
}
```

### Wall Material (Typed Constant)

```go
type WallMaterial string

const (
    WallMaterialStone WallMaterial = "stone"
    WallMaterialRock  WallMaterial = "rock"
    WallMaterialWood  WallMaterial = "wood"
    WallMaterialMetal WallMaterial = "metal"
)
```

Maps to proto enum; web app loads textures based on this.

### Pre-defined Themes

```go
var ThemeCrypt = Theme{
    ID:   "crypt",
    Name: "Ancient Crypt",
    MonsterPool: buildMonsterTable(
        weighted{refs.Monsters.Skeleton(), 0.25, RoleMelee, 40},
        weighted{refs.Monsters.Zombie(), 0.25, RoleMelee, 30},
        weighted{refs.Monsters.SkeletonArcher(), 0.25, RoleRanged, 20},
        weighted{refs.Monsters.Ghoul(), 1.0, RoleMelee, 10},
    ),
    BossPool: buildMonsterTable(
        weighted{refs.Monsters.SkeletonCaptain(), 2.0, RoleBoss, 100},
    ),
    RoomTables:   environments.GetDefaultRoomTables(),
    WallMaterial: WallMaterialStone,
}

var ThemeCave = Theme{...}
var ThemeBanditLair = Theme{...}
```

Themes are hardcoded in toolkit as config. Future: API can provide custom themes using the same struct.

## Internal Components

### Component Architecture

```go
type Generator struct {
    envGenerator *environments.GraphBasedGenerator
    spawnEngine  spawn.SpawnEngine
}

type GeneratorConfig struct {
    // Optional - defaults provided if nil
    EnvironmentGenerator *environments.GraphBasedGenerator
    SpawnEngine          spawn.SpawnEngine
}
```

Dependency injection allows testing with mocks.

### Generation Pipeline

```go
func (g *Generator) Generate(ctx context.Context, input *GenerateInput) (*GenerateOutput, error) {
    // 1. Validate input

    // 2. Generate spatial layout (rooms, connections, walls)
    envConfig := toEnvironmentConfig(input)
    env, err := g.envGenerator.Generate(ctx, envConfig)

    // 3. Allocate CR budget across rooms
    allocation := allocateBudget(&allocateBudgetInput{
        RoomIDs:    env.GetRoomIDs(),
        BossRoomID: findBossRoom(env),
        TargetCR:   input.TargetCR,
        PartySize:  input.PartySize,
    })

    // 4. Generate encounters for each room
    encounters := make(map[string]*Encounter)
    for roomID, budget := range allocation.RoomBudgets {
        encounters[roomID] = generateEncounter(&generateEncounterInput{
            Budget:      budget,
            MonsterPool: input.Theme.MonsterPool,
            IsBossRoom:  roomID == allocation.BossRoomID,
            BossPool:    input.Theme.BossPool,
        })
    }

    // 5. Place monster entities using spawn engine
    for roomID, encounter := range encounters {
        g.placeMonsters(ctx, env, roomID, encounter)
    }

    // 6. Build DungeonData from environment + encounters
    dungeonData := buildDungeonData(env, encounters, allocation.BossRoomID)

    // 7. Load into runtime Dungeon
    dungeon, err := LoadFromData(&LoadFromDataInput{Data: dungeonData})

    return &GenerateOutput{Dungeon: dungeon, Seed: actualSeed}, nil
}
```

### Encounter Generation (Internal)

Budget allocation:
- Boss room gets ~40% of total budget
- Remaining rooms use progressive scaling (earlier = easier)
- Minimum CR 0.25 per room

Monster selection:
- Select from pool until budget filled
- Role distribution: ~50% melee, 30% ranged, 20% support
- Boss room: one boss + minions to fill remaining budget

These are internal functions - not exposed in public API. We learn from usage before exposing configuration.

## Existing Building Blocks

| Package | What it provides |
|---------|------------------|
| `tools/environments` | `GraphBasedGenerator` for room layout, connections, walls |
| `tools/selectables` | `SelectionTable[T]` for weighted random selection |
| `tools/spawn` | `SpawnEngine` for entity placement in rooms |
| `tools/spatial` | Coordinate systems, room orchestration |
| `rulebooks/dnd5e/monster` | Monster factories (`NewSkeleton()`, etc.) |

## Error Handling

```go
var (
    ErrNilInput         = rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
    ErrInvalidTheme     = rpgerr.New(rpgerr.CodeInvalidArgument, "theme cannot be empty")
    ErrInvalidCR        = rpgerr.New(rpgerr.CodeInvalidArgument, "target CR must be positive")
    ErrInvalidRoomCount = rpgerr.New(rpgerr.CodeInvalidArgument, "room count must be at least 1")
)
```

Principles:
- Validation errors are clear, actionable
- Internal errors wrapped with context
- Never return `(nil, nil)`

## File Structure

```
rulebooks/dnd5e/dungeon/
├── dungeon.go           # existing - runtime wrapper
├── dungeon_data.go      # existing - persistence types
├── types.go             # existing - enums (RoomType, MonsterRole, etc.)
├── generator.go         # NEW - Generator, GenerateInput/Output
├── theme.go             # NEW - Theme struct, ThemeCrypt, ThemeCave, etc.
├── encounter.go         # NEW - allocateBudget, generateEncounter (internal)
├── errors.go            # NEW - error definitions
├── generator_test.go    # NEW - integration tests
├── encounter_test.go    # NEW - unit tests
├── theme_test.go        # NEW - unit tests
└── example/
    ├── README.md        # NEW - how to run, what to look for
    └── example_test.go  # NEW - narrated examples (issue #564)
```

## Testing Strategy

### Unit Tests
Each internal component in isolation:
- `encounter_test.go` - budget allocation, encounter generation
- `theme_test.go` - monster pool selection, theme lookup

### Integration Tests
Generator with real components:
- `generator_test.go` - full pipeline tests

### Example Tests (Issue #564)
Narrated tests that serve as living documentation:

```go
// example/example_test.go
func TestDungeonGeneration(t *testing.T) {
    fmt.Println("=== Generating a 5-room crypt dungeon ===")

    generator := dungeon.NewGenerator(nil)
    output, _ := generator.Generate(ctx, &dungeon.GenerateInput{
        Theme:     dungeon.ThemeCrypt,
        TargetCR:  3.0,
        RoomCount: 5,
    })

    fmt.Printf("Generated dungeon with seed: %d\n", output.Seed)
    fmt.Printf("Rooms: %d\n", len(output.Dungeon.RoomIDs()))

    for _, roomID := range output.Dungeon.RoomIDs() {
        room := output.Dungeon.Room(roomID)
        fmt.Printf("Room %s (%s): %d monsters\n",
            roomID, room.Type, len(room.Encounter.Monsters))
    }
}
```

Run `go test -v ./example/` to see the system in action.

## Go Doc Comments

Package-level documentation:

```go
// Package dungeon provides D&D 5e dungeon generation and runtime management.
//
// # Generation
//
// Create dungeons using the Generator:
//
//     generator := dungeon.NewGenerator(nil)
//     output, err := generator.Generate(ctx, &dungeon.GenerateInput{
//         Theme:     dungeon.ThemeCrypt,
//         TargetCR:  3.0,
//         RoomCount: 5,
//     })
//
// # Persistence
//
// Convert to data for storage, reload later:
//
//     data := dungeon.ToData()
//     dungeon, err := dungeon.LoadFromData(&LoadFromDataInput{Data: data})
//
// # Themes
//
// Available themes: ThemeCrypt, ThemeCave, ThemeBanditLair.
// Custom themes can be created using the Theme struct.
//
// # Coordinates
//
// All positions use dungeon-absolute cube coordinates.
// No conversion needed between rooms.
package dungeon
```

## Migration from rpg-api

The prototype in `rpg-api/internal/components/dungeon/` contains:
- Theme definitions with monster pools
- CR budgeting logic
- Encounter generation with role distribution
- Shape/feature generators (wrap toolkit)

**Migration approach:**
1. Build new generation in toolkit (this design)
2. Update rpg-api to call toolkit generator
3. Remove prototype code from rpg-api

The rpg-api prototype serves as reference implementation for the logic.

## Future Extensions

**Tier 2 config (when needed):**
- `RoomSize` - small, medium, large
- `BossRoom` - include boss encounter
- `TreasureRooms` - number of treasure rooms

**Tier 3 config (maybe never):**
- `EmptyRoomRatio` - combat vs non-combat rooms
- `BossCRMultiplier` - boss difficulty scaling
- `DifficultyRamp` - earlier rooms easier

**Custom themes from API:**
- API provides `Theme` struct directly
- Same structure as toolkit themes
- No changes to generator needed

## Related Issues

- #564 - Add example/ documentation pattern to dungeon module
- rpg-api prototype code to be deprecated after migration
