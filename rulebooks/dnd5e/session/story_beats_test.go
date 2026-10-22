// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// story_beats_test.go reads a member's story as the encounter recorded it,
// beat by beat — what the deleted v1alpha1 projection (rpg-project#631) used
// to hand tests as typed events. A test that pins a gameplay fact reads it
// here, from the beat's own payload, or from the act stream
// ([actCapture], [session.Manager.Acts]) where an act body states it.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// witnessed is one beat of a member's story.
type witnessed struct {
	// Seq is the record's own sequence for the beat — internal, global, and
	// only ever compared within one test.
	Seq uint64
	// Beat is the payload's `beat` value: the encounter's word for it.
	Beat string
	// Correlation is the act id the beat carries, `<verb>:<id>`.
	Correlation string
	// Fields is the payload, decoded.
	Fields map[string]any
	// Payload is the payload, whole.
	Payload json.RawMessage
}

// decode unmarshals the beat's payload into v.
func (w witnessed) decode(t testing.TB, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(w.Payload, v))
}

// storyBeats is every retained beat member has witnessed, in record order.
func storyBeats(t testing.TB, mgr *session.Manager, sessionID, member string) []witnessed {
	t.Helper()
	return storyBeatsAfter(t, mgr, sessionID, member, 0)
}

// storyBeatsAfter is member's witnessed beats with a record sequence after
// afterSeq — what one verb told them, when afterSeq is the last seq they held
// before it ([lastSeq]).
func storyBeatsAfter(t testing.TB, mgr *session.Manager, sessionID, member string, afterSeq uint64) []witnessed {
	t.Helper()
	enc, err := mgr.LoadWorldForTest(context.Background(), sessionID)
	require.NoError(t, err)
	from := afterSeq
	if from > 0 {
		from++ // the encounter's bound is inclusive
	}
	entries, err := enc.Story(&encounter.StoryInput{Audience: encounter.MemberID(member), AfterSeq: from})
	require.NoError(t, err)
	out := make([]witnessed, 0, len(entries))
	for _, e := range entries {
		w := witnessed{Seq: e.Seq, Correlation: e.Correlation, Payload: e.Payload}
		require.NoError(t, json.Unmarshal(e.Payload, &w.Fields))
		w.Beat, _ = w.Fields["beat"].(string)
		out = append(out, w)
	}
	return out
}

// lastSeq is the record sequence of the last beat in beats, or 0.
func lastSeq(beats []witnessed) uint64 {
	if len(beats) == 0 {
		return 0
	}
	return beats[len(beats)-1].Seq
}

// beatsNamed is the beats whose `beat` is one of names, in order.
func beatsNamed(beats []witnessed, names ...string) []witnessed {
	var out []witnessed
	for _, b := range beats {
		for _, n := range names {
			if b.Beat == n {
				out = append(out, b)
				break
			}
		}
	}
	return out
}

// beatNames is each beat's `beat`, in order.
func beatNames(beats []witnessed) []string {
	out := make([]string, 0, len(beats))
	for _, b := range beats {
		out = append(out, b.Beat)
	}
	return out
}

// walkSteps is how many cells one walk segment entered. A later segment
// (Segment > 1) starts at the cell its earlier segment left the mover on, so
// its first cell is not a step.
func walkSteps(act session.Act, walk session.WalkAct) int {
	if act.Segment > 1 && len(walk.Path) > 0 {
		return len(walk.Path) - 1
	}
	return len(walk.Path)
}
