# Unified Dungeon Pathfinding Design

**Date**: 2026-01-07
**Status**: Draft
**Related Issues**:
- rpg-toolkit#525 (Multi-room pathfinding for monster pursuit)
- rpg-api#399 (Unified Dungeon Coordinate System)

## Summary

Enable pathfinding across entire dungeons using unified coordinates. Rooms become event boundaries, not pathfinding boundaries. A single A* pathfinder operates on the whole environment grid.

## Key Insight

**Rooms are event boundaries, not coordinate boundaries.**

- Pathfinding: One unified grid, A* works across the entire environment
- Events: Server filters events by which room entities are in
- Doors: Just passable chokepoints on the grid, not coordinate system transitions

This eliminates the need for room-segment stitching or coordinate conversion.

## Architecture

```
tools/spatial/
    pathfinder.go         # PathFinder interface + SimplePathFinder (A*)

tools/environments/
    interfaces.go         # Add FindPathCube to Environment interface
    environment.go        # Implement FindPathCube using spatial.PathFinder

rulebooks/dnd5e/monster/
    action.go             # Extended PerceptionData (MyRoom, Room on entities)
    movement.go           # Use environment pathfinding
```

**Dependency direction**: `rulebooks → environments → spatial`

## Changes by Package

### 1. tools/spatial - PathFinder (New Location)

Move existing A* implementation from `rulebooks/dnd5e/monster/pathfinder.go` to `tools/spatial/pathfinder.go`.

```go
// PathFinder finds paths between hex positions avoiding obstacles.
type PathFinder interface {
    // FindPath returns a path from start to goal avoiding blocked hexes.
    // Returns the path excluding start, including goal.
    // Returns empty slice if no path exists or start == goal.
    FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate
}

// SimplePathFinder uses A* algorithm with uniform movement cost.
type SimplePathFinder struct{}

func NewSimplePathFinder() *SimplePathFinder
func (p *SimplePathFinder) FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate
```

No logic changes - just relocation to make it available to all packages.

### 2. tools/environments - FindPathCube

Add new method to Environment interface (keep existing `FindPath` for future Position-based use).

**Interface addition** (`interfaces.go`):
```go
type Environment interface {
    // ... existing methods ...

    // FindPathCube finds a path between cube coordinates across the environment.
    // Uses A* on the unified coordinate grid.
    FindPathCube(input *FindPathCubeInput) (*FindPathCubeOutput, error)
}
```

**Types** (`types.go`):
```go
type FindPathCubeInput struct {
    From    spatial.CubeCoordinate
    To      spatial.CubeCoordinate
    Blocked map[spatial.CubeCoordinate]bool  // All blocked hexes (walls, obstacles)
}

type FindPathCubeOutput struct {
    Path          []spatial.CubeCoordinate  // Full path, start excluded, goal included
    TotalDistance int
    Found         bool  // False if no path exists
}
```

**Implementation** (`environment.go`):
```go
func (e *BasicEnvironment) FindPathCube(input *FindPathCubeInput) (*FindPathCubeOutput, error) {
    if input == nil {
        return nil, fmt.Errorf("input cannot be nil")
    }

    pathfinder := spatial.NewSimplePathFinder()
    path := pathfinder.FindPath(input.From, input.To, input.Blocked)

    return &FindPathCubeOutput{
        Path:          path,
        TotalDistance: len(path),
        Found:         len(path) > 0 || input.From == input.To,
    }, nil
}
```

### 3. rulebooks/dnd5e/monster - PerceptionData

Extend for event filtering context (not pathfinding).

```go
type PerceptionData struct {
    MyPosition   spatial.CubeCoordinate
    MyRoom       string                    // NEW: For event filtering
    Enemies      []PerceivedEntity
    Allies       []PerceivedEntity
    BlockedHexes []spatial.CubeCoordinate
}

type PerceivedEntity struct {
    Entity   core.Entity
    Position spatial.CubeCoordinate
    Distance int
    Adjacent bool
    HP       int
    AC       int
    Room     string  // NEW: For event filtering
}
```

### 4. rulebooks/dnd5e/monster - Movement Update

Update `moveTowardEnemy` to use unified pathfinding.

```go
func (m *Monster) moveTowardEnemy(input *MoveTowardEnemyInput) (*MoveTowardEnemyOutput, error) {
    closest := input.Perception.ClosestEnemy()
    if closest == nil {
        return &MoveTowardEnemyOutput{Moved: false}, nil
    }

    // Unified pathfinding - rooms don't matter for pathing
    path := input.PathFinder.FindPath(
        input.Perception.MyPosition,
        closest.Position,
        toBlockedMap(input.Perception.BlockedHexes),
    )

    if len(path) == 0 {
        return &MoveTowardEnemyOutput{Moved: false}, nil
    }

    // Move along path up to speed limit
    return m.moveAlongPath(input, path)
}
```

## Perception Rules

For MVP:
- Monster sees entities in its **own room**
- Monster sees entities in **adjacent rooms through open doors**
- Walls block line of sight
- Closed doors block perception (not just movement)

Future (sound propagation):
- Sounds have range in hexes/rooms
- Doors muffle (reduce effective range)
- Combat noise alerts monsters behind closed doors

## What This Enables

**Immediate:**
- Monsters path to players anywhere in the dungeon
- No coordinate conversion or room-stitching
- Clean separation: spatial does pathfinding, rooms do events

**Future:**
- Smart enemies using multiple paths to flank
- Sound-based alerting of distant monsters
- Tactical mode with environment-level event streaming

## Implementation Order

Each is a separate PR with CI version bump:

1. **spatial: Add PathFinder** - Move A* to `tools/spatial/pathfinder.go`
2. **environments: Add FindPathCube** - New method using spatial.PathFinder (depends on #1)
3. **monster: Extend PerceptionData** - Add room fields for event filtering
4. **monster: Update movement** - Use spatial.PathFinder directly (depends on #1)

Note: #3 and #4 can be combined if they're in the same package.

## Non-Goals (This Design)

- Sound propagation / alerting distant monsters
- Multi-path flanking AI
- Line-of-sight calculations (separate concern)
- Room transition events (rpg-api concern)

## Testing

- Unit tests for PathFinder in spatial (already exist, just move them)
- Unit tests for FindPathCube in environments
- Integration test: monster paths across multi-room dungeon
- Edge cases: no path exists, start == goal, blocked goal
