package encounter

import (
	"errors"
	"fmt"
	"math"

	rpgcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// InitRoom builds a walled room for this encounter via environments.QuickRoom,
// snapshots its walls into Data.Space for persistence, and registers a
// reconstructed room (see rebuildRoomFromData) with a fresh RoomOrchestrator
// for the encounter's lifetime. Call once, right after New, before any
// Move/AddMonster/LOS query relies on wall geometry.
//
// Deliberately does NOT register QuickRoom's own *spatial.Room as e.room:
// environments' wall generator places walls at continuous positions (e.g.
// X=3.7), not hex-cell-snapped ones, while every LOS/movement check in this
// package queries at the INTEGER positions core.Hex.ToPosition() produces —
// those two would essentially never collide in the room's position-keyed
// occupancy map, making most generated walls silently non-blocking. Routing
// through the same snapshot -> rebuildRoomFromData path New/LoadFromData use
// guarantees e.room's walls always sit at the exact integer hex positions
// this package's checks query.
//
// An encounter with no room (InitRoom never called) falls back to the
// pre-wave-1 radius-only LoS and unblocked movement throughout this package —
// every room-aware call site is nil-checked.
//
// Wall-pattern generation is entropy-seeded by default (rpg-toolkit#787):
// two InitRoom calls with the same pattern produce different layouts. Pass
// an explicit seed for a reproducible layout instead -- e.g. devseed
// fixtures or regression tests. Only the first seed argument is used; it's
// optional so most callers (including rpg-api's spawn-engine wiring today)
// never need to think about it. See environments.QuickRoom.
func (e *Encounter) InitRoom(width, height int, pattern string, seed ...int64) error {
	room, err := environments.QuickRoom(width, height, pattern, seed...)
	if err != nil {
		return fmt.Errorf("build room: %w", err)
	}
	e.data.Space = &SpaceData{
		Walls:  snapshotWalls(room),
		Width:  width,
		Height: height,
	}
	return e.rebuildRoomFromData()
}

// Room returns the encounter's spatial room, or nil if InitRoom was never
// called (or LoadFromData found no persisted Space). Not stable across a
// door mutation — see RoomOrchestrator's doc.
func (e *Encounter) Room() spatial.Room { return e.room }

// RoomOrchestrator returns the orchestrator the encounter's room is
// registered with, or nil under the same conditions as Room. Exposed so a
// host (e.g. rpg-api's spawn-engine wiring) can share the exact same room
// instance the encounter uses for LoS/movement, rather than reconstructing
// its own from Data.Space.
//
// Not stable across a door mutation (rpg-toolkit#790): AddDoor/OpenDoor
// call rebuildRoomFromData, which replaces e.room/e.roomOrchestrator with
// FRESH objects rather than mutating the existing ones. A caller holding a
// reference from before such a call is holding stale geometry — re-fetch
// Room()/RoomOrchestrator() after any door mutates rather than caching
// them for the encounter's lifetime.
func (e *Encounter) RoomOrchestrator() spatial.RoomOrchestrator { return e.roomOrchestrator }

// snapshotWalls converts a built room's wall entities into the persisted
// WallSegmentData snapshot. One degenerate (Start == End) segment per
// discretized wall hex — see SpaceData.Walls doc. Positions are rounded to
// the nearest integer hex cell before converting to cube coordinates — see
// InitRoom's doc for why this matters (environments' wall generator doesn't
// snap to hex cells on its own).
//
// Adjacent discretized wall entities from the same original segment can
// round to the SAME cell, so entries are deduplicated by cube coordinate,
// OR-merging the blocking flags. This is load-bearing, not tidiness:
// rebuildRoomFromData places each entry via room.PlaceEntity, which rejects
// placing a blocking entity onto an already-occupied hex — a duplicate entry
// would make every InitRoom/LoadFromData of that snapshot fail (Copilot
// review, PR #759).
func snapshotWalls(room spatial.Room) []environments.WallSegmentData {
	entities := environments.GetWallEntitiesInRoom(room)
	walls := make([]environments.WallSegmentData, 0, len(entities))
	seen := make(map[spatial.CubeCoordinate]int, len(entities))
	for _, w := range entities {
		pos := w.GetPosition()
		rounded := spatial.Position{X: math.Round(pos.X), Y: math.Round(pos.Y)}
		cube := spatial.OffsetCoordinateToCubeWithOrientation(rounded, spatial.HexOrientationPointyTop)
		if i, dup := seen[cube]; dup {
			walls[i].BlocksMovement = walls[i].BlocksMovement || w.BlocksMovement()
			walls[i].BlocksLoS = walls[i].BlocksLoS || w.BlocksLineOfSight()
			continue
		}
		seen[cube] = len(walls)
		walls = append(walls, environments.WallSegmentData{
			Start:          cube,
			End:            cube,
			BlocksMovement: w.BlocksMovement(),
			BlocksLoS:      w.BlocksLineOfSight(),
		})
	}
	return walls
}

// registerRoom wires room into a fresh orchestrator for this Encounter
// instance. Both are transient — reconstructed at New/LoadFromData exactly
// like e.bus and e.combatants, never serialized.
func (e *Encounter) registerRoom(room spatial.Room) error {
	orch := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID: spatial.OrchestratorID(e.data.ID),
	})
	if err := orch.AddRoom(room); err != nil {
		return fmt.Errorf("register room: %w", err)
	}
	e.room = room
	e.roomOrchestrator = orch
	return nil
}

// rebuildRoomFromData reconstructs the spatial room + orchestrator from a
// persisted Data.Space snapshot (the LoadFromData path). Walls are placed
// exactly as persisted, not regenerated from a pattern — the snapshot
// decision (SpaceData doc) exists precisely so this is a replay, not a
// re-roll. No-op when data.Space is nil (no room to rebuild).
//
// rpg-toolkit#790: also places a blocking wall entity at every CLOSED
// door's Position, derived fresh from Data.Doors each call rather than
// persisted into Space.Walls — DoorData stays the single source of door
// truth, and reuses the exact same wall machinery (PlaceEntity,
// BlocksMovement/BlocksLoS, room.IsLineOfSightBlocked/CanPlaceEntity) that
// already gates movement and LoS for solid walls, instead of a parallel
// door-blocking system. An OPEN door places nothing, so re-running this
// after OpenDoor flips door.Open naturally drops that door's block.
func (e *Encounter) rebuildRoomFromData() error {
	sd := e.data.Space
	if sd == nil {
		return nil
	}
	grid := spatial.NewHexGrid(spatial.HexGridConfig{
		Width:       float64(sd.Width),
		Height:      float64(sd.Height),
		Orientation: spatial.HexOrientationPointyTop,
	})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   string(e.data.ID),
		Type: "encounter_room",
		Grid: grid,
	})
	placed := make(map[spatial.CubeCoordinate]struct{}, len(sd.Walls)+len(e.data.Doors))
	for i, w := range sd.Walls {
		// snapshotWalls dedupes on write; this read-side skip additionally
		// tolerates a hand-built or legacy snapshot carrying duplicates —
		// PlaceEntity rejects stacking a blocking entity on an occupied hex,
		// which would otherwise fail the whole load.
		if _, dup := placed[w.Start]; dup {
			continue
		}
		placed[w.Start] = struct{}{}
		pos := w.Start.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		entity := environments.NewWallEntity(environments.WallEntityConfig{
			SegmentID: fmt.Sprintf("space-%d", i),
			// Wave 1 doesn't persist WallType (no destruction model yet —
			// see design doc Wave 2+); Indestructible is the correct default
			// since BlocksMovement/BlocksLoS (the only behavior wave 1
			// exercises) are preserved from the original snapshot regardless.
			WallType: environments.WallTypeIndestructible,
			Properties: environments.WallProperties{
				BlocksMovement: w.BlocksMovement,
				BlocksLoS:      w.BlocksLoS,
			},
			Position: pos,
		})
		if err := room.PlaceEntity(entity, pos); err != nil {
			return fmt.Errorf("place wall %d: %w", i, err)
		}
	}
	for id, door := range e.data.Doors {
		if door.Open {
			continue
		}
		cube := door.Position.ToCube()
		// A door co-located with a solid wall cell (shouldn't happen by
		// design, but PlaceEntity would reject stacking a second blocker on
		// an occupied hex) is skipped defensively rather than failing the
		// whole rebuild — the existing wall already blocks that cell.
		if _, dup := placed[cube]; dup {
			continue
		}
		placed[cube] = struct{}{}
		pos := door.Position.ToPosition()
		entity := environments.NewWallEntity(environments.WallEntityConfig{
			SegmentID: fmt.Sprintf("door-%s", id),
			WallType:  environments.WallTypeIndestructible,
			Properties: environments.WallProperties{
				BlocksMovement: true,
				BlocksLoS:      true,
			},
			Position: pos,
		})
		if err := room.PlaceEntity(entity, pos); err != nil {
			return fmt.Errorf("place door wall %s: %w", id, err)
		}
	}
	if err := validateObstacles(sd.Obstacles); err != nil {
		return fmt.Errorf("validate obstacles: %w", err)
	}
	for _, o := range sd.Obstacles {
		// Unlike walls/doors, an obstacle's occupancy conflict is NOT
		// silently tolerated — obstacles are host-authored (via
		// AddObstacle), not generator output with rounding artifacts, so a
		// hex collision with an existing wall/door/obstacle is a genuine
		// data error the caller must see (rpg-toolkit#818 done bar), not a
		// dedup candidate. room.PlaceEntity itself rejects placing a
		// blocking entity onto a hex an existing blocker occupies, which
		// naturally surfaces that error here.
		pos := o.Position.ToPosition()
		entity := environments.NewWallEntity(environments.WallEntityConfig{
			SegmentID: fmt.Sprintf("obstacle-%s", o.ID),
			WallType:  environments.WallTypeIndestructible,
			Properties: environments.WallProperties{
				BlocksMovement: o.BlocksMovement,
				BlocksLoS:      o.BlocksLoS,
			},
			Position: pos,
		})
		if err := room.PlaceEntity(entity, pos); err != nil {
			return fmt.Errorf("place obstacle %q: %w", o.ID, err)
		}
	}
	return e.registerRoom(room)
}

// validateObstacles checks the structural invariant AddObstacle-authored
// obstacle data depends on: every obstacle must have a non-empty, unique
// ID. Obstacles is an order-preserving SLICE (unlike the map-keyed
// Doors), so a duplicate ID is not rejected implicitly by key-overwrite —
// room.PlaceEntity keys placement by entity.GetID(), so two obstacles
// sharing an ID would silently relocate one another's room occupancy
// (PlaceEntity's own doc: "Remove entity from old position if it exists")
// rather than erroring, corrupting the room without this check. Run from
// rebuildRoomFromData so both AddObstacle and a direct LoadFromData of
// hand-built/legacy Data get the same guarantee.
func validateObstacles(obstacles []ObstacleData) error {
	seen := make(map[core.EntityID]int, len(obstacles))
	for i, o := range obstacles {
		if o.ID == "" {
			return fmt.Errorf("obstacle %d: id required", i)
		}
		if first, dup := seen[o.ID]; dup {
			return fmt.Errorf("obstacle %d (%q): duplicate obstacle id (already used by obstacle %d)", i, o.ID, first)
		}
		seen[o.ID] = i
	}
	return nil
}

// AddObstacle registers a static obstacle instance into the encounter's
// SpaceData (rpg-toolkit#818): a generic, content-agnostic blocker at a
// hex, distinct from a Wall (a boundary-edge segment) or a Door (a
// connector with open/locked state). Mirrors AddDoor's atomicity: on
// rebuild failure the newly-appended obstacle is rolled back so a failed
// AddObstacle leaves Data.Space/e.room exactly as they were before the
// call — no partial state (rpg-toolkit#818 done bar).
//
// Unlike AddDoor, which is a no-op on the room side when there's no room
// yet (Data.Doors persists regardless), an obstacle's ONLY representation
// is a SpaceData.Obstacles entry — there is nowhere to persist it without
// a Space to attach to, so AddObstacle requires InitRoom/InitDungeon to
// have already run.
//
// id must be non-empty and not already used by another obstacle in this
// Space (see validateObstacles for why a duplicate ID is unsafe, not just
// unwanted). position, blocksMovement, and blocksLoS are stored verbatim
// and drive the entity rebuildRoomFromData places into the room.
func (e *Encounter) AddObstacle(
	id core.EntityID, ref string, position core.Hex, blocksMovement, blocksLoS bool,
) error {
	if e.data.Space == nil {
		return errors.New("add obstacle: no room initialized (call InitRoom/InitDungeon first)")
	}
	if id == "" {
		return errors.New("add obstacle: id required")
	}
	for _, existing := range e.data.Space.Obstacles {
		if existing.ID == id {
			return fmt.Errorf("add obstacle: obstacle %q already exists", id)
		}
	}
	e.data.Space.Obstacles = append(e.data.Space.Obstacles, ObstacleData{
		ID:             id,
		Ref:            ref,
		Position:       position,
		BlocksMovement: blocksMovement,
		BlocksLoS:      blocksLoS,
	})
	if err := e.rebuildRoomFromData(); err != nil {
		e.data.Space.Obstacles = e.data.Space.Obstacles[:len(e.data.Space.Obstacles)-1]
		return fmt.Errorf("add obstacle %q: rebuild room: %w", id, err)
	}
	return nil
}

// truncateAtWall returns the prefix of path up to (not including) the first
// wall-blocked hex, checked via room.CanPlaceEntity (which also rejects
// out-of-bounds hexes via the grid's IsValidPosition). Nil room (no
// SpaceData) returns path unchanged — the pre-wave-1 unblocked-movement
// behavior. Used by both the player (Move) and NPC (applyNPCMovementSteps)
// movement paths so neither can walk through a wall.
func (e *Encounter) truncateAtWall(path []core.Hex) []core.Hex {
	if e.room == nil {
		return path
	}
	for i, hex := range path {
		if !e.room.CanPlaceEntity(wallCheckEntity{}, hex.ToPosition()) {
			return path[:i]
		}
	}
	return path
}

// wallCheckEntity is a throwaway core.Entity used to query room.CanPlaceEntity
// for wall-blocking purposes. Players and monsters are never themselves
// placed into the spatial room (wave 1 doesn't track their positions there —
// only walls occupy it), so this identity is never compared against a real
// occupant; it exists only to satisfy the core.Entity parameter.
type wallCheckEntity struct{}

func (wallCheckEntity) GetID() string               { return "wall-check" }
func (wallCheckEntity) GetType() rpgcore.EntityType { return "wall-check" }
