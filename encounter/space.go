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

// validateCanvasSpace validates the durable canvas floor and semantic facts as
// one snapshot. Viewer ZoneIDs are validated against the same immutable scope
// set, never against a coordinate-derived replacement.
func validateCanvasSpace(space *SpaceData, players map[core.PlayerID]*PlayerData) error {
	if _, err := ValidateCanvasDimensions(space.Width, space.Height); err != nil {
		return fmt.Errorf("validate canvas dimensions: %w", err)
	}
	if len(space.Regions) != 0 {
		return fmt.Errorf("canvas space must not contain room-chain regions")
	}
	if err := validateSemanticRegionData(space.SemanticRegions, canvasFloorHexes(space.Width, space.Height)); err != nil {
		return fmt.Errorf("validate semantic regions: %w", err)
	}
	if err := validateObservedZoneIDs(players, space); err != nil {
		return fmt.Errorf("validate viewer zone observations: %w", err)
	}
	return nil
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
// generated connector door's Position, derived fresh from Data.Doors each
// call rather than persisted into Space.Walls. Authored doors are edge-native:
// their normalized two-endpoint AuthoredEdge registers a spatial Boundary when
// closed and no boundary when open, never a cell blocker. DoorData remains the
// single source of each door's live open state in both representations.
func (e *Encounter) rebuildRoomFromData() error {
	sd := e.data.Space
	if sd == nil {
		return nil
	}
	if err := validatePersistedDoors(e.data.Doors); err != nil {
		return fmt.Errorf("validate doors: %w", err)
	}
	if err := validatePersistedAuthoredEdges(sd, e.data.Doors); err != nil {
		return fmt.Errorf("validate authored edges: %w", err)
	}
	// An omitted marker is legacy room-chain data. Canvas is explicit and its
	// complete structural floor is derived from the already-persisted dimensions.
	source, err := floorSourceKind(sd.FloorSource)
	if err != nil {
		return fmt.Errorf("validate floor source: %w", err)
	}
	if source == FloorSourceCanvas {
		if err := validateCanvasSpace(sd, e.data.Players); err != nil {
			return err
		}
	}
	authoredByKey := authoredEdgesByKey(sd)
	if _, err := e.canonicalGeneratedEdgeRecordsWithOverlay(authoredByKey); err != nil {
		return fmt.Errorf("validate effective generated edges: %w", err)
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
	// placeWallBlocker places (or, if cube is already occupied, no-ops) an
	// indestructible blocking WallEntity at cube — shared by both the
	// degenerate and boundary-edge branches below so they can't drift on
	// WallType/dedup semantics.
	placeWallBlocker := func(segmentID string, cube spatial.CubeCoordinate, blocksMovement, blocksLoS bool) error {
		if _, dup := placed[cube]; dup {
			return nil
		}
		placed[cube] = struct{}{}
		pos := cube.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		entity := environments.NewWallEntity(environments.WallEntityConfig{
			SegmentID: segmentID,
			// Wave 1 doesn't persist WallType (no destruction model yet —
			// see design doc Wave 2+); Indestructible is the correct default
			// since BlocksMovement/BlocksLoS (the only behavior wave 1
			// exercises) are preserved from the original snapshot regardless.
			WallType: environments.WallTypeIndestructible,
			Properties: environments.WallProperties{
				BlocksMovement: blocksMovement,
				BlocksLoS:      blocksLoS,
			},
			Position: pos,
		})
		return room.PlaceEntity(entity, pos)
	}
	for i, w := range sd.Walls {
		if w.Start == w.End {
			// snapshotWalls dedupes on write; this read-side skip
			// additionally tolerates a hand-built or legacy snapshot
			// carrying duplicates — PlaceEntity rejects stacking a
			// blocking entity on an occupied hex, which would otherwise
			// fail the whole load.
			if err := placeWallBlocker(fmt.Sprintf("space-%d", i), w.Start, w.BlocksMovement, w.BlocksLoS); err != nil {
				return fmt.Errorf("place wall %d: %w", i, err)
			}
			continue
		}
		if _, replaced := authoredByKey[newGeneratedEdgeKey(
			core.HexFromCube(w.Start), core.HexFromCube(w.End),
		)]; replaced {
			// An authored edge is the effective runtime barrier for this
			// physical crossing. In particular, do not retain the legacy
			// boundary segment's End-cell blocker underneath an authored door.
			continue
		}
		// A boundary-edge segment (Start != End) is primarily the render
		// contract (rpg-dnd5e-web#566's hexDistance==1 client branch)
		// catching up to the room's actual shape — Start is always real
		// walkable floor (placing a WallEntity there would wrongly block
		// legitimate floor), so Start itself never becomes a blocker here.
		// End is one of two cases:
		//   - outer perimeter (rpg-toolkit#834): End lies entirely outside
		//     this room's grid bounds — already unreachable by construction
		//     (every LOS/movement check already runs through
		//     spatial.HexGrid.IsValidPosition) — so placing anything there
		//     would either error or collide with an unrelated in-bounds
		//     cube on a differently-sized room. No-op, exactly as before.
		//   - connector column flanking cell (rpg-toolkit#848): End IS a
		//     valid in-grid position — a real, interior cell that must
		//     keep blocking movement/LOS even though it's no longer its
		//     own degenerate entry in sd.Walls. Gets its own blocker here.
		endPos := w.End.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		if !grid.IsValidPosition(endPos) {
			continue
		}
		if err := placeWallBlocker(fmt.Sprintf("space-%d-end", i), w.End, w.BlocksMovement, w.BlocksLoS); err != nil {
			return fmt.Errorf("place wall %d end: %w", i, err)
		}
	}
	for id, door := range e.data.Doors {
		// Authored doors register their two-cell boundary below. Keeping them
		// out of this legacy connector cell-blocker loop preserves placeability
		// at both authored endpoints.
		if door.Open || e.isAuthoredDoor(id) {
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
	if len(sd.AuthoredEdges) > 0 {
		for _, edge := range sd.AuthoredEdges {
			blocks := edge.Kind == GeneratedEdgeKindSolid
			if edge.Kind == GeneratedEdgeKindDoor {
				blocks = !e.data.Doors[edge.DoorID].Open
			}
			if !blocks {
				continue // Open authored doors deliberately register no blocker.
			}
			if err := room.RegisterBoundary(spatial.Boundary{
				From:              edge.From.ToPosition(),
				To:                edge.To.ToPosition(),
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			}); err != nil {
				return fmt.Errorf("register authored boundary %v to %v: %w", edge.From, edge.To, err)
			}
		}
	}
	if err := validateObstacles(sd.Obstacles); err != nil {
		return fmt.Errorf("validate obstacles: %w", err)
	}
	for _, o := range sd.Obstacles {
		cube := o.Position.ToCube()
		// Unlike walls/doors, an obstacle's occupancy conflict is NOT
		// tolerated at all — obstacles are either host-authored (via
		// AddObstacle) or generator-authored (rpg-toolkit#819's
		// InitDungeon + placeRegionObstacles, which composes crypt-style
		// callers like CryptDungeonParams) but never rounding artifacts
		// either way — AddObstacle takes an exact caller-given position,
		// and placeRegionObstacles only ever chooses from candidate
		// cells it has already confirmed are wall-free — so ANY hex
		// collision with an existing wall/door/obstacle is a genuine
		// data error the caller must see (rpg-toolkit#818 done bar), regardless of whether the colliding entities block
		// movement. room.PlaceEntity's own occupancy check
		// (canPlaceEntityUnsafe) only rejects placement when the EXISTING
		// occupant BlocksMovement() — so two BlocksMovement=false
		// obstacles (or an obstacle and a non-blocking wall) sharing a hex
		// would otherwise silently co-locate rather than erroring. Checked
		// explicitly against the shared `placed` occupancy set instead
		// (Copilot review, PR #823) — not delegated to PlaceEntity.
		if _, dup := placed[cube]; dup {
			return fmt.Errorf("place obstacle %q: hex %v already occupied", o.ID, cube)
		}
		placed[cube] = struct{}{}
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

// validateObstacles checks the structural invariants AddObstacle-authored
// obstacle data depends on: every obstacle has a non-empty, unique ID.
// Obstacles are ordinary content, not legacy wall/door cell geometry, so they
// may share an authored-edge endpoint and independently block that cell.
// Obstacles is an order-preserving SLICE (unlike the map-keyed Doors), so a
// duplicate ID is not rejected implicitly by key-overwrite. Uniqueness matters
// even though environments.WallEntity's derived entity ID embeds position
// (making a same-ID-different-position pair placement-safe on its own) — a
// stable ID is this package's contract for referencing the SAME obstacle across
// ticks/reloads (a host or client keying a future interaction verb off it), and
// a same-ID-SAME-position pair would collide to one entity via PlaceEntity's
// move-in-place semantics, silently swallowing what should be a rejected
// duplicate. Run from rebuildRoomFromData so both AddObstacle and a direct
// LoadFromData of hand-built/legacy Data get the same guarantee.
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
// unwanted). Obstacles may share an authored edge endpoint and independently
// block that cell; the authored boundary remains a separate crossing. blocksMovement
// and blocksLoS are stored verbatim and drive the entity rebuildRoomFromData
// places into the room.
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

// truncateAtWall returns the requested prefix whose ordered direct segments
// are fully traversable. It expands each sparse or multi-cell request through
// the room grid's ray, checking every interior cell blocker and every
// boundary crossing in order. A malformed, discontinuous, or out-of-grid ray
// fails closed at that requested segment. Nil room preserves the pre-wave-1
// unblocked-movement behavior. Used by both player and NPC movement paths.
func (e *Encounter) truncateAtWall(moverStart core.Hex, path []core.Hex) []core.Hex {
	if e.room == nil {
		return path
	}
	from := moverStart
	for i, to := range path {
		if !e.isRoomSegmentTraversable(from, to) {
			return path[:i]
		}
		from = to
	}
	return path
}

// truncateAtOccupiedDestination removes trailing destinations occupied by a
// different creature. Encounter Data is authoritative for creature positions:
// players and monsters deliberately are not placed in the reconstructed
// spatial.Room, which owns only walls, doors, and obstacles. Keeping this check
// in the encounter loop therefore reconciles the actual endpoint before rule
// resolution, economy spending, state mutation, or audience-projected events.
//
// Only the endpoint is constrained. Occupied intermediate cells remain in the
// path so this invariant does not silently introduce a movement-through rule.
// No creature-sharing exception exists in the current encounter contract; a
// future explicit rule must be tested and applied at this seam.
func (e *Encounter) truncateAtOccupiedDestination(moverID core.EntityID, path []core.Hex) []core.Hex {
	occupied := make(map[core.Hex]struct{}, len(e.data.Players)+len(e.data.Monsters))
	for _, player := range e.data.Players {
		if player.EntityID != moverID && player.View != nil {
			occupied[player.View.Position] = struct{}{}
		}
	}
	for _, monster := range e.data.Monsters {
		if monster.ID != moverID {
			occupied[monster.Position] = struct{}{}
		}
	}

	// Consult the full authoritative encounter state, never a viewer's
	// filtered perception. The set contains no occupant identities, so this
	// refusal path cannot leak a hidden creature.
	for len(path) > 0 {
		if _, blocked := occupied[path[len(path)-1]]; !blocked {
			break
		}
		path = path[:len(path)-1]
	}
	return path
}

// isRoomSegmentTraversable rejects any direct requested segment that cannot
// be verified as a complete, contiguous in-grid ray. Encounter movement does
// not place players or monsters in spatial.Room, so it must perform this
// crossing-aware validation before mutating its own position data.
func (e *Encounter) isRoomSegmentTraversable(from, to core.Hex) bool {
	if !from.ToCube().IsValid() || !to.ToCube().IsValid() {
		return false
	}
	grid := e.room.GetGrid()
	fromPos, toPos := from.ToPosition(), to.ToPosition()
	if !grid.IsValidPosition(fromPos) || !grid.IsValidPosition(toPos) {
		return false
	}
	// Cell blockers retain the caller-requested direct ray. A canonical ray is
	// only for undirected boundary crossings; applying it to cells would let a
	// reverse-only wall/obstacle disappear from sparse movement validation.
	ray := grid.GetLineOfSight(fromPos, toPos)
	if len(ray) == 0 || !ray[0].Equals(fromPos) || !ray[len(ray)-1].Equals(toPos) {
		return false
	}
	if len(ray)-1 != int(grid.Distance(fromPos, toPos)) {
		return false
	}
	for i, position := range ray {
		if !grid.IsValidPosition(position) {
			return false
		}
		if i == 0 {
			continue
		}
		previous := ray[i-1]
		if previous.Equals(position) || !grid.IsAdjacent(previous, position) {
			return false
		}
		if !e.room.CanPlaceEntity(wallCheckEntity{}, position) {
			return false
		}
	}

	boundaryRoom, boundaryAware := e.room.(spatial.BoundaryAwareRoom)
	if !boundaryAware {
		return true
	}
	boundaryRay := spatial.CanonicalBoundaryRay(grid, fromPos, toPos)
	if len(boundaryRay) == 0 || !boundaryRay[0].Equals(fromPos) ||
		!boundaryRay[len(boundaryRay)-1].Equals(toPos) ||
		len(boundaryRay)-1 != int(grid.Distance(fromPos, toPos)) {
		return false
	}
	for i := 1; i < len(boundaryRay); i++ {
		previous, position := boundaryRay[i-1], boundaryRay[i]
		if !grid.IsValidPosition(position) || previous.Equals(position) || !grid.IsAdjacent(previous, position) {
			return false
		}
		if boundaryRoom.IsBoundaryMovementBlocked(previous, position) {
			return false
		}
	}
	return true
}

// wallCheckEntity is a throwaway core.Entity used to query room.CanPlaceEntity
// for wall-blocking purposes. Players and monsters are never themselves
// placed into the spatial room (wave 1 doesn't track their positions there —
// only walls occupy it), so this identity is never compared against a real
// occupant; it exists only to satisfy the core.Entity parameter.
type wallCheckEntity struct{}

func (wallCheckEntity) GetID() string               { return "wall-check" }
func (wallCheckEntity) GetType() rpgcore.EntityType { return "wall-check" }
