// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// state_party_test.go is THE READ-BACK SAYS A STATE'S NOTE ABOUT THE PARTY
// (rpg-project#1134, slice rpg-project#1135): each note says the word its
// author wrote, in that word's label, and connects it as the who it is about.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

func TestStateParty_EachNoteSaysItsWordsLabel(t *testing.T) {
	both := "    outcomes:\n      back-to-gate:\n" +
		"        - give: { topic: standing, as: barred, to: itself, about: party_in_sight }\n" +
		"        - give: { topic: standing, as: wanted, to: itself, about: the_gone }\n"
	site := strings.Replace(valueSite(), valueOutcome, both, 1)
	require.NotEqual(t, valueSite(), site, "precondition: the outcome is replaced")
	rb, err := readback.Of([]byte(site))
	require.NoError(t, err)

	said := map[string]readback.Statement{}
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineOutcome {
			said[st.Path] = st
		}
	}
	const at = "behaviorDefinitions.captain-policy.outcomes.back-to-gate"
	seen, gone := said[at+"[0]"], said[at+"[1]"]
	require.NotEmpty(t, seen.Path, "the first note is said")
	require.NotEmpty(t, gone.Path, "the second note is said")

	require.Contains(t, seen.Sentence, "the party of the one the beat names that it sees")
	require.Contains(t, gone.Sentence, "the party of the one the beat names that it saw and does not see now")
	require.Equal(t, []string{"party_in_sight"}, connsOf(seen, "who", "about"))
	require.Equal(t, []string{"the_gone"}, connsOf(gone, "who", "about"))
}
