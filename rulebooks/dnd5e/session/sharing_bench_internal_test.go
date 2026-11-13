// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// sharing_bench_internal_test.go is the merge's cost per commit
// (rpg-project#935): four players, one commit's 40-entry delta of steps,
// ticks, per-observer sightings and speech versions, each member's story read
// once (as the commit caches it), folded for every recipient. Run:
//
//	go test -run '^$' -bench BenchmarkCommitFold -benchmem .

// benchDelta is a commit's delta for players a–d.
func benchDelta(n int) []record.Entry {
	players := []string{"a", "b", "c", "d"}
	everyone := make([]core.EntityID, len(players))
	for i, p := range players {
		everyone[i] = core.EntityID(p)
	}
	var out []record.Entry
	for i := 1; i <= n; i++ {
		var audience []core.EntityID
		var fields map[string]any
		switch i % 4 {
		case 0:
			audience = everyone
			fields = map[string]any{"beat": encounter.BeatMoved, "member": "a", "position": map[string]any{"X": i, "Y": 0}}
		case 1:
			audience = []core.EntityID{everyone[i%3]}
			fields = map[string]any{"beat": encounter.BeatSighted, "gained": []string{"goblin"}}
		case 2:
			audience = []core.EntityID{everyone[i%3]}
			fields = map[string]any{"beat": encounter.BeatSpeech, "utterance": fmt.Sprintf("u%d", i/8), "speaker": "a",
				"words": "Evening.", "place": map[string]any{"x": 1, "y": 1}, "loudness": "speak"}
		default:
			audience = everyone
			fields = map[string]any{"beat": encounter.BeatTick}
		}
		payload, _ := json.Marshal(fields)
		out = append(out, record.Entry{Seq: uint64(i), ID: fmt.Sprintf("01HZZZZZZZZZZZZZZZZZZ%05d", i), Audience: audience,
			Payload: payload, Tags: map[string]string{encounter.ActTag: "move:01HZZZZZZZZZZZZZZZZZZZWALK"}})
	}
	return out
}

func benchCommitFold(b *testing.B, sharing encounter.Sharing) {
	delta := benchDelta(40)
	roster := []encounter.MemberData{
		{ID: "a", Kind: encounter.KindPlayer}, {ID: "b", Kind: encounter.KindPlayer},
		{ID: "c", Kind: encounter.KindPlayer}, {ID: "d", Kind: encounter.KindPlayer},
	}
	read := map[encounter.MemberID][]record.Entry{}
	for _, m := range roster {
		read[m.ID] = storiesOf(delta...)(m.ID)
	}
	storyOf := func(id encounter.MemberID) []record.Entry { return read[id] }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		party := dataParty(sharing, roster)
		for _, m := range roster {
			_ = actsAfter("sess", string(m.ID), party.storyFor(m.ID, storyOf(m.ID), storyOf), "")
		}
	}
}

// BenchmarkCommitFoldOwn is one commit's fold for four players at each their own.
func BenchmarkCommitFoldOwn(b *testing.B) { benchCommitFold(b, encounter.SharingOwn) }

// BenchmarkCommitFoldView is the same commit at shared view.
func BenchmarkCommitFoldView(b *testing.B) { benchCommitFold(b, encounter.SharingView) }
