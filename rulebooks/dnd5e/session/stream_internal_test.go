// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// stream_internal_test.go pins numberEntries' arithmetic at the edges the
// scene suite cannot reach cheaply: the fail-closed refusals, and the exact
// continuation across a trim. The design is stream.go's own doc; the scenes
// in conceal_test.go prove the same arithmetic end to end.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// entriesAt builds a member's surviving entries at the given global seqs.
func entriesAt(seqs ...uint64) []record.Entry {
	out := make([]record.Entry, 0, len(seqs))
	for _, seq := range seqs {
		out = append(out, record.Entry{Seq: seq})
	}
	return out
}

func TestNumberEntriesSeedsExactlyOnAnUntrimmedLog(t *testing.T) {
	seqs, count, err := numberEntries("m", entriesAt(1, 2, 5), StreamCursor{}, 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), count)
	assert.Equal(t, map[uint64]uint64{1: 1, 2: 2, 5: 3},
		seqs, "the member's numbers count THEIR deliveries — global 5 is their 3")
}

func TestNumberEntriesContinuesAcrossATrim(t *testing.T) {
	// The cursor remembers 6 deliveries through global 20; the window now
	// holds only two of those (18, 20) plus two newer ones. The survivors
	// keep the numbers they were delivered under — 5 and 6, counted back
	// from the cursor — and the new entries continue 7, 8. Nothing
	// restarts, nothing skips.
	cursor := StreamCursor{UpTo: 20, Count: 6}
	seqs, count, err := numberEntries("m", entriesAt(18, 20, 21, 24), cursor, 15)
	require.NoError(t, err)
	assert.Equal(t, uint64(8), count)
	assert.Equal(t, map[uint64]uint64{18: 5, 20: 6, 21: 7, 24: 8}, seqs)
}

func TestNumberEntriesRefusesACursorTheTrimOutran(t *testing.T) {
	// Globals 11..14 were trimmed after the cursor last counted at 10: how
	// many of them were the member's can no longer be proven, so numbers
	// issued under this cursor cannot be extended — refused loudly, never
	// guessed (stream.go's fail-closed arm).
	_, _, err := numberEntries("m", entriesAt(15), StreamCursor{UpTo: 10, Count: 4}, 15)
	require.ErrorIs(t, err, ErrInvalidWorld)
	assert.Contains(t, err.Error(), `stream for "m"`)
}

func TestNumberEntriesRefusesACursorThatCountsTooFew(t *testing.T) {
	// Three survivors at or below the watermark, a cursor claiming two ever
	// existed: the persisted pair contradict each other, and arithmetic on
	// a contradiction would hand out wrong numbers with a straight face.
	_, _, err := numberEntries("m", entriesAt(1, 2, 3), StreamCursor{UpTo: 3, Count: 2}, 1)
	require.ErrorIs(t, err, ErrInvalidWorld)
}

func TestNumberEntriesZeroCursorOnATrimmedLogSeedsFromTheWindow(t *testing.T) {
	// No number was ever issued (nil cursor: a pre-cursor blob, or a member
	// whose beats cannot predate this verb), so seeding from what survives
	// is the one legal restart — it restarts nothing anyone ever saw.
	seqs, count, err := numberEntries("m", entriesAt(40, 41), StreamCursor{}, 40)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), count)
	assert.Equal(t, map[uint64]uint64{40: 1, 41: 2}, seqs)
}
