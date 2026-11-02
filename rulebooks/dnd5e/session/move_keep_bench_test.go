// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// move_keep_bench_test.go prices one plain world-clock Move of one cell on
// the garrison keep (rpg-project#753, S7 T12), the probe's method: the keep
// launched through Manager.Launch, the encounter stored as JSON both ways the
// way a host's repository stores it, Moves 1–5 after each of 9 launches, the
// median of the 45. Gated on T12_MOVE_BENCH=1 because one run launches the
// 10k keep nine times.

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// jsonEncounters is fakeEncounters with the save side encoded too, so a Move
// pays the encode a host's repository pays.
type jsonEncounters struct{ *fakeEncounters }

func (j jsonEncounters) SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var stored encounter.EncounterData
	if err := json.Unmarshal(raw, &stored); err != nil {
		return err
	}
	return j.fakeEncounters.SaveEncounter(ctx, id, &stored)
}

// garrisonKeep is the shelf's bench-keep-garrison document: keepgen -hexes
// 10000 -seed 1 -garrison 99 -patrols 20 with posts listening.
func garrisonKeep(t testing.TB) dungeonspec.Compiled {
	t.Helper()
	opts := keepgen.DefaultOptions()
	opts.Hexes, opts.Seed, opts.Key = 10000, 1, "bench-keep-garrison"
	opts.Garrison, opts.Patrols, opts.PostsListen = 99, 20, true
	raw, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func TestMoveMedian_KeepGarrison10k(t *testing.T) {
	if os.Getenv("T12_MOVE_BENCH") != "1" {
		t.Skip("set T12_MOVE_BENCH=1 to price a Move on the garrison keep")
	}
	const launches, moves = 9, 5
	compiled := garrisonKeep(t)
	ctx := context.Background()
	var samples []time.Duration
	for range launches {
		mgr, err := session.NewManager(&session.Config{
			Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
			PresentationIDs: testPresentationIDs{}, Dice: &castleDice{d20s: []int{10}}, TurnDriver: session.Driver(),
			Sessions: newFakeSessions(), Encounters: jsonEncounters{newFakeEncounters()}, Characters: newFakeCharacters(talkingFighter("alice")),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := mgr.Launch(ctx, &session.LaunchInput{
			Session: "sess", Dungeon: &compiled, Party: []string{"alice"},
		}); err != nil {
			t.Fatal(err)
		}
		at := aliceAt(ctx, t, mgr)
		for i := range moves {
			next := spatial.Position{X: at.X + 1, Y: at.Y}
			start := time.Now()
			_, err := mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{next}})
			took := time.Since(start)
			if err != nil {
				t.Fatalf("move %d east of %v: %v", i+1, at, err)
			}
			samples = append(samples, took)
			at = next
		}
	}
	slices.Sort(samples)
	t.Logf("Move on bench-keep-garrison, %d samples: median %.1f ms, min %.1f ms, max %.1f ms",
		len(samples), ms(samples[len(samples)/2]), ms(samples[0]), ms(samples[len(samples)-1]))
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// aliceAt is where alice stands.
func aliceAt(ctx context.Context, t testing.TB, mgr *session.Manager) spatial.Position {
	t.Helper()
	enc, err := mgr.LoadWorldForTest(ctx, "sess")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range enc.WorldView().Members {
		if m.ID == "alice" && m.Cell != nil {
			return spatial.Position{X: m.Cell.X, Y: m.Cell.Y}
		}
	}
	t.Fatal("alice is not placed")
	return spatial.Position{}
}
