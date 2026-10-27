// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BenchmarkMadKingKitchenToGate is the castle's own cost of a walk
// (rpg-toolkit#2049): the hero leaves the kitchen, opens the kitchen and
// hall doors, crosses the courtyard, opens the gate and walks to the
// captain — every Step a world tick with the garrison thinking, so every
// sight lane, crossing and standing read the castle's walls and doors cost.
// Opening the castle is outside the timer.
//
//	go test -run XXX -bench BenchmarkMadKingKitchenToGate -benchtime 3x -count 3 ./dungeonspec
//
// One machine, an AMD 7945HX, -count=3 at 3x. "Before" is the same benchmark
// with the encounter module at 7ebfc0c3 (every placed footprint traced on
// every read); "after" is with the bounding-box pre-check
// (footprint_bounds.go):
//
//	before  18.9 s – 19.7 s/op
//	after   4.18 s – 4.20 s/op
func BenchmarkMadKingKitchenToGate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		s := openTheCastle(b, besideTheCook)
		b.StartTimer()

		s.outOfTheKitchen()
		s.throughTheGate(kingCaptain)
	}
}

// BenchmarkCastleMove4Cells is the operator's walk-lag walk (rpg-project#662,
// rpg-toolkit#2097): the hero at the file's outside start walks four cells
// north-east, toward the gatehouse, then back, one walk per op — four Steps,
// each a world tick with the garrison thinking, which is what one 4-cell
// session Move asks of the encounter. Opening the castle is outside the
// timer, and the castle stays open across ops, so a guard who has not moved
// since the last walk is a guard who has not moved since the last verb.
//
//	go test -run XXX -bench BenchmarkCastleMove4Cells -benchtime 10x -count 3 ./castletest
//
// One machine, an AMD 3950X, -count=3 at 10x. "Before" is the encounter
// module at 95cf84b9 (every observer re-sweeps every unknown floor cell on
// every pass); "after" is with the sight sweep skip (sightsweep.go):
//
//	before  995 ms – 1018 ms/op
//	after   189 ms –  200 ms/op
func BenchmarkCastleMove4Cells(b *testing.B) {
	b.StopTimer()
	// A session sees what a sheet that states no senses sees: the rulebook's
	// 120 feet (combat.DefaultSightFeet), twice this host's usual sixty.
	s := openTheCastleAs(b, castleHost{at: loadContent(b, kingFixture).PartyStart[0].At, sightFt: sessionSightFeet})
	begin := s.at(hero)
	if begin != (spatial.Position{X: -2, Y: 16}) {
		b.Fatalf("the hero starts at %v, not the outside start", begin)
	}
	out := []spatial.Position{{X: -1, Y: 15}, {X: 0, Y: 14}, {X: 1, Y: 13}, {X: 2, Y: 12}}
	back := []spatial.Position{{X: 1, Y: 13}, {X: 0, Y: 14}, {X: -1, Y: 15}, {X: -2, Y: 16}}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		walk := out
		if i%2 == 1 {
			walk = back
		}
		for j, cell := range walk {
			if _, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: j == len(walk)-1}); err != nil {
				b.Fatalf("op %d step %d to %v: %v", i, j, cell, err)
			}
		}
	}
}
