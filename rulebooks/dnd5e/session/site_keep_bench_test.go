// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// site_keep_bench_test.go prices a load of the garrison keep through the site
// tier (rpg-project#879 S5, sites.go): the keep launched once into a store
// that keeps the site piece, then 9 loads each on the warm process (its
// provider holds the site) and on a cold one (a fresh provider, the piece
// read from the store and compiled), and the stored checkpoint's bytes.
// Gated on S5_SITE_BENCH=1 because it builds the 10k keep.

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

func TestSiteLoadMedian_KeepGarrison10k(t *testing.T) {
	if os.Getenv("S5_SITE_BENCH") != "1" {
		t.Skip("set S5_SITE_BENCH=1 to price a load of the garrison keep through the site tier")
	}
	const loads = 9
	compiled := garrisonKeep(t)
	ctx := context.Background()
	store := newSiteStore()
	sessions, seats := newFakeSessions(), newFakeSeats()
	characters := newFakeCharacters(talkingFighter("alice"))
	manager := func(sites encounter.SiteProvider) *session.Manager {
		mgr, err := session.NewManager(&session.Config{
			Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: seats, CharacterConfig: newFakeCharacterConfig(),
			PresentationIDs: testPresentationIDs{}, Dice: &castleDice{d20s: []int{10}}, TurnDriver: session.Driver(),
			Sessions: sessions, Encounters: store, Characters: characters, Sites: sites,
		})
		if err != nil {
			t.Fatal(err)
		}
		return mgr
	}
	warm := manager(encounter.NewMemorySites(nil))
	if _, err := warm.Launch(ctx, &session.LaunchInput{
		Session: "sess", Dungeon: &compiled, Party: []string{"alice"},
	}); err != nil {
		t.Fatal(err)
	}
	world := store.byID["sess"]
	blob, err := json.Marshal(world)
	if err != nil {
		t.Fatal(err)
	}
	piece, err := json.Marshal(store.pieces[world.SiteHash])
	if err != nil {
		t.Fatal(err)
	}

	sample := func(mgr func() *session.Manager) []time.Duration {
		out := make([]time.Duration, 0, loads)
		for range loads {
			m := mgr()
			start := time.Now()
			if _, err := m.LoadWorldForTest(ctx, "sess"); err != nil {
				t.Fatal(err)
			}
			out = append(out, time.Since(start))
		}
		slices.Sort(out)
		return out
	}
	warmLoads := sample(func() *session.Manager { return warm })
	coldLoads := sample(func() *session.Manager { return manager(encounter.NewMemorySites(nil)) })

	t.Logf("load on bench-keep-garrison, %d samples each: warm median %.1f ms (min %.1f, max %.1f); cold from the piece median %.1f ms (min %.1f, max %.1f)",
		loads, ms(warmLoads[loads/2]), ms(warmLoads[0]), ms(warmLoads[loads-1]),
		ms(coldLoads[loads/2]), ms(coldLoads[0]), ms(coldLoads[loads-1]))
	t.Logf("checkpoint %d bytes; site piece %d bytes, stored once", len(blob), len(piece))
}
