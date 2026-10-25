// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_bundles_test.go is THE CASTLE'S BUNDLES IN THE RECORD
// (rpg-project#670): gate-1's table is a tree now (pursuer, alarm-reactor
// and patrol are `use:` lines), and the record says so. Its one `table` beat
// carries the groups with their bundle, parameters and rows, and every
// `answered` names the path it took through them: `[4, 0]` to the corner,
// `[4, 1]` searching on, `[6, 0]` home on the loop.

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
	require.Len(t, rows, 7, "four word rows and three `use:` lines")
	for i, want := range []struct {
		from   string
		inner  int
		params map[string]string
	}{
		{"pursuer", 2, map[string]string{"distance": "6", "heading": "went"}},
		{"alarm-reactor", 1, map[string]string{}},
		{"patrol", 1, map[string]string{
			"path": "[{q: -5, r: 8, wait: 1, facing: ne}, {q: -5, r: 10, wait: 1, facing: se}, {q: -7, r: 10}]"}},
	} {
		group := rows[4+i]
		require.Equal(t, want.from, group.From)
		require.Len(t, group.Rows, want.inner)
		require.Equal(t, want.params, group.Params)
		require.NotNil(t, group.As, "a group's `as` is written, empty for none")
		require.Empty(t, group.Word, "a group carries no word")
	}
	require.Equal(t, "search", rows[4].Rows[1].Word)
	require.Nil(t, rows[0].Rows, "a word row carries no tree")

	paths := make([][]int, 0, len(answers))
	for _, a := range answers {
		require.NotEmpty(t, a.Path)
		require.Equal(t, a.Path[0], a.Entry, "entry is the top of the path")
		paths = append(paths, a.Path)
	}
	require.Contains(t, paths, []int{4, 0}, "to the corner, through the pursuer")
	require.Contains(t, paths, []int{4, 1}, "the search, through the pursuer")
	require.Contains(t, paths, []int{6, 0}, "home, through the patrol")

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
