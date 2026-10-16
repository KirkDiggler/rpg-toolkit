// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// v4_at_path_test.go pins `toward: { at }` and `toward: { path }` in the
// single-room dialect, in the room's own axial frame (rpg-project#605, the mad
// king castle's D1a). rpg-toolkit#1826 ruling 1 refused both "until the sites
// layer gives a creature somewhere to walk to"; a patrol and a march inside
// one room are that somewhere.

const v4PatrolsPath = "testdata/world-builder-v4-patrols.yaml"

// thePatrolsInVersionTwo is the patrols fixture's two creatures and their
// orders in the OTHER dialect, on the same hexes: every cell below is the
// pointy-top offset of the axial cell the v4 fixture writes, so the two
// documents name the same dungeon-absolute cells in two spellings. The floor
// is a rectangle that covers the v4 room's hexagon, because a v2 region is
// painted in rows; no route below leaves the hexagon.
const thePatrolsInVersionTwo = `
version: 2
key: castle-patrols
name: Gatehouse
orientation: pointy
void: transparent
regions:
  - id: room-1-region
    name: Gatehouse
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[-3,-3],[-2,-3],[-1,-3],[0,-3],[1,-3],[2,-3],[3,-3]]
      - [[-3,-2],[-2,-2],[-1,-2],[0,-2],[1,-2],[2,-2],[3,-2]]
      - [[-3,-1],[-2,-1],[-1,-1],[0,-1],[1,-1],[2,-1],[3,-1]]
      - [[-3,0],[-2,0],[-1,0],[0,0],[1,0],[2,0],[3,0]]
      - [[-3,1],[-2,1],[-1,1],[0,1],[1,1],[2,1],[3,1]]
      - [[-3,2],[-2,2],[-1,2],[0,2],[1,2],[2,2],[3,2]]
      - [[-3,3],[-2,3],[-1,3],[0,3],[1,3],[2,3],[3,3]]
start: { at: [-3,0] }
place:
  - id: gate-patrol
    ref: "dnd5e:monsters:goblin"
    at: [-2,0]
    on:
      time:
        - { when: { enemy: none }, toward: { path: [[-2,0], [-1,-2], [2,0], [1,2]] } }
  - id: keep-patrol
    ref: "dnd5e:monsters:goblin"
    at: [0,0]
    on:
      time:
        - { when: { enemy: none }, toward: { at: [1,1] } }
`

// TestTheRoomFrameCompilesTheSamePatrolAsV2 is the claim the axial spelling
// makes: a route and a cell written in the single room's `{ q, r }` compile to
// EXACTLY the selector a v2 document's offset cells compile to for the same
// hexes. The compiled value is compared, not the YAML — the spelling is the
// dialect's, the cell is the engine's.
func TestTheRoomFrameCompilesTheSamePatrolAsV2(t *testing.T) {
	raw, err := os.ReadFile(v4PatrolsPath)
	require.NoError(t, err)
	room, err := Load(raw)
	require.NoError(t, err)
	twin, err := Load([]byte(thePatrolsInVersionTwo))
	require.NoError(t, err)

	byID := func(c Compiled, id string) MonsterPlacement {
		for _, m := range c.Monsters {
			if m.ID == id {
				return m
			}
		}
		t.Fatalf("no monster %q", id)

		return MonsterPlacement{}
	}
	for _, id := range []string{"gate-patrol", "keep-patrol"} {
		t.Run(id, func(t *testing.T) {
			got := byID(room, id).Table[encounter.AnswerTime]
			want := byID(twin, id).Table[encounter.AnswerTime]
			require.NotEmpty(t, want)
			require.NotNil(t, want[0].Toward)
			require.Equal(t, want[0].Toward, got[0].Toward, "the same compiled selector in either spelling")
		})
	}

	// Not two empty selectors agreeing: the route really is the four hexes
	// the room wrote, in order, and the cell is the one it wrote.
	gate := byID(room, "gate-patrol").Table[encounter.AnswerTime][0].Toward
	require.Equal(t, hexCell(RoomCell{Q: -2, R: 0}), gate.Path[0])
	require.Equal(t, hexCell(RoomCell{Q: 0, R: -2}), gate.Path[1])
	require.Equal(t, hexCell(RoomCell{Q: 2, R: 0}), gate.Path[2])
	require.Equal(t, hexCell(RoomCell{Q: 0, R: 2}), gate.Path[3])
	keep := byID(room, "keep-patrol").Table[encounter.AnswerTime][0].Toward
	require.NotNil(t, keep.At)
	require.Equal(t, hexCell(RoomCell{Q: 1, R: 1}), *keep.At)
}

// patrolsRefusal loads the patrols fixture with one line swapped and returns
// what it was refused with.
func patrolsRefusal(t *testing.T, old, repl string) []FieldError {
	t.Helper()
	raw, err := os.ReadFile(v4PatrolsPath)
	require.NoError(t, err)
	_, err = Load(swapOneIn(t, raw, old, repl))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBadSpec)
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)

	return validation.Errors
}

const (
	keepAt    = "toward: {at: {q: 1, r: 1}}"
	gateRoute = "toward: {path: [{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}]}"
)

// TestTheOffsetSpellingIsRefusedInTheRoom: `[4, 2]` is an offset in a document
// orientation the single room does not have, so the same bytes would name a
// different cell than they do in v2. Refused at the author's path, in a
// sentence that names the spelling to write.
func TestTheOffsetSpellingIsRefusedInTheRoom(t *testing.T) {
	errs := patrolsRefusal(t, keepAt, "toward: {at: [4, 2]}")
	require.Contains(t, errs, FieldError{
		Path: "room.room.monsterBindings.keep-patrol.on.time[0].toward.at",
		Message: "at names a cell of this room, written { q, r } as its walkable hexes are: " +
			"[col, row] is an offset in a document orientation this dialect does not have",
	})

	// And a route with ONE offset waypoint is refused whole, once.
	errs = patrolsRefusal(t, gateRoute, "toward: {path: [{q: -2, r: 0}, [0, -2], {q: 2, r: 0}]}")
	require.Contains(t, errs, FieldError{
		Path: "room.room.monsterBindings.gate-patrol.on.time[0].toward.path",
		Message: "path names a route of this room's cells, each written { q, r } as its walkable hexes are: " +
			"[col, row] is an offset in a document orientation this dialect does not have",
	})
}

// TestAWaypointOffTheFloorIsRefusedByIndex: the room's floor is its walkable
// hexes, and a waypoint that is not one is refused at its own index in the
// sentence the v2 dialect uses for its floor, spelt the way this author wrote
// it. A cell off the floor is the same refusal at `at`.
func TestAWaypointOffTheFloorIsRefusedByIndex(t *testing.T) {
	errs := patrolsRefusal(t, gateRoute,
		"toward: {path: [{q: -2, r: 0}, {q: 5, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}]}")
	require.Contains(t, errs, FieldError{
		Path:    "room.room.monsterBindings.gate-patrol.on.time[0].toward.path[1]",
		Message: "this walks to {q: 5, r: -2}, which is not floor",
	})

	errs = patrolsRefusal(t, keepAt, "toward: {at: {q: 9, r: 9}}")
	require.Contains(t, errs, FieldError{
		Path:    "room.room.monsterBindings.keep-patrol.on.time[0].toward.at",
		Message: "this walks to {q: 9, r: 9}, which is not floor",
	})
}

// TestARouteThatRetracesItselfReachesTheRoomAuthor: the route rules live
// below the grammar ([grammar.routeLegs], [encounter.RetracedCell]), so a
// route whose legs share a cell is refused in this dialect exactly as in v2 —
// at the route's path, naming the shared cell in the author's own spelling.
func TestARouteThatRetracesItselfReachesTheRoomAuthor(t *testing.T) {
	// Out east along r = 0 and back again: the second leg walks the first.
	errs := patrolsRefusal(t, gateRoute, "toward: {path: [{q: -2, r: 0}, {q: 2, r: 0}, {q: 0, r: 0}]}")
	require.Contains(t, errs, FieldError{
		Path: "room.room.monsterBindings.gate-patrol.on.time[0].toward.path",
		Message: "this route retraces itself at {q: 1, r: 0}: a patrol walks a loop — " +
			"add a waypoint so no leg crosses another",
	})
}

// TestALegOffTheFloorReachesTheRoomAuthor: the other route rule. Both
// waypoints are walkable, but the hex line between them crosses a hex the
// room does not have.
func TestALegOffTheFloorReachesTheRoomAuthor(t *testing.T) {
	raw, err := os.ReadFile(v4PatrolsPath)
	require.NoError(t, err)
	// Cut the centre out of the floor: the diamond's legs go round it, but a
	// route straight across does not.
	holed := swapOneIn(t, raw, "{q: 0, r: 0}, {q: 0, r: 1}", "{q: 0, r: 1}")
	holed = swapOneIn(t, holed, "startingCell: { location: { q: 0, r: 0 } }", "startingCell: { location: { q: 1, r: 0 } }")
	holed = swapOneIn(t, holed, gateRoute, "toward: {path: [{q: -2, r: 0}, {q: 2, r: 0}, {q: 0, r: 2}]}")
	_, err = Load(holed)
	require.Error(t, err)
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Contains(t, validation.Errors, FieldError{
		Path: "room.room.monsterBindings.gate-patrol.on.time[0].toward.path",
		Message: "leg 1 of this route walks through {q: 0, r: 0}, which is not floor: " +
			"move a waypoint so the line between them stays on the floor",
	})
}

// TestACellUnderAttackIsTheWordsDefectInTheRoom: once the spelling is the
// room's, the word carrying it is judged — a cell is somewhere to walk
// toward, never something to attack — exactly as v2 judges it.
func TestACellUnderAttackIsTheWordsDefectInTheRoom(t *testing.T) {
	errs := patrolsRefusal(t, keepAt, "attack: {at: {q: 1, r: 1}}")
	var found bool
	for _, e := range errs {
		if e.Path == "room.room.monsterBindings.keep-patrol.on.time[0].attack" &&
			strings.HasPrefix(e.Message, "a cell is somewhere to walk toward, and `attack` acts on a creature") {
			found = true
		}
	}
	require.True(t, found, "the word's refusal, at the word: %v", errs)
}

// TestTheFactionLevelCellTakesTheRoomFrameToo: a faction's inherited `on:` is
// judged by the same grammar a binding's is, so it takes `{ q, r }` and
// refuses the offset at `factions[0]…` in the same sentences.
func TestTheFactionLevelCellTakesTheRoomFrameToo(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	const row = "        - {when: {enemy: reach}, attack: enemy}"

	_, err = Load(swapOneIn(t, raw, row, "        - {when: {enemy: none}, toward: {at: {q: 1, r: 0}}}"))
	require.NoError(t, err, "a walkable hex of the room, at the faction level")
	_, err = Load(swapOneIn(t, raw, row,
		"        - {when: {enemy: none}, toward: {path: [{q: -1, r: 0}, {q: 1, r: -1}, {q: 0, r: 1}]}}"))
	require.NoError(t, err, "a route of walkable hexes, at the faction level")

	_, err = Load(swapOneIn(t, raw, row, "        - {when: {enemy: none}, toward: {at: {q: 4, r: 4}}}"))
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Contains(t, validation.Errors, FieldError{
		Path: "factions[0].on.time[0].toward.at", Message: "this walks to {q: 4, r: 4}, which is not floor",
	})
}

// TestTheV2DialectRefusesTheRoomsSpelling is the other side: v2's frame is its
// orientation's offset pairs, so `{ q, r }` in a v2 file is refused by path
// rather than read as a cell — one spelling, one meaning, in each dialect.
func TestTheV2DialectRefusesTheRoomsSpelling(t *testing.T) {
	refused := func(t *testing.T, repl string) []FieldError {
		t.Helper()
		source := strings.Replace(theSameCampInVersionTwo,
			"        - { when: { enemy: reach }, hold: {} }",
			"        - { when: { enemy: none }, "+repl+" }", 1)
		_, err := Load([]byte(source))
		require.Error(t, err)
		var validation *ValidationError
		require.ErrorAs(t, err, &validation)

		return validation.Errors
	}
	require.Contains(t, refused(t, "toward: { at: { q: 3, r: 1 } }"), FieldError{
		Path: "place[0].on.time[0].toward.at",
		Message: "at names a cell of this document in its orientation, written [col, row]: " +
			"{ q, r } is the single-room dialect's axial cell",
	})
	require.Contains(t, refused(t, "toward: { path: [[0,0], { q: 3, r: 1 }] }"), FieldError{
		Path: "place[0].on.time[0].toward.path",
		Message: "path names a route of this document's cells, each written [col, row]: " +
			"{ q, r } is the single-room dialect's axial cell",
	})
}

// TestAnAxialCellIsReadStrictly: a nested decode does not keep the document's
// KnownFields, so `{ q, r }` is read strictly by hand — a missing `r` read as
// 0 would send a creature to a cell nobody wrote.
func TestAnAxialCellIsReadStrictly(t *testing.T) {
	raw, err := os.ReadFile(v4PatrolsPath)
	require.NoError(t, err)
	for name, tc := range map[string]struct{ repl, want string }{
		"a missing r":     {"toward: {at: {q: 1}}", "a cell is { q, r }, and this one has no r"},
		"a third key":     {"toward: {at: {q: 1, r: 1, s: -2}}", "s"},
		"a fractional q":  {"toward: {at: {q: 0.5, r: 1}}", "a cell's q must be an integer"},
		"a null r":        {"toward: {at: {q: 1, r: ~}}", "a cell's r must be an integer"},
		"a waypoint's r":  {"toward: {path: [{q: 1}, {q: 2, r: 0}]}", "a cell is { q, r }, and this one has no r"},
		"a text waypoint": {"toward: {path: [{q: '1', r: 0}, {q: 2, r: 0}]}", "a cell's q must be an integer"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(swapOneIn(t, raw, keepAt, tc.repl))
			require.Error(t, err)
			require.ErrorIs(t, err, ErrBadSpec)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestADefaultTableCannotNameAnAxialCell: a kind's table has no room, so
// neither spelling of a cell is legal in it.
func TestADefaultTableCannotNameAnAxialCell(t *testing.T) {
	for _, source := range []string{
		"time:\n  - { toward: { at: { q: 1, r: 1 } } }\n",
		"time:\n  - { toward: { path: [{ q: 1, r: 1 }, { q: 2, r: 1 }] } }\n",
	} {
		_, err := CompileTable(source)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBadSpec)
		require.Contains(t, err.Error(), "belongs to a kind and not to a map")
	}
}

// TestTheDecodeHintsSpeakTheRoomsSpelling: the bad-selector and `patrol`
// refusals are the decoder's, shared by both dialects, and their hint names a
// cell spelling. A single-room author is told to write `{ q, r }`; a v2 author
// still reads `[col, row]` word for word (hearing_test.go, table_test.go).
func TestTheDecodeHintsSpeakTheRoomsSpelling(t *testing.T) {
	raw, err := os.ReadFile(v4PatrolsPath)
	require.NoError(t, err)
	for name, tc := range map[string]struct{ repl, want string }{
		"a word nothing resolves": {"toward: sideways",
			`"sideways" is not a selector this build resolves: they are enemy, attacker, actor, sound, ` +
				"or { at: { q, r } }, or { path: [{ q, r }, …] }"},
		"a selector that is a list": {"toward: [1, 1]",
			"a selector is one of enemy, attacker, actor, sound, or { at: { q, r } }, or { path: [{ q, r }, …] }"},
		"`patrol`": {"patrol: {}",
			"`patrol` is not a word: a patrol is a route, write toward: { path: [{ q, r }, …] }"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(swapOneIn(t, raw, keepAt, tc.repl))
			require.Error(t, err)
			require.ErrorIs(t, err, ErrBadSpec)
			require.Contains(t, err.Error(), tc.want)
			require.NotContains(t, err.Error(), "[col, row]", "no v2 spelling reaches a room author")
		})
	}
}
