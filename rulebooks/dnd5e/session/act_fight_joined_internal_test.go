// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const joinRoll = `{"member":"ogre","roll":9,"total":10,"calculation":{"components":[` +
	`{"source":{"ref":"dnd5e:initiative:roll","name":"Initiative","source_id":"ogre"},"dice":{"notation":"1d20","die_size":20,"original_rolls":[9],"final_rolls":[9],"subtotal":9}},` +
	`{"source":{"ref":"dnd5e:abilities:dex","name":"DEX","source_id":"ogre"},"modifier":1}],"total":10}}`

// Catches a join folding to nothing, or losing its roll, its round or the
// round of its first turn on the way to the act stream.
func TestClockPiecesFoldsAFightJoinWithItsRoll(t *testing.T) {
	payload := []byte(`{"member":"ogre","to":"turn","round":3,"first_round":4,"rolls":[` + joinRoll + `]}`)

	pieces, err := clockPieces(encounter.BeatTransferred, payload)

	require.NoError(t, err)
	require.Len(t, pieces, 1)
	entry, ok := pieces[0].body.(ClockEntry)
	require.True(t, ok)
	require.Equal(t, ClockFightJoined, entry.Mark)
	require.NotNil(t, entry.Joined)
	require.Equal(t, "ogre", entry.Joined.Member.ID)
	require.Equal(t, 3, entry.Joined.Round)
	require.Equal(t, 4, entry.Joined.FirstRound)
	require.NotNil(t, entry.Joined.Roll)
	require.Equal(t, "ogre", entry.Joined.Roll.Member.ID)
	require.Equal(t, 10, entry.Joined.Roll.Roll.Total)
}

// Catches a placed joiner being given an invented roll.
func TestClockPiecesFoldsAPlacedFightJoinWithNoRoll(t *testing.T) {
	payload := []byte(`{"member":"ogre","to":"turn","round":3,"first_round":3}`)

	pieces, err := clockPieces(encounter.BeatTransferred, payload)

	require.NoError(t, err)
	require.Len(t, pieces, 1)
	entry := pieces[0].body.(ClockEntry)
	require.Equal(t, ClockFightJoined, entry.Mark)
	require.Nil(t, entry.Joined.Roll)
}

// Catches a creature leaving the fight reading as a join.
func TestClockPiecesFoldsALeaveFromTheFightToNothing(t *testing.T) {
	payload := []byte(`{"member":"ogre","to":"world"}`)

	pieces, err := clockPieces(encounter.BeatTransferred, payload)

	require.NoError(t, err)
	require.Empty(t, pieces)
}

// Catches a join entry quietly dropping a roll whose calculation does not
// replay, or taking more than one roll for one joiner.
func TestClockPiecesRefusesAJoinRollThatDoesNotReplay(t *testing.T) {
	bad := []byte(`{"member":"ogre","to":"turn","round":3,"first_round":4,"rolls":[` +
		`{"member":"ogre","roll":9,"total":10,"calculation":{"components":"not a list","total":3}}]}`)
	pieces, err := clockPieces(encounter.BeatTransferred, bad)
	require.Error(t, err)
	require.Nil(t, pieces)

	two := []byte(`{"member":"ogre","to":"turn","round":3,"first_round":4,"rolls":[` + joinRoll + `,` + joinRoll + `]}`)
	pieces, err = clockPieces(encounter.BeatTransferred, two)
	require.Error(t, err)
	require.Nil(t, pieces)
}

// Catches a join that reaches the act stream as nothing, or whose roll is not
// the die the joiner was dealt plus its own Dexterity: alice walks into sight
// of the goblin while bob and the goblin are already fighting, and the stream
// must carry her roll, the round she joined and the round of her first turn.
func TestAWalkerJoiningARunningFightShowsItsInitiativeRoll(t *testing.T) {
	occluders := make([]encounter.PropInput, 0, 7)
	blocks, passes := true, false
	for y := 0; y < 8; y++ {
		if y == 3 {
			continue // the gap
		}
		occluders = append(occluders, encounter.PropInput{Ref: "pillar", At: spatial.Position{X: 3, Y: float64(y)},
			BlocksMovement: &passes, BlocksLineOfSight: &blocks})
	}
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	characters := &strikeCharacters{byID: map[string]*character.Data{
		"alice": strikeFixtureFighter("alice"), "bob": strikeFixtureFighter("bob"),
	}}
	stream := &stabilizationStream{}
	// bob and the goblin roll at launch (sorted: bob, ogre); alice's die is the third.
	dice := &scriptedDice{rolls: []int{10, 12, 4}}
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: stream, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: dice, TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters, Characters: characters,
	})
	require.NoError(t, err)

	cell := func(col, row int) spatial.Position {
		return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
	}
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}, Props: occluders},
		Party:    []sceneSeat{seatAt("alice", 1, 0), seatAt("bob", 1, 3)},
		Monsters: []dungeonspec.MonsterPlacement{monsterAt("ogre", refs.Monsters.Goblin().String(), 5, 3)},
	})
	_, err = mgr.Move(context.Background(), &MoveInput{Session: "sess", Member: "alice",
		Path: []spatial.Position{cell(1, 1), cell(1, 2), cell(1, 3), cell(1, 4)}})
	require.NoError(t, err)

	var joined *FightJoined
	for _, act := range stream.acts {
		if entry, ok := act.Body.(ClockEntry); ok && entry.Mark == ClockFightJoined && act.Recipient == "alice" {
			joined = entry.Joined
		}
	}
	require.NotNil(t, joined, "the act stream carries the join")
	require.Equal(t, "alice", joined.Member.ID)
	require.NotNil(t, joined.Roll)
	require.Equal(t, []int{4}, joined.Roll.Roll.Dice)
	require.Equal(t, 4+2, joined.Roll.Roll.Total, "the scripted face plus Dexterity 14")
	require.Positive(t, joined.Round)
	require.GreaterOrEqual(t, joined.FirstRound, joined.Round)
}
