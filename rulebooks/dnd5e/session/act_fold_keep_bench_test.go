// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// act_fold_keep_bench_test.go prices the live act fold on the garrison keep
// (rpg-project#891): twenty world-clock Moves of one cell, back and forth,
// after one launch, with the acts going to a stream that says only alice
// listens and to one that cannot say. It logs each Move's time and what each
// commit's fold read against what a fold of every member's whole story would
// have. Gated on S7_FOLD_BENCH=1 because it launches the 10k keep.

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestActFold_KeepGarrison20Moves(t *testing.T) {
	if os.Getenv("S7_FOLD_BENCH") != "1" {
		t.Skip("set S7_FOLD_BENCH=1 to price the act fold on the garrison keep")
	}
	for _, run := range []struct {
		name   string
		stream session.ActStream
		watch  bool
	}{
		{name: "alice listens", stream: &listeningCapture{listening: []string{"alice"}}},
		{name: "stream cannot say", stream: &actCapture{}},
		{name: "alice listens, reads counted", stream: &listeningCapture{listening: []string{"alice"}}, watch: true},
	} {
		t.Run(run.name, func(t *testing.T) {
			var watch *session.ActFoldWatch
			if run.watch {
				watch = session.WatchActFoldForTest(t, "sess")
			}
			samples, folds := keepMoves(t, run.stream, 20, watch)
			sorted := slices.Clone(samples)
			slices.Sort(sorted)
			t.Logf("%s: p50 %.1f ms, first %.1f ms, last %.1f ms, max %.1f ms",
				run.name, ms(sorted[len(sorted)/2]), ms(samples[0]), ms(samples[len(samples)-1]), ms(sorted[len(sorted)-1]))
			for i, f := range folds {
				t.Logf("  move %2d: fold read %d entries; every member's whole story is %d", i+1, f[0], f[1])
			}
		})
	}
}

// keepMoves launches the garrison keep and walks alice one cell east and
// back, n Moves, returning each Move's time and, when watched, each Move's
// fold reads: entries read, and entries every member's whole story holds.
func keepMoves(t *testing.T, stream session.ActStream, n int, watch *session.ActFoldWatch) ([]time.Duration, [][2]int) {
	t.Helper()
	compiled := garrisonKeep(t)
	ctx := context.Background()
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: stream, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: &castleDice{d20s: []int{10}}, TurnDriver: session.Driver(),
		Sessions: newFakeSessions(), Encounters: jsonEncounters{newFakeEncounters()}, Characters: newFakeCharacters(talkingFighter("alice")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}}); err != nil {
		t.Fatal(err)
	}
	at := aliceAt(ctx, t, mgr)
	var samples []time.Duration
	var folds [][2]int
	seen := 0
	if watch != nil {
		seen = len(watch.Folds())
	}
	for i := range n {
		step := 1
		if i%2 == 1 {
			step = -1
		}
		next := spatial.Position{X: at.X + float64(step), Y: at.Y}
		start := time.Now()
		if _, err := mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{next}}); err != nil {
			t.Fatalf("move %d to %v: %v", i+1, next, err)
		}
		samples = append(samples, time.Since(start))
		at = next
		if watch == nil {
			continue
		}
		all := watch.Folds()
		read, whole := 0, 0
		for _, f := range all[seen:] {
			read += f.Read
		}
		seen = len(all)
		enc, err := mgr.LoadWorldForTest(ctx, "sess")
		if err != nil {
			t.Fatal(err)
		}
		for _, member := range enc.WorldView().EverMembers {
			story, err := enc.Story(&encounter.StoryInput{Audience: member})
			if err != nil {
				t.Fatal(err)
			}
			whole += len(story)
		}
		folds = append(folds, [2]int{read, whole})
	}
	return samples, folds
}
