// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// world_test.go is ONE WORLD, ALWAYS INSTALLED (rpg-toolkit#1114, #1090).
//
// There is nothing here about how the world is BUILT, because this package no
// longer builds one. It used to: a room assembled out of the encounter's
// persisted description, with its own copy of grid construction and no walls in
// it at all, and the tests that went with it were a differential suite whose
// whole job was keeping that copy honest. The copy is gone
// (encounter v0.21.0's Encounter.Canvas hands out the map itself), and a test
// that two things agree has no subject when there is only one of them.
//
// What is left is what this package still decides: that the world is installed,
// that it is installed for EVERY interaction, and that what gets installed is
// the composition's map rather than something assembled here.

// worldProbe is a machine that does nothing except report the world it was run
// in. It is how these tests see what Resolve actually installed, rather than
// what a helper would have returned if asked.
type worldProbe struct {
	room  spatial.Room
	found bool
}

func (p *worldProbe) Start(ctx context.Context, _ *Participants) (Step, error) {
	p.room, p.found = gamectx.Room(ctx)

	return Done{Outcome: probeOutcome{}}, nil
}

type probeOutcome struct{}

func (probeOutcome) isOutcome() {}

// walledWorld is two chambers side by side with a real wall on the seam.
//
// room-1 owns absolute columns 0-9 and room-2 owns 10-19, with boundary edges
// between them — every edge, diagonals included, because spatial registers a
// boundary between ANY two adjacent cells and a same-row wall has a hole at
// each corner (rpg-toolkit#1106's lesson).
func walledWorld(t *testing.T) encounter.EncounterData {
	t.Helper()

	// SAME-ROW PAIRS ONLY. This loop used to author the diagonals too — "every
	// edge, diagonals included, because spatial registers a boundary between
	// ANY two adjacent cells and a same-row wall has a hole at each corner
	// (rpg-toolkit#1106's lesson)". That lesson was learned on a SQUARE grid,
	// where (9,y) and (10,y±1) touch at a corner. The product runs on pointy-top
	// hex exclusively, where they do not touch at all, and the composition now
	// refuses a wall between cells that are not adjacent — by name, which is how
	// this surfaced rather than compiling into a wall that blocks nothing.
	//
	// Whether a hex seam needs its own hole-free pattern is the composition's
	// question and not this package's. What this fixture owes the tests below is
	// one authored wall on the installed map, at the pair they probe.
	var seam []encounter.WallInput
	for y := 0; y < 8; y++ {
		seam = append(seam, encounter.WallInput{Boundary: spatial.Boundary{
			From:              spatial.Position{X: 9, Y: float64(y)},
			To:                spatial.Position{X: 10, Y: float64(y)},
			BlocksMovement:    true,
			BlocksLineOfSight: true,
		}})
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas: hexCanvas(),
			// Two regions on ONE canvas in ONE absolute frame: room-1 owns
			// columns 0-9, room-2 owns 10-19. They used to be Rooms with
			// their own Origins and their own local coordinates; the seam
			// between them is now a wall on the field rather than a boundary
			// belonging to a room, which is the same fact said by the layer
			// that actually owns it.
			Regions: []encounter.RegionInput{
				rectRegion("room-1", 0, 0, 10, 8),
				rectRegion("room-2", 10, 0, 10, 8),
			},
			Walls: seam,
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 9, Y: 4}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	return enc.ToData()
}

// probeSheet is the smallest legal participant. These tests are about the
// world, and the only thing read off a sheet here is that it can be loaded.
func probeSheet(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-1", Name: "Probe", Level: 1,
		ClassID: "barbarian", RaceID: "human",
		HitPoints: 10, MaxHitPoints: 10, ProficiencyBonus: 2,
	}
}

func runProbe(t *testing.T, world encounter.EncounterData, participants []Participant) *worldProbe {
	t.Helper()

	probe := &worldProbe{}
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight:        everyoneSeesTheWholeMap{},
		Roller:       dice.NewRoller(),
		World:        world,
		Participants: participants,
		Machine:      probe,
	})
	require.NoError(t, err)
	require.NotNil(t, out)

	return probe
}

// THE INSTALLED WORLD KNOWS ABOUT A WALL THIS PACKAGE NEVER REGISTERED, which
// is how a test can tell "the composition's map" from "a room built here".
//
// The wall is authored on the encounter's field and compiled onto its canvas.
// Nothing in this package places an occluder, registers a boundary, or knows
// that walls exist — so if the world an interaction runs in were assembled
// here, it would be a bare grid and this sightline would be clear. It is
// blocked, and the only way it can be is that what was installed IS the map.
//
// That is not a hypothetical regression. The room this package used to build
// had no walls in it, for as long as it existed, and nothing noticed because
// the rules reading it ask about distance rather than about line of sight. The
// first one to ask would have been told nothing blocks.
func TestTheInstalledWorldKnowsAboutWallsThisPackageNeverBuilt(t *testing.T) {
	probe := runProbe(t, walledWorld(t), []Participant{{Character: probeSheet(heroID)}})
	require.True(t, probe.found, "a world was installed")

	// ASKED IN THE ROOM'S OWN FRAME, not in the authored one. These used to be
	// the literal authored cells, and they were the same numbers: rooms were
	// local and square. The canvas is one absolute pointy-top hex field now, so
	// what is authored as offset column 9 of row 4 is not the position the room
	// reports back — and a test that hardcodes one frame while the composition
	// speaks the other passes or fails for reasons that have nothing to do with
	// walls. Compare rpg-toolkit#1150.
	west, ok := probe.room.GetEntityPosition(heroID)
	require.True(t, ok, "the hero is on the installed world")
	east := spatial.Position{X: west.X + 1, Y: west.Y}

	require.Equal(t, float64(1), probe.room.GetGrid().Distance(west, east),
		"the two cells are adjacent, so nothing but a wall can separate them")
	require.True(t, probe.room.IsLineOfSightBlocked(west, east),
		"the seam wall is on the installed world")
}

// A WORLD FOR EVERY INTERACTION, including the ones that do not want one.
//
// A saving throw needs no geometry, and that used to be one of two reasons this
// package installed nothing. The other was rpg-toolkit#1090. Collapsing them
// cost the game every range predicate whenever the party split up, so they are
// told apart here: the world is installed even when the cast has nothing to do
// with it, and a rule that cannot find somebody on it answers "nobody knows
// where these two are standing" rather than "there is no world".
func TestAWorldIsInstalledForEveryInteraction(t *testing.T) {
	probe := runProbe(t, walledWorld(t), []Participant{{Character: probeSheet("a-stranger")}})

	require.True(t, probe.found, "a world is installed for every interaction")
	require.NotNil(t, probe.room)

	_, placed := probe.room.GetEntityPosition("a-stranger")
	require.False(t, placed, "and somebody who is not in this world is simply not on it")
}

// THE INSTALLED WORLD IS READ-ONLY, and this package hands it over as it was
// given rather than unwrapping it.
//
// The composition refuses a write through the view it hands out, by name. What
// this pins is that the refusal survives the trip: an effect running inside an
// interaction cannot move a member behind the encounter's back, because the
// thing in its context is the same guarded view the composition produced.
func TestTheInstalledWorldRefusesToBeWrittenTo(t *testing.T) {
	probe := runProbe(t, walledWorld(t), []Participant{{Character: probeSheet(heroID)}})

	before, ok := probe.room.GetEntityPosition(heroID)
	require.True(t, ok)

	err := probe.room.MoveEntity(heroID, spatial.Position{X: 1, Y: 1})
	require.ErrorIs(t, err, encounter.ErrReadOnly)

	// UNCHANGED, not merely "not where we aimed". The authored cell used to be
	// written here as a literal; reading it back first says the same thing
	// without asserting which coordinate frame the room answers in, and says
	// the stronger half — that the refusal left the position alone — rather
	// than only that the move did not land.
	after, ok := probe.room.GetEntityPosition(heroID)
	require.True(t, ok)
	require.Equal(t, before, after, "and the hero did not move")
}

// TestNoCodePathProducesARoomlessInteraction is the STRUCTURAL half, and it is
// deliberately not a behavioural one.
//
// A test can only demonstrate that the interactions it happened to run got a
// world. The claim rpg-toolkit#1090 needs is stronger — that no input CAN
// produce one without — and the shape of the old defect is exactly what a
// behavioural suite misses: the roomless path was reachable only when the cast
// spanned two rooms, which no test in this package did, for two releases.
//
// So the source is the assertion. The install is one statement at the top level
// of the door's body, and a condition wrapped around it is the defect, by
// construction: whatever the condition said, there would be inputs it answered
// no for. Reintroducing one fails here whether or not anybody writes a scene
// that reaches it.
//
// It reads [installTruth] rather than resolveOn because that is where the
// install moved when the door was extracted. That the door is reached at all —
// the half this test cannot see from inside it — is
// TestOnlyTheDoorInstallsGameContext's.
func TestNoCodePathProducesARoomlessInteraction(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, doorFile, nil, 0)
	require.NoError(t, err)

	fn := funcDecl(t, doorFile, file, "installTruth")

	installs := installsOf(fn, "WithRoom")
	require.Len(t, installs, 1, "the world is installed in exactly one place")

	requireUnconditional(t, fset, fn, installs[0],
		"installs the world inside a condition — some input answers no, and that is "+
			"rpg-toolkit#1090")
}

// funcDecl finds a top-level function by name in filename's parsed source, and
// says so if it has been renamed out from under the pin that asked for it.
func funcDecl(t *testing.T, filename string, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}

	t.Fatalf("%s has no function %q — if it was renamed, this pin must follow it", filename, name)

	return nil
}
