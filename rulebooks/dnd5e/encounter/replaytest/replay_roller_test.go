// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/replaytest"
)

// TestRollsOf_FailsClosed is the roller's own refusals: a body that is not
// JSON names its beat rather than dropping out of the rolls, and a roll
// that is no face of a die is refused.
func TestRollsOf_FailsClosed(t *testing.T) {
	t.Parallel()
	_, err := replaytest.RollsOf([]record.EntryData{{Seq: 7, Payload: []byte("not json")}})
	require.ErrorIs(t, err, replaytest.ErrBadBeat)
	require.ErrorContains(t, err, "beat seq 7")

	_, err = replaytest.RollsOf([]record.EntryData{{Seq: 3, Payload: []byte(`{"beat":"tempered","member":"m","roll":0,"of":3}`)}})
	require.ErrorIs(t, err, replaytest.ErrBadBeat)
}

// TestRecordRoller_ReadsBackInOrder is the roller over a hand-written
// record: a deal, then a pick that descends one group, read back in that
// order, then exhaustion; a die of the wrong size is refused.
func TestRecordRoller_ReadsBackInOrder(t *testing.T) {
	t.Parallel()
	log := []record.EntryData{
		{Seq: 1, Payload: []byte(`{"beat":"tempered","member":"m","roll":2,"of":3}`)},
		{Seq: 2, Payload: []byte(`{"beat":"moved","member":"m"}`)},
		{Seq: 3, Payload: []byte(`{"beat":"answered","creature":"m","roll":40,"of":100,"path":[1,0],` +
			`"rows":[{"entry":0},{"entry":1,"roll":5,"of":6,"rows":[{"entry":0}]}]}`)},
		{Seq: 4, Payload: []byte(`{"beat":"answered","creature":"m","roll":0,"of":0,"path":[]}`)},
	}
	r, err := replaytest.NewRecordRoller(log)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = r.Roll(ctx, 6)
	require.ErrorIs(t, err, replaytest.ErrReplayMismatch, "the first roll was a d3")
	face, err := r.Roll(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, 2, face)
	faces, err := r.RollN(ctx, 1, 100)
	require.NoError(t, err)
	require.Equal(t, []int{40}, faces)
	r.During("the test's verb")
	face, err = r.Roll(ctx, 6)
	require.NoError(t, err)
	require.Equal(t, 5, face, "the group's die comes after the top level's")
	require.Zero(t, r.Remaining(), "a hold rolled nothing")
	_, err = r.Roll(ctx, 20)
	require.ErrorIs(t, err, replaytest.ErrReplayExhausted)
	require.ErrorContains(t, err, "the test's verb")
}
