// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// flood_memo_internal_test.go is the flood memo (rpg-toolkit#2097): a read
// floods a shared origin once however many creatures it routes to, and the
// memo dies with the read, so the next read after a step floods the ground as
// it now stands.
//
// White-box because the memo and its count are the read's own business:
// nothing outside the package can tell one flood from four but the clock.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// floodMemoEncounter is a goblin in an open crypt with the given players,
// everybody walking thirty feet, the goblin with a club. A sighted crypt lets
// everybody see everybody; a blind one lets nobody see anybody, and makes the
// goblins hostile to the party, so a player's cell is closed to the goblin
// without a fight forming.
func floodMemoEncounter(t *testing.T, sighted bool, goblinAt spatial.Position, players map[MemberID]spatial.Position) *Encounter {
	t.Helper()
	club := ActionView{Ref: core.Ref{Module: "test", Type: "actions", ID: "club"}, Name: "Club", RangeFeet: 5}
	field := FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("crypt", 0, 0, 12, 12)}}
	var sight Sight = everyoneSeesTheWholeMap{}
	goblin := MemberInput{ID: "goblin", Kind: KindMonster, Position: goblinAt}
	if !sighted {
		sight = nobodySees{}
		field.Factions = []FactionInput{{ID: "goblins"}}
		field.Dispositions = []DispositionInput{{Between: [2]FactionID{"goblins", FactionParty}, Stance: StanceHostile}}
		goblin.Faction = "goblins"
	}
	members := []MemberInput{goblin}
	sheets := sheetFacts{"goblin": {SpeedFeet: 30, Actions: []ActionView{club}}}
	for _, id := range []MemberID{"alice", "bob", "carol", "dave"} {
		at, ok := players[id]
		if !ok {
			continue
		}
		members = append(members, MemberInput{ID: id, Kind: KindPlayer, Position: at})
		sheets[id] = SheetFacts{SpeedFeet: 30}
	}
	enc, err := NewEncounter(&SetupInput{
		Field:   field,
		Members: members,
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:         sight,
			Equipment:     UnobservedEquipment{},
			Conditions:    UnobservedEquipment{},
			Sheets:        sheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc
}

// nobodySees is a pitch-dark crypt: everybody's sight reaches zero cells, so
// nobody sees anybody.
type nobodySees struct{}

func (nobodySees) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}

	return out, nil
}

// TestAViewFloodsItsOwnCellOnceForEveryCreatureItSees: a goblin that sees
// four players, none in its club's reach, routes to each and away from each.
// Those eight routes read two fields, the walk out from its cell and the one
// step away, and the view floods each once. Without the memo it floods
// eight times.
func TestAViewFloodsItsOwnCellOnceForEveryCreatureItSees(t *testing.T) {
	enc := floodMemoEncounter(t, true, spatial.Position{X: 1, Y: 1}, map[MemberID]spatial.Position{
		"alice": {X: 8, Y: 2}, "bob": {X: 8, Y: 5}, "carol": {X: 8, Y: 8}, "dave": {X: 5, Y: 9},
	})

	before := enc.floodsComputed
	view, err := enc.buildMonsterView(enc.members["goblin"], TurnBudget{AttacksLeft: 1, MovementFeet: 30}, 1)
	require.NoError(t, err)

	require.Len(t, view.Seen, 4, "precondition: the goblin sees all four")
	for _, seen := range view.Seen {
		require.False(t, seen.InReach[core.Ref{Module: "test", Type: "actions", ID: "club"}], "precondition: %s is out of the club's reach, so the goblin routes to them", seen.ID)
		require.NotEmpty(t, seen.Path, "the goblin has a route to %s", seen.ID)
		require.NotEmpty(t, seen.AwayPath, "and a step away from them")
	}
	require.Equal(t, 2, enc.floodsComputed-before,
		"one field for every route to them and one for every step away, each flooded once")
}

// TestTheMemoDiesWithTheRead: a read floods from the goblin's cell with bob
// standing on a cell, closed to the goblin while bob is up; bob steps off;
// the next read floods again and reaches the cell he left. A memo that outlived
// its read would hand the second read the first field, with the cell still
// closed.
func TestTheMemoDiesWithTheRead(t *testing.T) {
	enc := floodMemoEncounter(t, false, spatial.Position{X: 1, Y: 1}, map[MemberID]spatial.Position{"bob": {X: 6, Y: 6}})
	require.True(t, enc.opposed("goblin", "bob"), "precondition: the goblins are hostile to the party")
	own, err := enc.cellOf(enc.members["goblin"])
	require.NoError(t, err)
	bobFrom, err := enc.cellOf(enc.members["bob"])
	require.NoError(t, err)

	first, err := enc.cellQuery()
	require.NoError(t, err)
	field, err := first.floodFrom("goblin", "goblin", own, nil, 0, true)
	require.NoError(t, err)
	_, reached := field.Dist[bobFrom]
	require.False(t, reached, "precondition: the cell bob stands on is closed to the goblin")

	// Bob steps to a neighbouring cell the goblin's flood reached.
	var bobTo spatial.Position
	for _, n := range enc.canvas.GetGrid().GetNeighbors(bobFrom) {
		if _, ok := field.Dist[n]; ok {
			bobTo = n
			break
		}
	}
	require.Equal(t, 1.0, enc.Distance(bobFrom, bobTo), "precondition: bob has a neighbouring cell to step to")
	_, err = enc.Step(&StepInput{Member: "bob", To: bobTo, EndWalk: true})
	require.NoError(t, err)

	next, err := enc.cellQuery()
	require.NoError(t, err)
	before := enc.floodsComputed
	field, err = next.floodFrom("goblin", "goblin", own, nil, 0, true)
	require.NoError(t, err)
	require.Equal(t, 1, enc.floodsComputed-before, "the next read floods afresh")
	_, reached = field.Dist[bobFrom]
	require.True(t, reached, "the cell bob left is open to the goblin now")
	_, reached = field.Dist[bobTo]
	require.False(t, reached, "and the cell he stands on is closed")
}
