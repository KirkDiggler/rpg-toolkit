// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_symbols_test.go is THE RECORD CARRIES ITS OWN SYMBOLS on the
// castle (rpg-project#674): a fresh castle's record holds one `table` beat
// per gate guard per key, before that guard's first `answered` on the key —
// and, with the harness reloading the castle from its blob at every EndTurn
// and world round (the default), still exactly one.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// symbolBeat is the fields of a `table` or `answered` beat this route reads.
type symbolBeat struct {
	Beat     string `json:"beat"`
	Creature string `json:"creature"`
	Option   string `json:"option"`
	Key      string `json:"key"`
}

// TestMadKingGateGuardsCarryTheirTablesInTheRecord: the hero turns gate-1,
// the fight forms, and the gate guards take three rounds of turns. Every
// (guard, key) a guard answered on is described exactly once, before its
// first answer.
func TestMadKingGateGuardsCarryTheirTablesInTheRecord(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, gatehouseSeat, nil)
	s.struck(kingGate1, false)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "precondition: the fight is on")
	for range 3 {
		s.endTurn()
	}

	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(t, err)

	type scope struct{ creature, option, key string }
	tables := map[scope]int{}
	described := map[scope]bool{}
	answered := map[string]bool{}
	for _, entry := range story {
		var b symbolBeat
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		at := scope{b.Creature, b.Option, b.Key}
		switch b.Beat {
		case encounter.BeatTable:
			tables[at]++
			described[at] = true
		case encounter.BeatAnswered:
			if b.Creature == "" {
				continue // a prop's answer is read by index
			}
			answered[b.Creature] = true
			require.True(t, described[at], "%s's answer on %q has its table before it", b.Creature, b.Key)
		}
	}

	for _, guard := range []encounter.MemberID{kingGate1, kingGate2} {
		require.True(t, answered[string(guard)], "precondition: %s took a turn", guard)
	}
	for at, n := range tables {
		require.Equal(t, 1, n, "%s on %q is described exactly once", at.creature, at.key)
	}
}
