// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

func journalTestCapabilities() Capabilities {
	caps := appearanceTestCapabilities()
	caps.CheckResolver = RefusingCheckResolver{}
	caps.Witness = nobodyPerceives{}
	return caps
}

// TestAKnowledgeFactAndAHoldingsFactReplayInTheOrderTheyHappened saves a run in
// which a knowledge fact, a holdings fact and another knowledge fact were
// appended in that order, and loads it: the journal comes back with the same
// facts in the same order and the same sequence numbers.
//
// The journal used to be saved under two keys and rejoined as "all knowledge,
// then all holdings", so the second knowledge fact came back with the sequence
// number the holdings fact had held. A fold that asks which of two facts came
// later would have answered differently after a reload.
func TestAKnowledgeFactAndAHoldingsFactReplayInTheOrderTheyHappened(t *testing.T) {
	no := false
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("hall", 0, 0, 10, 10)},
			Props: []PropInput{{
				ID: "idol", Holdable: true, Ref: "test:props:idol", At: spatial.Position{X: 2, Y: 1},
				BlocksMovement: &no, BlocksLineOfSight: &no,
			}},
			Concealments: []ConcealmentInput{
				{ID: "first", Checks: []CheckApproach{{Ability: "perception", DC: 10}}, Cells: []spatial.Position{{X: 8, Y: 8}}},
				{ID: "second", Checks: []CheckApproach{{Ability: "perception", DC: 10}}, Cells: []spatial.Position{{X: 9, Y: 9}}},
			},
		},
		Members:      []MemberInput{{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}}},
		Endings:      []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: journalTestCapabilities(),
	})
	require.NoError(t, err)

	learn := func(id ConcealmentID) {
		t.Helper()
		require.NoError(t, enc.revealConcealmentTo("alice", enc.field.concealmentOf(id), "test", 0))
	}
	learn("first")
	_, err = enc.Hold(&HoldInput{Member: "alice", Target: "idol"})
	require.NoError(t, err)
	learn("second")

	kinds := func(facts []FactData) []string {
		out := make([]string, 0, len(facts))
		for _, f := range facts {
			out = append(out, f.Kind)
		}
		return out
	}
	saved := enc.ToData()
	require.NotNil(t, saved.Journal)
	want := kinds(saved.Journal.Facts)
	require.Contains(t, want, "known:concealment:first")
	require.Contains(t, want, "known:concealment:second")
	require.Contains(t, want, "held:idol")
	indexOf := func(kind string) int {
		for i, k := range want {
			if k == kind {
				return i
			}
		}
		return -1
	}
	require.Less(t, indexOf("known:concealment:first"), indexOf("held:idol"), "saved in append order")
	require.Less(t, indexOf("held:idol"), indexOf("known:concealment:second"), "saved in append order")

	raw, err := json.Marshal(saved)
	require.NoError(t, err)
	var restored EncounterData
	require.NoError(t, json.Unmarshal(raw, &restored))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: restored, Capabilities: journalTestCapabilities()})
	require.NoError(t, err)

	require.Equal(t, want, kinds(loaded.ToData().Journal.Facts), "the reloaded journal saves in the same order")
	replayed := func(e *Encounter) map[journal.Kind]int {
		out := map[journal.Kind]int{}
		for _, f := range e.world.log.All() {
			out[f.Kind] = f.Seq
		}
		return out
	}
	before, after := replayed(enc), replayed(loaded)
	for _, kind := range []journal.Kind{"known:concealment:first", "held:idol", "known:concealment:second"} {
		require.Contains(t, after, kind)
		require.Equal(t, before[kind], after[kind], "%s keeps the sequence number it was appended at", kind)
	}
	require.Less(t, after["known:concealment:first"], after["held:idol"])
	require.Less(t, after["held:idol"], after["known:concealment:second"])
}
