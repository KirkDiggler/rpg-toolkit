// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// benchmarkApproaches prices one [encounter.Encounter.Approaches] on the 10k
// keep with its garrison: the party's best seat asked of every monster at a
// sword's five feet: the free-roam walk (budget zero, one search per
// target) or a fight's thirty feet (one bounded flood). Afford asks it once,
// of the candidates its attacker sees (rpg-project#872).
func benchmarkApproaches(b *testing.B, budget int) { benchmarkApproachesOf(b, budget, 0) }

// benchmarkApproachesOf is [benchmarkApproaches] asked of only the first n
// monsters (zero is all of them).
func benchmarkApproachesOf(b *testing.B, budget, n int) {
	enc, _ := openSite(b, 10000, true)
	roster, err := enc.Members()
	require.NoError(b, err)
	var targets []encounter.MemberID
	for _, m := range roster {
		if m.Kind == encounter.KindMonster {
			targets = append(targets, m.ID)
		}
	}
	require.NotEmpty(b, targets)
	b.Logf("%d monsters", len(targets))
	if n > 0 && n < len(targets) {
		targets = targets[:n]
	}
	in := encounter.ApproachesInput{Mover: keepPlayers[0], Targets: targets, RangeFeet: 5, Budget: budget, Unbounded: budget == 0}
	for b.Loop() {
		if _, err := enc.Approaches(in); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkApproaches_Keep10kFreeRoam is the unbounded walk.
func BenchmarkApproaches_Keep10kFreeRoam(b *testing.B) { benchmarkApproaches(b, 0) }

// BenchmarkApproaches_Keep10kFight is a fight's thirty feet.
func BenchmarkApproaches_Keep10kFight(b *testing.B) { benchmarkApproaches(b, 6) }

// BenchmarkApproaches_Keep10kFreeRoamOne is the unbounded walk asked of one
// monster: one search.
func BenchmarkApproaches_Keep10kFreeRoamOne(b *testing.B) { benchmarkApproachesOf(b, 0, 1) }
