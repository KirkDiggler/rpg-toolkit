// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
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

// CompiledDungeon is a dungeon spec compiled down to what the engine and
// SeedMonsters actually consume: engine params ready for InitDungeon, and
// a spawn plan ready for SeedMonsters. Its private canvas source occupancy is
// immutable compiler state used only by LoadWithPrevious; it contains no
// pointers into decoded YAML and is safe to cache/share as a value.
type CompiledDungeon struct {
	// Params feeds Encounter.InitDungeon directly. Params.RandomSeed is
	// deliberately left at its zero value -- Load has no opinion on
	// reproducibility, so the caller MUST set it before calling
	// InitDungeon, or repeated encounters compiled from the same spec
	// bytes won't roll the same layout.
	Params encounter.DungeonParams
	Spawns []SpawnInstruction // boss first, then entrance->boss chain order
	canvas *canvasCompiled    // private opaque prior-validation/source state
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
		Spawns: spawns,
	}, nil
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
			})
		case refTypeMonsters:
			at := encounter.LocalHex{Col: entry.At[0], Row: entry.At[1]}
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
		if wall.Lock != nil {
			edge.LockOptions = make([]encounter.AuthoredLockOption, len(wall.Lock.Options))
			for optionIndex, option := range wall.Lock.Options {
				edge.LockOptions[optionIndex] = encounter.AuthoredLockOption{Ability: option.Ability, DC: option.DC}
			}
		}
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

type canvasCompiled struct {
	width, height int
	entrance      FloorPlanCell
	occupancy     []canvasSourceOccupancy // source order for LoadWithPrevious
	regionCells   []namedCanvasCell       // internal-only synthetic test seam
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
	// Load validates before compilation, but keep this compilation boundary safe
	// for direct/internal callers before NewCanvasFloorSource allocates cells.
	if _, err := encounter.ValidateCanvasDimensions(spec.Canvas.Width, spec.Canvas.Height); err != nil {
		return CompiledDungeon{}, err
	}
	edges, err := compileWalls(spec)
	if err != nil {
		return CompiledDungeon{}, err
	}
	cc := &canvasCompiled{width: spec.Canvas.Width, height: spec.Canvas.Height}
	if spec.Start != nil {
		cc.entrance = FloorPlanCell{spec.Start[0], spec.Start[1]}
	} else {
		cc.entrance = FloorPlanCell{0, 0}
	}

	params := encounter.DungeonParams{
		Key: spec.Key, Height: spec.Canvas.Height, Theme: spec.Theme,
		Canvas:        encounter.NewCanvasFloorSource(spec.Canvas.Width, spec.Canvas.Height),
		PartyStart:    encounter.PartyStartParams{SeatCount: config.PartyStartSeatCount},
		AuthoredEdges: edges,
	}
	anchor := canvasHex(cc.entrance)
	params.PartyStart.Anchor = &anchor
	spawns := make([]SpawnInstruction, 0, len(spec.Place))
	for index, entry := range spec.Place {
		refType, err := refParts(entry.Ref)
		if err != nil {
			return CompiledDungeon{}, fmt.Errorf("place[%d]: %w", index, err)
		}
		at := canvasHex(FloorPlanCell{Column: entry.At[0], Row: entry.At[1]})
		cc.occupancy = append(cc.occupancy, canvasSourceOccupancy{source: fmt.Sprintf("place[%d]", index), cell: entry.At})
		switch refType {
		case refTypeProps:
			facing, err := compileFacing(entry.Facing)
			if err != nil {
				return CompiledDungeon{}, fmt.Errorf("place[%d].facing: %w", index, err)
			}
			params.CanvasPlacedObstacles = append(params.CanvasPlacedObstacles, encounter.CanvasPlacedObstacleSpec{
				ID: core.EntityID(fmt.Sprintf("canvas-prop-%d", index)), Ref: entry.Ref, At: at,
				BlocksMovement: boolOrTrue(entry.BlocksMovement), BlocksLoS: boolOrTrue(entry.BlocksLoS), Facing: facing,
			})
		case refTypeMonsters:
			absoluteAt := at
			spawns = append(spawns, SpawnInstruction{MonsterRef: entry.Ref, Count: 1, AbsoluteAt: &absoluteAt})
			params.CanvasReservedCells = append(params.CanvasReservedCells, encounter.CanvasReservedCell{
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
	return CompiledDungeon{Params: params, Spawns: spawns, canvas: cc}, nil
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
