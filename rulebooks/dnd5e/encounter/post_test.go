// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// post_test.go pins A POST HOLDS A PASSAGE (rpg-project#852, S1b of the
// guard holds the gate; design R5): a state may name a post, the door whose
// opening it holds and the cells a holder stands on. While a member is in
// that state, up and on a post cell, the door's crossings are closed to
// every mover not allied with him, open or shut, at any width; the stop
// names him. Internal, because the machine blob, the participation read and
// the walk's door pass are the composition's own.

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The hold hall's doors: a footprint leaf lying between rows 0 and 1 across
// columns 3–5, and an edge door standing in two of the same crossings.
const (
	holdLeaf  = DoorID("hall/gate")
	holdEdge  = DoorID("hall/edge-gate")
	holdAlarm = FactID("alarm")
)

// The hold hall's cells. The leaf's crossings are (3,1)-(4,0), (4,0)-(4,1)
// and (4,1)-(5,0) (the precondition in [holdHall] checks it): the north
// side is the gatehouse, where the guard stands; the south side is where
// the party comes from.
var (
	holdNorthWest = spatial.Position{X: 4, Y: 0}
	holdNorthEast = spatial.Position{X: 5, Y: 0}
	holdSouthWest = spatial.Position{X: 3, Y: 1}
	holdSouthEast = spatial.Position{X: 4, Y: 1}
	holdBeside    = spatial.Position{X: 5, Y: 1}
	holdOffPost   = spatial.Position{X: 8, Y: 0}
)

// holdLeafDoor is the footprint leaf in a state.
func holdLeafDoor(state DoorState) DoorInput {
	return DoorInput{ID: holdLeaf, State: state, Placement: &spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 6}},
		Origin:    spatial.Point{X: 21.25, Y: 2.165},
	}}
}

// holdEdgeDoor is an edge door in two of the leaf's crossings.
func holdEdgeDoor(state DoorState) DoorInput {
	return DoorInput{ID: holdEdge, State: state, Edges: []DoorEdge{
		{From: holdNorthWest, To: holdSouthEast},
		{From: holdSouthEast, To: holdNorthEast},
	}}
}

// holdRecipe is a gate post's states: posted, standing on time, alert on
// the alarm, cowed on gate-cowed. Posted and alert hold; cowed holds
// nothing.
func holdRecipe(t *testing.T) *machine.Recipe {
	t.Helper()
	fact := func(f FactID) *machine.Guard {
		return &machine.Guard{Fact: &machine.FactLeaf{Name: MachineFact(string(f)), Op: machine.OpEq, Value: true}}
	}

	return recipe(t, "posted", map[string]machine.State{
		"posted": {Transitions: []machine.Transition{
			{On: beatFact, When: fact(postedCowed), To: "cowed", Reason: "cowed"},
			{On: beatFact, When: fact(holdAlarm), To: "alert", Reason: "alarm"},
			pickTime,
		}},
		"alert": {Transitions: []machine.Transition{
			{On: beatFact, When: fact(postedCowed), To: "cowed", Reason: "cowed"},
			pickTime,
		}},
		"cowed": {Transitions: []machine.Transition{pickTime}},
	})
}

// holder is a garrison guard on a cell, posted and alert on the door at
// the cells given.
func holder(t *testing.T, id MemberID, at spatial.Position, door DoorID, cells ...spatial.Position) MemberInput {
	t.Helper()
	post := Post{Door: door, Cells: cells}

	return MemberInput{
		ID: id, Kind: KindMonster, Name: "Gate guard", Faction: postedGarrison,
		Position: at, Table: holds(), Machine: holdRecipe(t),
		BlockingStates: []string{"posted", "alert"},
		Posts:          map[string]Post{"posted": post, "alert": post},
	}
}

// holdSetup is the posted hall with the door given and alice on the south
// side at the cell given.
func holdSetup(sight Sight, door DoorInput, alice spatial.Position, members ...MemberInput) *SetupInput {
	in := postedSetup(sight, members...)
	in.Field.Regions = []RegionInput{rectRegion("hall", 0, 0, 10, 6)}
	in.Field.Doors = []DoorInput{door}
	in.Members[0].Position = alice

	return in
}

// holdHall opens [holdSetup] with everyone seeing everything, and checks
// the leaf's crossings are the ones this file names.
func holdHall(t *testing.T, door DoorInput, alice spatial.Position, members ...MemberInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(holdSetup(everyoneSeesTheWholeMap{}, door, alice, members...))
	require.NoError(t, err)
	if d := enc.doorsByID[holdLeaf]; d != nil {
		require.ElementsMatch(t, [][2]spatial.Position{
			{holdSouthWest, holdNorthWest}, {holdNorthWest, holdSouthEast}, {holdSouthEast, holdNorthEast},
		}, enc.field.doorCrossings(d), "precondition: the leaf lies across the crossings this file names")
	}

	return enc
}

// heldStop steps alice and returns the stop's public reason, requiring the
// step was refused by a body and moved nobody.
func heldStop(t *testing.T, enc *Encounter, to spatial.Position) string {
	t.Helper()
	before, _ := enc.canvas.GetEntityPosition("alice")
	_, err := enc.Step(&StepInput{Member: "alice", To: to})
	require.ErrorIs(t, err, ErrBodyInTheWay, "a held crossing stops the step as a body does")
	require.NotErrorIs(t, err, ErrDoorShut, "and never as the door")
	var stop *StepObstructedError
	require.ErrorAs(t, err, &stop, "the walk stops; it does not break")
	after, _ := enc.canvas.GetEntityPosition("alice")
	require.Equal(t, before, after, "the refused step moved nobody")

	return stop.PublicReason()
}

// steps requires alice's step to land.
func steps(t *testing.T, enc *Encounter, to spatial.Position, msg string) {
	t.Helper()
	_, err := enc.Step(&StepInput{Member: "alice", To: to})
	require.NoError(t, err, msg)
	at, _ := enc.canvas.GetEntityPosition("alice")
	require.Equal(t, to, at, msg)
}

// placeAt moves a member on the canvas, as any walk or push does.
func placeAt(t *testing.T, enc *Encounter, id MemberID, cell spatial.Position) {
	t.Helper()
	_, err := enc.moveMember(enc.members[id], cell)
	require.NoError(t, err)
}

func TestAPostHoldsBothCellsOfAFootprintOpening(t *testing.T) {
	for _, tc := range []struct {
		name          string
		guard, beside spatial.Position
	}{
		{"on the west cell, the east cell is held", holdNorthWest, holdNorthEast},
		{"on the east cell, the west cell is held", holdNorthEast, holdNorthWest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
				holder(t, postedGuard, tc.guard, holdLeaf, holdNorthWest, holdNorthEast))

			require.Equal(t, "Gate guard bars the way", heldStop(t, enc, tc.beside),
				"the cell beside him is held, by his name")
			_, err := enc.Step(&StepInput{Member: "alice", To: tc.guard})
			require.ErrorIs(t, err, ErrBodyInTheWay, "his own cell is refused as before")
		})
	}
}

func TestAPostHoldsAnEdgeDoor(t *testing.T) {
	enc := holdHall(t, holdEdgeDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdEdge, holdNorthWest))

	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "the door's other edge is held")
	steps(t, enc, holdBeside, "a crossing that is not the door's is open floor")
	steps(t, enc, holdNorthEast, "and round the door's edges, the far side is reached")
}

// TestAHoldWorksOnAnOpenAndAShutDoor: the guard stands on the party's side
// of the door, as the castle's does, so a shut door does not hide him.
func TestAHoldWorksOnAnOpenAndAShutDoor(t *testing.T) {
	for name, state := range map[string]DoorState{"open": DoorIsOpen(), "shut": DoorIsClosed()} {
		t.Run(name, func(t *testing.T) {
			for _, door := range []DoorInput{holdLeafDoor(state), holdEdgeDoor(state)} {
				id := door.ID
				enc := holdHall(t, door, holdSouthEast, holder(t, postedGuard, holdSouthWest, id, holdSouthWest))
				require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "%s %s: held, by his name", name, id)
			}
		})
	}
}

func TestAWalkNeverOpensAHeldDoor(t *testing.T) {
	walk := func(t *testing.T, enc *Encounter) {
		t.Helper()
		alice := enc.members["alice"]
		_, err := enc.walkPath(context.Background(), "alice", alice, []spatial.Position{holdNorthEast}, nil, 0, core.Ref{}, "", false)
		require.NoError(t, err)
	}
	door := func(t *testing.T, enc *Encounter) DoorStateKind {
		t.Helper()
		return enc.doorsByID[holdLeaf].state.Kind()
	}

	t.Run("held, the walk leaves the door shut and stops", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast,
			holder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest))
		walk(t, enc)
		require.Equal(t, DoorClosed, door(t, enc), "a walk never opens a held door")
		at, _ := enc.canvas.GetEntityPosition("alice")
		require.Equal(t, holdSouthEast, at, "and stops before it")
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "the stop names the holder, not the door")
	})

	t.Run("not held, the same walk opens it", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsClosed()), holdSouthEast,
			holder(t, postedGuard, holdOffPost, holdLeaf, holdSouthWest))
		walk(t, enc)
		require.Equal(t, DoorOpen, door(t, enc), "a walk opens a door nobody holds")
		at, _ := enc.canvas.GetEntityPosition("alice")
		require.Equal(t, holdNorthEast, at, "and crosses")
	})
}

func TestAnAllyCrossesAHeldOpening(t *testing.T) {
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: holdSouthEast, Table: holds()}
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdBeside,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest), porter)
	_, err := enc.Step(&StepInput{Member: "porter", To: holdNorthEast})
	require.NoError(t, err, "his own side crosses the opening he holds")
	require.Equal(t, "Gate guard bars the way", func() string {
		placeAt(t, enc, "alice", holdSouthEast)
		return heldStop(t, enc, holdNorthEast)
	}(), "and the party does not")
}

func TestTheHoldIsAStateAtAPlace(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdOffPost, holdLeaf, holdNorthWest))

	steps(t, enc, holdNorthEast, "posted but off his cell, he holds nothing")
	placeAt(t, enc, "alice", holdSouthEast)

	placeAt(t, enc, postedGuard, holdNorthWest)
	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "back on his cell, held in the same step")

	enc.members[postedGuard].Machine.State = "cowed"
	steps(t, enc, holdNorthEast, "on his cell in a state that holds nothing, the opening passes")
}

func TestAnyLeavingEmptiesTheHold(t *testing.T) {
	t.Run("pushed by a directive", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
			holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "precondition: held")
		out, err := enc.Direct(context.Background(), DirectInput{
			Mover: postedGuard, Cause: core.Ref{Module: "test", Type: "effects", ID: "shove"},
			Route: []spatial.Position{{X: 3, Y: 0}, {X: 2, Y: 0}}, Kind: MoveDisplaced,
		})
		require.NoError(t, err)
		require.Positive(t, out.Moved, "precondition: the push moved him")
		steps(t, enc, holdNorthEast, "pushed off his post, he holds nothing")
	})

	t.Run("his own walk, as every row that walks him walks", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
			holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "precondition: held")
		guard := enc.members[postedGuard]
		_, err := enc.walkPath(context.Background(), postedGuard, guard,
			[]spatial.Position{{X: 3, Y: 0}, {X: 2, Y: 0}}, nil, 0, core.Ref{Module: "test", Type: "sounds", ID: "voice"}, "", false)
		require.NoError(t, err)
		at, _ := enc.canvas.GetEntityPosition(string(postedGuard))
		require.NotEqual(t, holdNorthWest, at, "precondition: he walked off")
		steps(t, enc, holdNorthEast, "walked off his post, he holds nothing")
	})
}

func TestAnAlertGuardStillHolds(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
	require.NoError(t, enc.learnFact(postedGuard, holdAlarm, BeatAnswered, uint64(enc.clock.ToData().HighWater)))
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "alert", stateOf(enc, postedGuard), "precondition: the alarm made him alert")

	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "alert on his post, he still holds")
}

func TestAnOccupiedPostCellIsNotRetaken(t *testing.T) {
	stranger := MemberInput{ID: "stranger", Kind: KindPlayer, Position: holdNorthWest}

	t.Run("one post cell, taken: he holds nothing", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
			holder(t, postedGuard, holdOffPost, holdLeaf, holdNorthWest), stranger)
		_, holds := enc.memberHolds(postedGuard, nil)
		require.False(t, holds, "nobody stands on his one post cell but the stranger")
		steps(t, enc, holdNorthEast, "and the opening passes")
	})

	t.Run("two post cells: he holds from the other", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
			holder(t, postedGuard, holdNorthEast, holdLeaf, holdNorthWest, holdNorthEast), stranger)
		door, holds := enc.memberHolds(postedGuard, nil)
		require.True(t, holds, "he stands on his other post cell")
		require.Equal(t, holdLeaf, door)
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthWest), "the crossing into the stranger's cell is still his")
	})
}

func TestADownedHolderHoldsNothing(t *testing.T) {
	down := postedDown{}
	in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
	in.Participation = down
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "precondition: standing, he holds")

	down[postedGuard] = true
	steps(t, enc, holdNorthEast, "down, he holds nothing")
}

func TestTheGroupHoldsTheGate(t *testing.T) {
	down := postedDown{}
	second := holder(t, "gate-2", holdNorthEast, holdLeaf, holdNorthEast)
	in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsOpen()), holdSouthWest,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest), second)
	in.Participation = down
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	down[postedGuard] = true
	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthWest),
		"one down, the other still holds the opening, even across the downed one's cell")
	_, err = enc.Step(&StepInput{Member: "alice", To: holdSouthEast})
	require.NoError(t, err)
	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthWest), "from either side of the opening")

	down["gate-2"] = true
	steps(t, enc, holdNorthWest, "both down, the opening passes")
}

func TestARouteGoesRoundAHeldOpening(t *testing.T) {
	route := func(t *testing.T, enc *Encounter) []spatial.Position {
		t.Helper()
		q, err := enc.cellQuery()
		require.NoError(t, err)
		path, ok, err := q.routeWithin("alice", holdSouthEast, holdNorthEast, 0)
		require.NoError(t, err)
		require.True(t, ok, "the open hall has a way")

		return path
	}

	open := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdOffPost, holdLeaf, holdNorthWest))
	require.Equal(t, []spatial.Position{holdNorthEast}, route(t, open), "unheld, the route goes straight through")

	held := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
	require.Equal(t, []spatial.Position{holdBeside, holdNorthEast}, route(t, held), "held, the route goes round the opening")
}

func TestADrivenWalkStopsSilentlyAtAHeldEdge(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
	_, stepped, err := enc.stepTo(enc.members["alice"], holdNorthEast)
	require.NoError(t, err, "a driven walk swallows the hold's refusal")
	require.False(t, stepped, "and stops")
}

func TestAnUnseenHolderIsSomeone(t *testing.T) {
	enc, err := NewEncounter(holdSetup(wakeBlind{}, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest)))
	require.NoError(t, err)
	require.Equal(t, "someone bars the way", heldStop(t, enc, holdNorthEast))
}

func TestWhatAnObserverHoldsOfTheHold(t *testing.T) {
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: holdBeside, Table: holds()}
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest), porter)
	seen := func(viewer MemberID) map[MemberID]DoorID {
		t.Helper()
		held, err := enc.ObservedHolds(&ViewInput{Member: viewer})
		require.NoError(t, err)
		return held
	}

	require.Equal(t, holdLeaf, seen("alice")[postedGuard], "alice sees him bar the gate against her")
	require.NotContains(t, seen("porter"), postedGuard, "an ally is not barred, and is told nothing")

	cowGuard(t, enc)
	require.NotContains(t, seen("alice"), postedGuard, "cowed, she sees him hold nothing, in the round he stood aside")
}

func TestTheObservedHoldFollowsHimOffHisPost(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
	guard := enc.members[postedGuard]
	_, err := enc.walkPath(context.Background(), postedGuard, guard,
		[]spatial.Position{{X: 3, Y: 0}}, nil, 0, core.Ref{Module: "test", Type: "sounds", ID: "voice"}, "", false)
	require.NoError(t, err)
	_, _, err = enc.refreshSight(nil)
	require.NoError(t, err)
	held, err := enc.ObservedHolds(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.NotContains(t, held, postedGuard, "seen off his post, he is seen to hold nothing")
}

func TestAPostIsRefusedAtTheDoor(t *testing.T) {
	for name, edit := range map[string]func(*MemberInput){
		"a state the recipe lacks": func(m *MemberInput) { m.Posts["asleep"] = Post{Door: holdLeaf} },
		"a door the field lacks":   func(m *MemberInput) { m.Posts["posted"] = Post{Door: "hall/postern"} },
		"a cell that is not floor": func(m *MemberInput) {
			m.Posts["posted"] = Post{Door: holdLeaf, Cells: []spatial.Position{{X: 40, Y: 40}}}
		},
		"a post with no recipe": func(m *MemberInput) { m.Machine, m.BlockingStates = nil, nil },
	} {
		t.Run(name, func(t *testing.T) {
			guard := holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest)
			edit(&guard)
			_, err := NewEncounter(holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsOpen()), holdSouthEast, guard))
			require.ErrorIs(t, err, ErrBadMember)
		})
	}

	t.Run("and at the join door", func(t *testing.T) {
		enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast)
		_, err := enc.Join(&JoinInput{
			Member: postedGuard, Kind: KindMonster, Faction: postedGarrison, Cell: holdNorthWest,
			Table: holds(), Machine: holdRecipe(t), Posts: map[string]Post{"posted": {Door: "hall/postern"}},
		})
		require.ErrorIs(t, err, ErrBadMember)
	})
}

func TestAPostWithNoCellsIsHeldWhereHeIsPlaced(t *testing.T) {
	guard := holder(t, postedGuard, holdNorthWest, holdLeaf)
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, guard)
	require.Equal(t, []spatial.Position{holdNorthWest}, enc.members[postedGuard].Posts["posted"].Cells,
		"no cells means the cell he is placed at")
	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast))

	joined := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast)
	_, err := joined.Join(&JoinInput{
		Member: postedGuard, Kind: KindMonster, Name: "Gate guard", Faction: postedGarrison, Cell: holdNorthWest,
		Table: holds(), Machine: holdRecipe(t), Posts: map[string]Post{"posted": {Door: holdLeaf}},
	})
	require.NoError(t, err)
	require.Equal(t, "Gate guard bars the way", heldStop(t, joined, holdNorthEast), "a joiner holds where it joins")
}

func TestAStoredPostLoads(t *testing.T) {
	waiting := holder(t, "relief", spatial.Position{X: 7, Y: 3}, holdLeaf, holdNorthEast)
	waiting.Arrives = TriggerRound{Round: 99}

	saved := func(t *testing.T) []byte {
		t.Helper()
		raw, err := json.Marshal(holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
			holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest), waiting).ToData())
		require.NoError(t, err)

		return raw
	}
	load := func(raw []byte) (*Encounter, error) {
		var data EncounterData
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}

		return LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: postedSetup(everyoneSeesTheWholeMap{}).Capabilities})
	}
	// edit rewrites the stored posts of every member and reserve that has
	// them: nil drops the key, as a record from before posts existed.
	edit := func(t *testing.T, raw []byte, posts any) []byte {
		t.Helper()
		var doc map[string]any
		require.NoError(t, json.Unmarshal(raw, &doc))
		for _, list := range []string{"members", "reserve"} {
			rows, _ := doc[list].([]any)
			for _, row := range rows {
				m := row.(map[string]any)
				if _, ok := m["posts"]; !ok {
					continue
				}
				if posts == nil {
					delete(m, "posts")
				} else {
					m["posts"] = posts
				}
			}
		}
		out, err := json.Marshal(doc)
		require.NoError(t, err)

		return out
	}

	t.Run("a member and a reserve keep their posts", func(t *testing.T) {
		enc, err := load(saved(t))
		require.NoError(t, err)
		data := enc.ToData()
		require.True(t, slices.ContainsFunc(data.Members, func(m MemberData) bool {
			return m.ID == postedGuard && m.Posts["alert"].Door == holdLeaf &&
				slices.Equal(m.Posts["alert"].Cells, []spatial.Position{holdNorthWest})
		}), "the member keeps its posts")
		require.True(t, slices.ContainsFunc(data.Reserve, func(r ReserveData) bool {
			return r.ID == "relief" && r.Posts["posted"].Door == holdLeaf
		}), "the reserve keeps its posts too")
		require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "and the loaded guard still holds")
	})

	t.Run("a record without them loads as none", func(t *testing.T) {
		enc, err := load(edit(t, saved(t), nil))
		require.NoError(t, err)
		_, holds := enc.memberHolds(postedGuard, nil)
		require.False(t, holds, "no posts: a body that holds its cell only")
	})

	for name, posts := range map[string]any{
		"an unknown state":     map[string]any{"asleep": map[string]any{"door": string(holdLeaf), "cells": []any{map[string]any{"x": 4, "y": 0}}}},
		"an unknown door":      map[string]any{"posted": map[string]any{"door": "hall/postern", "cells": []any{map[string]any{"x": 4, "y": 0}}}},
		"a cell off the floor": map[string]any{"posted": map[string]any{"door": string(holdLeaf), "cells": []any{map[string]any{"x": 40, "y": 40}}}},
		"no cells":             map[string]any{"posted": map[string]any{"door": string(holdLeaf)}},
	} {
		t.Run(name+" is refused", func(t *testing.T) {
			_, err := load(edit(t, saved(t), posts))
			require.ErrorIs(t, err, ErrInvalidData)
		})
	}
}

func TestTheAtlasListsAFootprintDoorsCrossings(t *testing.T) {
	in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	in.Field.Doors = append(in.Field.Doors, DoorInput{ID: "hall/side", State: DoorIsClosed(),
		Edges: []DoorEdge{{From: spatial.Position{X: 7, Y: 0}, To: spatial.Position{X: 8, Y: 0}}}})
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	atlas, err := enc.Atlas()
	require.NoError(t, err)
	var crossings []AtlasDoorway
	for _, dw := range atlas.Doorways {
		switch dw.Door {
		case holdLeaf:
			require.True(t, dw.Crossing, "a footprint door's doorway is marked a crossing")
			crossings = append(crossings, AtlasDoorway{Door: dw.Door, From: dw.From, To: dw.To, Crossing: true})
		case "hall/side":
			require.False(t, dw.Crossing, "an edge door's doorway is not")
		}
	}
	require.ElementsMatch(t, []AtlasDoorway{
		{Door: holdLeaf, From: holdSouthWest, To: holdNorthWest, Crossing: true},
		{Door: holdLeaf, From: holdNorthWest, To: holdSouthEast, Crossing: true},
		{Door: holdLeaf, From: holdSouthEast, To: holdNorthEast, Crossing: true},
	}, normalizedDoorways(crossings), "every crossing the leaf lies across")
	require.True(t, slices.ContainsFunc(atlas.Doorways, func(dw AtlasDoorway) bool { return dw.Door == "hall/side" }),
		"precondition: the edge door is listed")
}

func TestAHiddenFootprintDoorsCrossingsAreWithheld(t *testing.T) {
	in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	in.Field.Concealments = []ConcealmentInput{{ID: "secret-gate", Checks: []CheckApproach{{Ability: "perception", DC: 30}}, Doors: []DoorID{holdLeaf}}}
	in.CheckResolver = holdNeverFinds{}
	in.Witness = nobodyPerceives{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	full, err := enc.Atlas()
	require.NoError(t, err)
	require.True(t, slices.ContainsFunc(full.Doorways, func(dw AtlasDoorway) bool { return dw.Door == holdLeaf }),
		"precondition: the author's atlas lists the hidden door's crossings")
	mine, err := enc.AtlasFor("alice")
	require.NoError(t, err)
	require.False(t, slices.ContainsFunc(mine.Doorways, func(dw AtlasDoorway) bool { return dw.Door == holdLeaf }),
		"withheld from a member who has not found it, as an edge door's are")
}

// holdNeverFinds fails every check, so a hidden door stays hidden.
type holdNeverFinds struct{}

func (holdNeverFinds) ResolveCheck(in *ResolveCheckInput) (*ResolveCheckOutput, error) {
	return &ResolveCheckOutput{Applied: in.Approaches[0]}, nil
}

func (holdNeverFinds) ResolveDiscoveryCheck(in *ResolveCheckInput) (*ResolveCheckOutput, error) {
	return &ResolveCheckOutput{Applied: in.Approaches[0]}, nil
}

// normalizedDoorways orders each doorway's two cells as the atlas does.
func normalizedDoorways(in []AtlasDoorway) []AtlasDoorway {
	out := make([]AtlasDoorway, 0, len(in))
	for _, dw := range in {
		e := normalizeDoorEdge(DoorEdge{From: dw.From, To: dw.To})
		out = append(out, AtlasDoorway{Door: dw.Door, From: e.From, To: e.To, Crossing: dw.Crossing})
	}

	return out
}

// TestARevealedFootprintDoorBringsItsCrossings: a footprint door revealed
// under fog reaches the map with the crossings it closes, marked, on the
// door's own reveal and on a room reveal alike, so a planner reading the
// patch does not walk through it (rpg-project#852, H6).
func TestARevealedFootprintDoorBringsItsCrossings(t *testing.T) {
	in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsClosed()), holdSouthEast)
	in.Field.Concealments = []ConcealmentInput{{ID: "secret-gate", Checks: []CheckApproach{{Ability: "perception", DC: 30}}, Doors: []DoorID{holdLeaf}}}
	in.CheckResolver = holdNeverFinds{}
	in.Witness = nobodyPerceives{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	before, err := enc.AtlasFor("alice")
	require.NoError(t, err)

	require.NoError(t, enc.revealConcealmentTo("alice", enc.field.concealmentOf("secret-gate"), "test", 0))
	require.NoError(t, enc.appendCellsRevealedBeat("alice", before))

	// crossingsIn is every doorway of the leaf a payload's list carries,
	// requiring each is marked a crossing.
	crossingsIn := func(t *testing.T, list any) int {
		t.Helper()
		n := 0
		rows, _ := list.([]any)
		for _, row := range rows {
			dw := row.(map[string]any)
			if door, named := dw["door"]; named && door != string(holdLeaf) {
				continue
			}
			require.Equal(t, true, dw["crossing"], "a footprint door's doorway is marked a crossing")
			n++
		}
		return n
	}
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	var onTheDoor, onTheRoom int
	for _, entry := range story {
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		if doors, ok := b["doors"].([]any); ok {
			for _, d := range doors {
				door := d.(map[string]any)
				if door["door"] == string(holdLeaf) {
					onTheDoor += crossingsIn(t, door["doorways"])
				}
			}
		}
		if b["beat"] == BeatCellsRevealed {
			onTheRoom += crossingsIn(t, b["doorways"])
		}
	}
	require.Positive(t, onTheDoor, "the door's own reveal lists its crossings")
	require.Positive(t, onTheRoom, "and so does the room reveal")
}

// TestAHoldThatDoesNotBlockIsStillSeenToEnd: a post whose states hold the
// passage without blocking the body changes nothing about his blocking when
// he stands aside, and the observers still see the hold end in that round.
func TestAHoldThatDoesNotBlockIsStillSeenToEnd(t *testing.T) {
	guard := holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest)
	guard.BlockingStates = nil
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast, guard)
	held, err := enc.ObservedHolds(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, holdLeaf, held[postedGuard], "precondition: alice sees him hold the gate")

	cowGuard(t, enc)

	held, err = enc.ObservedHolds(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.NotContains(t, held, postedGuard, "cowed, she sees the hold end, though his body never blocked")
}

// TestTheHoldIsNotDeliveredInASighting: the stored sighting carries the door
// a seen holder holds, and the sighting a host relays to a client does not.
// Whom a hold bars is a fact about a viewer, told by [Encounter.ObservedHolds].
func TestTheHoldIsNotDeliveredInASighting(t *testing.T) {
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
	payloadOf := func(holdings []perception.Holding) map[string]any {
		t.Helper()
		for _, h := range holdings {
			if h.Subject == postedGuard && h.Channel == perception.Sight {
				var fields map[string]any
				require.NoError(t, json.Unmarshal(h.Payload, &fields))
				return fields
			}
		}
		require.FailNow(t, "alice holds a sighting of the guard")
		return nil
	}

	stored, err := enc.storedView(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, string(holdLeaf), payloadOf(stored)["holds_door"], "precondition: the stored sighting holds the door")

	delivered, err := enc.View(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.NotContains(t, payloadOf(delivered), "holds_door", "the delivered sighting does not")
}

// TestAHoldOnADoorNotFoundIsNotTold: a seen holder of a door the viewer has
// not found is not told as holding it; once the viewer finds the door, he is.
func TestAHoldOnADoorNotFoundIsNotTold(t *testing.T) {
	in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsClosed()), holdSouthEast,
		holder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest))
	in.Field.Concealments = []ConcealmentInput{{ID: "secret-gate", Checks: []CheckApproach{{Ability: "perception", DC: 30}}, Doors: []DoorID{holdLeaf}}}
	in.CheckResolver = holdNeverFinds{}
	in.Witness = nobodyPerceives{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	require.True(t, enc.hiddenDoorTo("alice", holdLeaf), "precondition: alice has not found the door")
	door, holds := enc.memberHolds(postedGuard, nil)
	require.True(t, holds, "precondition: he holds it")
	require.Equal(t, holdLeaf, door)

	held, err := enc.ObservedHolds(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.NotContains(t, held, postedGuard, "a door she has not found is not named to her")

	require.NoError(t, enc.revealConcealmentTo("alice", enc.field.concealmentOf("secret-gate"), "test", 0))
	held, err = enc.ObservedHolds(&ViewInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, holdLeaf, held[postedGuard], "found, it is")
}

// TestAPushStopsBeforeAHeldCrossing: a directive's walk is cut before a held
// crossing, as before a shut door, so a push never leaves the mover on a cell
// it may only pass through.
func TestAPushStopsBeforeAHeldCrossing(t *testing.T) {
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: holdSouthEast, Table: holds()}
	enc := holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthWest,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest), porter)
	require.Equal(t, PassagePassThrough, passageAt(t, enc, holdSouthEast), "precondition: the porter's cell is passed, not stood on")

	out, err := enc.Direct(context.Background(), DirectInput{
		Mover: "alice", Cause: core.Ref{Module: "test", Type: "effects", ID: "shove"},
		Route: []spatial.Position{holdSouthEast, holdNorthEast}, Kind: MoveDisplaced,
	})
	require.NoError(t, err)
	require.Zero(t, out.Moved, "the push is cut before the held crossing, back to where she may stand")
	at, _ := enc.canvas.GetEntityPosition("alice")
	require.Equal(t, holdSouthWest, at, "she stays where she stood")
}

// holdFlakyDown is a participation whose read fails while fail is set.
type holdFlakyDown struct{ fail *bool }

func (d holdFlakyDown) Standing([]MemberID) ([]MemberID, error) { return nil, nil }

func (d holdFlakyDown) Assess(members []MemberID) (*ParticipationAssessment, error) {
	if *d.fail {
		return nil, errors.New("participation is down")
	}

	return testAssessmentFromDown(members, nil), nil
}

// TestAFailedStandingReadHolds: a hold read with no participation in hand
// asks for one, and a read that fails holds: a world that cannot say who
// is down does not open a gate on a guess.
func TestAFailedStandingReadHolds(t *testing.T) {
	fail := false
	in := holdSetup(everyoneSeesTheWholeMap{}, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdNorthWest, holdLeaf, holdNorthWest))
	in.Participation = holdFlakyDown{fail: &fail}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	fail = true
	door, holds := enc.memberHolds(postedGuard, nil)
	require.True(t, holds, "the failed read holds")
	require.Equal(t, holdLeaf, door)
}
