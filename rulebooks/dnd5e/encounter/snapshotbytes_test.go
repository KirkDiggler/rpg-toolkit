// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// snapshotbytes_test.go pins the storage boundary's byte-identical promise
// (snapshot, data.go) on a keep whose creatures carry behaviors: the same
// encounter, encoded again with nothing changed, writes the same bytes, member
// by member. Every comparison by bytes or digest at the storage boundary
// (rpg-project#897, leg b: a piece is written when its mark moves) stands on
// it. Before rpg-project#897 a BehaviorData recipe's `states` and `table`
// maps came out of encoding/json/v2 in a fresh key order on every encode.

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// encodes is how many times each snapshot is encoded. One encode against
// another can agree by chance when a member's maps are small; four cannot,
// for the dozens of members carrying recipes.
const encodes = 4

// TestSnapshot_ByteIdenticalWithBehaviors encodes an unchanged small keep
// garrison, after a pace that raised the world clock so its creatures'
// behaviors woke and walked, through encoding/json v1 as a host does: ToData,
// ToCheckpoint's rest and every member alone, four times each, and every
// encode must be the first one's bytes.
func TestSnapshot_ByteIdenticalWithBehaviors(t *testing.T) {
	enc := openDrivenSeeing(t, smallKeep(t, true), 8)
	walker := keepPlayers[0]
	start := whereIs(t, enc, walker)
	var beside spatial.Position
	for _, d := range hexSteps {
		cell := spatial.Position{X: start.X + d.X, Y: start.Y + d.Y}
		if _, err := enc.Step(&encounter.StepInput{Member: walker, To: cell, EndWalk: true}); err == nil {
			beside = cell
			break
		}
	}
	require.NotEqual(t, start, beside, "the walker has a cell to pace to")
	round := enc.WorldRound()
	for i := 0; i < 12 && enc.WorldRound() == round; i++ {
		to := start
		if whereIs(t, enc, walker) == start {
			to = beside
		}
		_, err := enc.Step(&encounter.StepInput{Member: walker, To: to, EndWalk: true})
		require.NoError(t, err)
	}
	require.Greater(t, enc.WorldRound(), round, "the pace raised the clock, so behaviors woke")

	first := enc.ToData()
	withBehaviors := 0
	for _, m := range first.Members {
		if m.Behaviors != nil {
			withBehaviors++
		}
	}
	require.Positive(t, withBehaviors, "some member carries behaviors, or the promise was never asked")

	data := encodeEach(t, func() any { return enc.ToData() })
	rest := encodeEach(t, func() any {
		out, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{})
		require.NoError(t, err)
		return out.Rest
	})
	// Member by member, each encode round from its own snapshot.
	members := make([][][]byte, 0, encodes)
	for i := 0; i < encodes; i++ {
		var round [][]byte
		for _, m := range enc.ToData().Members {
			raw, err := json.Marshal(m)
			require.NoError(t, err)
			round = append(round, raw)
		}
		members = append(members, round)
	}
	differ := 0
	for m := range members[0] {
		for i := 1; i < encodes; i++ {
			if !bytes.Equal(members[0][m], members[i][m]) {
				differ++
				break
			}
		}
	}
	t.Logf("%d members, %d with behaviors, %d differing across %d encodes", len(first.Members), withBehaviors, differ, encodes)
	require.Zero(t, differ, "members whose bytes differ with nothing changed")
	for i := 1; i < encodes; i++ {
		require.True(t, bytes.Equal(data[0], data[i]), "ToData encode %d is the first one's bytes", i)
		require.True(t, bytes.Equal(rest[0], rest[i]), "ToCheckpoint rest encode %d is the first one's bytes", i)
	}
}

// TestInteractionData_ByteIdentical is the same promise for an interaction's
// record, whose blob carries the machine's bag, a map: a bag of a dozen keys
// encodes to the same bytes every time.
func TestInteractionData_ByteIdentical(t *testing.T) {
	bag := machine.Bag{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		bag[k] = len(k)
	}
	d := encounter.InteractionData{ID: "lever", Recipe: json.RawMessage(`{}`), Blob: machine.Blob{State: "up", Bag: bag}}
	raws := encodeEach(t, func() any { return d })
	for i := 1; i < encodes; i++ {
		require.Equal(t, string(raws[0]), string(raws[i]), "encode %d is the first one's bytes", i)
	}
}

// encodeEach encodes what snap returns [encodes] times through encoding/json v1.
func encodeEach(t *testing.T, snap func() any) [][]byte {
	t.Helper()
	out := make([][]byte, 0, encodes)
	for i := 0; i < encodes; i++ {
		raw, err := json.Marshal(snap())
		require.NoError(t, err)
		out = append(out, raw)
	}
	return out
}
