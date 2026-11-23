// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// seenfilter_bench_internal_test.go measures what the seen filter saves
// (rpg-project#1126): a hall of forty guards who all see the whole map, each
// in a state with one `seen` step, while the party comes into and goes out of
// everyone's sight once per iteration and the world moves on. The sight
// toggles every guard's range, so the guards also come into each other's
// sight; none holds a note.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const sfBenchGuards = 40

// sfBenchHall is the forty guards, each running a recipe whose one `seen`
// step is on `when` (nil: it requires nothing), and four in the party.
func sfBenchHall(b *testing.B, when *machine.Guard) (*Encounter, soSight, []MemberID) {
	b.Helper()
	var members []MemberInput
	var guards []MemberID
	sight := soSight{}
	for i := 0; i < sfBenchGuards; i++ {
		id := MemberID(fmt.Sprintf("guard-%02d", i))
		r := recipe(&testing.T{}, "calm", map[string]machine.State{
			"calm":  {Transitions: []machine.Transition{{On: beatSeen, When: when, To: "alert", Reason: "seen"}}},
			"alert": {Transitions: []machine.Transition{{On: beatSeen, When: when, To: "calm", Reason: "seen again"}}},
		})
		m := listener(id, float64(i%10), float64(i/10), r)
		m.Faction = postedGarrison
		members = append(members, m)
		guards = append(guards, id)
		sight[id] = 0
	}
	for i, id := range []MemberID{"bob", "cat", "dan", "eve"} {
		members = append(members, MemberInput{ID: id, Kind: KindPlayer, Position: spatial.Position{X: float64(5 + i), Y: 4}})
	}
	enc, err := NewEncounter(postedSetup(sight, members...))
	require.NoError(b, err)

	return enc, sight, guards
}

func benchSeenFilter(b *testing.B, when *machine.Guard, filter bool) {
	was := seenFilterOn
	seenFilterOn = filter
	b.Cleanup(func() { seenFilterOn = was })
	enc, sight, guards := sfBenchHall(b, when)
	look := func(r int) (beats int) {
		for _, id := range guards {
			sight[id] = r
		}
		before := len(enc.landed)
		_, _, err := enc.refreshSight(enc.rosterIDs())
		require.NoError(b, err)
		for _, l := range enc.landed[before:] {
			if l.Beat == beatSeen {
				beats++
			}
		}
		require.NoError(b, enc.spendWorldAction("alice"))

		return beats
	}
	startSeq, err := enc.NextStorySeq()
	require.NoError(b, err)
	beats := 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		beats += look(1_000_000)
		beats += look(0)
	}
	b.StopTimer()
	endSeq, err := enc.NextStorySeq()
	require.NoError(b, err)
	b.ReportMetric(float64(beats)/float64(b.N), "seen-beats/op")
	b.ReportMetric(float64(endSeq-startSeq)/float64(b.N), "log-entries/op")
}

func BenchmarkSeenFilter(b *testing.B) {
	for _, tc := range []struct {
		name string
		when *machine.Guard
	}{
		{"requiring-a-note", sfBarred()},
		{"requiring-nothing", nil},
	} {
		for _, filter := range []bool{true, false} {
			name := tc.name + "/filter-off"
			if filter {
				name = tc.name + "/filter-on"
			}
			b.Run(name, func(b *testing.B) { benchSeenFilter(b, tc.when, filter) })
		}
	}
}
