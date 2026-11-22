// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// state_outcomes_test.go is THE READ-BACK SAYS WHAT A STATE GIVES
// (rpg-project#989, rpg-project#1104): a behaviour definition's `outcomes` is
// one `routine.outcome` statement per consequence, in the routine's group, at
// the author's path, traced as any note is.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// TestStateOutcomesAreSaid: the machine site's `watch` routine gives a note on
// entering `alert`, which only a bump (`barred`) steps into.
func TestStateOutcomesAreSaid(t *testing.T) {
	raw, err := os.ReadFile("testdata/machine-site.yaml")
	require.NoError(t, err)
	const table = "    table:\n      post:\n"
	require.Equal(t, 1, strings.Count(string(raw), table))
	source := strings.Replace(string(raw), table, "    outcomes:\n      alert:\n        - give:\n"+
		"            topic: caught-lying\n            to: itself\n            about: the_one_named\n"+table, 1)
	rb, err := readback.Of([]byte(source))
	require.NoError(t, err)

	const path = "behaviorDefinitions.watch.outcomes.alert[0]"
	var said *readback.Statement
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineOutcome && st.Path == path {
			require.Nil(t, said, "said once")
			st := st
			said = &st
		}
	}
	require.NotNil(t, said, "the outcome is said at the author's path")
	require.Equal(t, readback.KindRoutineOutcome, said.Kind)
	require.Equal(t, readback.Subject{Kind: "routine", ID: "watch"}, said.About, "in the routine's group")
	require.True(t, strings.HasPrefix(said.Sentence, "Entering `alert`, it notes `caught-lying` about the one who walked into "),
		said.Sentence)

	require.Equal(t, []string{"caught-lying"}, connsOf(*said, "topic", "grants"))
	require.Equal(t, []string{"the_one_named"}, connsOf(*said, "who", "about"))
	require.Equal(t, []string{"itself"}, connsOf(*said, "who", "takes"))
	require.Equal(t, []string{"g"}, connsOf(*said, "creature", "may-note"), "the creature whose body runs it")
	require.Equal(t, []string{"alert"}, connsOf(*said, "state", "enters"), "the state it is entered at")

	for _, un := range rb.NotYetSaid {
		require.NotContains(t, un.Path, "outcomes", "an outcome is said, never left unsaid: %v", un)
	}
}
