// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// observers_test.go pins A REFRESH COSTS ITS REACH (rpg-project#762): a verb's
// look is the members within sight of what it changed, and the percepts it
// leaves are the ones the whole roster's look would have written.

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

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
			Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees, Machine: m.Machine,
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
			Driver:        encounter.TableDriver{Roller: rollsLowest{}},
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

// TestObserversOf_EqualsFullRefresh is the property: after any verb, the
// percepts the bounded refresh left are the ones a refresh of every member
// writes over the same world. Random players take random steps (and every
// raise those steps cause walks the garrison through the wake pass's own
// refresh), and random doors open and shut, on the castle and on the 10k
// garrison keep; after each, every member's holdings and known cells are held
// to a full refresh's ([encounter.Encounter.ExportedAgreesWithFullRefresh]).
func TestObserversOf_EqualsFullRefresh(t *testing.T) {
	for _, tc := range []struct {
		name  string
		site  func(t *testing.T) dungeonspec.Compiled
		moves int
	}{
		{"castle", func(t *testing.T) dungeonspec.Compiled { return compiledSite(t, 0) }, 60},
		{"keep-10k", garrisonKeep, 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc := openDriven(t, tc.site(t))
			require.NoError(t, enc.ExportedAgreesWithFullRefresh(), "the launch's look is the full one")
			started := monsterCells(t, enc)
			rng := rand.New(rand.NewPCG(762, 1)) //nolint:gosec // a seeded walk, not a secret
			doors := enc.Doors()
			stepped, swung := 0, 0
			for i := 0; i < tc.moves; i++ {
				if len(doors) > 0 && rng.IntN(4) == 0 {
					d := doors[rng.IntN(len(doors))]
					var err error
					if enc.Doors()[indexOfDoor(enc.Doors(), d.ID)].State.Kind() == encounter.DoorOpen {
						_, err = enc.CloseDoor(&encounter.CloseDoorInput{Door: d.ID})
					} else {
						_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: d.ID})
					}
					if err == nil {
						swung++
						require.NoError(t, enc.ExportedAgreesWithFullRefresh(), "move %d: door %q", i, d.ID)
					}
					continue
				}
				// Mostly the one walker, so the world clock raises (six of
				// a thirty-foot walker's cells is a round) and the garrison
				// walks through the wake pass's refresh.
				player := keepPlayers[0]
				if rng.IntN(4) == 0 {
					player = keepPlayers[rng.IntN(len(keepPlayers))]
				}
				at := whereIs(t, enc, player)
				d := hexSteps[rng.IntN(len(hexSteps))]
				to := spatial.Position{X: at.X + d.X, Y: at.Y + d.Y}
				if _, err := enc.Step(&encounter.StepInput{Member: player, To: to, EndWalk: true}); err != nil {
					continue
				}
				stepped++
				require.NoError(t, enc.ExportedAgreesWithFullRefresh(), "move %d: %q to %v", i, player, to)
			}
			require.Positive(t, stepped, "some step landed, or the property was never asked")
			walked := 0
			for id, cell := range monsterCells(t, enc) {
				if started[id] != cell {
					walked++
				}
			}
			require.Positive(t, walked, "some creature walked in a wake pass, or its refresh was never asked")
			t.Logf("%s: %d steps, %d door swings, %d creatures walked; every one held to a full refresh",
				tc.name, stepped, swung, walked)
		})
	}
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

// TestObserversOf_FarObserverUntouched is the cost: a player's step on the
// 10k keep asks no rebuild of any member farther than its own sight (plus a
// cover's face) from both ends of the step. The probe sees every observer
// list a rebuild is handed, the wake pass's included.
func TestObserversOf_FarObserverUntouched(t *testing.T) {
	enc := openDriven(t, garrisonKeep(t))
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
	require.NoError(t, enc.ExportedAgreesWithFullRefresh())
}
