package encounter

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Region IDs InitTwoChamberRoom assigns. Hosts (e.g. rpg-api's per-chamber
// monster seeding, design doc §Q5) key off these constants rather than
// hardcoding the strings.
const (
	// RegionChamber1 tags chamber 1's hexes — the entrance chamber.
	RegionChamber1 = "chamber-1"
	// RegionChamber2 tags chamber 2's hexes — across the door.
	RegionChamber2 = "chamber-2"
)

// chamberPathWidth is the corridor width reserved for the required
// entrance-to-door (and door-to-chamber) connectivity paths, and the
// MinPathWidth fed to each chamber's wall generation — matches
// RandomPattern's own margin heuristic (generateRandomWall: margin =
// max(2, MinPathWidth)), so the reserved path and the walls it clears
// agree on scale.
const chamberPathWidth = 2.0

// TwoChamberRoomParams configures Encounter.InitTwoChamberRoom.
type TwoChamberRoomParams struct {
	// ChamberWidth and ChamberHeight size EACH chamber (not the combined
	// space) — minimum 4x4, the same floor RandomPattern's margin
	// heuristic needs to leave any interior open. The combined space's
	// Width is 2*ChamberWidth+1 (one column reserved for the shared
	// boundary + doorway); Height is ChamberHeight.
	ChamberWidth, ChamberHeight int

	// Pattern is the interior wall pattern generated independently for
	// EACH chamber (e.g. environments.PatternRandom for tactical cover,
	// environments.PatternEmpty for none). Defaults to
	// environments.PatternRandom when empty.
	Pattern string

	// RandomSeed reproduces the WHOLE layout (both chambers' interior
	// walls) when non-zero — entropy-seeded otherwise, matching InitRoom
	// (rpg-toolkit#787).
	RandomSeed int64

	// DoorID is the entity id for the plain door generated between the
	// two chambers. Required — mirrors AddDoor, which this composes with
	// internally.
	DoorID core.EntityID
}

// InitTwoChamberRoom builds a two-chamber dungeon: two chambers placed
// side by side in ONE continuous Space (design doc Fork 1 — chambers are
// a cheap region TAG on SpaceData, not separate spatial.Rooms), connected
// by a single plain door in their shared boundary wall, with a designated
// entrance cell in chamber 1 (SpaceData.Entrance — replaces the
// roomCenterHex() placeholder downstream, rpg-api#648) and per-chamber
// region tags (SpaceData.Regions) for spawn placement and, via LoS,
// combat pockets (rpg-toolkit#796).
//
// Call once, right after New, exactly like InitRoom — which this doesn't
// reuse directly (single continuous space, not two independent QuickRoom
// calls) but composes the same environments wall-pattern machinery.
//
// Connectivity is guaranteed BY CONSTRUCTION, not validated after the
// fact: each chamber's interior wall pattern reserves a straight-line
// path — chamber 1 from its entrance to its door-adjacent cell, chamber 2
// from its door-adjacent cell to its own midpoint — via the same
// PathSafetyParams.RequiredPaths mechanism RandomPattern already uses for
// shape connections.
//
// The door is added via AddDoor — closed by default, blocking movement
// and LoS through rpg-toolkit#790's existing wall machinery — so the two
// chambers start disconnected from a player's perspective until OpenDoor
// runs, exactly like a hand-placed door.
func (e *Encounter) InitTwoChamberRoom(params TwoChamberRoomParams) error {
	if params.ChamberWidth < 4 || params.ChamberHeight < 4 {
		return fmt.Errorf("chamber dimensions must be at least 4x4 (got %dx%d)",
			params.ChamberWidth, params.ChamberHeight)
	}
	if params.DoorID == "" {
		return fmt.Errorf("door id required")
	}

	layout, err := generateTwoChamberLayout(params)
	if err != nil {
		return fmt.Errorf("generate two-chamber layout: %w", err)
	}

	e.data.Space = &SpaceData{
		Walls:    layout.walls,
		Width:    layout.width,
		Height:   layout.height,
		Entrance: core.HexFromCube(layout.entrance),
		Regions: []RegionData{
			{ID: RegionChamber1, Hexes: core.NewHexSet(hexesFromCubes(layout.chamber1)...)},
			{ID: RegionChamber2, Hexes: core.NewHexSet(hexesFromCubes(layout.chamber2)...)},
		},
	}

	return e.AddDoor(params.DoorID, core.HexFromCube(layout.door), false)
}

// hexesFromCubes converts a slice of spatial.CubeCoordinate to encounter
// Hexes — the boundary conversion between tools/spatial's coordinate math
// and this package's Hex type (mirrors core.HexFromCube for a single
// value).
func hexesFromCubes(cubes []spatial.CubeCoordinate) []core.Hex {
	out := make([]core.Hex, len(cubes))
	for i, c := range cubes {
		out[i] = core.HexFromCube(c)
	}
	return out
}

// twoChamberLayout is the geometry result of generateTwoChamberLayout,
// entirely in tools/spatial's coordinate types. Kept private — it's an
// intermediate; InitTwoChamberRoom is the only caller, and a host reads
// the result back off SpaceData/DoorData after the call, not off this.
type twoChamberLayout struct {
	walls         []environments.WallSegmentData
	width, height int
	chamber1      []spatial.CubeCoordinate
	chamber2      []spatial.CubeCoordinate
	entrance      spatial.CubeCoordinate
	door          spatial.CubeCoordinate
}

// generateTwoChamberLayout builds two independently wall-generated
// chambers side by side, joined by exactly one doorway gap in their
// shared boundary column. See InitTwoChamberRoom's doc for the
// construction argument.
//
// Layout (offset coordinates): chamber 1 occupies columns
// [0, ChamberWidth); the boundary column sits at ChamberWidth (every row
// solid except doorRow, which carries no wall — that cell is the door);
// chamber 2 occupies columns [ChamberWidth+1, 2*ChamberWidth+1). The
// entrance sits at chamber 1's far edge (column 0, doorRow) — "just
// inside chamber 1's entrance, not center" per the design doc's playtest
// script.
func generateTwoChamberLayout(params TwoChamberRoomParams) (*twoChamberLayout, error) {
	pattern := params.Pattern
	if pattern == "" {
		pattern = environments.PatternRandom
	}

	seed := params.RandomSeed
	if seed == 0 {
		//nolint:gosec // G404: deterministic game generation, not cryptographic
		seed = rand.Int63()
	}
	// Both chambers derive independent (but reproducible) sub-seeds from
	// the one input seed, so a caller-supplied seed reproduces the WHOLE
	// layout without the two chambers rolling identical wall patterns.
	//nolint:gosec // G404: deterministic derivation from the caller's seed
	sub := rand.New(rand.NewSource(seed))
	seed1, seed2 := sub.Int63(), sub.Int63()

	doorRow := params.ChamberHeight / 2
	entranceLocal := spatial.Position{X: 0, Y: float64(doorRow)}
	doorAdjacent1 := spatial.Position{X: float64(params.ChamberWidth - 1), Y: float64(doorRow)}
	doorAdjacent2 := spatial.Position{X: 0, Y: float64(doorRow)}
	center2 := spatial.Position{X: float64(params.ChamberWidth) / 2, Y: float64(doorRow)}

	walls1, err := generateChamberWalls(params.ChamberWidth, params.ChamberHeight, pattern, seed1,
		[]environments.Path{
			{From: entranceLocal, To: doorAdjacent1, Width: chamberPathWidth, Purpose: "entrance-to-door"},
		})
	if err != nil {
		return nil, fmt.Errorf("generate chamber 1 walls: %w", err)
	}
	walls2, err := generateChamberWalls(params.ChamberWidth, params.ChamberHeight, pattern, seed2,
		[]environments.Path{
			{From: doorAdjacent2, To: center2, Width: chamberPathWidth, Purpose: "door-to-chamber"},
		})
	if err != nil {
		return nil, fmt.Errorf("generate chamber 2 walls: %w", err)
	}

	chamber2OffsetX := params.ChamberWidth + 1

	segs := chamberWallSegments(walls1, 0, 0)
	segs = append(segs, chamberWallSegments(walls2, chamber2OffsetX, 0)...)

	// The boundary "party wall": every cell in the shared column except
	// doorRow, which carries no wall — that gap cell is the door itself.
	for y := 0; y < params.ChamberHeight; y++ {
		if y == doorRow {
			continue
		}
		cube := spatial.OffsetCoordinateToCubeWithOrientation(
			spatial.Position{X: float64(params.ChamberWidth), Y: float64(y)}, spatial.HexOrientationPointyTop)
		segs = append(segs, environments.WallSegmentData{Start: cube, End: cube, BlocksMovement: true, BlocksLoS: true})
	}

	return &twoChamberLayout{
		walls:    segs,
		width:    2*params.ChamberWidth + 1,
		height:   params.ChamberHeight,
		chamber1: chamberCubes(params.ChamberWidth, params.ChamberHeight, 0),
		chamber2: chamberCubes(params.ChamberWidth, params.ChamberHeight, chamber2OffsetX),
		entrance: spatial.OffsetCoordinateToCubeWithOrientation(entranceLocal, spatial.HexOrientationPointyTop),
		door: spatial.OffsetCoordinateToCubeWithOrientation(
			spatial.Position{X: float64(params.ChamberWidth), Y: float64(doorRow)}, spatial.HexOrientationPointyTop),
	}, nil
}

// generateChamberWalls generates one chamber's interior wall pattern,
// reusing the same pattern registry (environments.WallPatterns) and
// default rectangle shape InitRoom's QuickRoom uses internally. Called
// directly against the pattern function — bypassing
// environments.BasicRoomBuilder — because
// BasicRoomBuilder.generateWalls unconditionally overwrites
// PatternParams.Safety.RequiredPaths with paths derived from the shape's
// own connections, which would silently drop the entrance/door
// connectivity guarantee this generator depends on.
func generateChamberWalls(
	width, height int, pattern string, seed int64, requiredPaths []environments.Path,
) ([]environments.WallSegment, error) {
	patternFunc, ok := environments.WallPatterns[pattern]
	if !ok {
		return nil, fmt.Errorf("unknown wall pattern %q", pattern)
	}
	size := spatial.Dimensions{Width: float64(width), Height: float64(height)}
	shape := environments.ScaleShape(environments.GetDefaultShapes()[environments.ShapeRectangle], size)
	params := environments.PatternParams{
		Density:           0.4,
		DestructibleRatio: 0.7,
		RandomSeed:        seed,
		Material:          "stone",
		WallHeight:        3.0,
		Safety: environments.PathSafetyParams{
			MinPathWidth:      chamberPathWidth,
			MinOpenSpace:      0.6,
			EntitySize:        1.0,
			RequiredPaths:     requiredPaths,
			EmergencyFallback: true,
		},
	}
	return patternFunc(context.Background(), shape, size, params)
}

// chamberWallSegments discretizes one chamber's raw (continuous-position)
// wall segments into per-hex WallSegmentData in the COMBINED space's
// absolute coordinates, translating by offsetX/offsetY before rounding.
// Mirrors this package's own snapshotWalls (space.go) — same rounding and
// cube-coordinate dedup, because generated walls don't naturally align to
// hex cells (see InitRoom's doc) — but operates directly on raw
// WallSegments via environments.CreateWallEntities rather than a
// spatial.Room, since there's no single-chamber Room to snapshot here
// (the two chambers only ever exist merged, in the encounter's one
// e.room).
func chamberWallSegments(walls []environments.WallSegment, offsetX, offsetY int) []environments.WallSegmentData {
	entities := environments.CreateWallEntities(walls)
	out := make([]environments.WallSegmentData, 0, len(entities))
	seen := make(map[spatial.CubeCoordinate]int, len(entities))
	for _, ent := range entities {
		we, ok := ent.(*environments.WallEntity)
		if !ok {
			continue
		}
		pos := we.GetPosition()
		rounded := spatial.Position{
			X: math.Round(pos.X) + float64(offsetX),
			Y: math.Round(pos.Y) + float64(offsetY),
		}
		cube := spatial.OffsetCoordinateToCubeWithOrientation(rounded, spatial.HexOrientationPointyTop)
		if i, dup := seen[cube]; dup {
			out[i].BlocksMovement = out[i].BlocksMovement || we.BlocksMovement()
			out[i].BlocksLoS = out[i].BlocksLoS || we.BlocksLineOfSight()
			continue
		}
		seen[cube] = len(out)
		out = append(out, environments.WallSegmentData{
			Start: cube, End: cube,
			BlocksMovement: we.BlocksMovement(),
			BlocksLoS:      we.BlocksLineOfSight(),
		})
	}
	return out
}

// chamberCubes enumerates every hex in a width x height chamber (offset
// coordinates x in [0,width), y in [0,height), translated by offsetX) as
// absolute cube coordinates — a region tag's full membership, wall and
// floor cells alike (callers needing walkable-only cells filter against
// Walls).
func chamberCubes(width, height, offsetX int) []spatial.CubeCoordinate {
	out := make([]spatial.CubeCoordinate, 0, width*height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			out = append(out, spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x + offsetX), Y: float64(y)}, spatial.HexOrientationPointyTop))
		}
	}
	return out
}
