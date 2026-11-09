// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// observers_test.go pins A REFRESH COSTS ITS REACH (rpg-project#762): a verb's
// look is the members within sight of what it changed, and the percepts it
// leaves are the ones the whole roster's look would have written.

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// hexSteps are the six hex neighbours a random walker picks from.
var hexSteps = []spatial.Position{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}}

// openDriven opens a compiled site with the party and every monster, on the
// table driver the garrison bench runs: creatures that walk when the world
// clock raises, so a player's step reaches the wake pass's own refresh.
func openDriven(t testing.TB, compiled dungeonspec.Compiled) *encounter.Encounter {
	t.Helper()
	return openDrivenSeeing(t, compiled, 24)
}

// openDrivenSeeing is [openDriven] with every member seeing sight cells.
func openDrivenSeeing(t testing.TB, compiled dungeonspec.Compiled, sight int) *encounter.Encounter {
	t.Helper()
	members := seatedParty(compiled)
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options,
			Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees, Behaviors: m.Behaviors, Machine: m.Machine, BlockingStates: m.BlockingStates, Posts: m.Posts,
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{fallback: sight},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        walksThirty{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        encounter.BehaviorDriver{},
			Roller:        rollsLowest{},
			CheckResolver: findsNothing{},
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc
}

// garrisonKeep is the bench's posts-asleep garrison: the 10k keep with 99
// creatures given orders and 20 of them walking patrols.
func garrisonKeep(t testing.TB) dungeonspec.Compiled {
	t.Helper()
	return compiledOnce(t, "keep-garrison-10000-posts-listen", func() ([]byte, error) {
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = 10000, 1, "bench-keep-garrison"
		opts.Garrison, opts.Patrols, opts.PostsListen = 99, 20, true
		raw, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
		return raw, err
	})
}

// TestObserversOf_EqualsFullRefresh is the property: the percepts a bounded
// look leaves are the ones the whole roster's look writes. Two twins open the
// same site; the oracle looks with everyone on every look, as before
// rpg-project#762 ([encounter.Encounter.ExportedEveryoneLooks]). The same
// random verbs land on both — players' steps (mostly one walker, so the clock
// raises and the garrison walks through the wake pass) and doors opened and
// shut — on the castle and on a garrison keep, and after each one every
// member's holdings, known cells and every MEMORY's Confirmed stamp must
// match the oracle's ([encounter.Encounter.ExportedPercepts] says what a
// current holding's stamp is not compared for).
//
// THE KEEP IS THE SMALL ONE (rpg-project#909). The fast path is
// [Encounter.observersWithin]'s bound — the members within sight of a
// changed cell — and the oracle is the whole roster. What a row needs is a
// step that lands, a door that swings, a creature that walks in a wake pass
// and a memory that forms, each compared at once. The castle brings what no
// generated keep has: a posted guard, arrivals and creatures on a table
// alone. The keep row is the 10k garrison's row as it was — twelve verbs,
// one walker pacing so its steps raise the clock, posts listening, a patrol
// walking — on [smallKeep]'s garrison, with sight scaled to the keep: 8
// cells across a site 41 wide, as 24 is across the 10k one's 100, so
// creatures stand one cell past a door's sight as they do in the 10k keep's
// grounds.
// That rim is the case the bound's +1 exists for: shrunk to bare sight, the
// keep row fails at move 6 (the 10k row failed at move 10). About a second
// where the 10k keep took fifteen. It is still NOT RUN UNDER -race, as the
// 10k row was not: the oracle's every look is the whole garrison's, which the
// race detector slows to 8 s here, the property is single-goroutine, and
// under -race TestDoor_RefreshesTheRimObserver fails the same mutant. Every
// plain `go test` runs both rows. The 10k garrison's comparison is
// [BenchmarkObserversOfEqualsFullRefreshKeep10k].
func TestObserversOf_EqualsFullRefresh(t *testing.T) {
	for _, row := range []refreshRow{
		// The castle compares after every one of twenty verbs, a quarter of
		// them doors.
		{"castle", func(tb testing.TB) dungeonspec.Compiled { return compiledSite(tb, 0) }, 20, 4, false, false, 0},
		// The keep: twelve verbs, one walker pacing, a door in six, sight
		// scaled to the keep.
		{"keep", func(tb testing.TB) dungeonspec.Compiled { return smallKeep(tb, true) }, 12, 6, true, true, 8},
	} {
		t.Run(row.name, func(t *testing.T) {
			if row.noRace && raceDetector {
				t.Skip("the keep's twin looks with the whole garrison, ~8x slower under -race; plain go test runs it")
			}
			requireEqualsFullRefresh(t, row)
		})
	}
}

// BenchmarkObserversOfEqualsFullRefreshKeep10k is the comparison on the 10k
// garrison keep (rpg-project#909): the row [TestObserversOf_EqualsFullRefresh]
// ran, outside -race only, before the small keep — a number a measurement
// asks for by name, never a CI gate. Each iteration opens both twins and
// lands the row's twelve verbs, failing at the first percept that differs.
//
//	go test -run '^$' -bench BenchmarkObserversOfEqualsFullRefreshKeep10k -benchtime 1x .
func BenchmarkObserversOfEqualsFullRefreshKeep10k(b *testing.B) {
	row := refreshRow{"keep-10k", garrisonKeep, 12, 6, true, true, 0}
	for i := 0; i < b.N; i++ {
		requireEqualsFullRefresh(b, row)
	}
}

// refreshRow is one site [requireEqualsFullRefresh] lands verbs on.
type refreshRow struct {
	name  string
	site  func(tb testing.TB) dungeonspec.Compiled
	moves int
	// doorOdds is one door swing in this many moves.
	doorOdds int
	// pace makes every step the one walker's, back and forth beside its
	// start, so a short run lands every step and still raises the clock
	// (six of its cells are a round).
	pace bool
	// noRace skips the row in a -race binary.
	noRace bool
	// sight is every member's sight in cells; zero is the rulebook's 120
	// feet, 24.
	sight int
}

// requireEqualsFullRefresh opens the row's site twice, the oracle looking
// with everyone, lands the row's seeded verbs on both and requires every
// member's percepts alike after each.
func requireEqualsFullRefresh(tb testing.TB, row refreshRow) {
	tb.Helper()
	compiled := row.site(tb)
	sight := row.sight
	if sight == 0 {
		sight = 24
	}
	enc, oracle := openDrivenSeeing(tb, compiled, sight), openDrivenSeeing(tb, compiled, sight)
	oracle.ExportedEveryoneLooks()
	agree := func(what string, args ...interface{}) {
		tb.Helper()
		got, err := enc.ExportedPercepts()
		require.NoError(tb, err)
		want, err := oracle.ExportedPercepts()
		require.NoError(tb, err)
		for id, w := range want {
			require.Equal(tb, w, got[id], "member %q after %s", id, fmt.Sprintf(what, args...))
		}
	}
	agree("first light")
	started := monsterCells(tb, enc)
	rng := rand.New(rand.NewPCG(762, 1)) //nolint:gosec // a seeded walk, not a secret
	doors := enc.Doors()
	stepped, swung, memories := 0, 0, 0
	start := whereIs(tb, enc, keepPlayers[0])
	var beside spatial.Position
	if row.pace {
		// Found as the bench finds it: step there on both twins and
		// back, held to the oracle like every other verb.
		for _, d := range hexSteps {
			cell := spatial.Position{X: start.X + d.X, Y: start.Y + d.Y}
			_, err := enc.Step(&encounter.StepInput{Member: keepPlayers[0], To: cell, EndWalk: true})
			_, oerr := oracle.Step(&encounter.StepInput{Member: keepPlayers[0], To: cell, EndWalk: true})
			require.Equal(tb, oerr == nil, err == nil, "probe step to %v answers alike", cell)
			if err != nil {
				continue
			}
			beside = cell
			agree("the probe step to %v", cell)
			break
		}
		require.NotEqual(tb, start, beside, "the walker has a cell to pace to")
	}
	for i := 0; i < row.moves; i++ {
		if len(doors) > 0 && rng.IntN(row.doorOdds) == 0 {
			d := doors[rng.IntN(len(doors))]
			open := enc.Doors()[indexOfDoor(enc.Doors(), d.ID)].State.Kind() == encounter.DoorOpen
			var err, oerr error
			if open {
				_, err = enc.CloseDoor(&encounter.CloseDoorInput{Door: d.ID})
				_, oerr = oracle.CloseDoor(&encounter.CloseDoorInput{Door: d.ID})
			} else {
				_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: d.ID})
				_, oerr = oracle.OpenDoor(&encounter.OpenDoorInput{Door: d.ID})
			}
			require.Equal(tb, oerr == nil, err == nil, "move %d: door %q answers alike", i, d.ID)
			if err == nil {
				swung++
				agree("move %d: door %q", i, d.ID)
			}
			continue
		}
		player := keepPlayers[0]
		if !row.pace && rng.IntN(4) == 0 {
			player = keepPlayers[rng.IntN(len(keepPlayers))]
		}
		at := whereIs(tb, enc, player)
		d := hexSteps[rng.IntN(len(hexSteps))]
		to := spatial.Position{X: at.X + d.X, Y: at.Y + d.Y}
		if row.pace {
			to = beside
			if at == beside {
				to = start
			}
		}
		_, err := enc.Step(&encounter.StepInput{Member: player, To: to, EndWalk: true})
		_, oerr := oracle.Step(&encounter.StepInput{Member: player, To: to, EndWalk: true})
		require.Equal(tb, oerr == nil, err == nil, "move %d: %q to %v answers alike", i, player, to)
		if err != nil {
			continue
		}
		stepped++
		agree("move %d: %q to %v", i, player, to)
	}
	got, err := enc.ExportedPercepts()
	require.NoError(tb, err)
	for _, p := range got {
		memories += strings.Count(p, "confirmed=")
	}
	require.Positive(tb, stepped, "some step landed, or the property was never asked")
	walked := 0
	for id, cell := range monsterCells(tb, enc) {
		if started[id] != cell {
			walked++
		}
	}
	tb.Logf("%s: %d steps, %d door swings, %d creatures walked, %d memories",
		row.name, stepped, swung, walked, memories)
	require.Positive(tb, walked, "some creature walked in a wake pass, or its refresh was never asked")
	require.Positive(tb, memories, "some memory formed, or its stamp was never compared")
	require.Positive(tb, swung, "some door swung, or a door's look was never compared")
}

// TestObserversOf_AMemoryIsStampedAtTheLastRoundItWasSeen is the probe from
// the review of rpg-toolkit#2224: a watcher outside every changed cell's
// bound watches a subject through four quiet rounds, then the subject steps
// out of its sight. The memory that forms is stamped 4 — the last round it
// was seen — exactly what the whole roster's look stamps it, not the 0 of
// the watcher's last look of its own.
func TestObserversOf_AMemoryIsStampedAtTheLastRoundItWasSeen(t *testing.T) {
	open := func() *encounter.Encounter { return openPacingRow(t) }
	for _, tc := range []struct {
		name     string
		everyone bool
	}{{"bounded", false}, {"everyone looks", true}} {
		t.Run(tc.name, func(t *testing.T) {
			enc := open()
			if tc.everyone {
				enc.ExportedEveryoneLooks()
			}
			// Twenty-four cells, there and back: four rounds of a thirty-foot
			// walker, and none of the watcher's business.
			for i := 0; i < 12; i++ {
				_, err := enc.Step(&encounter.StepInput{Member: "p", To: cellAt(1, 1), EndWalk: true})
				require.NoError(t, err)
				_, err = enc.Step(&encounter.StepInput{Member: "p", To: cellAt(0, 1), EndWalk: true})
				require.NoError(t, err)
			}
			watched, ok, err := enc.ExportedHolding("g", "a")
			require.NoError(t, err)
			require.True(t, ok && watched.CurrentOn(perception.Sight), "g watches a through the pacing")
			high := enc.ToData().Clock.HighWater
			require.EqualValues(t, 4, high, "the pacing raised the clock to 4")

			_, err = enc.Step(&encounter.StepInput{Member: "a", To: cellAt(54, 1), EndWalk: true})
			require.NoError(t, err)
			memory, ok, err := enc.ExportedHolding("g", "a")
			require.NoError(t, err)
			require.True(t, ok, "g remembers a")
			require.False(t, memory.CurrentOn(perception.Sight), "a stepped out of g's sight")
			require.EqualValues(t, 4, memory.Confirmed, "the memory is stamped at the last round a was seen")
		})
	}
}

// TestObserversOf_SortedWithDeclared is observersOf's own shape: the members
// within their own sight (+1) of a changed cell and every declared member,
// however far, in ID order — and nobody else.
func TestObserversOf_SortedWithDeclared(t *testing.T) {
	enc := openPacingRow(t)
	got, err := enc.ExportedObserversOf([]spatial.Position{cellAt(53, 1)}, []encounter.MemberID{"p"})
	require.NoError(t, err)
	require.Equal(t, []encounter.MemberID{"a", "g", "p"}, got,
		"a stands on the cell, g at its own sight of it, p is declared from 53 away")
	got, err = enc.ExportedObserversOf([]spatial.Position{cellAt(0, 1)}, nil)
	require.NoError(t, err)
	require.Equal(t, []encounter.MemberID{"p"}, got, "nobody but p is within sight of p's cell")
}

// openPacingRow is the review's probe field: one row 70 long, the pacer p at
// col 0 (sight 24), the watcher g at col 50 (sight 3), and a at col 53, on
// the edge of g's sight. All players, so nothing forms a fight.
func openPacingRow(t *testing.T) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("row", 0, 0, 70, 3)},
		},
		Members: []encounter.MemberInput{
			{ID: "p", Kind: encounter.KindPlayer, Position: cellAt(0, 1)},
			{ID: "g", Kind: encounter.KindPlayer, Position: cellAt(50, 1)},
			{ID: "a", Kind: encounter.KindPlayer, Position: cellAt(53, 1)},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: pacingRowCapabilities(),
	})
	require.NoError(t, err)

	return enc
}

// pacingRowCapabilities is what the pacing row runs on, at setup and at load:
// g sees three cells, everybody else twenty-four.
func pacingRowCapabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         &sightList{reach: map[encounter.MemberID]int{"g": 3}, fallback: 24},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        walksThirty{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Actors: encounter.Actors{
			Striker:   passStriker{},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// TestObserversOf_AMemoryIsStampedTheSameAcrossAReload is the probe through
// a save and a load: the last look is never stored, a loaded encounter takes
// it from the saved high-water (a verb ends with a look), and the memory the
// next look forms is stamped exactly as the encounter that never left memory
// stamps it — 4, the last round the subject was seen.
func TestObserversOf_AMemoryIsStampedTheSameAcrossAReload(t *testing.T) {
	kept := openPacingRow(t)
	for i := 0; i < 12; i++ {
		_, err := kept.Step(&encounter.StepInput{Member: "p", To: cellAt(1, 1), EndWalk: true})
		require.NoError(t, err)
		_, err = kept.Step(&encounter.StepInput{Member: "p", To: cellAt(0, 1), EndWalk: true})
		require.NoError(t, err)
	}
	raw, err := json.Marshal(kept.ToData())
	require.NoError(t, err)
	require.NotContains(t, string(raw), "last_look", "the last look is derived, never stored")
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: pacingRowCapabilities()})
	require.NoError(t, err)

	for _, enc := range []*encounter.Encounter{kept, loaded} {
		_, err := enc.Step(&encounter.StepInput{Member: "a", To: cellAt(54, 1), EndWalk: true})
		require.NoError(t, err)
	}
	want, ok, err := kept.ExportedHolding("g", "a")
	require.NoError(t, err)
	require.True(t, ok)
	got, ok, err := loaded.ExportedHolding("g", "a")
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, got.CurrentOn(perception.Sight), "a stepped out of g's sight")
	require.EqualValues(t, 4, want.Confirmed, "the kept encounter stamps the last round a was seen")
	require.Equal(t, want.Confirmed, got.Confirmed, "and the reloaded one stamps it the same")
	keptPercepts, err := kept.ExportedPercepts()
	require.NoError(t, err)
	loadedPercepts, err := loaded.ExportedPercepts()
	require.NoError(t, err)
	require.Equal(t, keptPercepts, loadedPercepts, "every percept and every memory's stamp alike")
}

// monsterCells is where every monster stands now.
func monsterCells(t testing.TB, enc *encounter.Encounter) map[encounter.MemberID]spatial.Position {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	out := map[encounter.MemberID]spatial.Position{}
	for _, m := range members {
		if m.Kind == encounter.KindMonster {
			out[m.ID] = m.Position
		}
	}
	return out
}

// indexOfDoor is where id stands in doors.
func indexOfDoor(doors []encounter.Door, id encounter.DoorID) int {
	for i, d := range doors {
		if d.ID == id {
			return i
		}
	}
	return -1
}

// TestObserversOf_FarObserverUntouched is the cost: a player's step asks no
// rebuild of any member farther than its own sight (plus a cover's face)
// from both ends of the step. The probe sees every observer list a rebuild is
// handed. On the castle and on the 10k keep; the keep's launch alone is a
// minute under -race, so a -race binary runs the castle only.
func TestObserversOf_FarObserverUntouched(t *testing.T) {
	for _, tc := range []struct {
		name   string
		site   func(tb testing.TB) dungeonspec.Compiled
		noRace bool
	}{
		{"castle", func(tb testing.TB) dungeonspec.Compiled { return compiledSite(tb, 0) }, false},
		{"keep-10k", garrisonKeep, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.noRace && raceDetector {
				t.Skip("the 10k keep's launch is a minute under -race; plain go test runs it")
			}
			farObserverUntouched(t, openDriven(t, tc.site(t)))
		})
	}
}

func farObserverUntouched(t *testing.T, enc *encounter.Encounter) {
	t.Helper()
	player := keepPlayers[0]
	from := whereIs(t, enc, player)
	var to spatial.Position
	for _, d := range hexSteps {
		cell := spatial.Position{X: from.X + d.X, Y: from.Y + d.Y}
		if _, err := enc.Step(&encounter.StepInput{Member: player, To: cell, EndWalk: true}); err == nil {
			to = cell
			break
		}
	}
	require.NotEqual(t, from, to, "the player found a cell to step to")
	// Back again, now under the probe: one step, from `to` to `from`. Two
	// cells of a thirty-foot walker raise no round (six do), so no patrol
	// walks and the step's own look is the only one the probe can see.
	rebuilt := map[encounter.MemberID]bool{}
	restore := encounter.ExportedProbeRebuilds(func(observers []encounter.MemberID) {
		for _, id := range observers {
			rebuilt[id] = true
		}
	})
	defer restore()
	_, err := enc.Step(&encounter.StepInput{Member: player, To: from, EndWalk: true})
	require.NoError(t, err)

	members, err := enc.Members()
	require.NoError(t, err)
	far := 0
	for _, m := range members {
		if enc.Distance(m.Position, from) <= 25 || enc.Distance(m.Position, to) <= 25 {
			continue
		}
		far++
		require.False(t, rebuilt[m.ID], "%q stands %v from the step and was rebuilt", m.ID, enc.Distance(m.Position, from))
	}
	require.Positive(t, far, "some member stands beyond the bound, or nothing was asked")
	require.True(t, rebuilt[player], "the mover looks")
}

// TestDoor_RefreshesRayCrossers is the door's bound: a door changes every ray
// that crosses it, so opening one refreshes the observer 20 hexes away whose
// line runs through it — and who now sees the member beyond — and not the
// one 30 hexes away, whose sight ends short of the door.
func TestDoor_RefreshesRayCrossers(t *testing.T) {
	const seam = 30
	field := encounter.FieldInput{
		Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
		Regions: []encounter.RegionInput{rectRegion(doorWest, 0, 0, seam+1, 3), rectRegion(doorEast, seam+1, 0, 6, 3)},
		Walls:   seamWallExcept(seam, 3, 1),
		Doors:   []encounter.DoorInput{{ID: theDoor, Edges: doorEdgesAcross(seam, 1), State: encounter.DoorIsClosed()}},
	}
	near, far, beyond := encounter.MemberID("near"), encounter.MemberID("far"), encounter.MemberID("beyond")
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: field,
		Members: []encounter.MemberInput{
			{ID: near, Kind: encounter.KindPlayer, Position: cellAt(seam-20, 1)},
			{ID: far, Kind: encounter.KindPlayer, Position: cellAt(seam-30, 1)},
			{ID: beyond, Kind: encounter.KindPlayer, Position: cellAt(seam+2, 1)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{fallback: 24},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	door := cellAt(seam, 1)
	require.InDelta(t, 20, enc.Distance(whereIs(t, enc, near), door), 0, "near stands 20 hexes from the door")
	require.InDelta(t, 30, enc.Distance(whereIs(t, enc, far), door), 0, "far stands 30 hexes from the door")

	rebuilt := map[encounter.MemberID]bool{}
	restore := encounter.ExportedProbeRebuilds(func(observers []encounter.MemberID) {
		for _, id := range observers {
			rebuilt[id] = true
		}
	})
	defer restore()
	out, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoor})
	require.NoError(t, err)

	require.True(t, rebuilt[near], "the observer whose line crosses the door looks again")
	require.False(t, rebuilt[far], "the observer whose sight ends short of the door does not")
	gained := map[encounter.MemberID]bool{}
	if delta := out.IntelDeltas[near]; delta != nil {
		for _, p := range delta.FirstContact {
			gained[encounter.MemberID(p.ID)] = true
		}
	}
	require.True(t, gained[beyond], "near sees through the opened door to the member beyond it")
}

// TestDoor_RefreshesTheRimObserver is the "+ 1" in the bound: an observer
// standing one cell beyond its own sight of a footprint door's cells sees
// the shut leaf by its face ([sightReach.seesCell]), so when the door opens
// it must look again and learn the door is open. The bounded look is held to
// an everyone-looks twin; the twin's own percept says the rim observer holds
// the door, so the comparison cannot pass by holding nothing.
func TestDoor_RefreshesTheRimObserver(t *testing.T) {
	open := func() *encounter.Encounter {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: footprintDoorField(encounter.DoorIsClosed()),
			// rim stands two cells from the leaf's covered cell (2,1) and
			// sees one: the face, and nothing past it.
			Members: []encounter.MemberInput{{ID: "rim", Kind: encounter.KindPlayer, Position: cellAt(0, 1)}},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
			Capabilities: encounter.Capabilities{
				Sight:         &sightList{reach: map[encounter.MemberID]int{"rim": 1}},
				Equipment:     encounter.UnobservedEquipment{},
				Conditions:    encounter.UnobservedEquipment{},
				Sheets:        zeroSheets{},
				Participation: everyoneStanding{},
				Initiative:    orderAsGiven{},
				Driver:        passDriver{},
				Actors: encounter.Actors{
					Striker:   passStriker{},
					Mover:     quietMover{},
					Announcer: quietAnnouncer{},
				},
			},
		})
		require.NoError(t, err)
		return enc
	}
	enc, oracle := open(), open()
	oracle.ExportedEveryoneLooks()
	before, err := oracle.ExportedPercepts()
	require.NoError(t, err)
	require.Contains(t, before["rim"], "door|"+string(theLeaf), "rim holds the shut leaf by its face")

	for _, e := range []*encounter.Encounter{enc, oracle} {
		_, err := e.OpenDoor(&encounter.OpenDoorInput{Door: theLeaf})
		require.NoError(t, err)
	}
	want, err := oracle.ExportedPercepts()
	require.NoError(t, err)
	got, err := enc.ExportedPercepts()
	require.NoError(t, err)
	require.NotEqual(t, before["rim"], want["rim"], "the opening changed what rim holds")
	require.Equal(t, want["rim"], got["rim"], "and the bounded look told rim, from one cell past its sight")
}
