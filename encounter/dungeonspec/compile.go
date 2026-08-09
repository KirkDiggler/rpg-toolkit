// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SpawnInstruction is encounter's OWN type (defined in
// encounter/seed_monsters.go, alongside SeedMonsters -- rpg-toolkit#842)
// -- dungeonspec re-exports it via a plain type ALIAS, not a second
// struct, so CompiledDungeon.Spawns can be passed directly to
// enc.SeedMonsters(compiled.Spawns) with zero conversion.
type SpawnInstruction = encounter.SpawnInstruction

// normalProductPartyStartSeats is the effective API lobby PartyCap for this
// authored-dungeon product configuration. It is compiled into DungeonParams,
// not treated as a universal encounter limit.
const normalProductPartyStartSeats = 4

// LoadConfig supplies host-owned composition values to dungeonspec
// compilation. PartyStartSeatCount must be positive; it is the host's party
// capacity for this initialized dungeon, not a universal toolkit maximum.
type LoadConfig struct {
	PartyStartSeatCount int
}

// CompiledPlacement is one authoring placement copied from source into the
// compiler's absolute coordinate frame. Offset and Facing preserve optional
// presence; neither value changes the owning cell or any toolkit mechanics.
type CompiledPlacement struct {
	SourcePath     string
	Ref            string
	At             FloorPlanCell
	Facing         *uint32
	Offset         *PlacementOffset
	BlocksMovement bool
	BlocksLoS      bool
}

// CompiledDungeon is a dungeon spec compiled down to what the engine and
// SeedMonsters actually consume: engine params ready for InitDungeon, and
// a spawn plan ready for SeedMonsters. Placements is a separate authoring
// projection and is never joined to toolkit-minted runtime identity. Its
// private canvas source occupancy is immutable compiler state used only by
// LoadWithPrevious; no pointer aliases decoded YAML.
type CompiledDungeon struct {
	// Params feeds Encounter.InitDungeon directly. Params.RandomSeed is
	// deliberately left at its zero value -- Load has no opinion on
	// reproducibility, so the caller MUST set it before calling
	// InitDungeon, or repeated encounters compiled from the same spec
	// bytes won't roll the same layout.
	Params     encounter.DungeonParams
	Spawns     []SpawnInstruction  // boss first, then entrance->boss chain order
	Placements []CompiledPlacement // stable compiler order in absolute coordinates
	canvas     *canvasCompiled     // private opaque prior-validation/source state
}

// Load decodes, validates, and compiles a spec in one call -- the only
// entry point callers use. Door entity ids are COMPILER-GENERATED from
// content ("<key>-door-<from>-<to>" per connector), so callers supply
// nothing but the bytes.
//
// Expected call sequence once Load returns: enc.InitDungeon(compiled.Params)
// (after setting Params.RandomSeed, see CompiledDungeon.Params), then
// enc.AddPlayer for each player, then enc.SeedMonsters(compiled.Spawns) --
// in that order. SeedMonsters' combat-entry check only evaluates players
// already added; calling it before every AddPlayer silently skips combat
// entry for that seeding pass rather than erroring, so an author-placed
// monster in a party's starting sightline needs the party added first.
func Load(raw []byte) (CompiledDungeon, error) {
	return LoadWithConfig(raw, LoadConfig{PartyStartSeatCount: normalProductPartyStartSeats})
}

// LoadWithConfig decodes, validates, and compiles a spec using the supplied
// host party capacity. Preview and runtime should pass the same positive
// PartyStartSeatCount so they build the same reservation.
func LoadWithConfig(raw []byte, config LoadConfig) (CompiledDungeon, error) {
	if config.PartyStartSeatCount < 1 {
		return CompiledDungeon{}, fmt.Errorf(
			"party start seat count must be at least 1 (got %d)", config.PartyStartSeatCount)
	}
	spec, err := Decode(raw)
	if err != nil {
		return CompiledDungeon{}, err
	}
	if err := Validate(spec); err != nil {
		return CompiledDungeon{}, err
	}
	return compileWithConfig(spec, config)
}

// LoadWithPrevious validates candidate source against opaque prior compiled state before compiling it.
func LoadWithPrevious(raw []byte, config LoadConfig, previous CompiledDungeon) (CompiledDungeon, error) {
	candidate, err := LoadWithConfig(raw, config)
	if err != nil {
		return CompiledDungeon{}, err
	}
	if err := validatePreviousCanvas(candidate, previous); err != nil {
		return CompiledDungeon{}, err
	}
	return candidate, nil
}

// compile assumes spec has already passed Validate -- every ref shape,
// cell bound, and boss-cardinality invariant it relies on is guaranteed by
// that pass. It still returns an error rather than panicking, matching
// this package's style elsewhere, in case that assumption is ever violated
// by a future caller.
func compile(spec *DungeonSpec) (CompiledDungeon, error) {
	return compileWithConfig(spec, LoadConfig{PartyStartSeatCount: normalProductPartyStartSeats})
}

// compileWithConfig compiles a validated spec using the same host party-start
// reservation supplied at LoadWithConfig's public boundary.
func compileWithConfig(spec *DungeonSpec, config LoadConfig) (CompiledDungeon, error) {
	if spec.Canvas != nil {
		return compileCanvas(spec, config)
	}
	var bossRoom *RoomSpec
	for i := range spec.Rooms {
		if spec.Rooms[i].Boss != nil {
			bossRoom = &spec.Rooms[i]
			break // validateBossCardinality already guarantees exactly one
		}
	}
	if bossRoom == nil {
		return CompiledDungeon{}, fmt.Errorf("dungeon %q: no boss room (Validate should have rejected this spec)", spec.Key)
	}

	// Boss spawn goes first, regardless of the boss room's position in the
	// chain (design.md, defense-in-depth ahead of Slice C's own invariant).
	//
	// bossRoom.Boss.At is nil only for an unpinned boss -- Validate's M1-only
	// restriction (Task B2) rejects that shape today, so Load can never
	// reach this branch; it stays here so compile() never panics regardless
	// (this function's own doc), and it's exactly where M2's Task C0 lands:
	// an unpinned boss becomes a rolled (Count-based, no At) spawn candidate,
	// which is precisely what Count:1, At:nil already expresses.
	var bossAt *encounter.LocalHex
	if bossRoom.Boss.At != nil {
		at := encounter.LocalHex{Col: bossRoom.Boss.At[0], Row: bossRoom.Boss.At[1]}
		bossAt = &at
	}
	bossTargeting, err := compileTargeting(bossRoom.Boss.Targeting)
	if err != nil {
		return CompiledDungeon{}, fmt.Errorf("room %q: boss targeting: %w", bossRoom.ID, err)
	}
	// Capacity is a hint, not a bound: sized for the boss spawn plus one
	// per room, but a room's place list can name several monster entries
	// (growing past the hint) or none at all (falling short of it) --
	// neither direction is guaranteed.
	spawns := make([]SpawnInstruction, 0, len(spec.Rooms)+1)
	spawns = append(spawns, SpawnInstruction{
		RoomID:     bossRoom.ID,
		MonsterRef: bossRoom.Boss.Ref,
		Count:      1,
		At:         bossAt,
		Targeting:  bossTargeting,
		Offset:     clonePlacementOffset(bossRoom.Boss.Offset),
	})

	regions := make([]encounter.DungeonRegionParams, len(spec.Rooms))
	for i := range spec.Rooms {
		room := &spec.Rooms[i]
		region, roomSpawns, err := compileRoom(room)
		if err != nil {
			return CompiledDungeon{}, fmt.Errorf("room %q: %w", room.ID, err)
		}
		regions[i] = region
		spawns = append(spawns, roomSpawns...)
	}

	connectors := make([]encounter.DungeonConnectorParams, len(spec.Connectors))
	for i := range spec.Connectors {
		connectors[i] = compileConnector(spec.Key, &spec.Connectors[i])
	}
	authoredEdges, err := compileWalls(spec)
	if err != nil {
		return CompiledDungeon{}, err
	}
	placements, err := compileRoomChainPlacements(spec)
	if err != nil {
		return CompiledDungeon{}, err
	}

	partyStart := encounter.PartyStartParams{SeatCount: config.PartyStartSeatCount}
	if spec.Start != nil {
		anchor := core.HexFromPosition(spatial.Position{
			X: float64(spec.Start[0]),
			Y: float64(spec.Start[1]),
		})
		partyStart.Anchor = &anchor
	}

	return CompiledDungeon{
		Params: encounter.DungeonParams{
			Key:           spec.Key,
			Regions:       regions,
			Connectors:    connectors,
			Height:        spec.Height,
			Theme:         spec.Theme,
			PartyStart:    partyStart,
			AuthoredEdges: authoredEdges,
		},
		Spawns:     spawns,
		Placements: placements,
	}, nil
}

// compileRoomChainPlacements copies every authored room placement into the
// absolute compiled coordinate frame without altering optional metadata.
func compileRoomChainPlacements(spec *DungeonSpec) ([]CompiledPlacement, error) {
	var placements []CompiledPlacement
	startColumn := 0
	for roomIndex := range spec.Rooms {
		room := &spec.Rooms[roomIndex]
		for entryIndex := range room.Place {
			entry := &room.Place[entryIndex]
			refType, err := refParts(entry.Ref)
			if err != nil {
				return nil, fmt.Errorf("rooms[%d].place[%d].ref: %w", roomIndex, entryIndex, err)
			}
			facing, err := compileFacing(entry.Facing)
			if err != nil {
				return nil, fmt.Errorf("rooms[%d].place[%d].facing: %w", roomIndex, entryIndex, err)
			}
			placements = append(placements, CompiledPlacement{
				SourcePath: fmt.Sprintf("rooms[%d].place[%d]", roomIndex, entryIndex),
				Ref:        entry.Ref, At: FloorPlanCell{Column: startColumn + entry.At[0], Row: entry.At[1]},
				Facing: facing, Offset: clonePlacementOffset(entry.Offset),
				BlocksMovement: refType == refTypeProps && boolOrTrue(entry.BlocksMovement),
				BlocksLoS:      refType == refTypeProps && boolOrTrue(entry.BlocksLoS),
			})
		}
		if room.Boss != nil && room.Boss.At != nil {
			placements = append(placements, CompiledPlacement{
				SourcePath: fmt.Sprintf("rooms[%d].boss", roomIndex), Ref: room.Boss.Ref,
				At:     FloorPlanCell{Column: startColumn + room.Boss.At[0], Row: room.Boss.At[1]},
				Offset: clonePlacementOffset(room.Boss.Offset),
			})
		}
		startColumn += room.Width
		if roomIndex < len(spec.Rooms)-1 {
			startColumn++
		}
	}
	return placements, nil
}

func clonePlacementOffset(offset *PlacementOffset) *PlacementOffset {
	if offset == nil {
		return nil
	}
	clone := *offset
	return &clone
}

// compileRoom compiles one room's geometry/obstacles/place block. It
// returns the room's placed monster spawns separately (never the boss
// spawn, which compile builds directly from room.Boss) so compile can
// prepend the boss spawn once, ahead of every room's placed spawns, rather
// than every call site re-deriving "is this room the boss room."
func compileRoom(room *RoomSpec) (encounter.DungeonRegionParams, []SpawnInstruction, error) {
	// room.Monsters is unreachable via Load today -- Validate's M1-only
	// restriction (validateM1Restrictions) rejects any non-empty
	// room.Monsters -- but compileRoom does not yet know how to compile
	// count-based rolled monsters (M2's Slice C, count-based compiling --
	// see plan), so it errors loudly rather than silently dropping them if
	// that assumption is ever violated by a future caller, mirroring
	// compile()'s own nil-boss.At guard.
	if len(room.Monsters) > 0 {
		return encounter.DungeonRegionParams{}, nil,
			fmt.Errorf("rolled monsters not compiled yet (Validate should have rejected this spec)")
	}

	region := encounter.DungeonRegionParams{
		ID:        room.ID,
		Archetype: encounter.RegionArchetype(room.Archetype),
		Width:     room.Width,
		Pattern:   compilePattern(room.Pattern),
	}

	// The boss room's own pinned boss.at cell is a MONSTER placement exactly
	// like a place: monster entry below -- compile() builds its
	// SpawnInstruction directly from room.Boss, never through this room's
	// Place loop, so without reserving it here separately, InitDungeon's
	// rolled-obstacle draw has no idea that cell is spoken for (rpg-toolkit#842
	// gate finding: see encounter.DungeonRegionParams.ReservedCells' doc).
	if room.Boss != nil && room.Boss.At != nil {
		region.ReservedCells = append(region.ReservedCells,
			encounter.LocalHex{Col: room.Boss.At[0], Row: room.Boss.At[1]})
	}

	if len(room.Obstacles) > 0 {
		region.Obstacles = make([]encounter.ObstacleSpec, 0, len(room.Obstacles))
	}
	for _, o := range room.Obstacles {
		region.Obstacles = append(region.Obstacles, encounter.ObstacleSpec{
			Ref:            o.Ref,
			Count:          o.Count,
			BlocksMovement: boolOrTrue(o.BlocksMovement),
			BlocksLoS:      boolOrTrue(o.BlocksLoS),
			PreferBorder:   o.PreferBorder,
		})
	}

	if len(room.Place) > 0 {
		// Capacity is an upper bound, not exact: place entries split between
		// props (appended below to PlacedObstacles) and monsters (appended
		// to spawns), so a place block that's all monsters never grows
		// PlacedObstacles at all.
		region.PlacedObstacles = make([]encounter.PlacedObstacleSpec, 0, len(room.Place))
	}
	var spawns []SpawnInstruction
	for _, entry := range room.Place {
		refType, err := refParts(entry.Ref)
		if err != nil {
			return encounter.DungeonRegionParams{}, nil, fmt.Errorf("place %q: %w", entry.Ref, err)
		}
		switch refType {
		case refTypeProps:
			facing, err := compileFacing(entry.Facing)
			if err != nil {
				return encounter.DungeonRegionParams{}, nil, fmt.Errorf("place %q facing: %w", entry.Ref, err)
			}
			region.PlacedObstacles = append(region.PlacedObstacles, encounter.PlacedObstacleSpec{
				Ref:            entry.Ref,
				At:             encounter.LocalHex{Col: entry.At[0], Row: entry.At[1]},
				BlocksMovement: boolOrTrue(entry.BlocksMovement),
				BlocksLoS:      boolOrTrue(entry.BlocksLoS),
				Facing:         facing,
				Offset:         clonePlacementOffset(entry.Offset),
			})
		case refTypeMonsters:
			at := encounter.LocalHex{Col: entry.At[0], Row: entry.At[1]}
			targeting, err := compileTargeting(entry.Targeting)
			if err != nil {
				return encounter.DungeonRegionParams{}, nil, fmt.Errorf("place %q targeting: %w", entry.Ref, err)
			}
			// Reserve this cell the same way as the boss's own pinned
			// boss.at above -- a placed monster never becomes a
			// PlacedObstacleSpec, so without this InitDungeon's
			// rolled-obstacle draw would have no idea the cell is taken
			// (rpg-toolkit#842 gate finding: see ReservedCells' doc).
			region.ReservedCells = append(region.ReservedCells, at)
			spawns = append(spawns, SpawnInstruction{
				RoomID:     room.ID,
				MonsterRef: entry.Ref,
				Count:      1,
				At:         &at,
				Targeting:  targeting,
				Offset:     clonePlacementOffset(entry.Offset),
			})
		default:
			// Phrasing matches validatePlaceBlock's "must be props or
			// monsters" message (validate.go) -- this branch is unreachable
			// via Load (Validate already rejects it), but the two should
			// read as the same rule stated twice, not two different ones.
			return encounter.DungeonRegionParams{}, nil, fmt.Errorf("place ref %q must be props or monsters, got type %q",
				entry.Ref, refType)
		}
	}

	return region, spawns, nil
}

// compilePattern maps the spec's author-facing pattern vocabulary
// (patternEmpty/patternScattered, validate.go) onto the engine's own.
//
// PATTERN-DEFAULT TRAP: the engine treats an empty DungeonRegionParams.
// Pattern as PatternRandom, but the spec's default -- "" or patternEmpty
// -- means no interior walls at all, so the zero value must never pass
// through unmapped. This is the one place that mapping happens; anything
// that wants to reason about it again should point back here rather than
// re-deriving it.
func compilePattern(pattern string) string {
	switch pattern {
	case patternScattered:
		return environments.PatternRandom
	default: // "", patternEmpty, and (Validate having already run) nothing else
		return environments.PatternEmpty
	}
}

// compileConnector generates this connector's door id from content
// ("<key>-door-<from>-<to>") and copies its lock config verbatim.
func compileConnector(key string, c *ConnectorSpec) encounter.DungeonConnectorParams {
	params := encounter.DungeonConnectorParams{
		DoorID: core.EntityID(fmt.Sprintf("%s-door-%s-%s", key, c.From, c.To)),
	}
	if c.Locked != nil {
		params.Locked = true
		params.LockDC = c.Locked.DC
		params.LockAbility = c.Locked.Ability
	}
	return params
}

// compileWalls converts validated absolute YAML pairs into normalized
// pointy-top encounter records. It independently checks pointer presence so a
// future direct compile() caller cannot turn malformed source into a panic or
// silently lose an authored edge.
func compileWalls(spec *DungeonSpec) ([]encounter.AuthoredEdge, error) {
	if len(spec.Walls) == 0 {
		return nil, nil
	}
	edges := make([]encounter.AuthoredEdge, 0, len(spec.Walls))
	for index, wall := range spec.Walls {
		if wall.From == nil || wall.To == nil {
			return nil, fmt.Errorf("walls[%d]: missing endpoint (Validate should have rejected this spec)", index)
		}
		from := core.HexFromPosition(spatial.Position{X: float64(wall.From[0]), Y: float64(wall.From[1])})
		to := core.HexFromPosition(spatial.Position{X: float64(wall.To[0]), Y: float64(wall.To[1])})
		if wallHexLess(to, from) {
			from, to = to, from
		}
		edge := encounter.AuthoredEdge{From: from, To: to, Kind: encounter.GeneratedEdgeKind(wall.Kind)}
		switch edge.Kind {
		case encounter.GeneratedEdgeKindSolid:
		case encounter.GeneratedEdgeKindDoor:
			edge.DoorID = encounter.AuthoredDoorID(spec.Key, from, to)
		default:
			return nil, fmt.Errorf("walls[%d].kind %q not compiled (Validate should have rejected this spec)", index, wall.Kind)
		}
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if wallHexLess(edges[i].From, edges[j].From) {
			return true
		}
		if wallHexLess(edges[j].From, edges[i].From) {
			return false
		}
		if wallHexLess(edges[i].To, edges[j].To) {
			return true
		}
		if wallHexLess(edges[j].To, edges[i].To) {
			return false
		}
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		return edges[i].DoorID < edges[j].DoorID
	})
	return edges, nil
}

// boolOrTrue returns *b, or true when b is nil -- the nil=>true blocking
// default shared by ObstacleEntry and PlacedEntry's BlocksMovement/
// BlocksLoS flags (design.md: an unflagged entry blocks both).
func boolOrTrue(b *bool) bool {
	if b == nil {
		return true
	}
	return *b
}

// compileFacing maps an optional canonical YAML facing label to its persisted
// numeric index. Validate normally guarantees the label is known; this guard
// keeps compileRoom from silently accepting a bad direct call.
func canvasHex(cell FloorPlanCell) core.Hex {
	return core.HexFromPosition(spatial.Position{X: float64(cell.Column), Y: float64(cell.Row)})
}

func compileFacing(label *string) (*uint32, error) {
	if label == nil {
		return nil, nil
	}
	value, err := facingValue(*label)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// compileTargeting parses an author-facing targeting label (Validate
// already checked it parses; this trusts that and re-checks defensively,
// mirroring compileFacing) into the value shape SpawnInstruction.Targeting
// requires — nil in, TargetingUnspecified out, so an omitted targeting
// field never overrides a monster's own ctor default (rpg-toolkit#895
// gate-review hardening: TargetingUnspecified is the zero value precisely
// so this no longer needs a pointer to express "unset").
func compileTargeting(label *string) (monster.TargetingStrategy, error) {
	if label == nil {
		return monster.TargetingUnspecified, nil
	}
	return monster.ParseTargetingStrategy(*label)
}

type canvasCompiled struct {
	width, height int
	floorSource   FloorSource
	floorCells    []FloorPlanCell
	entrance      *FloorPlanCell
	envelope      []encounter.GeneratedEdge
	regions       []FloorPlanRegion
	occupancy     []canvasSourceOccupancy // retained for legacy LoadWithPrevious callers
	regionCells   []namedCanvasCell
}

type canvasSourceOccupancy struct {
	source string
	cell   [2]int
}

type namedCanvasCell struct {
	name string
	cell [2]int
}

func compileCanvas(spec *DungeonSpec, config LoadConfig) (CompiledDungeon, error) {
	if _, err := encounter.ValidateCanvasDimensions(spec.Canvas.Width, spec.Canvas.Height); err != nil {
		return CompiledDungeon{}, err
	}
	edges, err := compileWalls(spec)
	if err != nil {
		return CompiledDungeon{}, err
	}
	cc := &canvasCompiled{
		width: spec.Canvas.Width, height: spec.Canvas.Height,
		floorSource: FloorSource(spec.Canvas.FloorSource),
	}
	if cc.floorSource == "" {
		cc.floorSource = FloorSourceBounds
	}
	floorSet := make(map[[2]int]struct{})
	if cc.floorSource == FloorSourceRegions {
		for _, region := range spec.Regions {
			for _, cell := range region.Cells {
				floorSet[cell] = struct{}{}
			}
		}
	} else {
		for col := 0; col < spec.Canvas.Width; col++ {
			for row := 0; row < spec.Canvas.Height; row++ {
				floorSet[[2]int{col, row}] = struct{}{}
			}
		}
	}
	for cell := range floorSet {
		cc.floorCells = append(cc.floorCells, FloorPlanCell{Column: cell[0], Row: cell[1]})
	}
	sort.Slice(cc.floorCells, func(i, j int) bool { return floorPlanCellLess(cc.floorCells[i], cc.floorCells[j]) })
	floorHexes := make([]core.Hex, len(cc.floorCells))
	for index, cell := range cc.floorCells {
		floorHexes[index] = canvasHex(cell)
	}
	cc.envelope = compileEnvelope(floorHexes)

	semanticRegions := make([]encounter.SemanticRegionParams, len(spec.Regions))
	for index, region := range spec.Regions {
		cells := make([]core.Hex, 0, len(region.Cells))
		seen := make(map[[2]int]struct{}, len(region.Cells))
		for _, cell := range region.Cells {
			if _, duplicate := seen[cell]; duplicate {
				continue
			}
			seen[cell] = struct{}{}
			cells = append(cells, canvasHex(FloorPlanCell{Column: cell[0], Row: cell[1]}))
		}
		sort.Slice(cells, func(i, j int) bool { return floorPlanCellLess(cellFromHex(cells[i]), cellFromHex(cells[j])) })
		var archetype *encounter.RegionArchetype
		if region.Archetype != nil {
			value := encounter.RegionArchetype(*region.Archetype)
			archetype = &value
		}
		semanticRegions[index] = encounter.SemanticRegionParams{
			ID: region.ID, Name: region.Name, Archetype: archetype, Cells: cells,
		}
		for _, cell := range region.Cells {
			cc.regionCells = append(cc.regionCells, namedCanvasCell{name: region.ID, cell: cell})
		}
	}
	params := encounter.DungeonParams{
		Key: spec.Key, Width: spec.Canvas.Width, Height: spec.Canvas.Height, Theme: spec.Theme,
		SemanticRegions: semanticRegions, FloorCells: floorHexes, EnvelopeEdges: cc.envelope,
		FloorSource: encounter.FloorSourceCanvas, RequireConnectedFloor: cc.floorSource == FloorSourceRegions,
		PartyStart: encounter.PartyStartParams{SeatCount: config.PartyStartSeatCount}, AuthoredEdges: edges,
	}
	if spec.Start != nil {
		anchor := canvasHex(FloorPlanCell{Column: spec.Start[0], Row: spec.Start[1]})
		params.PartyStart.Anchor = &anchor
	}
	spawns := make([]SpawnInstruction, 0, len(spec.Place))
	placements := make([]CompiledPlacement, 0, len(spec.Place))
	for index, entry := range spec.Place {
		refType, err := refParts(entry.Ref)
		if err != nil {
			return CompiledDungeon{}, fmt.Errorf("place[%d]: %w", index, err)
		}
		atCell := FloorPlanCell{Column: entry.At[0], Row: entry.At[1]}
		at := canvasHex(atCell)
		cc.occupancy = append(cc.occupancy, canvasSourceOccupancy{source: fmt.Sprintf("place[%d]", index), cell: entry.At})
		facing, err := compileFacing(entry.Facing)
		if err != nil {
			return CompiledDungeon{}, fmt.Errorf("place[%d].facing: %w", index, err)
		}
		placements = append(placements, CompiledPlacement{
			SourcePath: fmt.Sprintf("place[%d]", index), Ref: entry.Ref, At: atCell,
			Facing: facing, Offset: clonePlacementOffset(entry.Offset),
			BlocksMovement: refType == refTypeProps && boolOrTrue(entry.BlocksMovement),
			BlocksLoS:      refType == refTypeProps && boolOrTrue(entry.BlocksLoS),
		})
		switch refType {
		case refTypeProps:
			params.AbsolutePlacedObstacles = append(params.AbsolutePlacedObstacles, encounter.AbsolutePlacedObstacleSpec{
				ID: core.EntityID(fmt.Sprintf("canvas-prop-%d", index)), Ref: entry.Ref, At: at,
				BlocksMovement: boolOrTrue(entry.BlocksMovement), BlocksLoS: boolOrTrue(entry.BlocksLoS),
				Facing: facing, Offset: clonePlacementOffset(entry.Offset),
			})
		case refTypeMonsters:
			absoluteAt := at
			targeting, err := compileTargeting(entry.Targeting)
			if err != nil {
				return CompiledDungeon{}, fmt.Errorf("place[%d].targeting: %w", index, err)
			}
			spawns = append(spawns, SpawnInstruction{
				MonsterRef: entry.Ref, Count: 1, AbsoluteAt: &absoluteAt, Targeting: targeting,
				Offset: clonePlacementOffset(entry.Offset),
			})
			params.AbsoluteReservedCells = append(params.AbsoluteReservedCells, encounter.AbsoluteReservedCell{
				At: at, Name: fmt.Sprintf("placed monster %q (place[%d])", entry.Ref, index),
			})
		default:
			return CompiledDungeon{}, fmt.Errorf("place[%d].ref %q must be props or monsters", index, entry.Ref)
		}
	}
	for index, wall := range spec.Walls {
		cc.occupancy = append(cc.occupancy,
			canvasSourceOccupancy{source: fmt.Sprintf("walls[%d].from", index), cell: *wall.From},
			canvasSourceOccupancy{source: fmt.Sprintf("walls[%d].to", index), cell: *wall.To},
		)
	}
	if spec.Start != nil {
		cc.occupancy = append(cc.occupancy, canvasSourceOccupancy{source: "start", cell: *spec.Start})
	}
	cc.regions = compileFloorPlanRegions(spec.Regions)
	cc.entrance = resolveDraftEntrance(cc.floorCells, spec.Start, spec.Place, config.PartyStartSeatCount)
	return CompiledDungeon{Params: params, Spawns: spawns, Placements: placements, canvas: cc}, nil
}

func compileEnvelope(floorCells []core.Hex) []encounter.GeneratedEdge {
	floor := make(map[core.Hex]struct{}, len(floorCells))
	for _, cell := range floorCells {
		floor[cell] = struct{}{}
	}
	seen := make(map[wallEdgeKey]struct{})
	var edges []encounter.GeneratedEdge
	for owner := range floor {
		for _, cube := range owner.ToCube().GetNeighbors() {
			neighbor := core.HexFromCube(cube)
			if _, ok := floor[neighbor]; ok {
				continue
			}
			from, to := owner, neighbor
			if wallHexLess(to, from) {
				from, to = to, from
			}
			key := newWallEdgeKey(from, to)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			edges = append(edges, encounter.GeneratedEdge{From: from, To: to, Kind: encounter.GeneratedEdgeKindSolid})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if wallHexLess(edges[i].From, edges[j].From) {
			return true
		}
		if wallHexLess(edges[j].From, edges[i].From) {
			return false
		}
		return wallHexLess(edges[i].To, edges[j].To)
	})
	return edges
}

func compileFloorPlanRegions(regions []RegionSpec) []FloorPlanRegion {
	out := make([]FloorPlanRegion, len(regions))
	sets := make([]map[[2]int]struct{}, len(regions))
	for i, region := range regions {
		sets[i] = make(map[[2]int]struct{}, len(region.Cells))
		out[i] = FloorPlanRegion{ID: region.ID, Name: cloneString(region.Name), Archetype: cloneString(region.Archetype)}
		for _, cell := range region.Cells {
			sets[i][cell] = struct{}{}
		}
		for cell := range sets[i] {
			out[i].Cells = append(out[i].Cells, FloorPlanCell{Column: cell[0], Row: cell[1]})
		}
		sort.Slice(out[i].Cells, func(a, b int) bool { return floorPlanCellLess(out[i].Cells[a], out[i].Cells[b]) })
	}
	for child := range sets {
		if len(sets[child]) == 0 {
			continue
		}
		parent := -1
		for candidate := range sets {
			if child == candidate || len(sets[candidate]) <= len(sets[child]) {
				continue
			}
			contains := true
			for cell := range sets[child] {
				if _, ok := sets[candidate][cell]; !ok {
					contains = false
					break
				}
			}
			if contains && (parent == -1 || len(sets[candidate]) < len(sets[parent])) {
				parent = candidate
			}
		}
		if parent >= 0 {
			id := regions[parent].ID
			out[child].ParentID = &id
		}
	}
	return out
}

func resolveDraftEntrance(
	floorCells []FloorPlanCell,
	authored *[2]int,
	placements []PlacedEntry,
	seatCount int,
) *FloorPlanCell {
	if seatCount < 1 {
		return nil
	}
	floor := make(map[[2]int]struct{}, len(floorCells))
	for _, cell := range floorCells {
		floor[[2]int{cell.Column, cell.Row}] = struct{}{}
	}
	blocked := make(map[[2]int]struct{}, len(placements))
	for _, placement := range placements {
		blocked[placement.At] = struct{}{}
	}
	anchors := append([]FloorPlanCell(nil), floorCells...)
	if authored != nil {
		anchors = []FloorPlanCell{{Column: authored[0], Row: authored[1]}}
	}
	for _, anchor := range anchors {
		key := [2]int{anchor.Column, anchor.Row}
		if _, ok := floor[key]; !ok {
			continue
		}
		if _, no := blocked[key]; no {
			continue
		}
		component := map[[2]int]struct{}{key: {}}
		queue := [][2]int{key}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			h := canvasHex(FloorPlanCell{Column: current[0], Row: current[1]})
			for _, cube := range h.ToCube().GetNeighbors() {
				pos := core.HexFromCube(cube).ToPosition()
				next := [2]int{int(pos.X), int(pos.Y)}
				if _, ok := floor[next]; !ok {
					continue
				}
				if _, seen := component[next]; seen {
					continue
				}
				component[next] = struct{}{}
				queue = append(queue, next)
			}
		}
		available := 0
		for cell := range component {
			if _, no := blocked[cell]; !no {
				available++
			}
		}
		if available >= seatCount {
			value := anchor
			return &value
		}
	}
	return nil
}

func validatePreviousCanvas(candidate, previous CompiledDungeon) error {
	if candidate.canvas == nil || previous.canvas == nil {
		return nil
	}
	in := func(c [2]int) bool {
		return c[0] >= 0 && c[0] < candidate.canvas.width && c[1] >= 0 && c[1] < candidate.canvas.height
	}
	for _, item := range previous.canvas.occupancy {
		if !in(item.cell) {
			return fmt.Errorf(
				"%s: previous compiled occupancy at [%d,%d] is outside candidate canvas",
				item.source, item.cell[0], item.cell[1],
			)
		}
	}
	for _, item := range previous.canvas.regionCells {
		if !in(item.cell) {
			return fmt.Errorf("region cell %q at [%d,%d] is outside candidate canvas", item.name, item.cell[0], item.cell[1])
		}
	}
	return nil
}
