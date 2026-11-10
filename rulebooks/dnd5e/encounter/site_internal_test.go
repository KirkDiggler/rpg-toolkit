// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// site_internal_test.go pins THE SITE IS CONTENT THE RECORD REFERENCES
// (rpg-project#879 S5): one compiled site per content per provider, shared
// read-only by every encounter on it; a door opened in one game changes
// nothing another sees and nothing in the site; a checkpoint carries the hash
// and no field, loads through a provider that knows it and fails closed
// through one that does not; and the raw answer to what a cell sees is the
// same after a reload, read from the site rather than swept again.

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sweepOn is the sweep scene built on sites.
func sweepOn(t *testing.T, sites SiteProvider) *Encounter {
	t.Helper()
	in := sweepSetup()
	in.Sites = sites
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	return enc
}

// siteBytes is everything a site holds that a game could write into, as
// bytes: its stored form, its doors' openings and its partitions.
func siteBytes(t *testing.T, s *Site) string {
	t.Helper()
	parts, err := s.field.partitionsNow()
	require.NoError(t, err)
	raw, err := json.Marshal(struct {
		Data       SiteData
		Openings   map[DoorID][]DoorEdge
		Partitions map[string]PartitionID
	}{s.Data(), s.field.doorOpenings, partitionsByCell(parts)})
	require.NoError(t, err)
	return string(raw)
}

func partitionsByCell(p *fieldPartitions) map[string]PartitionID {
	out := make(map[string]PartitionID, len(p.of))
	for cell, id := range p.of {
		raw, _ := json.Marshal(cell)
		out[string(raw)] = id
	}
	return out
}

// TestTheProviderCompilesOnce: the same content set up twice, and loaded
// back, is one site compiled once; a hash the provider never saw fails
// closed by name; a source that hashes otherwise than asked is refused; at
// the bound the least recently used site goes. Mutants: a provider that
// compiles per call (the counter climbs, the pointers differ); a miss that
// answers (nil, nil) (the load panics or loads nothing).
func TestTheProviderCompilesOnce(t *testing.T) {
	sites := NewMemorySites(nil)
	a := sweepOn(t, sites)
	b := sweepOn(t, sites)
	require.Same(t, a.site, b.site, "one content, one site")
	require.Equal(t, 1, sites.compiles, "compiled once: the second Setup is found by its input's fingerprint")

	// A door authored open is a different input and the same site: it
	// compiles once more, and resolves to the site already held.
	open := sweepSetup()
	open.Sites = sites
	open.Field.Doors[0].State = DoorIsOpen()
	c, err := NewEncounter(open)
	require.NoError(t, err)
	require.Same(t, a.site, c.site, "a door's state is the game's, not the site's")
	require.Equal(t, 2, sites.compiles, "a new input compiles once")
	_, err = NewEncounter(open)
	require.NoError(t, err)
	require.Equal(t, 2, sites.compiles, "and never again")

	loaded, err := LoadEncounter(&LoadEncounterInput{Data: a.ToData(), Sites: sites, Capabilities: sweepCapabilities()})
	require.NoError(t, err)
	require.Same(t, a.site, loaded.site, "a load on the warm provider reads the same site")
	require.Equal(t, 2, sites.compiles, "and compiles nothing")

	_, err = sites.Site(context.Background(), &SiteInput{Hash: "no-such-site"})
	require.ErrorIs(t, err, ErrUnknownSite)

	piece := a.site.Data()
	_, err = NewMemorySites(nil).Site(context.Background(), &SiteInput{Hash: "not-its-hash", Data: &piece})
	require.ErrorIs(t, err, ErrInvalidData, "a piece that hashes otherwise is not the site asked for")

	cold := NewMemorySites(nil)
	fromPiece, err := cold.Site(context.Background(), &SiteInput{Hash: a.site.Hash(), Data: &piece})
	require.NoError(t, err)
	require.Equal(t, a.site.Hash(), fromPiece.Site.Hash(), "the stored piece compiles to the hash it was taken under")

	small := NewMemorySites(&MemorySitesConfig{Entries: 1})
	first := sweepOn(t, small)
	other := sweepSetup()
	other.Sites = small
	other.Field.Regions = append(other.Field.Regions, rectRegion("annex", 0, 4, 2, 2))
	_, err = NewEncounter(other)
	require.NoError(t, err)
	_, err = small.Site(context.Background(), &SiteInput{Hash: first.site.Hash()})
	require.ErrorIs(t, err, ErrUnknownSite, "at the bound the least recently used site is evicted")
	require.NotNil(t, first.site.field, "and an encounter already on it runs on")
}

// TestADoorInOneGameIsNotAnothers: two games on one site; opening the neck
// door in A teaches A's watcher the east room and changes nothing B sees,
// and the site is byte-identical before and after. Mutant: an instance that
// runs on the site's own field rather than a copy (B's door records are
// replaced by A's, so B sees the open door too).
func TestADoorInOneGameIsNotAnothers(t *testing.T) {
	sites := NewMemorySites(nil)
	a := sweepOn(t, sites)
	b := sweepOn(t, sites)
	require.Same(t, a.site, b.site)
	before := siteBytes(t, a.site)
	farEast := a.field.cellAt(spatial.Position{X: 9, Y: 1})
	seenByB, err := b.VisibleCellsFor(&VisibleCellsInput{Member: sweepWatchA})
	require.NoError(t, err)

	_, err = a.OpenDoor(&OpenDoorInput{Door: sweepNeckDor})
	require.NoError(t, err)
	require.True(t, a.world.knownCells(sweepWatchA)[farEast], "A's watcher sees down the opened neck")

	walkAround(t, b, 2)
	require.False(t, b.world.knownCells(sweepWatchA)[farEast], "B's door is still shut")
	require.Equal(t, DoorIsClosed().Kind(), b.doorsByID[sweepNeckDor].state.Kind())
	after, err := b.VisibleCellsFor(&VisibleCellsInput{Member: sweepWatchA})
	require.NoError(t, err)
	require.Equal(t, seenByB.Cells, after.Cells, "and B's watcher sees what it saw")
	require.Equal(t, before, siteBytes(t, a.site), "the site is untouched by either game")
}

// TestTwoGamesOnOneSiteRunAtOnce: two games on one site, each on its own
// goroutine, walking and opening the door. Under -race this is the pin that
// nothing an instance does writes shared memory outside the site's two
// locked memos.
func TestTwoGamesOnOneSiteRunAtOnce(t *testing.T) {
	sites := NewMemorySites(nil)
	games := []*Encounter{sweepOn(t, sites), sweepOn(t, sites)}
	before := siteBytes(t, games[0].site)
	var wg sync.WaitGroup
	errs := make([]error, len(games))
	for i, enc := range games {
		wg.Add(1)
		go func() {
			defer wg.Done()
			there := enc.field.cellAt(spatial.Position{X: 1, Y: 2})
			home := enc.field.cellAt(spatial.Position{X: 0, Y: 2})
			for step := 0; step < 4; step++ {
				to := there
				if step%2 == 1 {
					to = home
				}
				if _, err := enc.Step(&StepInput{Member: sweepWalker, To: to, EndWalk: true}); err != nil {
					errs[i] = err
					return
				}
				if step == 1 && i == 0 {
					if _, err := enc.OpenDoor(&OpenDoorInput{Door: sweepNeckDor}); err != nil {
						errs[i] = err
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, before, siteBytes(t, games[0].site), "the site is untouched by either game")
}

// TestTheCheckpointCarriesTheHashNotTheField: the saved bytes name the site
// and carry no field; a load through a provider that holds the site answers
// what a cell sees exactly as the original does, and one that does not holds
// it fails closed by name. Mutant: ToData dropping the hash (the load is
// refused as a pre-site blob).
func TestTheCheckpointCarriesTheHashNotTheField(t *testing.T) {
	sites := NewMemorySites(nil)
	enc := sweepOn(t, sites)
	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &keys))
	require.NotContains(t, keys, "field", "the field is the site's, not the checkpoint's")
	require.JSONEq(t, `"`+string(enc.site.Hash())+`"`, string(keys["site_hash"]))

	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	back, err := LoadEncounter(&LoadEncounterInput{Data: data, Sites: sites, Capabilities: sweepCapabilities()})
	require.NoError(t, err)
	for _, m := range sweepMembers {
		want, err := enc.VisibleCellsFor(&VisibleCellsInput{Member: m})
		require.NoError(t, err)
		got, err := back.VisibleCellsFor(&VisibleCellsInput{Member: m})
		require.NoError(t, err)
		require.Equal(t, want.Cells, got.Cells, "%s sees the same after the load", m)
	}

	_, err = LoadEncounter(&LoadEncounterInput{Data: data, Sites: NewMemorySites(nil), Capabilities: sweepCapabilities()})
	require.ErrorIs(t, err, ErrUnknownSite, "a provider that does not hold the site fails closed")
	require.Contains(t, err.Error(), string(enc.site.Hash()), "and names the site")
}

// TestTheRawAnswerIsTheSitesAfterAReload: what a cell sees, for each door
// state, is the same answer before and after a reload — and after the reload
// it is read from the site, not swept again. Mutant: a reload that keeps the
// raw answers on the instance (the walk after the load misses).
func TestTheRawAnswerIsTheSitesAfterAReload(t *testing.T) {
	sites := NewMemorySites(nil)
	enc := sweepOn(t, sites)
	walkAround(t, enc, 2)

	for _, open := range []bool{false, true} {
		if open {
			_, err := enc.OpenDoor(&OpenDoorInput{Door: sweepNeckDor})
			require.NoError(t, err)
			walkAround(t, enc, 2)
		}
		back, err := LoadEncounter(&LoadEncounterInput{Data: enc.ToData(), Sites: sites, Capabilities: sweepCapabilities()})
		require.NoError(t, err)
		require.Zero(t, back.visibleMisses, "precondition: a load sweeps nothing")
		walkAround(t, back, 2)
		require.Zero(t, back.visibleMisses, "door open %v: every cell the walk visits is read from the site", open)
		require.Positive(t, back.visibleHits)
		for _, m := range sweepMembers {
			want, err := enc.VisibleCellsFor(&VisibleCellsInput{Member: m})
			require.NoError(t, err)
			got, err := back.VisibleCellsFor(&VisibleCellsInput{Member: m})
			require.NoError(t, err)
			require.Equal(t, want.Cells, got.Cells, "door open %v: %s sees the same", open, m)
		}
		requireNothingUnlearned(t, back, "after the reloaded walk")
	}
}

// TestAFootprintDoorIsTheGamesOwn: two games on one site with a door that
// stands as a rectangle; opening it in A clears A's way and leaves B's leaf
// shut. The leaf's state is read through the records each game attached to
// its own copy of the field, so this is the pin, without the race detector,
// that an instance never runs on the site's own field. Mutant: instanceField
// returning the site's field (the last game to attach owns everybody's door).
func TestAFootprintDoorIsTheGamesOwn(t *testing.T) {
	a := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	b := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	require.Same(t, a.site, b.site, "precondition: one content, one site")
	require.NotSame(t, a.field, a.site.field, "an instance runs on its own copy")

	_, err := a.OpenDoor(&OpenDoorInput{Door: holdLeaf})
	require.NoError(t, err)
	require.Empty(t, a.field.blockingDoorFootprints(), "A's leaf is open: nothing of it blocks in A")
	require.Len(t, b.field.blockingDoorFootprints(), 1, "B's leaf is still shut")
	require.Equal(t, DoorIsClosed().Kind(), b.field.blockingDoorFootprints()[0].state.Kind())
}

// TestConcurrentAskersCompileOnce: eight goroutines asking a cold provider
// for one site at once — by hash with its stored piece, and by the same
// construction input — compile it once each way and share the answer. The
// compile is slowed so the askers overlap. Mutant: no per-key sharing (the
// counter reads eight).
func TestConcurrentAskersCompileOnce(t *testing.T) {
	piece := sweepOn(t, NewMemorySites(nil)).site.Data()
	hash := sweepOn(t, NewMemorySites(nil)).site.Hash()
	for _, by := range []string{"hash", "input"} {
		t.Run(by, func(t *testing.T) {
			sites := NewMemorySites(nil)
			sites.beforeCompile = func() { time.Sleep(20 * time.Millisecond) }
			const askers = 8
			got := make([]*Site, askers)
			errs := make([]error, askers)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := 0; i < askers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					in := &SiteInput{Hash: hash, Data: &piece}
					if by == "input" {
						field := sweepField()
						in = &SiteInput{Field: &field}
					}
					out, err := sites.Site(context.Background(), in)
					errs[i] = err
					if err == nil {
						got[i] = out.Site
					}
				}()
			}
			close(start)
			wg.Wait()
			for i := range got {
				require.NoError(t, errs[i])
				require.Same(t, got[0], got[i], "every asker gets the one site")
			}
			require.Equal(t, 1, sites.compiles, "compiled once")
		})
	}
}

// TestTwoGamesDeriveTheSitesPartitionsAtOnce: two games on one cold site,
// whose partitions are derived when first asked, both ask at once from two
// goroutines. Under -race this pins the memo's lock; either way the pass runs
// once. Mutant: the memo unlocked (a data race on the derivation).
func TestTwoGamesDeriveTheSitesPartitionsAtOnce(t *testing.T) {
	base := sweepOn(t, NewMemorySites(nil))
	piece := base.site.Data()
	sites := NewMemorySites(nil)
	load := func() *Encounter {
		enc, err := LoadEncounter(&LoadEncounterInput{Data: base.ToData(), Site: &piece, Sites: sites, Capabilities: sweepCapabilities()})
		require.NoError(t, err)
		return enc
	}
	games := []*Encounter{load(), load()}
	require.Same(t, games[0].site, games[1].site, "precondition: one site")
	require.Zero(t, games[0].site.field.partitions.derivations, "precondition: a cold load derives nothing")
	cell := base.field.cellAt(spatial.Position{X: 0, Y: 1})
	var wg sync.WaitGroup
	errs := make([]error, len(games))
	for i, enc := range games {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, errs[i] = enc.field.partitionAt(cell)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, games[0].site.field.partitions.derivations, "the site's pass ran once for both games")
}
