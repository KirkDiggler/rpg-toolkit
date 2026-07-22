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

// dungeonPathWidth is the corridor width reserved for the required
// connectivity paths within each region, and the MinPathWidth fed to each
// region's wall generation — matches RandomPattern's own margin heuristic
// (generateRandomWall: margin = max(2, MinPathWidth)), so the reserved
// path and the walls it clears agree on scale. Mirrors two_chamber.go's
// chamberPathWidth (rpg-toolkit#806) — retired in favor of this shared
// constant now that InitTwoChamberRoom delegates to InitDungeon.
const dungeonPathWidth = 2.0

// DungeonParams configures Encounter.InitDungeon: an ordered linear chain
// of Regions joined by Connectors, emitted as ONE continuous Space (design
// doc Fork 1 — regions are tags on a Space, not separate spatial.Rooms).
// Generalizes TwoChamberRoomParams (rpg-toolkit#806) from a fixed N=2 to
// any N>=2 (rpg-toolkit#814).
type DungeonParams struct {
	// Regions is the ordered linear chain, entrance-side first. At least 2
	// required — a single region has nowhere to connect to.
	Regions []DungeonRegionParams

	// Connectors is the ordered list of doors joining consecutive regions:
	// Connectors[i] joins Regions[i] to Regions[i+1]. Must have exactly
	// len(Regions)-1 entries.
	Connectors []DungeonConnectorParams

	// Height is shared by every region in the chain — the combined space's
	// grid height. Matches TwoChamberRoomParams.ChamberHeight generalized:
	// the old generator used one ChamberHeight for both chambers; this
	// keeps that simplicity rather than inventing per-region vertical
	// offsets/padding, which isn't required by any #814 done-bar case.
	// Minimum 4, the same floor RandomPattern's margin heuristic needs.
	Height int

	// RandomSeed reproduces the WHOLE layout (every region's interior
	// walls) when non-zero — entropy-seeded otherwise, matching InitRoom /
	// InitTwoChamberRoom (rpg-toolkit#787).
	RandomSeed int64

	// Theme is opaque metadata copied verbatim to SpaceData.Theme — see
	// that field's doc. Never interpreted here.
	Theme string
}

// DungeonRegionParams configures one region in an InitDungeon chain.
type DungeonRegionParams struct {
	// ID is this region's tag in SpaceData.Regions / RegionAt. Caller-
	// assigned so hosts control naming (e.g. "entrance", "corridor",
	// "boss", or the legacy "chamber-1"/"chamber-2" InitTwoChamberRoom
	// preserves).
	ID string

	// Archetype identifies the region's generic role. See RegionArchetype.
	Archetype RegionArchetype

	// Width sizes this region (the shared Height comes from
	// DungeonParams.Height). Minimum 4, the same floor RandomPattern's
	// margin heuristic needs.
	Width int

	// Pattern is the interior wall pattern generated independently for
	// this region (e.g. environments.PatternRandom for tactical cover,
	// environments.PatternEmpty for none). Defaults to
	// environments.PatternRandom when empty.
	Pattern string

	// Obstacles are generic, content-agnostic physical set-piece specs
	// InitDungeon places into THIS region's floor as rpg-toolkit#818
	// ObstacleData instances (rpg-toolkit#819). Nil/empty for a region
	// with no set pieces — matches every #814/#817/#818 fixture, which
	// never sets this field and gets zero obstacles. InitDungeon never
	// interprets Ref/BlocksMovement/BlocksLoS; a themed caller (e.g. the
	// crypt template — see CryptDungeonParams) decides content, this
	// package only places it safely. See placeRegionObstacles for the
	// placement algorithm and its safety invariants.
	Obstacles []ObstacleSpec
}

// ObstacleSpec describes one kind of physical set-piece instance a caller
// wants InitDungeon to place into a specific region's floor —
// rpg-toolkit#819. Count instances sharing the same opaque Ref/
// BlocksMovement/BlocksLoS are each placed at their own safe hex within
// the region (never a wall, door, required-path, or primary-combat-axis
// cell — see placeRegionObstacles). Content-agnostic: encounter never
// interprets any field. Placement is best-effort: a region whose safe
// floor can't fit every requested instance places as many as DID fit
// (down to zero) rather than failing InitDungeon — rpg-toolkit#819's
// "a crypt missing one statue is fine; a crypt that fails to generate
// ... is not" done-bar requirement, generalized to any caller.
type ObstacleSpec struct {
	// Ref is copied verbatim into each placed ObstacleData.Ref — an
	// opaque content identifier (e.g. "dnd5e:obstacles:pillar") this
	// package never interprets.
	Ref string

	// Count is how many instances of this spec to attempt to place.
	Count int

	// BlocksMovement/BlocksLoS are copied verbatim into each placed
	// ObstacleData.
	BlocksMovement bool
	BlocksLoS      bool
}

// DungeonConnectorParams configures the door joining two consecutive
// regions in an InitDungeon chain.
type DungeonConnectorParams struct {
	// DoorID is the entity id for the door generated at this connector.
	// Required — mirrors AddDoor, which InitDungeon composes with
	// internally.
	DoorID core.EntityID

	// Locked marks this connector's generated door as closed AND locked
	// (rpg-toolkit#815), reusing DoorData's existing Wave 2.9 lock-state
	// fields verbatim: AttemptUnlock/SubmitCheck become the path through
	// it (issuing and resolving a skill-check prompt) until a player
	// passes the configured check; OpenDoor alone does not gate on it —
	// see DoorData.Locked's doc for the full contract. Zero value
	// (false) generates a plain closed door, same as every connector
	// before #815 and the InitTwoChamberRoom compatibility wrapper,
	// which never sets this field.
	Locked bool

	// LockDC is the skill-check DC AttemptUnlock issues when Locked is
	// true, copied verbatim onto the generated door's DoorData.LockDC.
	// Required (> 0) when Locked is true — validateDungeonParams rejects
	// a locked connector with LockDC<=0 before any generation runs.
	// Ignored when Locked is false.
	LockDC int

	// LockAbility is the 3-letter ability code (e.g. "DEX") the check
	// rolls against, copied verbatim onto DoorData.LockAbility. Required
	// (non-empty) when Locked is true — validateDungeonParams rejects a
	// locked connector with an empty LockAbility. Ignored when Locked is
	// false.
	LockAbility string

	// LockTool is an optional toolkit ref (e.g.
	// "dnd5e:item:thieves-tools") granting a tool-proficiency bonus on
	// the check, copied verbatim onto DoorData.LockTool. Empty means no
	// tool bonus applies; never required. Ignored when Locked is false.
	LockTool string
}

// InitDungeon builds an N-region linear-chain dungeon: an ordered list of
// regions placed side by side in ONE continuous Space, each pair of
// consecutive regions joined by a plain door, with a designated entrance
// cell in the first region (SpaceData.Entrance) and per-region archetype-
// tagged regions (SpaceData.Regions) for spawn placement and, via LoS,
// combat pockets. Generalizes InitTwoChamberRoom (rpg-toolkit#806) from a
// fixed two chambers to any N>=2 regions (rpg-toolkit#814).
//
// InitDungeon is atomic: either the whole dungeon (Space + every
// connector door) commits, or a failure leaves the encounter exactly as
// it was before the call. Connector doors are staged directly into
// e.data.Doors and committed via a SINGLE rebuildRoomFromData call —
// rather than looping e.AddDoor once per connector, which would rebuild
// the room N-1 times and, if a LATER connector's door failed to place,
// leave the earlier connectors' doors (and the freshly-set Space) behind
// despite returning an error. rebuildRoomFromData only swaps in
// e.room/e.roomOrchestrator (via registerRoom) after every wall/door in
// the batch places successfully, so a failure here never touches the
// room side; restoring data.Space and removing the doors staged by this
// call below completes the rollback.
func (e *Encounter) InitDungeon(params DungeonParams) error {
	if err := validateDungeonParams(params); err != nil {
		return err
	}

	layout, err := generateDungeonLayout(params)
	if err != nil {
		return fmt.Errorf("generate dungeon layout: %w", err)
	}

	previousSpace := e.data.Space
	e.data.Space = &SpaceData{
		Walls:     layout.walls,
		Width:     layout.width,
		Height:    params.Height,
		Entrance:  core.HexFromCube(layout.entrance),
		Regions:   layout.regions,
		Theme:     params.Theme,
		Obstacles: layout.obstacles,
	}

	stagedDoorIDs := make([]core.EntityID, 0, len(layout.doors))
	for i, door := range layout.doors {
		connector := params.Connectors[i]
		doorID := connector.DoorID
		dd := &DoorData{ID: doorID, Position: core.HexFromCube(door), Open: false}
		if connector.Locked {
			dd.Locked = true
			dd.LockDC = connector.LockDC
			dd.LockAbility = connector.LockAbility
			dd.LockTool = connector.LockTool
		}
		e.data.Doors[doorID] = dd
		stagedDoorIDs = append(stagedDoorIDs, doorID)
	}

	if err := e.rebuildRoomFromData(); err != nil {
		for _, id := range stagedDoorIDs {
			delete(e.data.Doors, id)
		}
		e.data.Space = previousSpace
		return fmt.Errorf("init dungeon: rebuild room: %w", err)
	}
	return nil
}

// validateDungeonParams checks the structural and scale invariants
// InitDungeon depends on: at least 2 regions, exactly len(Regions)-1
// connectors with non-empty and unique DoorIDs, every region with a
// non-empty and unique ID, a known Archetype, every region/height at
// least 4, and the boss-room scale invariant (rpg-toolkit#814 Approved
// Slice 3 corrections) — a generation-time assertion, not eyeballing.
func validateDungeonParams(params DungeonParams) error {
	if len(params.Regions) < 2 {
		return fmt.Errorf("dungeon needs at least 2 regions (got %d)", len(params.Regions))
	}
	if len(params.Connectors) != len(params.Regions)-1 {
		return fmt.Errorf("dungeon needs exactly %d connectors for %d regions (got %d)",
			len(params.Regions)-1, len(params.Regions), len(params.Connectors))
	}
	if params.Height < 4 {
		return fmt.Errorf("dungeon height must be at least 4 (got %d)", params.Height)
	}
	for i, r := range params.Regions {
		if r.Width < 4 {
			return fmt.Errorf("region %d (%q) width must be at least 4 (got %d)", i, r.ID, r.Width)
		}
	}
	// Region IDs must be non-empty and unique: SpaceData.RegionAt returns
	// the FIRST matching region for a given hex, so an empty or duplicate
	// ID either produces a meaningless tag or silently makes every hex in
	// a later region misreport as belonging to an earlier one.
	seenRegionIDs := make(map[string]int, len(params.Regions))
	for i, r := range params.Regions {
		if r.ID == "" {
			return fmt.Errorf("region %d: id required", i)
		}
		if first, dup := seenRegionIDs[r.ID]; dup {
			return fmt.Errorf("region %d (%q): duplicate region id (already used by region %d)", i, r.ID, first)
		}
		seenRegionIDs[r.ID] = i
	}
	// Archetype is documented as a fixed, reusable vocabulary (data.go) —
	// enforce that here rather than letting RegionArchetype's underlying
	// string type accept anything.
	for i, r := range params.Regions {
		switch r.Archetype {
		case ArchetypeEntrance, ArchetypeChamber, ArchetypeCorridor, ArchetypeBoss:
		default:
			return fmt.Errorf("region %d (%q): unknown archetype %q", i, r.ID, r.Archetype)
		}
	}
	seenDoorIDs := make(map[core.EntityID]int, len(params.Connectors))
	for i, c := range params.Connectors {
		if c.DoorID == "" {
			return fmt.Errorf("connector %d: door id required", i)
		}
		if first, dup := seenDoorIDs[c.DoorID]; dup {
			return fmt.Errorf("connector %d (%q): duplicate door id (already used by connector %d)", i, c.DoorID, first)
		}
		seenDoorIDs[c.DoorID] = i
	}
	// A locked connector's check config is validated contextually, before
	// InitDungeon mutates any encounter data: LockDC/LockAbility only mean
	// anything when Locked is true, and AttemptUnlock/SubmitCheck depend
	// on both being present (a zero DC is never a meaningful skill-check
	// target; an empty ability leaves the CharacterResolver nothing to
	// resolve a modifier against). Rejecting here — rather than letting a
	// half-configured locked door generate — mirrors the file's other
	// pre-mutation validation gates (duplicate IDs, unknown archetypes).
	for i, c := range params.Connectors {
		if !c.Locked {
			continue
		}
		if c.LockDC <= 0 {
			return fmt.Errorf("connector %d (%q): locked connector requires LockDC > 0 (got %d)", i, c.DoorID, c.LockDC)
		}
		if c.LockAbility == "" {
			return fmt.Errorf("connector %d (%q): locked connector requires LockAbility", i, c.DoorID)
		}
	}
	for i, r := range params.Regions {
		if r.Archetype != ArchetypeBoss {
			continue
		}
		axis := r.Width
		if params.Height < axis {
			axis = params.Height
		}
		if axis <= 6 {
			return fmt.Errorf(
				"region %d (%q): boss room primary playable axis must exceed 6 hex steps (got %d)",
				i, r.ID, axis)
		}
	}
	return nil
}

// dungeonLayout is the geometry result of generateDungeonLayout, entirely
// in tools/spatial's coordinate types. Kept private — InitDungeon is the
// only caller, and a host reads the result back off SpaceData/DoorData
// after the call, not off this. Mirrors two_chamber.go's (now retired)
// twoChamberLayout, generalized to N regions.
type dungeonLayout struct {
	walls    []environments.WallSegmentData
	width    int
	regions  []RegionData
	entrance spatial.CubeCoordinate
	// doors[i] is the door cube coordinate joining Regions[i] to
	// Regions[i+1] — parallel to params.Connectors.
	doors []spatial.CubeCoordinate
	// obstacles are every placed ObstacleSpec instance across every
	// region, in absolute coordinates — rpg-toolkit#819. See
	// placeRegionObstacles.
	obstacles []ObstacleData
}

// generateDungeonLayout builds each region's independently wall-generated
// interior, placed side by side in local-column order, joined by exactly
// one doorway gap per connector in the shared boundary column between
// consecutive regions.
//
// Layout (offset coordinates): region i occupies columns
// [start[i], start[i]+Width[i]); the boundary column between region i and
// i+1 sits at start[i]+Width[i] (every row solid except doorRow, which
// carries no wall — that cell is connector i's door). The entrance sits
// at region 0's far edge (column 0, doorRow) — "just inside the entrance,
// not center" per the design doc's playtest script, same as
// InitTwoChamberRoom.
//
// Connectivity is guaranteed BY CONSTRUCTION: region 0 reserves a
// required path from the entrance to its outgoing door; each interior
// region (index 1..N-2) reserves a required path spanning its full local
// width, incoming-door-adjacent to outgoing-door-adjacent; the terminal
// region (index N-1) reserves a required path from its incoming-door-
// adjacent cell to its own local center (not the far edge — nothing lies
// beyond it) — generalizing InitTwoChamberRoom's two-chamber required
// paths to N regions.
func generateDungeonLayout(params DungeonParams) (*dungeonLayout, error) {
	n := len(params.Regions)
	doorRow := params.Height / 2

	seed := params.RandomSeed
	if seed == 0 {
		//nolint:gosec // G404: deterministic game generation, not cryptographic
		seed = rand.Int63()
	}
	// Every region derives an independent (but reproducible) sub-seed from
	// the one input seed, so a caller-supplied seed reproduces the WHOLE
	// layout without every region rolling identical wall patterns.
	//nolint:gosec // G404: deterministic derivation from the caller's seed
	sub := rand.New(rand.NewSource(seed))
	regionSeeds := make([]int64, n)
	for i := range regionSeeds {
		regionSeeds[i] = sub.Int63()
	}
	// A second, independent per-region sub-seed for obstacle placement
	// (rpg-toolkit#819) — drawn from the SAME `sub` stream right after
	// every region's wall seed, so adding/removing/reordering
	// ObstacleSpecs never perturbs wall generation (regionSeeds is
	// already fully drawn above) and a caller-supplied seed reproduces
	// the WHOLE layout, obstacles included.
	obstacleSeeds := make([]int64, n)
	for i := range obstacleSeeds {
		obstacleSeeds[i] = sub.Int63()
	}

	starts := make([]int, n)
	x := 0
	for i, r := range params.Regions {
		starts[i] = x
		x += r.Width + 1 // +1 reserves the boundary/door column after this region
	}
	totalWidth := x - 1 // no trailing boundary column after the last region

	var segs []environments.WallSegmentData
	regions := make([]RegionData, n)
	doors := make([]spatial.CubeCoordinate, n-1)
	var obstacles []ObstacleData

	for i, r := range params.Regions {
		local := spatial.Position{X: 0, Y: float64(doorRow)}
		var requiredPaths []environments.Path
		switch {
		case n == 1:
			// unreachable: validateDungeonParams enforces n>=2
		case i == 0:
			// entrance region: entrance -> outgoing door.
			farEdge := spatial.Position{X: float64(r.Width - 1), Y: float64(doorRow)}
			requiredPaths = []environments.Path{
				{From: local, To: farEdge, Width: dungeonPathWidth, Purpose: "entrance-to-door"},
			}
		case i == n-1:
			// terminal region: incoming-door-adjacent -> local center.
			// Purpose names the destination generically ("region", not
			// "chamber") since the terminal region can be any archetype
			// (boss, chamber, ...) — this string only ever surfaces in
			// environments.validatePathSafety's "required path '%s' is
			// blocked" error, but it must still describe what's actually
			// there.
			center := spatial.Position{X: float64(r.Width) / 2, Y: float64(doorRow)}
			requiredPaths = []environments.Path{
				{From: local, To: center, Width: dungeonPathWidth, Purpose: "door-to-region-center"},
			}
		default:
			// interior region: incoming-door-adjacent -> outgoing-door-adjacent.
			farEdge := spatial.Position{X: float64(r.Width - 1), Y: float64(doorRow)}
			requiredPaths = []environments.Path{
				{From: local, To: farEdge, Width: dungeonPathWidth, Purpose: "door-to-door"},
			}
		}

		pattern := r.Pattern
		if pattern == "" {
			pattern = environments.PatternRandom
		}
		walls, err := generateRegionWalls(r.Width, params.Height, pattern, regionSeeds[i], requiredPaths)
		if err != nil {
			return nil, fmt.Errorf("generate region %d (%q) walls: %w", i, r.ID, err)
		}
		regionWalls := regionWallSegments(walls, starts[i], 0)
		segs = append(segs, regionWalls...)
		regions[i] = RegionData{
			ID:        r.ID,
			Archetype: r.Archetype,
			Hexes:     core.NewHexSet(hexesFromCubes(regionCubes(r.Width, params.Height, starts[i]))...),
		}
		obstacles = append(obstacles, placeRegionObstacles(placeRegionObstaclesParams{
			regionID:  r.ID,
			specs:     r.Obstacles,
			width:     r.Width,
			height:    params.Height,
			offsetX:   starts[i],
			doorRow:   doorRow,
			wallCubes: wallCubeSet(regionWalls),
			seed:      obstacleSeeds[i],
		})...)

		if i < n-1 {
			doorX := starts[i] + r.Width
			for y := 0; y < params.Height; y++ {
				if y == doorRow {
					continue
				}
				cube := spatial.OffsetCoordinateToCubeWithOrientation(
					spatial.Position{X: float64(doorX), Y: float64(y)}, spatial.HexOrientationPointyTop)
				segs = append(segs,
					environments.WallSegmentData{Start: cube, End: cube, BlocksMovement: true, BlocksLoS: true})
			}
			doors[i] = spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(doorX), Y: float64(doorRow)}, spatial.HexOrientationPointyTop)
		}
	}

	entrance := spatial.OffsetCoordinateToCubeWithOrientation(
		spatial.Position{X: 0, Y: float64(doorRow)}, spatial.HexOrientationPointyTop)

	return &dungeonLayout{
		walls:     segs,
		width:     totalWidth,
		regions:   regions,
		entrance:  entrance,
		doors:     doors,
		obstacles: obstacles,
	}, nil
}

// generateRegionWalls generates one region's interior wall pattern,
// reusing the same pattern registry (environments.WallPatterns) and
// default rectangle shape InitRoom's QuickRoom uses internally. Called
// directly against the pattern function — bypassing
// environments.BasicRoomBuilder — because BasicRoomBuilder.generateWalls
// unconditionally overwrites PatternParams.Safety.RequiredPaths with
// paths derived from the shape's own connections, which would silently
// drop the connectivity guarantee this generator depends on. Mirrors
// two_chamber.go's (now retired) generateChamberWalls.
func generateRegionWalls(
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
			MinPathWidth:      dungeonPathWidth,
			MinOpenSpace:      0.6,
			EntitySize:        1.0,
			RequiredPaths:     requiredPaths,
			EmergencyFallback: true,
		},
	}
	return patternFunc(context.Background(), shape, size, params)
}

// regionWallSegments discretizes one region's raw (continuous-position)
// wall segments into per-hex WallSegmentData in the COMBINED space's
// absolute coordinates, translating by offsetX/offsetY before rounding.
// Mirrors this package's own snapshotWalls (space.go) — same rounding and
// cube-coordinate dedup, because generated walls don't naturally align to
// hex cells (see InitRoom's doc) — but operates directly on raw
// WallSegments via environments.CreateWallEntities rather than a
// spatial.Room, since there's no single-region Room to snapshot here (all
// regions only ever exist merged, in the encounter's one e.room). Mirrors
// two_chamber.go's (now retired) chamberWallSegments.
func regionWallSegments(walls []environments.WallSegment, offsetX, offsetY int) []environments.WallSegmentData {
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

// wallCubeSet builds a lookup set of every absolute cube coordinate a
// region's wall segments occupy — used by placeRegionObstacles to reject
// wall cells as obstacle candidates. walls is already in absolute
// coordinates (the caller's regionWallSegments output); Start == End for
// every entry (see WallSegmentData's doc), so only Start is read.
func wallCubeSet(walls []environments.WallSegmentData) map[spatial.CubeCoordinate]struct{} {
	set := make(map[spatial.CubeCoordinate]struct{}, len(walls))
	for _, w := range walls {
		set[w.Start] = struct{}{}
	}
	return set
}

// placeRegionObstaclesParams bundles placeRegionObstacles' inputs — one
// region's geometry plus its caller-supplied specs — so the function
// signature doesn't grow an eighth positional argument as #819 evolves.
type placeRegionObstaclesParams struct {
	regionID  string
	specs     []ObstacleSpec
	width     int
	height    int
	offsetX   int
	doorRow   int
	wallCubes map[spatial.CubeCoordinate]struct{}
	seed      int64
}

// placeRegionObstacles computes the ObstacleData instances for every
// ObstacleSpec in one region — rpg-toolkit#819. Purely geometric and
// content-agnostic: it never reads Ref/BlocksMovement/BlocksLoS, only
// copies them verbatim.
//
// Candidates are every LOCAL floor cell (x in [0,width), y in [0,height))
// that is NOT a wall cell AND NOT on doorRow. Excluding the whole doorRow
// row — not just the narrower required-path segment
// generateDungeonLayout reserves for connectivity — is deliberately the
// SAME reservation for every region regardless of archetype: it is a
// superset of the required path (entrance-to-door / door-to-door /
// door-to-region-center all run along this exact row), and for a boss-
// archetype region it ALSO satisfies #819's additional "primary playable
// axis (>6 hex steps) must stay clear" invariant, because the row spans
// the region's FULL width — always at least the validateDungeonParams
// size floor already enforces at construction time. One rule, no
// archetype-specific branching, provably sufficient for both invariants.
//
// The candidate pool is shuffled with a seeded Fisher-Yates (deterministic
// per seed, varies across seeds) and specs draw from it in order, each
// instance consuming one candidate so no two specs (or two instances of
// the same spec) can collide. A spec that asks for more instances than
// the remaining pool has places as many as fit and drops the rest —
// #819's "skip rather than invalidate the dungeon" requirement — this
// function never errors.
func placeRegionObstacles(p placeRegionObstaclesParams) []ObstacleData {
	if len(p.specs) == 0 {
		return nil
	}
	candidates := make([]spatial.CubeCoordinate, 0, p.width*p.height)
	for x := 0; x < p.width; x++ {
		for y := 0; y < p.height; y++ {
			if y == p.doorRow {
				continue // reserved: required path / primary combat axis
			}
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x + p.offsetX), Y: float64(y)}, spatial.HexOrientationPointyTop)
			if _, blocked := p.wallCubes[cube]; blocked {
				continue
			}
			candidates = append(candidates, cube)
		}
	}
	//nolint:gosec // G404: deterministic per-region obstacle seed, not cryptographic
	rng := rand.New(rand.NewSource(p.seed))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	var out []ObstacleData
	next := 0
	for _, spec := range p.specs {
		for n := 0; n < spec.Count && next < len(candidates); n++ {
			out = append(out, ObstacleData{
				ID:             core.EntityID(fmt.Sprintf("obstacle-%s-%d", p.regionID, len(out))),
				Ref:            spec.Ref,
				Position:       core.HexFromCube(candidates[next]),
				BlocksMovement: spec.BlocksMovement,
				BlocksLoS:      spec.BlocksLoS,
			})
			next++
		}
	}
	return out
}

// regionCubes enumerates every hex in a width x height region (offset
// coordinates x in [0,width), y in [0,height), translated by offsetX) as
// absolute cube coordinates — a region tag's full membership, wall and
// floor cells alike (callers needing walkable-only cells filter against
// Walls). Mirrors two_chamber.go's (now retired) chamberCubes.
func regionCubes(width, height, offsetX int) []spatial.CubeCoordinate {
	out := make([]spatial.CubeCoordinate, 0, width*height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			out = append(out, spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x + offsetX), Y: float64(y)}, spatial.HexOrientationPointyTop))
		}
	}
	return out
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
