// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// tracerecord_internal_test.go is what the studio's watch asked of the run's
// record (rpg-project#1001, items 11 and 13): a decision trace names its
// pick as the answer does, and the record says what it is a run of and
// where each member began, while no member's story changes.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// recordBeats is every entry of the run's record, decoded, with its
// audience.
func recordBeats(t *testing.T, enc *Encounter) ([]map[string]any, [][]MemberID) {
	t.Helper()
	all, err := enc.ReadRecord(&ReadRecordInput{})
	require.NoError(t, err)
	var beats []map[string]any
	var audiences [][]MemberID
	for _, entry := range all.Entries {
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		beats = append(beats, b)
		audiences = append(audiences, entry.Audience)
	}
	return beats, audiences
}

// TestTheTraceAndTheAnswerNameAPickAlike: the same pick from a role's table,
// written by the `answered` writer and by the decision trace's selection,
// carries one key: the role-qualified one, which the table beat is keyed by.
func TestTheTraceAndTheAnswerNameAPickAlike(t *testing.T) {
	enc, _ := wakeHall(t, twoRoles(t))
	m := enc.members["m-guard"]
	pick := &Pick{Key: AnswerTime, Answer: Answer{Weight: 1, Hold: true}}

	answered, err := enc.appendPickBeat("m-guard", pick, pickedFor{}, "patrol")
	require.NoError(t, err)
	require.NotEmpty(t, answered)
	_, err = enc.appendBehaviorDecision(&behaviorDecisionRecordInput{member: m, pick: pick, instance: "patrol",
		trace: &behaviorDecisionTrace{on: beatTime, operation: behaviorTraceDecide}})
	require.NoError(t, err)

	var answerKey, traceKey any
	beats, _ := recordBeats(t, enc)
	for _, b := range beats {
		switch b["beat"] {
		case BeatAnswered:
			answerKey = b["key"]
		case BeatBehaviorDecision:
			if sel, ok := b["selection"].(map[string]any); ok {
				traceKey = sel["key"]
			}
		}
	}
	require.Equal(t, "behavior/6:patrol/time", answerKey)
	require.Equal(t, answerKey, traceKey, "the trace names the pick as the answer does")
	require.Equal(t, AnswerTime, pick.Key, "the caller's pick is left as it was")
}

// TestTheRecordSaysWhatItIsARunOf: a scene_content beat carries the site's
// hash, the content's hash and the content's key, and a joined_at beat each member's start
// cell; both are told to nobody, so every member's story is the story it
// was. A member that joins later gets its own joined_at.
func TestTheRecordSaysWhatItIsARunOf(t *testing.T) {
	in := wakeSetup(BehaviorDriver{}, monster("m-guard", 6, 2, holds()))
	in.Key = "the-test-hall"
	in.Content = "c0ffee"
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	beats, audiences := recordBeats(t, enc)
	var content map[string]any
	starts := map[string]any{}
	for i, b := range beats {
		switch b["beat"] {
		case BeatSceneContent:
			content = b
			require.Empty(t, audiences[i], "scene_content is told to nobody")
		case BeatJoinedAt:
			starts[b["member"].(string)] = b["cell"]
			require.Empty(t, audiences[i], "joined_at is told to nobody")
		}
	}
	require.Equal(t, map[string]any{"beat": BeatSceneContent, "site_hash": string(enc.Site().Hash()), "key": "the-test-hall",
		"content_hash": "c0ffee"}, content,
		"no engine version in a test build of this module")
	for id, cell := range starts {
		where, ok := enc.canvas.GetEntityPosition(id)
		require.True(t, ok)
		require.Equal(t, map[string]any{"x": where.X, "y": where.Y}, cell, "%s's start cell", id)
	}
	require.Len(t, starts, 2, "every member given at setup")

	for _, who := range []MemberID{"alice", "m-guard"} {
		for _, beat := range watchStoryOf(t, enc, who) {
			require.NotContains(t, []any{BeatSceneContent, BeatJoinedAt}, beat["beat"], "%s is told neither", who)
		}
	}
}

// TestARoleStateNamesARoleTheCreatureHas: a step from a role the creature
// does not have writes nothing and is refused.
func TestARoleStateNamesARoleTheCreatureHas(t *testing.T) {
	enc, _ := wakeHall(t, twoRoles(t))
	err := enc.appendRoleStateBeat(enc.members["m-guard"], "nowhere", machine.Stepped{From: "idle", To: "looking"})
	require.ErrorContains(t, err, `has no role "nowhere"`)
}
