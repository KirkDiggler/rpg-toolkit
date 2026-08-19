// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// walledField is two chambers side by side with a real wall on the seam and a
// pillar in the eastern one.
//
// room-1 owns absolute columns 0-9, room-2 owns 10-19, and the wall is the
// edges between them. It is built the way a wall has to be built on a square
// grid — every edge, diagonals included, because spatial registers a boundary
// between ANY two adjacent cells and a same-row-only wall has a hole at each
// corner (rpg-toolkit#1106's testwalls lesson).
//
// Room-2 is then divided again by a column of OCCLUDER cells at absolute x=14,
// which is a different primitive: a wall is an edge, an occluder is a cell that
// is in the way. A FULL column rather than a single pillar, and that is a
// measured requirement rather than caution — spatial v0.9.1 lets a viewer lean
// around a one-cell obstruction, so a lone pillar blocks nothing and a fixture
// built on one would pass whether or not this file placed occluders at all.
// (It did, and mutation M3 caught it.)
func walledField() encounter.FieldInput {
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

	return encounter.FieldInput{
		Rooms: []encounter.RoomInput{
			{ID: "room-1", Width: 10, Height: 8, Boundaries: seam},
			{
				ID: "room-2", Width: 10, Height: 8,
				Origin:    spatial.Position{X: 10},
				Occluders: column(4, 8),
			},
		},
	}
}

// loadWalledWorld sets a walled field up, persists it, and loads it back the
// way Resolve does — so what the test measures is the world a resolution
// actually holds, not a construction-time one.
func loadWalledWorld(t *testing.T, members []encounter.MemberInput) *encounter.Encounter {
	t.Helper()

	built, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight:   everyoneSeesTheWholeMap{},
		Field:   walledField(),
		Members: members,
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: built.ToData(), Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{},
	})
	require.NoError(t, err)

	return loaded
}

// column is a room-local column of cells, the shape an occluding wall has to
// have to actually occlude.
func column(x, height int) []spatial.Position {
	out := make([]spatial.Position, 0, height)
	for y := 0; y < height; y++ {
		out = append(out, spatial.Position{X: float64(x), Y: float64(y)})
	}

	return out
}

// walledCast is a member in every interesting place: two either side of the
// seam wall on the same row, two more with the occluder column between them,
// and one far corner.
//
// east-behind and west-far sit on the field's LAST column and LAST row —
// absolute (19,4) and (0,7) — deliberately. The canvas has to span the whole
// field for line of sight to be traced the way the encounter traces it, and a
// canvas one cell short is a defect nobody standing in the middle can feel.
// Somebody has to be standing at the edge. (Mutation M4; the fixture did not
// have anybody there until it caught that.)
func walledCast() []encounter.MemberInput {
	return []encounter.MemberInput{
		{ID: "west-near", Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 9, Y: 4}},
		{ID: "east-near", Kind: encounter.KindMonster, Room: "room-2", Position: spatial.Position{X: 0, Y: 4}},
		{ID: "east-behind", Kind: encounter.KindMonster, Room: "room-2", Position: spatial.Position{X: 9, Y: 4}},
		{ID: "east-beside", Kind: encounter.KindMonster, Room: "room-2", Position: spatial.Position{X: 6, Y: 0}},
		{ID: "west-far", Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 7}},
	}
}

// TestTheInstalledWorldAgreesWithTheEncounter is the anti-divergence device,
// and it exists because the thing it checks used to be a PROMISE.
//
// This file carried its own copy of the encounter's grid construction with a
// comment saying it "tracks encounter.buildRoomGrid rather than choosing for
// itself" — and nothing whatsoever kept that true. What is left of the copy
// after rpg-toolkit#1114 is the span and the family; everything else is read
// off the encounter's own absolute reports. This checks the remainder against
// the encounter rather than asserting it, on the two questions a predicate can
// actually ask a room:
//
//   - WHERE somebody is — every member's cell, compared against
//     [encounter.Encounter.Members];
//   - WHAT can be seen — every ordered pair, compared against the sightings the
//     encounter itself computed on its own canvas, through its own walls.
//
// The second is the sharp one. A sighting exists in the encounter exactly when
// the subject is within reach AND the ray is unblocked (rebuildPercepts), and
// the fixtures give everybody unlimited reach, so a sighting IS "the geometry
// admits it". A canvas of the wrong span, the wrong family, or with a wall
// missing answers differently on some pair of these five, and this fails.
func TestTheInstalledWorldAgreesWithTheEncounter(t *testing.T) {
	cast := walledCast()
	enc := loadWalledWorld(t, cast)

	room, err := interactionRoom(enc, castOf(cast))
	require.NoError(t, err)
	require.NotNil(t, room)

	members, err := enc.Members()
	require.NoError(t, err)
	require.Len(t, members, len(cast))

	// WHERE: the installed world puts everybody exactly where the encounter
	// says they are, in the encounter's own absolute frame.
	for _, m := range members {
		at, placed := room.GetEntityPosition(string(m.ID))
		require.True(t, placed, "%s is on the installed world", m.ID)
		require.Equal(t, m.Position, at, "%s stands where the encounter says", m.ID)
	}

	// WHAT CAN BE SEEN: pair by pair, against the encounter's own answer.
	blockedPairs, clearPairs := 0, 0
	for _, observer := range members {
		seen := sightedBy(t, enc, observer.ID)

		for _, subject := range members {
			if subject.ID == observer.ID {
				continue
			}

			blocked := room.IsLineOfSightBlocked(observer.Position, subject.Position)
			require.Equal(t, !blocked, seen[subject.ID],
				"the installed world and the encounter disagree about whether %s can see %s",
				observer.ID, subject.ID)

			if blocked {
				blockedPairs++
			} else {
				clearPairs++
			}
		}
	}

	// A fixture where everything is visible would agree with anything. Both
	// answers have to be represented for the agreement above to mean something.
	require.Positive(t, blockedPairs, "the wall and the pillar block somebody")
	require.Positive(t, clearPairs, "and somebody can still see somebody")
}

// sightedBy is who this observer currently holds a sighting of.
func sightedBy(t *testing.T, enc *encounter.Encounter, observer encounter.MemberID) map[encounter.MemberID]bool {
	t.Helper()

	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(t, err)

	out := map[encounter.MemberID]bool{}
	for _, h := range holdings {
		if h.Status == intel.Current {
			out[encounter.MemberID(h.Subject)] = true
		}
	}

	return out
}

// TestAWorldIsInstalledEvenWhenNobodyIsOnIt is the unconditional half of
// rpg-toolkit#1114, at the one seam where "unconditional" can be observed.
//
// A saving throw needs no geometry, and this used to be one of two reasons the
// package installed nothing. The other was the bug. Collapsing them cost the
// game every range predicate whenever the party split up, so the two are told
// apart here instead: the world is installed, and it is simply empty — which
// is what makes a predicate answer "nobody knows where these two are standing"
// rather than "there is no world".
func TestAWorldIsInstalledEvenWhenNobodyIsOnIt(t *testing.T) {
	enc := loadWalledWorld(t, walledCast())

	room, err := interactionRoom(enc, []Participant{{Monster: &monster.Data{ID: "a-stranger"}}})
	require.NoError(t, err)
	require.NotNil(t, room, "a world is installed for every interaction")

	_, placed := room.GetEntityPosition("a-stranger")
	require.False(t, placed, "and a stranger to this world is simply not on it")

	for _, m := range walledCast() {
		_, onIt := room.GetEntityPosition(string(m.ID))
		require.False(t, onIt, "%s is not in the cast, so is not placed", m.ID)
	}
}

// TestNoCodePathProducesARoomlessInteraction is the STRUCTURAL half, and it is
// deliberately not a behavioural one.
//
// A test can only demonstrate that the worlds it happened to ask for were
// installed. The claim rpg-toolkit#1090 needs is stronger — that no input CAN
// produce a roomless interaction — and the shape of the old defect is exactly
// what a behavioural suite misses: `return nil, nil` was reachable only when
// the cast spanned rooms, which no test in this package did, for two releases.
//
// So the source itself is the assertion. Every return from interactionRoom
// either carries a world or carries an error; a bare `return nil, nil` is the
// defect, by name, and putting one back fails here whether or not anybody
// writes a scene that reaches it.
func TestNoCodePathProducesARoomlessInteraction(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "world.go", nil, 0)
	require.NoError(t, err)

	fn := funcDecl(t, file, "interactionRoom")

	returns := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		returns++
		require.Len(t, ret.Results, 2, "interactionRoom returns a world and an error")

		roomIsNil := isNilIdent(ret.Results[0])
		errIsNil := isNilIdent(ret.Results[1])
		require.False(t, roomIsNil && errIsNil,
			"world.go:%d returns no world and no error — that is rpg-toolkit#1090",
			fset.Position(ret.Pos()).Line)

		return true
	})

	require.Positive(t, returns, "interactionRoom was found and has returns")
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

	t.Fatalf("world.go has no function %q — if it was renamed, this pin must follow it", name)

	return nil
}

func isNilIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && ident.Name == "nil"
}

// castOf turns member IDs into participants. interactionRoom reads nothing off
// a sheet but its ID — this file tests geometry — so the sheets are bare.
func castOf(members []encounter.MemberInput) []Participant {
	out := make([]Participant, 0, len(members))
	for _, m := range members {
		out = append(out, Participant{Monster: &monster.Data{ID: string(m.ID)}})
	}

	return out
}
