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
func openDriven(t *testing.T, compiled dungeonspec.Compiled) *encounter.Encounter {
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
			Sight:         &sightList{fallback: 24},
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
func garrisonKeep(t *testing.T) dungeonspec.Compiled {
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
// shut — on the castle and on the 10k garrison keep, and after each one
// every member's holdings, known cells and every MEMORY's Confirmed stamp
// must match the oracle's ([encounter.Encounter.ExportedPercepts] says what a
// current holding's stamp is not compared for).
func TestObserversOf_EqualsFullRefresh(t *testing.T) {
	for _, tc := range []struct {
		name  string
		site  func(t *testing.T) dungeonspec.Compiled
		moves int
		// doorOdds is one door swing in this many moves.
		doorOdds int
		// noRace skips the run in a -race binary.
		noRace bool
		// pace makes every step the one walker's, back and forth beside
		// its start, so a short run lands every step and still raises the
		// clock (six of its cells are a round).
		pace bool
	}{
		// The castle compares after every one of twenty verbs, a quarter of
		// them doors, and runs under -race too.
		{"castle", func(t *testing.T) dungeonspec.Compiled { return compiledSite(t, 0) }, 20, 4, false, false},
		// The keep is BOUNDED, and NOT RUN UNDER -race: twelve verbs (ten
		// paces and two door swings at this seed, plus the probe step), one
		// walker pacing so its steps raise the clock and walk the garrison,
		// and a memory forms — all required below. Its twin looks with the
		// whole garrison on every look, which the race detector slows ~30x
		// (456 s locally against 15 s), past Go's ten-minute package
		// timeout. The property is single-goroutine, so -race adds nothing
		// here; every plain `go test` runs it, and it fails both mutants it
		// exists for (the bound shrunk to bare sight, at move 10; a step's
		// cells dropped, at move 4). Under -race the same two are failed by
		// TestDoor_RefreshesTheRimObserver and by the castle run.
		{"keep-10k", garrisonKeep, 12, 6, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.noRace && raceDetector {
				t.Skip("the 10k keep's twin is ~30x slower under -race; plain go test runs it")
			}
			compiled := tc.site(t)
			enc, oracle := openDriven(t, compiled), openDriven(t, compiled)
			oracle.ExportedEveryoneLooks()
			agree := func(what string, args ...interface{}) {
				t.Helper()
				got, err := enc.ExportedPercepts()
				require.NoError(t, err)
				want, err := oracle.ExportedPercepts()
				require.NoError(t, err)
				for id, w := range want {
					require.Equal(t, w, got[id], "member %q after %s", id, fmt.Sprintf(what, args...))
				}
			}
			agree("first light")
			started := monsterCells(t, enc)
			rng := rand.New(rand.NewPCG(762, 1)) //nolint:gosec // a seeded walk, not a secret
			doors := enc.Doors()
			stepped, swung, memories := 0, 0, 0
			start := whereIs(t, enc, keepPlayers[0])
			var beside spatial.Position
			if tc.pace {
				// Found as the bench finds it: step there on both twins and
				// back, held to the oracle like every other verb.
				for _, d := range hexSteps {
					cell := spatial.Position{X: start.X + d.X, Y: start.Y + d.Y}
					_, err := enc.Step(&encounter.StepInput{Member: keepPlayers[0], To: cell, EndWalk: true})
					_, oerr := oracle.Step(&encounter.StepInput{Member: keepPlayers[0], To: cell, EndWalk: true})
					require.Equal(t, oerr == nil, err == nil, "probe step to %v answers alike", cell)
					if err != nil {
						continue
					}
					beside = cell
					agree("the probe step to %v", cell)
					break
				}
				require.NotEqual(t, start, beside, "the walker has a cell to pace to")
			}
			for i := 0; i < tc.moves; i++ {
				if len(doors) > 0 && rng.IntN(tc.doorOdds) == 0 {
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
					require.Equal(t, oerr == nil, err == nil, "move %d: door %q answers alike", i, d.ID)
					if err == nil {
						swung++
						agree("move %d: door %q", i, d.ID)
					}
					continue
				}
				player := keepPlayers[0]
				if !tc.pace && rng.IntN(4) == 0 {
					player = keepPlayers[rng.IntN(len(keepPlayers))]
				}
				at := whereIs(t, enc, player)
				d := hexSteps[rng.IntN(len(hexSteps))]
				to := spatial.Position{X: at.X + d.X, Y: at.Y + d.Y}
				if tc.pace {
					to = beside
					if at == beside {
						to = start
					}
				}
				_, err := enc.Step(&encounter.StepInput{Member: player, To: to, EndWalk: true})
				_, oerr := oracle.Step(&encounter.StepInput{Member: player, To: to, EndWalk: true})
				require.Equal(t, oerr == nil, err == nil, "move %d: %q to %v answers alike", i, player, to)
				if err != nil {
					continue
				}
				stepped++
				agree("move %d: %q to %v", i, player, to)
			}
			got, err := enc.ExportedPercepts()
			require.NoError(t, err)
			for _, p := range got {
				memories += strings.Count(p, "confirmed=")
			}
			require.Positive(t, stepped, "some step landed, or the property was never asked")
			walked := 0
			for id, cell := range monsterCells(t, enc) {
				if started[id] != cell {
					walked++
				}
			}
			t.Logf("%s: %d steps, %d door swings, %d creatures walked, %d memories",
				tc.name, stepped, swung, walked, memories)
			require.Positive(t, walked, "some creature walked in a wake pass, or its refresh was never asked")
			require.Positive(t, memories, "some memory formed, or its stamp was never compared")
			require.Positive(t, swung, "some door swung, or a door's look was never compared")
		})
	}
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
func monsterCells(t *testing.T, enc *encounter.Encounter) map[encounter.MemberID]spatial.Position {
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
		site   func(t *testing.T) dungeonspec.Compiled
		noRace bool
	}{
		{"castle", func(t *testing.T) dungeonspec.Compiled { return compiledSite(t, 0) }, false},
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
	restore := enc.ExportedProbeRebuilds(func(observers []encounter.MemberID) {
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
	restore := enc.ExportedProbeRebuilds(func(observers []encounter.MemberID) {
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
