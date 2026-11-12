// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// speech_doorway_internal_test.go is a voice in a doorway (rpg-project#933):
// the 5x5 hall split by a footprint leaf across it. The partition pass shuts
// every door, so the cells the leaf stands on — (2,1) and (2,3) — are each a
// partition of their own; west of the leaf is one room, east the other.

const (
	doorwayLeaf DoorID   = "leaf"
	doorwayIn   MemberID = "in"   // stands in the doorway, at (2,1)
	doorwayWest MemberID = "west" // at (0,0)
	doorwayEast MemberID = "east" // at (4,0)
)

// doorwayHall opens the hall with the leaf in a state and its three members.
func doorwayHall(t *testing.T, state DoorState) *Encounter {
	t.Helper()
	leaf := spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 40, D: 1}},
		Origin:    spatial.Point{X: 12.5, Y: 8.66},
	}
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("hall", 0, 0, 5, 5)},
			Doors:   []DoorInput{{ID: doorwayLeaf, Placement: &leaf, State: state}},
		},
		Members: []MemberInput{
			{ID: doorwayIn, Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: doorwayWest, Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
			{ID: doorwayEast, Kind: KindPlayer, Position: spatial.Position{X: 4, Y: 0}},
		},
		Endings: []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight: everyoneSeesTheWholeMap{}, Equipment: UnobservedEquipment{}, Conditions: UnobservedEquipment{},
			Sheets: zeroSheets{}, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
			Actors: Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)
	parts, err := enc.field.partitionsNow()
	require.NoError(t, err)
	in := parts.of[enc.field.cellAt(spatial.Position{X: 2, Y: 1})]
	require.Len(t, parts.cells[in], 1, "precondition: the doorway is a partition of its own")
	west := parts.of[enc.field.cellAt(spatial.Position{X: 0, Y: 0})]
	east := parts.of[enc.field.cellAt(spatial.Position{X: 4, Y: 0})]
	require.NotEqual(t, west, east, "precondition: the leaf divides the hall into two rooms")
	return enc
}

// caught is who caught an utterance's words.
func caught(t *testing.T, enc *Encounter, u utterance) map[MemberID]bool {
	t.Helper()
	all, err := enc.perceive(u)
	require.NoError(t, err)
	out := map[MemberID]bool{}
	for _, f := range all {
		if f.words {
			out[f.member] = true
		}
	}
	return out
}

// TestAVoiceInAnOpenDoorwayIsInBothRooms: standing in an open doorway is
// standing in the rooms it joins — said there, a spoken line reaches both;
// said in either room, it reaches the one in the doorway. A doorway is no
// way through: a spoken line in the west room stays out of the east one.
func TestAVoiceInAnOpenDoorwayIsInBothRooms(t *testing.T) {
	enc := doorwayHall(t, DoorIsOpen())

	from := caught(t, enc, utterance{speaker: doorwayIn, words: "Over here."})
	require.True(t, from[doorwayWest], "the west room hears the doorway")
	require.True(t, from[doorwayEast], "and so does the east")

	west := caught(t, enc, utterance{speaker: doorwayWest, words: "Who's there?"})
	require.True(t, west[doorwayIn], "the doorway hears the west room")
	require.False(t, west[doorwayEast], "but a spoken line does not cross into the east room")
}

// TestAShutDoorwayIsNoRoom: with the leaf shut the doorway joins nothing at
// no cost — a spoken line from a body standing in it reaches nobody else.
func TestAShutDoorwayIsNoRoom(t *testing.T) {
	enc := doorwayHall(t, DoorIsClosed())
	from := caught(t, enc, utterance{speaker: doorwayIn, words: "Over here."})
	require.False(t, from[doorwayWest])
	require.False(t, from[doorwayEast])
}
