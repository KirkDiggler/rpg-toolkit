// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

// monster_facing_test.go is the single-room dialect's half of the author's
// own words on rpg-toolkit#1899: a monster's start is a cell WITH a facing,
// and the word is presentation — carried verbatim to the compiled dungeon,
// never turned into an angle, never snapped to the nearest compass point, and
// never read as anything but "which way the model is turned at spawn".

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMonsterStartingFacing drives one monster's authored direction through
// the real path: the YAML document, the single-room decoder, and the compile.
func TestMonsterStartingFacing(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(t, err)

	// author writes one facing onto the fixture's skeleton and loads the
	// marshalled document back through the public entry point.
	author := func(facing string) ([]MonsterPlacement, error) {
		decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		require.NoError(t, err)
		spec := decoded.Spec
		require.Len(t, spec.Room.Gameplay.Monsters, 1)
		spec.Room.Gameplay.Monsters[0].StartingCell.Facing = facing
		compiled, err := Load(mustEncode(t, spec))
		if err != nil {
			return nil, err
		}
		return compiled.Monsters, nil
	}

	t.Run("a facing word reaches the compiled monster verbatim", func(t *testing.T) {
		monsters, err := author("se")
		require.NoError(t, err)
		require.Len(t, monsters, 1)
		require.Equal(t, "se", monsters[0].Facing)
	})

	t.Run("silence is the asset's own default, not a gap", func(t *testing.T) {
		monsters, err := author("")
		require.NoError(t, err)
		require.Len(t, monsters, 1)
		require.Empty(t, monsters[0].Facing)
	})

	t.Run("a word outside the eight is refused by name", func(t *testing.T) {
		_, err := author("northeast")
		require.Error(t, err)

		var validation *ValidationError
		require.ErrorAs(t, err, &validation)
		require.NotEmpty(t, validation.Errors)
		require.Equal(t, "room.room.monsters[0].startingCell.facing", validation.Errors[0].Path)
		require.Contains(t, validation.Errors[0].Message, "northeast",
			"the refusal quotes the word the author wrote")
		require.Contains(t, validation.Errors[0].Message, "n|ne|e|se|s|sw|w|nw",
			"and lists the eight that exist")
	})
}

// TestTheCellRenameRefusesTheOldSpelling guards the rename itself: the bare
// `cell:` an author used to write is no longer a key this build reads, so an
// old document is told the new word rather than having its start silently
// dropped.
func TestTheCellRenameRefusesTheOldSpelling(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(t, err)

	swapped := swapOneIn(t, raw, "startingCell: { location: { q: 2, r: 0 } }", "cell: {q: 2, r: 0}")

	_, err = Load(swapped)
	require.Error(t, err)
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)

	var msgs []string
	for _, e := range validation.Errors {
		msgs = append(msgs, e.Message)
	}
	require.Contains(t, msgs,
		`"cell" is not a key this build reads: they are faction, id, ref, startingCell`,
		"the old spelling is refused and the replacement is offered")
}
