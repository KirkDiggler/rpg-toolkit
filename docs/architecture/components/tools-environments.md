---
name: tools/environments module
description: Environment persistence, multi-room dungeon graph generation, pathfinding across rooms
updated: 2026-05-02
confidence: high — verified by reading environment_persistence.go, environment_data.go, graph_generator.go
---

# tools/environments module

**Path:** `tools/environments/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/tools/environments`
**Grade:** B

Builds on `tools/spatial` to provide a multi-room environment graph: how rooms connect (passages, doors), how to persist the entire dungeon layout, and how to find paths across connected rooms.

## Files (key ones)

| File | Purpose |
|---|---|
| `environment.go` | `BasicEnvironment` — the environment aggregate |
| `environment_data.go` | `EnvironmentData`, `RoomData`, `PassageData`, `OriginData` |
| `environment_persistence.go` | `ToData()`, `LoadFromData()`, absolute-position entity tracking |
| `graph_generator.go` | Procedural room graph generation algorithms |
| `room_builder.go` | Fluent API for constructing rooms with walls, passages |
| `wall_patterns.go` | Pre-defined wall configurations (perimeter, cross, T-junction) |
| `pathfinding.go` | `FindPathCube` — hex A* across connected rooms |

## Persistence pattern

`BasicEnvironment` is stateful — it holds rooms, connections, and entity positions. It follows the `ToData`/`LoadFromData` convention:

```go
// Serialize: all room positions converted to dungeon-absolute coordinates
data := env.ToData()

// Reconstitute: absolute positions restored, rooms reconnected
env, err := environments.LoadFromData(ctx, LoadFromDataInput{Data: data, EventBus: bus})
```

Entity positions are stored in absolute (dungeon-wide) coordinates in `EnvironmentData`, not room-local coordinates. The persistence layer converts between absolute and room-local on save/load.

## Pathfinding

`FindPathCube(start, goal CubeCoordinate) ([]CubeCoordinate, error)` finds a path in cube (hex) coordinates across connected rooms. It uses the hex pathfinder from `tools/spatial`. This is the authoritative cross-room pathfinder for hex-grid dungeons.

**Square-grid cross-room pathfinding:** Not provided. Same gap as in `tools/spatial` — there is no square-grid pathfinder at any layer.

## Graph generation

`graph_generator.go` is substantial (~400 lines, estimated). It generates connected room graphs with configurable parameters (room count, connection density, branching factor). **It has no direct unit tests.** It is tested indirectly through the `BasicEnvironment` creation tests in `environment_persistence_test.go`.

## go.mod status
Clean. Published versions, no replace directives.

## Known gaps

- `graph_generator.go` is load-bearing but untested in isolation.
- No test for `SelectablesTable` integration within environment generation (mentioned as "missing" in quality.md).
- Large environments (100+ rooms) are not load-tested. Performance is extrapolated from small (4–8 room) tests.
