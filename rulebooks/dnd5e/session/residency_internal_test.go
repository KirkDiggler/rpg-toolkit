// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// resident is a copy of n estimated bytes, for the bound alone.
func resident(n int64) *Resident { return &Resident{bytes: n} }

// TestMemoryResidencyBoundsByCountAndBytes: Take removes; Keep replaces a
// session's copy; past either bound the least recently kept is evicted; a
// copy larger than the byte bound is not kept; a negative bound keeps none.
func TestMemoryResidencyBoundsByCountAndBytes(t *testing.T) {
	r := NewResidency(&ResidencyConfig{MaxEntries: 2, MaxBytes: 100})
	r.Keep("a", resident(10))
	r.Keep("b", resident(10))
	r.Keep("c", resident(10))
	require.Equal(t, 2, r.Len(), "the count bound holds")
	require.Nil(t, r.Take("a"), "the oldest kept went first")
	require.NotNil(t, r.Take("b"))
	require.Nil(t, r.Take("b"), "Take removes")
	require.Equal(t, 1, r.Len())

	r.Keep("c", resident(60))
	require.Equal(t, int64(60), r.Bytes(), "Keep replaces the session's copy")
	r.Keep("d", resident(30))
	r.Keep("c", resident(60)) // c is now the most recently kept
	r.Keep("e", resident(30))
	require.Nil(t, r.Take("d"), "past the byte bound the oldest kept went first")
	require.Equal(t, int64(90), r.Bytes())

	r.Keep("f", resident(101))
	require.Nil(t, r.Take("f"), "a copy over the byte bound is not kept")

	for _, cfg := range []*ResidencyConfig{{MaxEntries: -1}, {MaxBytes: -1}} {
		none := NewResidency(cfg)
		none.Keep("a", resident(1))
		require.Zero(t, none.Len(), "a negative bound keeps none")
	}
	def := NewResidency(nil)
	require.Equal(t, DefaultResidentEntries, def.maxEntries)
	require.Equal(t, int64(DefaultResidentMaxBytes), def.maxBytes)
}

// TestARecordThatMovedAfterTheSaveIsNotKept is the release's record guard on
// its own: a staged copy whose record's next seq is past the one its save
// left is dropped, and one at that seq is kept. The stage after a landed save
// is the first guard; this one holds even if a copy were staged wrongly (a
// stale save's copy has appended past what landed).
func TestARecordThatMovedAfterTheSaveIsNotKept(t *testing.T) {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 4)},
		},
		Endings:      []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.RefusingCapabilities(),
	})
	require.NoError(t, err)
	next, err := enc.NextStorySeq()
	require.NoError(t, err)
	require.Positive(t, next)

	res := NewResidency(nil)
	m := &Manager{versions: versionsNever{}, residency: res, staged: &stagedResidents{}}
	stage := func(recordNext uint64) {
		m.staged.put("s", &Resident{enc: enc, fence: checkpointFence{held: true, recordNext: recordNext}})
	}

	stage(next - 1) // saved before the record's last beat: it moved since
	m.keepStaged("s")
	require.Zero(t, res.Len(), "a copy whose record moved after its save is not kept")

	stage(next)
	m.keepStaged("s")
	require.Equal(t, 1, res.Len(), "a copy at its save's record is kept")
}

// versionsNever is a reader the guard test never reads: it only has to be
// there for the Manager to keep copies.
type versionsNever struct{}

func (versionsNever) GetCheckpointVersion(context.Context, string) (uint64, error) {
	return 0, errors.New("not read")
}
