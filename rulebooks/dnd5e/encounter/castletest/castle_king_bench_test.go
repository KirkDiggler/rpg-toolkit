// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import "testing"

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
