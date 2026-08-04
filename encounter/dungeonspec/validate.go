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
	facingFloorPropsOnly  = "facing only supported on room-scoped floor props"
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
		return fmt.Errorf("unsupported spec version %d (must be 1)", spec.Version)
	}
	if strings.TrimSpace(spec.Key) == "" {
		return fmt.Errorf("key must not be empty")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("name must not be empty")
	}
	if spec.Height < minHeight {
		return fmt.Errorf("height must be at least %d, got %d", minHeight, spec.Height)
	}
	if len(spec.Rooms) < minRooms {
		return fmt.Errorf("must have at least %d rooms, got %d", minRooms, len(spec.Rooms))
	}

	if err := validateUniqueRoomIDs(spec.Rooms); err != nil {
		return err
	}
	if err := validateChain(spec); err != nil {
		return err
	}

	for i := range spec.Rooms {
		room := &spec.Rooms[i]
		if room.Width < minWidth {
			return fmt.Errorf("room %q: width must be at least %d, got %d", room.ID, minWidth, room.Width)
		}
		if err := validateArchetype(room.Archetype); err != nil {
			return fmt.Errorf("room %q: %w", room.ID, err)
		}
		if err := validatePattern(room.Pattern); err != nil {
			return fmt.Errorf("room %q: %w", room.ID, err)
		}
	}

	bossRoom, err := validateBossCardinality(spec.Rooms)
	if err != nil {
		return err
	}

	if err := validateBossAxis(bossRoom, spec.Height); err != nil {
		return err
	}

	if err := validateM1Restrictions(spec, bossRoom); err != nil {
		return err
	}

	for i := range spec.Rooms {
		room := &spec.Rooms[i]
		for _, o := range room.Obstacles {
			if _, err := refParts(o.Ref); err != nil {
				return fmt.Errorf("room %q: obstacle %w", room.ID, err)
			}
			if o.Count < minCount {
				return fmt.Errorf("room %q: obstacle %q count must be at least %d, got %d",
					room.ID, o.Ref, minCount, o.Count)
			}
		}
	}

	if err := validateBossRef(bossRoom); err != nil {
		return err
	}

	for i := range spec.Rooms {
		if err := validatePlaceBlock(&spec.Rooms[i], spec.Height, i); err != nil {
			return err
		}
	}
	if err := validateTopLevelPlace(spec.Place); err != nil {
		return err
	}

	if err := validateStart(spec); err != nil {
		return err
	}
	if err := validateWalls(spec); err != nil {
		return err
	}

	for i := range spec.Connectors {
		c := &spec.Connectors[i]
		if c.Locked != nil {
			if err := validateLocked(c); err != nil {
				return err
			}
		}
	}

	return nil
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
		room := &spec.Rooms[i]
		if len(room.Monsters) > 0 {
			return fmt.Errorf("room %q: monsters: rolled monster placement lands in M2", room.ID)
		}
	}
	if bossRoom.Boss.At == nil {
		return fmt.Errorf("room %q: boss.at: rolled monster placement lands in M2", bossRoom.ID)
	}
	return nil
}

func validateUniqueRoomIDs(rooms []RoomSpec) error {
	seen := make(map[string]bool, len(rooms))
	for _, room := range rooms {
		if seen[room.ID] {
			return fmt.Errorf("duplicate room id %q", room.ID)
		}
		seen[room.ID] = true
	}
	return nil
}

// validateChain enforces the v1 generator constraint: connectors join rooms
// in a single linear chain, in room order (rooms[i] -> rooms[i+1]).
func validateChain(spec *DungeonSpec) error {
	if len(spec.Connectors) != len(spec.Rooms)-1 {
		return fmt.Errorf("connectors must form a linear chain: expected %d connectors for %d rooms, got %d",
			len(spec.Rooms)-1, len(spec.Rooms), len(spec.Connectors))
	}
	for i, c := range spec.Connectors {
		if c.From != spec.Rooms[i].ID || c.To != spec.Rooms[i+1].ID {
			return fmt.Errorf("connectors must form a linear chain: connector %d (%s -> %s) must join room %q to room %q",
				i, c.From, c.To, spec.Rooms[i].ID, spec.Rooms[i+1].ID)
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
func validateBossCardinality(rooms []RoomSpec) (*RoomSpec, error) {
	var bossRoom *RoomSpec
	bossCount := 0
	for i := range rooms {
		room := &rooms[i]
		if room.Archetype == archetypeBoss {
			bossCount++
			if room.Boss == nil {
				return nil, fmt.Errorf("room %q: boss room must declare boss", room.ID)
			}
			bossRoom = room
		} else if room.Boss != nil {
			return nil, fmt.Errorf("room %q: boss entry only on the boss room", room.ID)
		}
	}
	if bossCount != 1 {
		return nil, fmt.Errorf("dungeon must have exactly one boss room, found %d", bossCount)
	}
	return bossRoom, nil
}

func validateBossAxis(bossRoom *RoomSpec, height int) error {
	axis := min(bossRoom.Width, height)
	if axis <= bossAxisMin {
		return fmt.Errorf("room %q: boss room primary axis (min(width, height)=%d) must exceed %d",
			bossRoom.ID, axis, bossAxisMin)
	}
	return nil
}

// validateBossRef checks the boss room's monster ref: shape, that it names
// a monster (not a prop), and that it resolves via the registry.
func validateBossRef(bossRoom *RoomSpec) error {
	refType, err := refParts(bossRoom.Boss.Ref)
	if err != nil {
		return fmt.Errorf("room %q: boss %w", bossRoom.ID, err)
	}
	if refType != refTypeMonsters {
		return fmt.Errorf("room %q: boss ref %q must be a monster ref, got type %q", bossRoom.ID, bossRoom.Boss.Ref, refType)
	}
	if _, ok := monsters.ByRef(bossRoom.Boss.Ref); !ok {
		return fmt.Errorf("room %q: boss ref %q: unknown monster ref (known: %s)",
			bossRoom.ID, bossRoom.Boss.Ref, strings.Join(monsters.Refs(), ", "))
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
		return fmt.Errorf("room %q: place/boss.at not allowed with pattern: scattered", room.ID)
	}

	doorRow := height / 2
	occupied := make(map[[2]int]string, len(room.Place)+1)

	if room.Boss != nil && room.Boss.Facing != nil {
		return fmt.Errorf("rooms[%d].boss.facing: %s: %s", roomIndex, unsupportedCapability, facingFloorPropsOnly)
	}
	if room.Boss != nil && room.Boss.At != nil {
		at := *room.Boss.At
		if err := checkCellBounds(room, height, at); err != nil {
			return fmt.Errorf("room %q: boss.at %w", room.ID, err)
		}
		if at[1] == doorRow {
			return fmt.Errorf("room %q: boss.at %v is on the reserved row (height/2=%d)", room.ID, at, doorRow)
		}
		occupied[at] = room.Boss.Ref
	}

	for entryIndex, entry := range room.Place {
		if err := checkCellBounds(room, height, entry.At); err != nil {
			return fmt.Errorf("room %q: place %q %w", room.ID, entry.Ref, err)
		}
		if entry.At[1] == doorRow {
			return fmt.Errorf("room %q: place %q at %v is on the reserved row (height/2=%d)",
				room.ID, entry.Ref, entry.At, doorRow)
		}
		if occupant, ok := occupied[entry.At]; ok {
			return fmt.Errorf("room %q: place %q at %v is already placed (occupied by %q)",
				room.ID, entry.Ref, entry.At, occupant)
		}
		occupied[entry.At] = entry.Ref

		refType, err := refParts(entry.Ref)
		if err != nil {
			return fmt.Errorf("room %q: place %w", room.ID, err)
		}
		// Ref-type routing must be checked before the flags-only-on-props
		// check below: an entry with an unrecognized ref type should always
		// report that error, not whichever check happens to run first.
		if refType != refTypeProps && refType != refTypeMonsters {
			return fmt.Errorf("room %q: place ref %q must be props or monsters, got type %q",
				room.ID, entry.Ref, refType)
		}

		path := fmt.Sprintf("rooms[%d].place[%d]", roomIndex, entryIndex)
		if entry.Facing != nil {
			if refType != refTypeProps || entry.Mount != nil {
				return fmt.Errorf("%s.facing: %s: %s", path, unsupportedCapability, facingFloorPropsOnly)
			}
			if err := validateFacing(*entry.Facing); err != nil {
				return fmt.Errorf("%s.facing: %w", path, err)
			}
		}
		if entry.Mount != nil {
			return fmt.Errorf("%s.mount: %s: mounted placements are not supported", path, unsupportedCapability)
		}

		if room.Boss != nil && entry.Ref == room.Boss.Ref {
			return fmt.Errorf("room %q: boss ref may not also appear in place (ref %q)", room.ID, entry.Ref)
		}

		if refType == refTypeMonsters {
			if _, ok := monsters.ByRef(entry.Ref); !ok {
				return fmt.Errorf("room %q: place %q: unknown monster ref (known: %s)",
					room.ID, entry.Ref, strings.Join(monsters.Refs(), ", "))
			}
			if entry.BlocksMovement != nil {
				return fmt.Errorf("room %q: place %q: blocks_movement only valid on props", room.ID, entry.Ref)
			}
			if entry.BlocksLoS != nil {
				return fmt.Errorf("room %q: place %q: blocks_los only valid on props", room.ID, entry.Ref)
			}
		}
	}

	return nil
}

// validateTopLevelPlace retains the canonical syntax long enough to reject
// unsupported top-level placement capability at the author-supplied field.
// It deliberately never compiles or maps entries into room-scoped placement.
func validateTopLevelPlace(entries []PlacedEntry) error {
	for index, entry := range entries {
		if entry.Facing != nil {
			return fmt.Errorf("place[%d].facing: %s: %s", index, unsupportedCapability, facingFloorPropsOnly)
		}
		if entry.Mount != nil {
			return fmt.Errorf("place[%d].mount: %s: mounted placements are not supported", index, unsupportedCapability)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	return fmt.Errorf("place[0]: %s: top-level placement is not supported", unsupportedCapability)
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
		return fmt.Errorf("start %v out of bounds: row %d not in [0,%d)", at, at[1], spec.Height)
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
		return fmt.Errorf("start %v out of bounds: column %d not in [0,%d)", at, at[0], totalWidth)
	}

	roomIndex := -1
	for i, room := range spec.Rooms {
		if at[0] < starts[i] || at[0] >= starts[i]+room.Width {
			continue
		}
		if roomIndex != -1 {
			return fmt.Errorf("start %v belongs to more than one semantic room", at)
		}
		roomIndex = i
	}
	if roomIndex == -1 {
		return fmt.Errorf("start %v is a connector gap/door cell, not a semantic room floor cell", at)
	}

	room := &spec.Rooms[roomIndex]
	local := [2]int{at[0] - starts[roomIndex], at[1]}
	if room.Boss != nil && room.Boss.At != nil && *room.Boss.At == local {
		return fmt.Errorf("start %v conflicts with pinned boss in room %q", at, room.ID)
	}
	for _, entry := range room.Place {
		if entry.At != local {
			continue
		}
		refType, err := refParts(entry.Ref)
		if err != nil {
			return fmt.Errorf("start %v: place %q: %w", at, entry.Ref, err)
		}
		switch refType {
		case refTypeMonsters:
			return fmt.Errorf("start %v conflicts with placed monster %q in room %q", at, entry.Ref, room.ID)
		case refTypeProps:
			if boolOrTrue(entry.BlocksMovement) {
				return fmt.Errorf("start %v conflicts with movement-blocking prop %q in room %q", at, entry.Ref, room.ID)
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
	seen := make(map[wallEdgeKey]int, len(spec.Walls))
	seenKinds := make([]string, 0, len(spec.Walls))
	for index, wall := range spec.Walls {
		from, err := validateWallEndpoint(index, "from", wall.From, floor, totalWidth, spec.Height)
		if err != nil {
			return err
		}
		to, err := validateWallEndpoint(index, "to", wall.To, floor, totalWidth, spec.Height)
		if err != nil {
			return err
		}
		switch wall.Kind {
		case string(encounter.GeneratedEdgeKindSolid), string(encounter.GeneratedEdgeKindDoor):
		default:
			return fmt.Errorf("walls[%d].kind: invalid kind %q (must be %q or %q)", index, wall.Kind,
				encounter.GeneratedEdgeKindSolid, encounter.GeneratedEdgeKindDoor)
		}
		if from == to {
			return fmt.Errorf("walls[%d]: endpoints must be distinct", index)
		}
		if from.ToCube().Distance(to.ToCube()) != 1 {
			return fmt.Errorf("walls[%d]: endpoints must be adjacent pointy-top odd-q floor hexes", index)
		}

		key := newWallEdgeKey(from, to)
		if first, exists := seen[key]; exists {
			if seenKinds[first] == wall.Kind {
				return fmt.Errorf("walls[%d]: duplicate of walls[%d] (including reversed endpoints)", index, first)
			}
			return fmt.Errorf("walls[%d]: conflicting kind with walls[%d] on the same undirected edge", index, first)
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
		return core.Hex{}, fmt.Errorf("%s: required absolute [column, row] floor cell", path)
	}
	if at[0] < 0 || at[0] >= totalWidth || at[1] < 0 || at[1] >= height {
		return core.Hex{}, fmt.Errorf("%s %v is out of dungeon floor footprint", path, *at)
	}
	if _, ok := floor[*at]; !ok {
		return core.Hex{}, fmt.Errorf("%s %v is a connector gap/door cell, not a semantic floor cell", path, *at)
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
func validateLocked(c *ConnectorSpec) error {
	locked := c.Locked
	if locked.DC < minLockDC || locked.DC > maxLockDC {
		return fmt.Errorf("connector %q -> %q: lock dc must be between %d and %d, got %d",
			c.From, c.To, minLockDC, maxLockDC, locked.DC)
	}
	if _, err := abilities.GetByID(locked.Ability); err != nil {
		// abilities.GetByID's error (verified empirically) renders as just
		// "invalid ability" — its valid_options live only in structured
		// metadata callers don't see via Error(). Build the known-abilities
		// list ourselves, parallel to the monster known-refs treatment.
		known := make([]string, 0, len(abilities.List()))
		for _, a := range abilities.List() {
			known = append(known, string(a))
		}
		return fmt.Errorf("connector %q -> %q: lock ability %q: %w (known: %s)",
			c.From, c.To, locked.Ability, err, strings.Join(known, ", "))
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
