// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_bundles_test.go is THE CASTLE'S BUNDLES IN THE RECORD
// (rpg-project#670): gate-1's table is a tree now — one `use: guard-base`
// line, a bundle of bundles (pursuer, alarm-reactor, and patrol with the
// guard's own route) — and the record says so. Its one `table` beat carries
// the groups with their bundle, parameters and rows, and every `answered`
// names the path it took through them: `[4, 0, 0]` to the corner,
// `[4, 0, 1]` searching on, `[4, 2, 0]` home on the loop.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// treeRow is one row of a `table` beat, as far as this route reads it.
type treeRow struct {
	Entry  int               `json:"entry"`
	Word   string            `json:"word"`
	From   string            `json:"from"`
	Params map[string]string `json:"params"`
	As     *string           `json:"as"`
	Rows   []treeRow         `json:"rows"`
}

// pathBeat is the fields of a `table` or `answered` beat this route reads.
type pathBeat struct {
	Beat     string          `json:"beat"`
	Creature string          `json:"creature"`
	Key      string          `json:"key"`
	Entry    int             `json:"entry"`
	Path     []int           `json:"path"`
	Word     string          `json:"word"`
	Rows     json.RawMessage `json:"rows"`
}

// verdict is one row verdict of an `answered` beat.
type verdict struct {
	Entry int       `json:"entry"`
	Roll  int       `json:"roll"`
	Of    int       `json:"of"`
	Rows  []verdict `json:"rows"`
}

// TestMadKingGateGuardsTreeIsInTheRecord: the pursuit route's three phases,
// read back from the record as the Debug panel reads them.
func TestMadKingGateGuardsTreeIsInTheRecord(t *testing.T) {
	t.Parallel()
	s := openThePursuit(t, pursuitStart)
	require.True(t, s.seesLive(kingGate1, hero), "precondition: gate-1 sees the hero at the start")
	s.struck(kingGate1, false)
	route, err := s.enc.Route(encounter.RouteInput{
		Mover: hero, Policy: encounter.MoveToward, Anchor: castleCell(-6, 6), Budget: 40,
	})
	require.NoError(t, err)
	run := encounter.CellsFromFeet(60)
	s.runLeg(route.Path[:run])
	s.endTurn() // to the corner: the pursuer's first row
	s.runLeg(route.Path[run:])
	s.endTurn() // six on: the pursuer's search
	s.endTurn() // home: the patrol

	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(t, err)
	var tables []pathBeat
	var answers []pathBeat
	for _, entry := range story {
		var b pathBeat
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		if b.Creature != string(kingGate1) || b.Key != string(encounter.AnswerTime) {
			continue
		}
		switch b.Beat {
		case encounter.BeatTable:
			tables = append(tables, b)
		case encounter.BeatAnswered:
			answers = append(answers, b)
		}
	}

	require.Len(t, tables, 1, "gate-1's time table is described once")
	var rows []treeRow
	require.NoError(t, json.Unmarshal(tables[0].Rows, &rows))
	// THE DASHER IS THE APPROACH (rpg-project#671): gate-patrol keeps its
	// march and its alarm and carries no plain `reach` or `seen` row, so the
	// base's dasher is never split by a walk beside it.
	require.Len(t, rows, 3, "two word rows and one `use: guard-base` line")
	base := rows[2]
	require.Equal(t, "guard-base", base.From)
	require.Equal(t, map[string]string{
		"path": "[{q: -5, r: 8, wait: 1, facing: ne}, {q: -5, r: 10, wait: 1, facing: se}, {q: -7, r: 10}]"},
		base.Params, "the guard's own route, passed to the base")
	require.Empty(t, base.Word, "a group carries no word")
	require.Len(t, base.Rows, 4, "the base is a bundle of bundles")
	for i, want := range []struct {
		from   string
		inner  int
		params map[string]string
	}{
		{"dasher", 4, map[string]string{}},
		{"pursuer", 2, map[string]string{"distance": "6", "heading": "went"}},
		{"alarm-reactor", 1, map[string]string{}},
		{"patrol", 1, base.Params},
	} {
		group := base.Rows[i]
		require.Equal(t, want.from, group.From)
		require.Len(t, group.Rows, want.inner)
		require.Equal(t, want.params, group.Params, "%s's parameters; the route crossed one level down", want.from)
		require.NotNil(t, group.As, "a group's `as` is written, empty for none")
	}
	require.Equal(t, "toward", base.Rows[0].Rows[2].Word, "inside one run of reach the castle walks")
	require.Equal(t, "dash", base.Rows[0].Rows[3].Word, "and runs only beyond it")
	require.Equal(t, "search", base.Rows[1].Rows[1].Word)
	require.Nil(t, rows[0].Rows, "a word row carries no tree")

	paths := make([][]int, 0, len(answers))
	for _, a := range answers {
		require.NotEmpty(t, a.Path)
		require.Equal(t, a.Path[0], a.Entry, "entry is the top of the path")
		paths = append(paths, a.Path)
	}
	// TO THE CORNER IS THE DASHER'S WALK (rpg-project#671): the dasher is the
	// base's first bundle, and a remembered hero a walk brings into reach is
	// its `enemy: walk` row before the pursuer's `remembered` one. The same
	// cells, walked by the row that asks what a move buys.
	require.Contains(t, paths, []int{2, 0, 1}, "to the corner, through the base and the dasher's walk")
	require.Contains(t, paths, []int{2, 1, 1}, "the search, through the base and the pursuer")
	require.Contains(t, paths, []int{2, 3, 0}, "home, through the base and the patrol")

	for _, a := range answers {
		if len(a.Path) < 2 {
			continue
		}
		var verdicts []verdict
		require.NoError(t, json.Unmarshal(a.Rows, &verdicts))
		group := verdicts[a.Path[0]]
		require.Len(t, group.Rows, len(rows[a.Path[0]].Rows), "the group's verdict holds its rows' verdicts")
		require.Equal(t, 1, group.Roll, "the route host's die of 1, inside the group")
		require.Positive(t, group.Of)
	}
}
