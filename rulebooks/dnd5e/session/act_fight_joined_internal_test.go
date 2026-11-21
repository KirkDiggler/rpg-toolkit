// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
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
