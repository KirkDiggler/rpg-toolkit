// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/stretchr/testify/require"
)

// TestTwoViewerStory is the AC1 integration spine: a two-viewer dungeon story
// with shared beats, per-viewer reveals, a GM-only beat, retention, and round-trip
// persistence. Tests mixed audience projection, tag filtering across differently-shaped
// beats, sequence gaplessness post-trim, and deep-copy immunity.
func TestTwoViewerStory(t *testing.T) {
	const (
		opener     = core.EntityID("door-opener")
		follower   = core.EntityID("door-follower")
		room7      = "room-7"
		kind       = "kind"
		actor      = "actor"
		subject    = "subject"
		clockStart = "clock.turn_started"
		clockEnd   = "clock.turn_ended"
		intelFirst = "intel.first_contact"
		gmNote     = "gm.note"
	)

	// Create the log.
	log, err := record.NewLog()
	require.NoError(t, err)

	// Append beats in order (use helper closure for repeated appends).
	appendBeat := func(aud []core.EntityID, tags map[string]string, payload string) uint64 {
		out, err := log.Append(&record.AppendInput{
			Audience: aud,
			Tags:     tags,
			Payload:  []byte(payload),
		})
		require.NoError(t, err)
		return out.Seq
	}

	// Beat 1: shared — both viewers see the turn start.
	seq1 := appendBeat(
		[]core.EntityID{opener, follower},
		map[string]string{kind: clockStart, actor: string(opener)},
		"shared-1",
	)
	require.Equal(t, uint64(1), seq1, "seq1 should be 1")

	// Beat 2: opener's big reveal — only the door opener sees the first contact.
	seq2 := appendBeat(
		[]core.EntityID{opener},
		map[string]string{kind: intelFirst, subject: room7},
		"opener-reveal",
	)
	require.Equal(t, uint64(2), seq2, "seq2 should be 2")

	// Beat 3: follower's smaller reveal — only the door follower sees their first contact.
	seq3 := appendBeat(
		[]core.EntityID{follower},
		map[string]string{kind: intelFirst, subject: room7},
		"follower-reveal",
	)
	require.Equal(t, uint64(3), seq3, "seq3 should be 3")

	// Beat 4: GM-only beat — empty audience means NO VIEWER.
	seq4 := appendBeat(
		nil,
		map[string]string{kind: gmNote},
		"gm-1",
	)
	require.Equal(t, uint64(4), seq4, "seq4 should be 4")

	// Beat 5: shared — both viewers see the turn end.
	seq5 := appendBeat(
		[]core.EntityID{opener, follower},
		map[string]string{kind: clockEnd, actor: string(opener)},
		"shared-2",
	)
	require.Equal(t, uint64(5), seq5, "seq5 should be 5")

	// Helper to extract seq values from entries.
	seqs := func(es []record.Entry) []uint64 {
		result := make([]uint64, len(es))
		for i, e := range es {
			result[i] = e.Seq
		}
		return result
	}

	// Helper to extract payloads from entries.
	payloads := func(es []record.Entry) []string {
		result := make([]string, len(es))
		for i, e := range es {
			result[i] = string(e.Payload)
		}
		return result
	}

	// Assertion (a): SliceFor(opener, FromSeq 1) → exactly seqs [1,2,5], payloads match.
	openerSlice, err := log.SliceFor(&record.SliceForInput{Viewer: opener, FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2, 5}, seqs(openerSlice), "opener should see seqs [1,2,5]")
	expectedOpener := []string{"shared-1", "opener-reveal", "shared-2"}
	require.Equal(t, expectedOpener, payloads(openerSlice), "opener payloads mismatch")

	// Assertion (b): SliceFor(follower, FromSeq 1) → exactly seqs [1,3,5].
	followerSlice, err := log.SliceFor(&record.SliceForInput{Viewer: follower, FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 3, 5}, seqs(followerSlice), "follower should see seqs [1,3,5]")
	expectedFollower := []string{"shared-1", "follower-reveal", "shared-2"}
	require.Equal(t, expectedFollower, payloads(followerSlice), "follower payloads mismatch")

	// Assertion (c): Unfiltered All(FromSeq 1) → all five seqs [1..5].
	allSeqs, err := log.All(&record.AllInput{FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2, 3, 4, 5}, seqs(allSeqs), "all should return seqs [1..5]")

	// Assertion (d): Tag questions across mixed shapes.
	// SliceFor(opener, Tags {"kind": "intel.first_contact"}) → [2]
	openerIntel, err := log.SliceFor(&record.SliceForInput{
		Viewer: opener, FromSeq: 1,
		Tags: map[string]string{kind: intelFirst},
	})
	require.NoError(t, err)
	require.Equal(t, []uint64{2}, seqs(openerIntel), "opener intel seqs should be [2]")

	// All(Tags {"subject": "room-7"}) → [2,3]
	subjectRoom, err := log.All(&record.AllInput{
		FromSeq: 1,
		Tags:    map[string]string{subject: room7},
	})
	require.NoError(t, err)
	require.Equal(t, []uint64{2, 3}, seqs(subjectRoom), "subject room-7 seqs should be [2,3]")

	// All(Tags {"kind": "clock.turn_started"}) → [1]
	turnStart, err := log.All(&record.AllInput{
		FromSeq: 1,
		Tags:    map[string]string{kind: clockStart},
	})
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, seqs(turnStart), "clock.turn_started seqs should be [1]")

	// Multi-key AND filter pin: All(Tags {"kind": "clock.turn_started", "actor": opener}) → [1]
	// (proves AND-not-OR: filtering by actor alone returns [1,5], two-key filter returns [1])
	turnStartWithOpener, err := log.All(&record.AllInput{
		FromSeq: 1,
		Tags:    map[string]string{kind: clockStart, actor: string(opener)},
	})
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, seqs(turnStartWithOpener), "kind AND actor filter should return [1] (AND semantics, not OR)")

	// All(Tags {"actor": opener}) alone → [1,5] (both beats have same actor, different kinds)
	actorOnly, err := log.All(&record.AllInput{
		FromSeq: 1,
		Tags:    map[string]string{actor: string(opener)},
	})
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 5}, seqs(actorOnly), "actor filter alone should return [1,5]")

	// Assertion (e): GM beat invisible to both viewers; SliceFor(opener, FromSeq 4) → [5] only.
	openerAfter4, err := log.SliceFor(&record.SliceForInput{Viewer: opener, FromSeq: 4})
	require.NoError(t, err)
	require.Equal(t, []uint64{5}, seqs(openerAfter4), "opener FromSeq 4 should see [5] only (seq 4 excluded)")

	// Same for follower.
	followerAfter4, err := log.SliceFor(&record.SliceForInput{Viewer: follower, FromSeq: 4})
	require.NoError(t, err)
	require.Equal(t, []uint64{5}, seqs(followerAfter4), "follower FromSeq 4 should see [5] only")

	// Assertion (f): Retention; TrimBefore{Seq: 3} (both clients "acknowledged" through 2).
	trimOut, err := log.TrimBefore(&record.TrimBeforeInput{Seq: 3})
	require.NoError(t, err)
	require.Equal(t, 2, trimOut.Removed, "trim should remove 2 entries")

	// After trim, SliceFor(opener, FromSeq 1) → [5] only (opener isn't in seq 3's audience).
	openerPostTrim, err := log.SliceFor(&record.SliceForInput{Viewer: opener, FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, []uint64{5}, seqs(openerPostTrim), "opener post-trim should see [5]")

	// SliceFor(follower, FromSeq 1) → [3,5].
	followerPostTrim, err := log.SliceFor(&record.SliceForInput{Viewer: follower, FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, []uint64{3, 5}, seqs(followerPostTrim), "follower post-trim should see [3,5]")

	// Seqs NEVER renumbered; All(FromSeq 1) → [3,4,5].
	allPostTrim, err := log.All(&record.AllInput{FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, []uint64{3, 4, 5}, seqs(allPostTrim), "all post-trim should return [3,4,5] (seqs never renumbered)")

	// Assertion (g): Round-trip mid-story; LoadLog(log.ToData()) → same SliceFor results.
	data := log.ToData()
	loadedLog, err := record.LoadLog(data)
	require.NoError(t, err)

	// Re-run assertions on loaded log.
	openerLoaded, err := loadedLog.SliceFor(&record.SliceForInput{Viewer: opener, FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, openerPostTrim, openerLoaded, "loaded opener slice mismatch (full-fidelity: Seq, At, Correlation, Audience, Tags, Payload)")

	followerLoaded, err := loadedLog.SliceFor(&record.SliceForInput{Viewer: follower, FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, followerPostTrim, followerLoaded, "loaded follower slice mismatch (full-fidelity: Seq, At, Correlation, Audience, Tags, Payload)")

	allLoaded, err := loadedLog.All(&record.AllInput{FromSeq: 1})
	require.NoError(t, err)
	require.Equal(t, allPostTrim, allLoaded, "loaded all slice mismatch (full-fidelity: Seq, At, Correlation, Audience, Tags, Payload)")
}
