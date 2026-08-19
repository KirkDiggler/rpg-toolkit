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

	var seam []spatial.Boundary
	for y := 0; y < 8; y++ {
		for _, dy := range []int{-1, 0, 1} {
			to := y + dy
			if to < 0 || to >= 8 {
				continue
			}
			seam = append(seam, spatial.Boundary{
				From:              spatial.Position{X: 9, Y: float64(y)},
				To:                spatial.Position{X: 10, Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-1", Width: 10, Height: 8, Boundaries: seam},
				{ID: "room-2", Width: 10, Height: 8, Origin: spatial.Position{X: 10}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 9, Y: 4}},
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
		Initiative: orderAsGiven{}, Standing: everyoneStanding{},
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

	west := spatial.Position{X: 9, Y: 4}
	east := spatial.Position{X: 10, Y: 4}

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

	err := probe.room.MoveEntity(heroID, spatial.Position{X: 1, Y: 1})
	require.ErrorIs(t, err, encounter.ErrReadOnly)

	at, ok := probe.room.GetEntityPosition(heroID)
	require.True(t, ok)
	require.Equal(t, spatial.Position{X: 9, Y: 4}, at, "and the hero did not move")
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
// of resolveOn's body, and a condition wrapped around it is the defect, by
// construction: whatever the condition said, there would be inputs it answered
// no for. Reintroducing one fails here whether or not anybody writes a scene
// that reaches it.
func TestNoCodePathProducesARoomlessInteraction(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "resolve.go", nil, 0)
	require.NoError(t, err)

	fn := funcDecl(t, file, "resolveOn")

	var installs []token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithRoom" {
			installs = append(installs, call.Pos())
		}

		return true
	})
	require.Len(t, installs, 1, "the world is installed in exactly one place")

	ast.Inspect(fn, func(n ast.Node) bool {
		branch, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		require.False(t, branch.Pos() <= installs[0] && installs[0] < branch.End(),
			"resolve.go:%d installs the world inside a condition — some input answers no, "+
				"and that is rpg-toolkit#1090",
			fset.Position(installs[0]).Line)

		return true
	})
}

// funcDecl finds a top-level function by name, and says so if it has been
// renamed out from under the pin above.
func funcDecl(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}

	t.Fatalf("resolve.go has no function %q — if it was renamed, this pin must follow it", name)

	return nil
}
