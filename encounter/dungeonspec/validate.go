// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	minRooms    = 2
	minHeight   = 4
	minWidth    = 4
	minCount    = 1
	minLockDC   = 1
	maxLockDC   = 30
	bossAxisMin = 6 // primary axis (min(boss room width, height)) must exceed this
)

const (
	refTypeProps    = "props"
	refTypeMonsters = "monsters"

	unsupportedCapability = "unsupported capability"
	facingFloorPropsOnly  = "facing only supported on floor props"
)

// The spec's author-facing pattern vocabulary, shared between
// validatePattern/validatePlaceBlock (this file) and compile.go's
// compilePattern, which maps patternEmpty (and "") onto
// environments.PatternEmpty and patternScattered onto
// environments.PatternRandom -- named once here so both sides agree.
const (
	patternEmpty     = "empty"
	patternScattered = "scattered"
)

// The fixed room-archetype vocabulary, matching encounter's own
// RegionArchetype constants (encounter/data.go) exactly -- shared between
// validateArchetype (this file) and validateBossCardinality's "boss" check
// below, so both name the vocabulary the same way instead of repeating the
// literals.
const (
	archetypeEntrance = "entrance"
	archetypeChamber  = "chamber"
	archetypeCorridor = "corridor"
	archetypeBoss     = "boss"
)

// Validate checks a decoded DungeonSpec against every generator constraint
// the engine assumes, mirroring design.md's §Validation rules so a spec
// that loads is a spec that plays. Checks run in a fixed order chosen so an
// author always sees the most useful error first — e.g. a too-small boss
// room reports "primary axis" rather than an incidental out-of-bounds error
// on one of its placed props, because the boss-axis check runs before any
// per-cell place-block check.
func Validate(spec *DungeonSpec) error {
	if spec.Version != 1 {
		return authoredError("version", "unsupported_version", "unsupported spec version %d (must be 1)", spec.Version)
	}
	if strings.TrimSpace(spec.Key) == "" {
		return authoredError("key", "invalid_key", "key must not be empty")
	}
	if !validKey(spec.Key) {
		return authoredError("key", "invalid_key", "key %q must use only lowercase letters, digits, and hyphens", spec.Key)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return authoredError("name", "invalid_name", "name must not be empty")
	}
	if spec.Canvas != nil {
		return validateCanvas(spec)
	}
	if err := validateNoRegionsInRoomChain(spec); err != nil {
		return err
	}
	if spec.Height < minHeight {
		return authoredError("height", "invalid_dimension", "height must be at least %d, got %d", minHeight, spec.Height)
	}
	if len(spec.Rooms) < minRooms {
		return authoredError("rooms", "invalid_rooms", "must have at least %d rooms, got %d", minRooms, len(spec.Rooms))
	}
	if err := validateUniqueRoomIDs(spec.Rooms); err != nil {
		return err
	}
	if err := validateChain(spec); err != nil {
		return err
	}
	if err := validateRoomDefinitions(spec.Rooms); err != nil {
		return err
	}
	bossRoom, err := validateBossCardinality(spec.Rooms)
	if err != nil {
		return err
	}
	if err = validateBossAxis(bossRoom, spec.Height, roomPointerIndex(spec.Rooms, bossRoom)); err != nil {
		return err
	}
	if err = validateM1Restrictions(spec, bossRoom); err != nil {
		return err
	}
	for i := range spec.Rooms {
		room := &spec.Rooms[i]
		for obstacleIndex, obstacle := range room.Obstacles {
			path := fmt.Sprintf("rooms[%d].obstacles[%d]", i, obstacleIndex)
			if _, err := refParts(obstacle.Ref); err != nil {
				return authoredError(path+".ref", "invalid_ref", "%s.ref: %v", path, err)
			}
			if obstacle.Count < minCount {
				return authoredError(
					path+".count", "invalid_count", "%s.count must be at least %d, got %d",
					path, minCount, obstacle.Count,
				)
			}
		}
	}
	if err = validateBossRef(bossRoom, roomPointerIndex(spec.Rooms, bossRoom)); err != nil {
		return err
	}
	for i := range spec.Rooms {
		if err = validatePlaceBlock(&spec.Rooms[i], spec.Height, i); err != nil {
			return err
		}
	}
	if err = validateTopLevelPlace(spec.Place); err != nil {
		return err
	}
	if err = validateStart(spec); err != nil {
		return err
	}
	if err = validateWalls(spec); err != nil {
		return err
	}
	for i := range spec.Connectors {
		if spec.Connectors[i].Locked != nil {
			if err = validateLocked(&spec.Connectors[i], i); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRoomDefinitions(rooms []RoomSpec) error {
	for i := range rooms {
		room := &rooms[i]
		path := fmt.Sprintf("rooms[%d]", i)
		if room.Width < minWidth {
			return authoredError(
				path+".width", "invalid_dimension", "%s.width must be at least %d, got %d",
				path, minWidth, room.Width,
			)
		}
		if err := validateArchetype(room.Archetype); err != nil {
			return authoredError(path+".archetype", "invalid_archetype", "%s.archetype: %v", path, err)
		}
		if err := validatePattern(room.Pattern); err != nil {
			return authoredError(path+".pattern", "invalid_pattern", "%s.pattern: %v", path, err)
		}
	}
	return nil
}

func validateNoRegionsInRoomChain(spec *DungeonSpec) error {
	if len(spec.Regions) != 0 {
		return authoredError(
			"regions", "unsupported_capability",
			"regions require canvas mode (non-empty rooms and regions are incompatible)",
		)
	}
	return nil
}

func validateCanvas(spec *DungeonSpec) error {
	if spec.Canvas.Width < 1 {
		return authoredError("canvas.width", "invalid_dimension", "canvas width must be positive, got %d", spec.Canvas.Width)
	}
	if spec.Canvas.Height < 1 {
		return authoredError(
			"canvas.height", "invalid_dimension", "canvas height must be positive, got %d", spec.Canvas.Height,
		)
	}
	cellCount, err := encounter.ValidateCanvasDimensions(spec.Canvas.Width, spec.Canvas.Height)
	if err != nil {
		return authoredError("canvas", "invalid_dimension", "%v", err)
	}
	if spec.roomsShape != roomsSequence || len(spec.Rooms) != 0 {
		return authoredError(
			"rooms", "invalid_canvas_rooms", "canvas mode rooms must be an explicit empty sequence (rooms: [])",
		)
	}
	if len(spec.Connectors) != 0 {
		return authoredError("connectors", "unsupported_capability", "canvas mode does not support connectors")
	}
	bounds := make(map[[2]int]struct{}, cellCount)
	for c := 0; c < spec.Canvas.Width; c++ {
		for r := 0; r < spec.Canvas.Height; r++ {
			bounds[[2]int{c, r}] = struct{}{}
		}
	}
	if err := validateRegions(spec.Regions, bounds); err != nil {
		return err
	}
	floor := bounds
	switch spec.Canvas.FloorSource {
	case "", string(FloorSourceBounds):
	case string(FloorSourceRegions):
		floor = make(map[[2]int]struct{})
		for _, region := range spec.Regions {
			for _, cell := range region.Cells {
				floor[cell] = struct{}{}
			}
		}
	default:
		return authoredError(
			"canvas.floor_source", "invalid_floor_source", "invalid floor source %q (must be %q or %q)",
			spec.Canvas.FloorSource, FloorSourceBounds, FloorSourceRegions,
		)
	}
	occupied := map[[2]int]string{}
	for i, e := range spec.Place {
		path := fmt.Sprintf("place[%d]", i)
		if _, ok := floor[e.At]; !ok {
			return authoredError(path+".at", "outside_floor", "%s.at %v is outside structural floor", path, e.At)
		}
		if prior, ok := occupied[e.At]; ok {
			return authoredError(path+".at", "occupied", "%s.at %v is already occupied by %q", path, e.At, prior)
		}
		occupied[e.At] = e.Ref
		if err := validateCanvasPlacement(path, e); err != nil {
			return err
		}
	}
	if spec.Start != nil {
		if _, ok := floor[*spec.Start]; !ok {
			return authoredError("start", "outside_floor", "start %v is outside structural floor", *spec.Start)
		}
		if prior, ok := occupied[*spec.Start]; ok {
			return authoredError("start", "occupied", "start %v conflicts with %q", *spec.Start, prior)
		}
	}
	return validateWallsOnFloor(spec, floor, spec.Canvas.Width, spec.Canvas.Height)
}

// validateRegions enforces only the runnable semantic-scope structural rules.
// Content, role cardinality, connectedness, and empty scopes are intentionally
// not semantic validation concerns in this wave.
func validateRegions(regions []RegionSpec, floor map[[2]int]struct{}) error {
	seenIDs := make(map[string]int, len(regions))
	sets := make([]map[[2]int]struct{}, len(regions))
	for i, region := range regions {
		if region.ID == "" {
			return authoredError(fmt.Sprintf("regions[%d].id", i), "invalid_region", "regions[%d].id must not be empty", i)
		}
		if first, duplicate := seenIDs[region.ID]; duplicate {
			return authoredError(
				fmt.Sprintf("regions[%d].id", i), "duplicate_region",
				"regions[%d].id %q duplicates regions[%d]", i, region.ID, first,
			)
		}
		seenIDs[region.ID] = i
		if region.Archetype != nil {
			if *region.Archetype == "" {
				return authoredError(
					fmt.Sprintf("regions[%d].archetype", i), "invalid_region",
					"regions[%d].archetype: explicit empty archetype is unsupported", i,
				)
			}
			if err := validateArchetype(*region.Archetype); err != nil {
				return authoredError(fmt.Sprintf("regions[%d].archetype", i), "invalid_region", "regions[%d].archetype: %v", i, err)
			}
		}
		if region.Cells == nil {
			return authoredError(
				fmt.Sprintf("regions[%d].cells", i), "invalid_region",
				"regions[%d].cells is required (use [] for an empty scope)", i,
			)
		}
		sets[i] = make(map[[2]int]struct{}, len(region.Cells))
		for j, cell := range region.Cells {
			if _, ok := floor[cell]; !ok {
				return authoredError(
					fmt.Sprintf("regions[%d].cells[%d]", i, j), "outside_floor",
					"regions[%d].cells[%d] %v is out of canvas floor footprint", i, j, cell,
				)
			}
			sets[i][cell] = struct{}{} // same-region duplicates canonicalize
		}
	}
	for left := range sets {
		for right := left + 1; right < len(sets); right++ {
			intersection := 0
			for cell := range sets[left] {
				if _, ok := sets[right][cell]; ok {
					intersection++
				}
			}
			equal := len(sets[left]) == len(sets[right]) && intersection == len(sets[left])
			if equal {
				return authoredError(
					fmt.Sprintf("regions[%d].cells", right), "duplicate_region",
					"regions[%d].cells: equal cell set duplicates regions[%d].cells", right, left,
				)
			}
			if intersection == 0 {
				continue
			}
			leftInsideRight := intersection == len(sets[left]) && len(sets[left]) < len(sets[right])
			rightInsideLeft := intersection == len(sets[right]) && len(sets[right]) < len(sets[left])
			if !leftInsideRight && !rightInsideLeft {
				return authoredError(
					fmt.Sprintf("regions[%d].cells", right), "region_overlap",
					"regions[%d].cells: partial overlap with regions[%d].cells", right, left,
				)
			}
		}
	}
	return nil
}
func validateCanvasPlacement(path string, e PlacedEntry) error {
	typ, err := refParts(e.Ref)
	if err != nil {
		return authoredError(path+".ref", "invalid_ref", "%s.ref: %v", path, err)
	}
	if typ != refTypeProps && typ != refTypeMonsters {
		return authoredError(path+".ref", "invalid_ref", "%s.ref %q must be props or monsters", path, e.Ref)
	}
	if e.Facing != nil {
		if typ != refTypeProps || !isFloorMount(e.Mount) {
			return authoredError(
				path+".facing", "unsupported_capability", "%s.facing: %s: %s",
				path, unsupportedCapability, facingFloorPropsOnly,
			)
		}
		if err := validateFacing(*e.Facing); err != nil {
			return authoredError(path+".facing", "invalid_facing", "%s.facing: %v", path, err)
		}
	}
	if !isFloorMount(e.Mount) {
		return authoredError(
			path+".mount", "unsupported_capability", "%s.mount: %s: mounted placements are not supported",
			path, unsupportedCapability,
		)
	}
	if typ == refTypeMonsters {
		if _, ok := monsters.ByRef(e.Ref); !ok {
			return authoredError(
				path+".ref", "invalid_ref", "%s.ref %q: unknown monster ref (known: %s)",
				path, e.Ref, strings.Join(monsters.Refs(), ", "),
			)
		}
		if e.BlocksMovement != nil {
			return authoredError(path+".blocks_movement", "invalid_placement", "%s.blocks_movement: only valid on props", path)
		}
		if e.BlocksLoS != nil {
			return authoredError(path+".blocks_los", "invalid_placement", "%s.blocks_los: only valid on props", path)
		}
	}
	return nil
}

func validKey(key string) bool {
	for _, character := range key {
		if character < 'a' || character > 'z' {
			if character < '0' || character > '9' {
				if character != '-' {
					return false
				}
			}
		}
	}
	return true
}

// validateM1Restrictions enforces the M1-only monster/boss.at pinning
// restriction (design.md, Task B2's "M1-only monster-pinning restriction"):
// SeedMonsters only handles At-bearing spawns until M2's Task C0 lands
// count-based rolling, so a spec using count-based monsters: or an unpinned
// boss.at would compile but fail at runtime — the exact gap the load-time
// contract exists to prevent. Both checks live in this one function so
// C0 can lift the restriction as a pure deletion of this function and its
// one call site in Validate (the boss-required non-nil check in
// validateBossCardinality is a separate, permanent check C0 never touches).
func validateM1Restrictions(spec *DungeonSpec, bossRoom *RoomSpec) error {
	for i := range spec.Rooms {
		if len(spec.Rooms[i].Monsters) > 0 {
			return authoredError(
				fmt.Sprintf("rooms[%d].monsters", i), "unsupported_capability",
				"rolled monster placement lands in M2",
			)
		}
	}
	if bossRoom.Boss.At == nil {
		index := roomPointerIndex(spec.Rooms, bossRoom)
		return authoredError(
			fmt.Sprintf("rooms[%d].boss.at", index), "unsupported_capability",
			"rolled monster placement lands in M2",
		)
	}
	return nil
}

func validateUniqueRoomIDs(rooms []RoomSpec) error {
	seen := make(map[string]int, len(rooms))
	for index, room := range rooms {
		if first, duplicate := seen[room.ID]; duplicate {
			return authoredError(
				fmt.Sprintf("rooms[%d].id", index), "duplicate_room",
				"duplicate room id %q at rooms[%d].id (already used by rooms[%d].id)", room.ID, index, first,
			)
		}
		seen[room.ID] = index
	}
	return nil
}

// validateChain enforces the v1 generator constraint: connectors join rooms
// in a single linear chain, in room order (rooms[i] -> rooms[i+1]).
func validateChain(spec *DungeonSpec) error {
	if len(spec.Connectors) != len(spec.Rooms)-1 {
		return authoredError(
			"connectors", "invalid_chain", "connectors must form a linear chain: expected %d connectors for %d rooms, got %d",
			len(spec.Rooms)-1, len(spec.Rooms), len(spec.Connectors),
		)
	}
	for i, connector := range spec.Connectors {
		if connector.From != spec.Rooms[i].ID || connector.To != spec.Rooms[i+1].ID {
			return authoredError(
				fmt.Sprintf("connectors[%d]", i), "invalid_chain",
				"connectors must form a linear chain: connector %d (%s -> %s) must join room %q to room %q",
				i, connector.From, connector.To, spec.Rooms[i].ID, spec.Rooms[i+1].ID,
			)
		}
	}
	return nil
}

// validateArchetype checks a room's archetype against the fixed vocabulary
// the engine's own RegionArchetype accepts (encounter/data.go). Without
// this check, an author typo like "chambre" decodes and validates cleanly
// today, and only surfaces much later as an Internal error at
// StartEncounter (another repo) once InitDungeon's own archetype switch
// rejects it -- exactly the failure class this package's load-time
// contract exists to close, the same way validatePattern already does for
// the pattern vocabulary.
//
//nolint:misspell // "chambre" above is the deliberate typo example, not a real misspelling
func validateArchetype(archetype string) error {
	switch archetype {
	case archetypeEntrance, archetypeChamber, archetypeCorridor, archetypeBoss:
		return nil
	default:
		return fmt.Errorf("invalid archetype %q (must be %q, %q, %q, or %q)",
			archetype, archetypeEntrance, archetypeChamber, archetypeCorridor, archetypeBoss)
	}
}

func validatePattern(pattern string) error {
	switch pattern {
	case "", patternEmpty, patternScattered:
		return nil
	default:
		return fmt.Errorf("invalid pattern %q (must be %q, %q, or %q)", pattern, "", patternEmpty, patternScattered)
	}
}

// validateBossCardinality confirms exactly one room is the boss room (boss
// archetype with a non-nil Boss entry) and that no other room declares one.
// The boss-required half is permanent (never lifted by M2's Task C0); it is
// distinct from the at-pinning check in Validate, which is M1-only.
func roomPointerIndex(rooms []RoomSpec, target *RoomSpec) int {
	for index := range rooms {
		if &rooms[index] == target {
			return index
		}
	}
	return 0
}

func validateBossCardinality(rooms []RoomSpec) (*RoomSpec, error) {
	var bossRoom *RoomSpec
	bossCount := 0
	for i := range rooms {
		room := &rooms[i]
		if room.Archetype == archetypeBoss {
			bossCount++
			if room.Boss == nil {
				return nil, authoredError(
					fmt.Sprintf("rooms[%d].boss", i), "missing_boss", "boss room must declare boss",
				)
			}
			bossRoom = room
		} else if room.Boss != nil {
			return nil, authoredError(
				fmt.Sprintf("rooms[%d].boss", i), "invalid_boss", "boss entry only on the boss room",
			)
		}
	}
	if bossCount != 1 {
		return nil, authoredError(
			"rooms", "invalid_boss_count", "dungeon must have exactly one boss room, found %d", bossCount,
		)
	}
	return bossRoom, nil
}

func validateBossAxis(bossRoom *RoomSpec, height, roomIndex int) error {
	axis := min(bossRoom.Width, height)
	if axis <= bossAxisMin {
		return authoredError(
			fmt.Sprintf("rooms[%d].width", roomIndex), "invalid_boss_axis",
			"boss room primary axis (min(width, height)=%d) must exceed %d", axis, bossAxisMin,
		)
	}
	return nil
}

// validateBossRef checks the boss room's monster ref: shape, that it names
// a monster (not a prop), and that it resolves via the registry.
func validateBossRef(bossRoom *RoomSpec, roomIndex int) error {
	path := fmt.Sprintf("rooms[%d].boss.ref", roomIndex)
	refType, err := refParts(bossRoom.Boss.Ref)
	if err != nil {
		return authoredError(path, "invalid_ref", "%s: %v", path, err)
	}
	if refType != refTypeMonsters {
		return authoredError(
			path, "invalid_ref", "%s %q must be a monster ref, got type %q", path, bossRoom.Boss.Ref, refType,
		)
	}
	if _, ok := monsters.ByRef(bossRoom.Boss.Ref); !ok {
		return authoredError(
			path, "invalid_ref", "%s %q: unknown monster ref (known: %s)",
			path, bossRoom.Boss.Ref, strings.Join(monsters.Refs(), ", "),
		)
	}
	return nil
}

// validatePlaceBlock validates one room's place block plus its (optional)
// pinned boss.at, which share one collision domain.
func validatePlaceBlock(room *RoomSpec, height, roomIndex int) error {
	hasPinned := len(room.Place) > 0 || (room.Boss != nil && room.Boss.At != nil)
	if room.Pattern == patternScattered && hasPinned {
		// Scattered interior walls are seed-rolled — no at cell can be
		// guaranteed clear or non-wall at author time (design.md §Design
		// delta), so the load-time contract can't hold for this combination.
		return authoredError(
			fmt.Sprintf("rooms[%d].pattern", roomIndex), "invalid_pattern",
			"place/boss.at not allowed with pattern: scattered",
		)
	}

	doorRow := height / 2
	occupied := make(map[[2]int]string, len(room.Place)+1)

	if room.Boss != nil && room.Boss.Facing != nil {
		return authoredError(
			fmt.Sprintf("rooms[%d].boss.facing", roomIndex), "unsupported_capability",
			"rooms[%d].boss.facing: %s: %s", roomIndex, unsupportedCapability, facingFloorPropsOnly,
		)
	}
	if room.Boss != nil && room.Boss.At != nil {
		at := *room.Boss.At
		if err := checkCellBounds(room, height, at); err != nil {
			return authoredError(
				fmt.Sprintf("rooms[%d].boss.at", roomIndex), "outside_floor", "boss.at: %v", err,
			)
		}
		if at[1] == doorRow {
			return authoredError(
				fmt.Sprintf("rooms[%d].boss.at", roomIndex), "reserved_cell",
				"boss.at %v is on the reserved row (height/2=%d)", at, doorRow,
			)
		}
		occupied[at] = room.Boss.Ref
	}

	for entryIndex, entry := range room.Place {
		if err := checkCellBounds(room, height, entry.At); err != nil {
			return authoredError(
				fmt.Sprintf("rooms[%d].place[%d].at", roomIndex, entryIndex), "outside_floor",
				"place %q at %v: %v", entry.Ref, entry.At, err,
			)
		}
		if entry.At[1] == doorRow {
			return authoredError(
				fmt.Sprintf("rooms[%d].place[%d].at", roomIndex, entryIndex), "reserved_cell",
				"place %q at %v is on the reserved row (height/2=%d)", entry.Ref, entry.At, doorRow,
			)
		}
		if occupant, ok := occupied[entry.At]; ok {
			return authoredError(
				fmt.Sprintf("rooms[%d].place[%d].at", roomIndex, entryIndex), "occupied",
				"place %q at %v is already placed (occupied by %q)", entry.Ref, entry.At, occupant,
			)
		}
		occupied[entry.At] = entry.Ref

		refType, err := refParts(entry.Ref)
		if err != nil {
			return authoredError(
				fmt.Sprintf("rooms[%d].place[%d].ref", roomIndex, entryIndex), "invalid_ref", "%v", err,
			)
		}
		// Ref-type routing must be checked before the flags-only-on-props
		// check below: an entry with an unrecognized ref type should always
		// report that error, not whichever check happens to run first.
		if refType != refTypeProps && refType != refTypeMonsters {
			return authoredError(
				fmt.Sprintf("rooms[%d].place[%d].ref", roomIndex, entryIndex), "invalid_ref",
				"place ref %q must be props or monsters, got type %q", entry.Ref, refType,
			)
		}

		path := fmt.Sprintf("rooms[%d].place[%d]", roomIndex, entryIndex)
		if entry.Facing != nil {
			if refType != refTypeProps || !isFloorMount(entry.Mount) {
				return authoredError(
					path+".facing", "unsupported_capability", "%s.facing: %s: %s",
					path, unsupportedCapability, facingFloorPropsOnly,
				)
			}
			if err := validateFacing(*entry.Facing); err != nil {
				return authoredError(path+".facing", "invalid_facing", "%s.facing: %v", path, err)
			}
		}
		if !isFloorMount(entry.Mount) {
			return authoredError(
				path+".mount", "unsupported_capability", "%s.mount: %s: mounted placements are not supported",
				path, unsupportedCapability,
			)
		}

		if room.Boss != nil && entry.Ref == room.Boss.Ref {
			return authoredError(
				path+".ref", "duplicate_boss", "%s.ref: boss ref may not also appear in place (ref %q)",
				path, entry.Ref,
			)
		}

		if refType == refTypeMonsters {
			if _, ok := monsters.ByRef(entry.Ref); !ok {
				return authoredError(
					path+".ref", "invalid_ref", "%s.ref %q: unknown monster ref (known: %s)",
					path, entry.Ref, strings.Join(monsters.Refs(), ", "),
				)
			}
			if entry.BlocksMovement != nil {
				return authoredError(path+".blocks_movement", "invalid_placement", "%s.blocks_movement only valid on props", path)
			}
			if entry.BlocksLoS != nil {
				return authoredError(path+".blocks_los", "invalid_placement", "%s.blocks_los only valid on props", path)
			}
		}
	}

	return nil
}

// validateTopLevelPlace rejects top-level placement in room-chain mode at the
// author-supplied field. Canvas mode validates and compiles the same entries as
// absolute placement instead.
func validateTopLevelPlace(entries []PlacedEntry) error {
	for index, entry := range entries {
		if entry.Facing != nil {
			path := fmt.Sprintf("place[%d].facing", index)
			return authoredError(path, "unsupported_capability", "%s: %s: %s", path, unsupportedCapability, facingFloorPropsOnly)
		}
		if entry.Mount != nil {
			path := fmt.Sprintf("place[%d].mount", index)
			return authoredError(
				path, "unsupported_capability", "%s: %s: mounted placements are not supported",
				path, unsupportedCapability,
			)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	return authoredError("place[0]", "unsupported_capability", "top-level placement is not supported")
}

func isFloorMount(mount *string) bool {
	return mount == nil || *mount == "floor"
}

// validateFacing rejects labels outside the one canonical hex-facing vocabulary.
func validateFacing(label string) error {
	_, err := facingValue(label)
	return err
}

// facingValue maps one canonical YAML label to the persisted runtime index.
func facingValue(label string) (uint32, error) {
	switch label {
	case "E":
		return encounter.FacingEast, nil
	case "NE":
		return encounter.FacingNortheast, nil
	case "NW":
		return encounter.FacingNorthwest, nil
	case "W":
		return encounter.FacingWest, nil
	case "SW":
		return encounter.FacingSouthwest, nil
	case "SE":
		return encounter.FacingSoutheast, nil
	default:
		return 0, fmt.Errorf("invalid facing %q (must be %q, %q, %q, %q, %q, or %q)",
			label, "E", "NE", "NW", "W", "SW", "SE")
	}
}

func checkCellBounds(room *RoomSpec, height int, at [2]int) error {
	if at[0] < 0 || at[0] >= room.Width {
		return fmt.Errorf("out of bounds: col %d not in [0,%d)", at[0], room.Width)
	}
	if at[1] < 0 || at[1] >= height {
		return fmt.Errorf("out of bounds: row %d not in [0,%d)", at[1], height)
	}
	return nil
}

// validateStart checks the optional absolute party-start anchor against the
// declared linear semantic-room layout and known authored blocking content.
// It intentionally does not inspect generated walls or rolled obstacles: both
// depend on a later seed and are protected by encounter's party reservation.
func validateStart(spec *DungeonSpec) error {
	if spec.Start == nil {
		return nil
	}
	at := *spec.Start
	if at[1] < 0 || at[1] >= spec.Height {
		return authoredError("start", "outside_floor", "start %v out of bounds: row %d not in [0,%d)", at, at[1], spec.Height)
	}

	starts := make([]int, len(spec.Rooms))
	totalWidth := 0
	for i, room := range spec.Rooms {
		starts[i] = totalWidth
		totalWidth += room.Width
		if i < len(spec.Rooms)-1 {
			totalWidth++ // connector column
		}
	}
	if at[0] < 0 || at[0] >= totalWidth {
		return authoredError(
			"start", "outside_floor", "start %v out of bounds: column %d not in [0,%d)", at, at[0], totalWidth,
		)
	}

	roomIndex := -1
	for i, room := range spec.Rooms {
		if at[0] < starts[i] || at[0] >= starts[i]+room.Width {
			continue
		}
		if roomIndex != -1 {
			return authoredError("start", "invalid_start", "start %v belongs to more than one semantic room", at)
		}
		roomIndex = i
	}
	if roomIndex == -1 {
		return authoredError(
			"start", "outside_floor", "start %v is a connector gap/door cell, not a semantic room floor cell", at,
		)
	}

	room := &spec.Rooms[roomIndex]
	local := [2]int{at[0] - starts[roomIndex], at[1]}
	if room.Boss != nil && room.Boss.At != nil && *room.Boss.At == local {
		return authoredError("start", "occupied", "start %v conflicts with pinned boss in room %q", at, room.ID)
	}
	for _, entry := range room.Place {
		if entry.At != local {
			continue
		}
		refType, err := refParts(entry.Ref)
		if err != nil {
			return authoredError("start", "invalid_ref", "start %v: place %q: %v", at, entry.Ref, err)
		}
		switch refType {
		case refTypeMonsters:
			return authoredError(
				"start", "occupied", "start %v conflicts with placed monster %q in room %q", at, entry.Ref, room.ID,
			)
		case refTypeProps:
			if boolOrTrue(entry.BlocksMovement) {
				return authoredError(
					"start", "occupied", "start %v conflicts with movement-blocking prop %q in room %q",
					at, entry.Ref, room.ID,
				)
			}
		}
	}
	return nil
}

// validateWalls enforces the dungeon-scoped authored-edge grammar against
// the declared absolute room footprint. It deliberately treats a connector
// column as non-floor even though it lives inside the rectangle: authored
// edges are between semantic cells, never a re-expression of a connector door
// or its flanking collision geometry.
func validateWalls(spec *DungeonSpec) error {
	if len(spec.Walls) == 0 {
		return nil
	}
	floor, totalWidth := semanticFloorCells(spec)
	return validateWallsOnFloor(spec, floor, totalWidth, spec.Height)
}

func validateWallsOnFloor(spec *DungeonSpec, floor map[[2]int]struct{}, totalWidth, height int) error {
	seen := make(map[wallEdgeKey]int, len(spec.Walls))
	seenKinds := make([]string, 0, len(spec.Walls))
	for index, wall := range spec.Walls {
		from, err := validateWallEndpoint(index, "from", wall.From, floor, totalWidth, height)
		if err != nil {
			return err
		}
		to, err := validateWallEndpoint(index, "to", wall.To, floor, totalWidth, height)
		if err != nil {
			return err
		}
		switch wall.Kind {
		case string(encounter.GeneratedEdgeKindSolid), string(encounter.GeneratedEdgeKindDoor):
		default:
			return authoredError(
				fmt.Sprintf("walls[%d].kind", index), "invalid_wall",
				"walls[%d].kind: invalid kind %q (must be %q or %q)", index, wall.Kind,
				encounter.GeneratedEdgeKindSolid, encounter.GeneratedEdgeKindDoor,
			)
		}
		if wall.Lock != nil {
			if wall.Kind == string(encounter.GeneratedEdgeKindSolid) {
				return authoredError(
					fmt.Sprintf("walls[%d].lock", index), "invalid_wall",
					"walls[%d].lock: lock only valid on door", index,
				)
			}
			if len(wall.Lock.Options) == 0 {
				return authoredError(
					fmt.Sprintf("walls[%d].lock.options", index), "invalid_wall",
					"walls[%d].lock.options: must contain at least one option", index,
				)
			}
			for oi, option := range wall.Lock.Options {
				if option.DC < minLockDC || option.DC > maxLockDC {
					return authoredError(
						fmt.Sprintf("walls[%d].lock.options[%d].dc", index, oi), "invalid_wall",
						"walls[%d].lock.options[%d].dc: must be between %d and %d",
						index, oi, minLockDC, maxLockDC,
					)
				}
				if _, err := abilities.GetByID(option.Ability); err != nil {
					return authoredError(
						fmt.Sprintf("walls[%d].lock.options[%d].ability", index, oi), "invalid_wall",
						"walls[%d].lock.options[%d].ability: invalid ability %q", index, oi, option.Ability,
					)
				}
			}
		}
		if from == to {
			return authoredError(fmt.Sprintf("walls[%d]", index), "invalid_wall", "walls[%d]: endpoints must be distinct", index)
		}
		if from.ToCube().Distance(to.ToCube()) != 1 {
			return authoredError(
				fmt.Sprintf("walls[%d]", index), "invalid_wall",
				"walls[%d]: endpoints must be adjacent pointy-top odd-q floor hexes", index,
			)
		}

		key := newWallEdgeKey(from, to)
		if first, exists := seen[key]; exists {
			if seenKinds[first] == wall.Kind {
				return authoredError(
					fmt.Sprintf("walls[%d]", index), "duplicate_wall",
					"walls[%d]: duplicate of walls[%d] (including reversed endpoints)", index, first,
				)
			}
			return authoredError(
				fmt.Sprintf("walls[%d]", index), "conflicting_wall",
				"walls[%d]: conflicting kind with walls[%d] on the same undirected edge", index, first,
			)
		}
		seen[key] = index
		seenKinds = append(seenKinds, wall.Kind)
	}
	return nil
}

// semanticFloorCells lays out the v1 linear chain in the same absolute column
// frame compileWithConfig uses. It returns only cells owned by a semantic room;
// connector columns are intentionally absent.
func semanticFloorCells(spec *DungeonSpec) (map[[2]int]struct{}, int) {
	totalWidth := 0
	for index, room := range spec.Rooms {
		totalWidth += room.Width
		if index < len(spec.Rooms)-1 {
			totalWidth++
		}
	}
	floor := make(map[[2]int]struct{})
	offsetX := 0
	for index, room := range spec.Rooms {
		for col := 0; col < room.Width; col++ {
			for row := 0; row < spec.Height; row++ {
				floor[[2]int{offsetX + col, row}] = struct{}{}
			}
		}
		offsetX += room.Width
		if index < len(spec.Rooms)-1 {
			offsetX++
		}
	}
	return floor, totalWidth
}

func validateWallEndpoint(
	wallIndex int,
	field string,
	at *[2]int,
	floor map[[2]int]struct{},
	totalWidth, height int,
) (core.Hex, error) {
	path := fmt.Sprintf("walls[%d].%s", wallIndex, field)
	if at == nil {
		return core.Hex{}, authoredError(path, "invalid_wall", "%s: required absolute [column, row] floor cell", path)
	}
	if at[0] < 0 || at[0] >= totalWidth || at[1] < 0 || at[1] >= height {
		return core.Hex{}, authoredError(path, "outside_floor", "%s %v is out of dungeon floor footprint", path, *at)
	}
	if _, ok := floor[*at]; !ok {
		return core.Hex{}, authoredError(
			path, "outside_floor", "%s %v is a connector gap/door cell, not a semantic floor cell", path, *at,
		)
	}
	return core.HexFromPosition(spatial.Position{X: float64(at[0]), Y: float64(at[1])}), nil
}

type wallEdgeKey struct {
	first  core.Hex
	second core.Hex
}

func newWallEdgeKey(from, to core.Hex) wallEdgeKey {
	if wallHexLess(to, from) {
		return wallEdgeKey{first: to, second: from}
	}
	return wallEdgeKey{first: from, second: to}
}

func wallHexLess(left, right core.Hex) bool {
	if left.Q != right.Q {
		return left.Q < right.Q
	}
	if left.R != right.R {
		return left.R < right.R
	}
	return left.S < right.S
}

// validateLocked checks a connector's lock, prefixing errors with the
// connector's location (from -> to) since a spec can have several.
func validateLocked(c *ConnectorSpec, connectorIndex int) error {
	locked := c.Locked
	path := fmt.Sprintf("connectors[%d].locked", connectorIndex)
	if locked.DC < minLockDC || locked.DC > maxLockDC {
		return authoredError(
			path+".dc", "invalid_lock", "lock dc must be between %d and %d, got %d",
			minLockDC, maxLockDC, locked.DC,
		)
	}
	if _, err := abilities.GetByID(locked.Ability); err != nil {
		known := make([]string, 0, len(abilities.List()))
		for _, ability := range abilities.List() {
			known = append(known, string(ability))
		}
		return authoredError(
			path+".ability", "invalid_lock", "lock ability %q: %v (known: %s)",
			locked.Ability, err, strings.Join(known, ", "),
		)
	}
	return nil
}

// refParts returns a ref's type segment, erroring if the shape isn't
// module:type:id with all three segments non-empty.
func refParts(ref string) (refType string, err error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("ref %q must be shaped like module:type:id", ref)
	}
	return parts[1], nil
}
