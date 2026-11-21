// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// heldRow is the hall guard's missed-check row as the castle writes it.
const heldRow = "                apply_status: { status: held, to: party_in_sight }\n"

// theHeldRowSaid reads the castle back with the hall guard's row rewritten and
// returns the statement for that row and the folded text, its wrapping undone.
func theHeldRowSaid(t *testing.T, row string) (readback.Statement, string) {
	t.Helper()
	raw, err := os.ReadFile(castlePath)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(raw), heldRow), "the castle gives held once")
	rb, err := readback.Of([]byte(strings.Replace(string(raw), heldRow, row, 1)))
	require.NoError(t, err)
	const path = "room.room.monsterBindings.hall-1.options[0].failure[0].apply_status"
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindLineGrant && st.Path == path {
			return st, strings.Join(strings.Fields(rb.FoldedText()), " ")
		}
	}
	require.FailNow(t, "the row that holds the party is not said")
	return readback.Statement{}, ""
}

// A STATUS GIVEN FOR ROUNDS SAYS SO (rpg-project#1080): the clause sits between
// the id's closing bracket and the full stop, and the folded form says "for N
// rounds". Rounds are world rounds: they pass only when someone acts or waits
// (Wait is rpg-project#1079).
func TestStatusUntilIsSaid(t *testing.T) {
	given, folded := theHeldRowSaid(t,
		"                apply_status: { status: held, to: party_in_sight, until: { rounds: 3 } }\n")
	require.Contains(t, given.Sentence, "is given the status “Held” (`held`) until 3 rounds of world time have passed, "+
		"or sooner if one of the status's own endings comes first. The rules apply it to them;")
	require.Contains(t, folded, "gives the speaker's side in its sight the status “Held” for 3 rounds")
}

// One round is "1 round", never "1 rounds".
func TestStatusUntilOneRoundIsSaidInTheSingular(t *testing.T) {
	given, folded := theHeldRowSaid(t,
		"                apply_status: { status: held, to: party_in_sight, until: { rounds: 1 } }\n")
	require.Contains(t, given.Sentence, "until 1 round of world time has passed, or sooner")
	require.NotContains(t, given.Sentence, "until 1 rounds")
	require.Contains(t, folded, "the status “Held” for 1 round")
	t.Logf("%q", folded)
	require.NotContains(t, folded, "for 1 rounds")
}

// Without `until` the sentence is the one that has always been said.
func TestStatusWithoutUntilIsSaidAsBefore(t *testing.T) {
	given, folded := theHeldRowSaid(t, heldRow)
	require.Contains(t, given.Sentence, "is given the status “Held” (`held`). The rules apply it to them;")
	require.NotContains(t, given.Sentence, "rounds of world time")
	require.Contains(t, folded, "gives the speaker's side in its sight the status “Held”")
	require.NotContains(t, folded, "the status “Held” for")
}
