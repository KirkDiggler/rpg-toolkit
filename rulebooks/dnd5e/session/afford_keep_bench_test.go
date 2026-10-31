// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// afford_keep_bench_test.go prices one Afford on the benchmark keep
// (rpg-project#740) at 10,000 hexes: the read the S1 probe measured at 655
// atlas projections and minutes of wall clock, because the speak menu asked
// about every prop on the map (rpg-project#753). Within reach, it costs the
// cells within reach.
//
// THE KEEP IS GENERATED, NOT CHECKED IN: keepgen's embedded plan at a hex
// count and seed writes the same document every time, the one
// `go run ./cmd/keepgen -hexes 10000 -seed 1 -key bench-keep` writes in the
// encounter module, so a fixture file would only be a copy that could drift.

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// openTheKeep launches the benchmark keep at hexes with alice at its party
// start.
func openTheKeep(b *testing.B, hexes int) *session.Manager {
	b.Helper()
	opts := keepgen.DefaultOptions()
	opts.Hexes, opts.Seed, opts.Key = hexes, 1, "bench-keep"
	raw, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
	if err != nil {
		b.Fatal(err)
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		b.Fatal(err)
	}
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: &castleDice{d20s: []int{10}}, TurnDriver: session.Driver(),
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: newFakeCharacters(talkingFighter("alice")),
	})
	if err != nil {
		b.Fatal(err)
	}
	if _, err := mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "sess", Dungeon: &compiled, Party: []string{"alice"},
	}); err != nil {
		b.Fatal(err)
	}
	return mgr
}

// BenchmarkAfford_Keep10k is one whole Afford for alice at the keep's party
// start: the encounter loaded from its record, every row compiled, the speak
// menu read within reach. Target < 100 ms (rpg-project#753, S2).
func BenchmarkAfford_Keep10k(b *testing.B) {
	mgr := openTheKeep(b, 10000)
	ctx := context.Background()
	in := &session.AffordInput{Session: "sess", Member: "alice"}
	for b.Loop() {
		if _, err := mgr.Afford(ctx, in); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSpeakMenu_Keep10k is the speak menu alone on the same keep, over
// an encounter loaded once: what this read costs apart from the load every
// Afford pays before it. The load is the encounter's to price.
func BenchmarkSpeakMenu_Keep10k(b *testing.B) {
	mgr := openTheKeep(b, 10000)
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := session.SpeakMenuForTest(enc, "alice"); err != nil {
			b.Fatal(err)
		}
	}
}
