// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// bundles_test.go is THE NESTED TABLE AT THE FORM (rpg-project#670, R12–R16):
// `bundles:` declared once at the root, `use:` rows resolved into the group
// arm with their parameters substituted, every refusal in a sentence for the
// form, and patterns judged against the names the file authored.

// bundledPatrol is the two-patrol room (world-builder-v4-patrols.yaml) with
// the given `bundles:` at its root and the given rows as gate-patrol's
// `time` table. Each row is one YAML list item at the table's indent.
func bundledPatrol(t *testing.T, bundles string, rows ...string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/world-builder-v4-patrols.yaml")
	require.NoError(t, err)
	source := string(raw)
	old := "            - {when: {enemy: none}, toward: {path: [{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}]}}\n"
	require.Contains(t, source, old, "the fixture still carries gate-patrol's one row")
	table := ""
	for _, row := range rows {
		table += "            - " + strings.ReplaceAll(row, "\n", "\n              ") + "\n"
	}
	source = strings.Replace(source, old, table, 1)
	if bundles != "" {
		source = strings.Replace(source, "room:\n  version: 3", "bundles:\n"+bundles+"room:\n  version: 3", 1)
	}

	return source
}

// roomDefects is every defect a single-room document is refused with, as
// "path: message".
func roomDefects(t *testing.T, source string) []string {
	t.Helper()
	_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: []byte(source)})
	if err == nil {
		return nil
	}
	var verr *dungeonspec.ValidationError
	require.True(t, errors.As(err, &verr), "a refusal is a validation error: %v", err)
	out := make([]string, 0, len(verr.Errors))
	for _, e := range verr.Errors {
		out = append(out, e.Path+": "+e.Message)
	}

	return out
}

// gateTable is gate-patrol's compiled `time` table.
func gateTable(t *testing.T, source string) []encounter.Answer {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)
	for _, m := range compiled.Monsters {
		if m.ID == "gate-patrol" {
			return m.Table[encounter.AnswerTime]
		}
	}
	require.Fail(t, "the fixture still places gate-patrol")

	return nil
}

const pursuerBundle = `  pursuer:
    params: { distance: 6, heading: went }
    rows:
      - { when: { enemy: remembered }, toward: enemy }
      - { when: { enemy: lost }, search: { distance: $distance, heading: $heading } }
`

const patrolBundle = `  patrol:
    params: { path: ~ }
    rows:
      - { when: { enemy: none }, toward: { path: $path } }
`

const loop = `{ path: [{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}] }`

// TestAUseRowCompilesToAGroup: `use:` with `with`, `weight` and `as` compiles
// to one group row whose rows are the bundle's, the parameter substituted
// where it was written and the default where it was not, and whose record
// carries the bundle's name and every parameter's value.
func TestAUseRowCompilesToAGroup(t *testing.T) {
	source := bundledPatrol(t, pursuerBundle,
		"use: pursuer\nwith: { distance: 8 }\nweight: 2\nas: toward",
		"{ when: { enemy: none }, toward: "+loop+" }")
	require.Empty(t, roomDefects(t, source))

	rows := gateTable(t, source)
	require.Len(t, rows, 2)
	group := rows[0]
	require.Equal(t, 2, group.Weight)
	require.Equal(t, "toward", group.As)
	require.Equal(t, "pursuer", group.From)
	require.Equal(t, map[string]string{"distance": "8", "heading": "went"}, group.Params,
		"the written parameter and the default, as the record shows them")
	require.Len(t, group.Rows, 2)
	require.Equal(t, encounter.EnemyRemembered, group.Rows[0].When[0].Enemy)
	require.Equal(t, &encounter.Search{Distance: 8, Heading: encounter.HeadingWent}, group.Rows[1].Search)
	require.Equal(t, 1, group.Rows[1].Weight, "an inner row's omitted weight is 1, as any row's is")
	require.NoError(t, group.Validate())
}

// TestARouteIsAParameter: a parameter replaces its whole scalar, so a
// bundle's `$path` takes a route, and the route compiles to exactly the cells
// the literal row did.
func TestARouteIsAParameter(t *testing.T) {
	literal := gateTable(t, bundledPatrol(t, "", "{ when: { enemy: none }, toward: "+loop+" }"))
	used := gateTable(t, bundledPatrol(t, patrolBundle,
		"use: patrol\nwith: { path: [{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}] }"))
	require.Len(t, used, 1)
	require.Len(t, used[0].Rows, 1)
	require.Equal(t, literal[0], used[0].Rows[0], "the route row, one group down")
	require.Equal(t, "[{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}]", used[0].Params["path"])
}

// TestABundleMayUseABundle: R14, a group inside a group, resolved by name
// at every level.
func TestABundleMayUseABundle(t *testing.T) {
	source := bundledPatrol(t, pursuerBundle+`  guard-base:
    rows:
      - use: pursuer
      - { when: { enemy: seen }, toward: enemy }
`, "use: guard-base", "{ when: { enemy: none }, toward: "+loop+" }")
	require.Empty(t, roomDefects(t, source))
	rows := gateTable(t, source)
	require.Equal(t, "guard-base", rows[0].From)
	require.Equal(t, "pursuer", rows[0].Rows[0].From)
	require.Len(t, rows[0].Rows[0].Rows, 2)
}

// TestTheFormRefusesWhatABundleCannotBe is the design's list of refusals,
// each a sentence for the form at the path of the line that made it.
func TestTheFormRefusesWhatABundleCannotBe(t *testing.T) {
	const at = "room.room.monsterBindings.gate-patrol.on.time[0]"
	home := "{ when: { enemy: none }, toward: " + loop + " }"
	chain := ""
	for i := 1; i <= behavior.MaxGroupDepth+1; i++ {
		next := "{ hold: {} }"
		if i <= behavior.MaxGroupDepth {
			next = "{ use: b" + string(rune('0'+i+1)) + " }"
		}
		if i == behavior.MaxGroupDepth+1 {
			next = "{ hold: {} }"
		}
		chain += "  b" + string(rune('0'+i)) + ":\n    rows: [ " + next + " ]\n"
	}
	for _, tc := range []struct {
		name    string
		bundles string
		rows    []string
		want    string
	}{
		{"a bundle nobody declared", pursuerBundle, []string{"use: hunter", home},
			at + ".use: `use: hunter` names no bundle in this file: it declares pursuer"},
		{"a parameter the bundle does not take", pursuerBundle,
			[]string{"use: pursuer\nwith: { range: 3 }", home},
			at + ".with.range: bundle \"pursuer\" takes no parameter `range`: it takes distance, heading"},
		{"a parameter with no default, not given", patrolBundle, []string{"use: patrol"},
			at + ".with: bundle \"patrol\" needs `path`: it has no default, so the use gives one"},
		{"a word on the use row", pursuerBundle, []string{"use: pursuer\nhold: {}", home},
			at + ": a group carries `hold`; a group's answer is its rows, so the word goes on a row inside it"},
		{"a search on the use row", pursuerBundle,
			[]string{"use: pursuer\nsearch: { distance: 2 }", home},
			at + ": a group carries `search`; a group's answer is its rows, so the word goes on a row inside it"},
		{"a line on the use row", pursuerBundle, []string{"use: pursuer\nsay: Halt!", home},
			at + ": a group carries `say`; a group's answer is its rows, so the word goes on a row inside it"},
		{"a bundle with no rows", "  empty:\n    rows: []\n", []string{home},
			"bundles.empty.rows: a bundle holds no rows; a group is a table, and a table with no rows can never be picked"},
		{"a bundle that uses itself", "  loop:\n    rows: [ { use: loop } ]\n", []string{"use: loop", home},
			at + ".rows[0].use: bundle \"loop\" uses itself: loop → loop; a group cannot hold the table it is in"},
		{"nested past the guard", chain, []string{"use: b1", home},
			"a group nested 9 deep; groups nest at most 8 deep"},
		{"`as` a word no temperament multiplies", pursuerBundle, []string{"use: pursuer\nas: search", home},
			at + ": a group counts `as` \"search\"; it counts as one of attack, toward, away, flee or hold, " +
				"or as nothing"},
		{"a $name the bundle does not declare",
			"  sloppy:\n    rows: [ { when: { enemy: lost }, search: { distance: $range } } ]\n",
			[]string{"use: sloppy", home},
			"bundles.sloppy.rows: `$range` is not a parameter bundle \"sloppy\" declares"},
		{"`with` on a row that uses nothing", "", []string{"{ when: { enemy: none }, hold: {}, with: { x: 1 } }"},
			at + ".with: `with` gives a bundle its parameters, and this row uses no bundle"},
		{"`as` on a row that uses nothing", "", []string{"{ when: { enemy: none }, hold: {}, as: toward }"},
			at + ".as: `as` is the word a group counts as, and this row uses no bundle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defects := roomDefects(t, bundledPatrol(t, tc.bundles, tc.rows...))
			found := false
			for _, d := range defects {
				if strings.Contains(d, tc.want) {
					found = true
				}
			}
			require.True(t, found, "want %q among %q", tc.want, defects)
		})
	}
}

// TestAFactPatternMustMatchAFactTheFileSets is rpg-project#684 L3 for
// `fact`: a pattern that matches a fact some row teaches is read; one that
// matches none is refused, never accepted as "never fires". And L2: a row
// teaches a fact by name, never by pattern.
func TestAFactPatternMustMatchAFactTheFileSets(t *testing.T) {
	const at = "room.room.monsterBindings.gate-patrol.on.time"
	alarm := "{ when: { enemy: seen }, fact: guards-alarmed }"
	ok := bundledPatrol(t, "", alarm, "{ when: { fact: /guards-.*/ }, hold: {} }")
	require.Empty(t, roomDefects(t, ok))
	rows := gateTable(t, ok)
	require.Equal(t, behavior.Name{Pattern: "guards-.*"}, rows[1].When[0].Fact)

	for _, tc := range []struct{ name, row, want string }{
		{"matching nothing the file sets", "{ when: { fact: /king-.*/ }, hold: {} }",
			at + "[1].when: `fact` /king-.*/ matches no fact this file sets"},
		{"inside a not", "{ when: { not: { fact: /king-.*/ } }, hold: {} }",
			at + "[1].when: `fact` /king-.*/ matches no fact this file sets"},
		{"a pattern that does not compile", "{ when: { fact: /guards-(/ }, hold: {} }",
			at + "[1]: `fact` /guards-(/ is not a pattern: error parsing regexp: missing closing ): `guards-(`"},
		{"a pattern taught", "{ when: { enemy: lost }, fact: /guards-.*/ }",
			at + "[1].fact: a row teaches one fact by its name, and /guards-.*/ is a pattern"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defects := roomDefects(t, bundledPatrol(t, "", alarm, tc.row))
			found := false
			for _, d := range defects {
				if strings.Contains(d, tc.want) {
					found = true
				}
			}
			require.True(t, found, "want %q among %q", tc.want, defects)
		})
	}
}

// TestAHeardPatternMustMatchASoundTheFileMakes is #684 L3 for `heard`,
// on the two committed fixtures: `/clang|bell/` matches the clang a swing
// makes and is read; `/bell|gong/` matches nothing a file can make and is
// refused in the lane's own sentence.
func TestAHeardPatternMustMatchASoundTheFileMakes(t *testing.T) {
	raw, err := os.ReadFile("testdata/patterns/heard-pattern.yaml")
	require.NoError(t, err)
	require.Empty(t, roomDefects(t, string(raw)))
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err)
	var listener []encounter.Answer
	for _, m := range compiled.Monsters {
		if m.ID == "gate-patrol" {
			listener = m.Table[encounter.AnswerTime]
		}
	}
	require.NotEmpty(t, listener)
	require.Equal(t, behavior.Name{Pattern: "clang|bell"}, listener[0].When[0].Heard.Kind)
	require.True(t, listener[0].When[0].Heard.Kind.Matches(encounter.SoundClang))

	raw, err = os.ReadFile("testdata/patterns/heard-pattern-refused.yaml")
	require.NoError(t, err)
	_, err = dungeonspec.Load(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(),
		"`heard` kind /bell|gong/ matches no sound this file makes: they are boom, crash, clang, voice")
}

// TestAUseRowMarshalsBackAsWritten: a decoded `use:` row encodes as the
// author wrote it, `use:` and `with:`, and never as the rows, the bundle
// name or the parameters the compiler resolved into it.
func TestAUseRowMarshalsBackAsWritten(t *testing.T) {
	source := bundledPatrol(t, pursuerBundle, "use: pursuer\nwith: { distance: 8 }\nweight: 2")
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: []byte(source)})
	require.NoError(t, err)
	row := decoded.Spec.Room.Gameplay.MonsterBindings["gate-patrol"].On["time"][0]
	require.NotEmpty(t, row.Rows, "the decode resolved the row")
	again, err := yaml.Marshal(row)
	require.NoError(t, err)
	require.Contains(t, string(again), "use: pursuer")
	require.Contains(t, string(again), "distance: 8")
	for _, resolved := range []string{"rows", "from", "params", "enemy"} {
		require.NotContains(t, strings.ToLower(string(again)), resolved, "nothing resolved is written back")
	}
}

// someRoute pins the route the bundled patrol walks, so a route parameter
// that compiled to other cells could not pass [TestARouteIsAParameter] by
// both sides being empty.
func TestTheBundledRouteIsTheRoomsLoop(t *testing.T) {
	rows := gateTable(t, bundledPatrol(t, patrolBundle,
		"use: patrol\nwith: { path: [{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}] }"))
	require.Len(t, rows[0].Rows[0].Toward.Path, 4)
	require.NotEqual(t, spatial.Position{}, rows[0].Rows[0].Toward.Path[1])
}

// TestAParameterCrossesIntoABundleUsedInside is N2, a base per kind: a
// bundle of bundles whose own parameter is passed on to a bundle it uses,
// `with: { path: $path }`, one level down, and resolves to the route the use
// gave it.
func TestAParameterCrossesIntoABundleUsedInside(t *testing.T) {
	source := bundledPatrol(t, pursuerBundle+patrolBundle+`  guard-base:
    params: { path: ~ }
    rows:
      - use: pursuer
      - use: patrol
        with: { path: $path }
`, "use: guard-base\nwith: { path: [{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}] }")
	require.Empty(t, roomDefects(t, source))
	literal := gateTable(t, bundledPatrol(t, "", "{ when: { enemy: none }, toward: "+loop+" }"))
	rows := gateTable(t, source)
	require.Len(t, rows, 1)
	base := rows[0]
	require.Equal(t, "guard-base", base.From)
	patrol := base.Rows[1]
	require.Equal(t, "patrol", patrol.From)
	require.Equal(t, base.Params["path"], patrol.Params["path"], "the route crossed one level down as written")
	require.Equal(t, literal[0], patrol.Rows[0], "and compiles to the cells the literal row did")
	require.NoError(t, base.Validate())

	// The base's parameter is the base's: a use that gives none is refused at
	// the use, by the base's name.
	defects := roomDefects(t, bundledPatrol(t, pursuerBundle+patrolBundle+`  guard-base:
    params: { path: ~ }
    rows:
      - use: patrol
        with: { path: $path }
`, "use: guard-base"))
	require.Contains(t, defects, "room.room.monsterBindings.gate-patrol.on.time[0].with: "+
		"bundle \"guard-base\" needs `path`: it has no default, so the use gives one")
}

// TestABundleCycleThroughAnotherIsRefused: the depth guard's sibling, a
// cycle two bundles long, is refused at the use that closes it, naming the
// chain — never resolved until the guard trips.
func TestABundleCycleThroughAnotherIsRefused(t *testing.T) {
	defects := roomDefects(t, bundledPatrol(t, `  ping:
    rows: [ { use: pong } ]
  pong:
    rows: [ { use: ping } ]
`, "use: ping", "{ when: { enemy: none }, toward: "+loop+" }"))
	require.Contains(t, defects, "room.room.monsterBindings.gate-patrol.on.time[0].rows[0].rows[0].use: "+
		"bundle \"ping\" uses itself: ping → pong → ping; a group cannot hold the table it is in")
}
