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

// outcomeSite is the machine site with `watch` giving `first` on entering
// `alert`, run by `g`, and a second routine `watch2` (the same states, giving
// `second`) run by `g2` when second is set; with second empty `watch` is run
// by both `g` and `g2`.
func outcomeSite(t *testing.T, first, second string) string {
	raw, err := os.ReadFile("testdata/machine-site.yaml")
	require.NoError(t, err)
	src := string(raw)
	replace := func(old, repl string) {
		require.Equal(t, 1, strings.Count(src, old), old)
		src = strings.Replace(src, old, repl, 1)
	}
	outcomes := func(topic string) string {
		return "    outcomes:\n      alert:\n        - give:\n            topic: " + topic +
			"\n            to: itself\n            about: the_one_named\n"
	}
	replace("    table:\n      post:\n", outcomes(first)+"    table:\n      post:\n")
	replace("        startingCell: { location: { q: 2, r: 0 }, facing: e }\n",
		"        startingCell: { location: { q: 2, r: 0 }, facing: e }\n"+
			"      - id: g2\n        ref: dnd5e:monsters:guard\n        startingCell: { location: { q: 1, r: 0 }, facing: e }\n")
	use := "watch"
	if second != "" {
		use = "watch2"
	}
	replace("    monsterBindings:\n", "    monsterBindings:\n      g2:\n        faction: watch\n        behaviors:\n          - id: policy\n            use: "+
		use+"\n        behaviorState: policy\n")
	if second != "" {
		i := strings.Index(src, "behaviorDefinitions:\n  watch:\n")
		require.NotEqual(t, -1, i)
		def := strings.TrimPrefix(src[i:], "behaviorDefinitions:\n")
		def = strings.Replace(def, "  watch:\n", "  watch2:\n", 1)
		def = strings.Replace(def, "site:behaviors:watch", "site:behaviors:watch2", 1)
		def = strings.Replace(def, outcomes(first), outcomes(second), 1)
		src += def
	}
	return src
}

// TestAStatesNoteNamesOnlyTheCreaturesRunningItsRoutine: two routines, run by
// two creatures, each give a different topic on a state of the same name. Each
// statement names only its own runner as one who may note the topic.
func TestAStatesNoteNamesOnlyTheCreaturesRunningItsRoutine(t *testing.T) {
	rb, err := readback.Of([]byte(outcomeSite(t, "caught-lying", "other-topic")))
	require.NoError(t, err)

	byPath := map[string]readback.Statement{}
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineOutcome {
			byPath[st.Path] = st
		}
	}
	first := byPath["behaviorDefinitions.watch.outcomes.alert[0]"]
	second := byPath["behaviorDefinitions.watch2.outcomes.alert[0]"]
	require.Equal(t, "caught-lying", connsOf(first, "topic", "grants")[0])
	require.Equal(t, "other-topic", connsOf(second, "topic", "grants")[0])
	require.Equal(t, []string{"g"}, connsOf(first, "creature", "may-note"), "only the creature running `watch`")
	require.Equal(t, []string{"g2"}, connsOf(second, "creature", "may-note"), "only the creature running `watch2`")
}

// TestAStatesNoteNamesEveryCreatureRunningItsRoutine: one routine run by two
// creatures names both.
func TestAStatesNoteNamesEveryCreatureRunningItsRoutine(t *testing.T) {
	rb, err := readback.Of([]byte(outcomeSite(t, "caught-lying", "")))
	require.NoError(t, err)
	found := false
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineOutcome {
			found = true
			require.Equal(t, []string{"g", "g2"}, connsOf(st, "creature", "may-note"))
		}
	}
	require.True(t, found)
}

// TestTheFoldedRoutineSaysWhatAStateGives: the fold drops nothing, so the
// routine's folded text says, under its state, what entering it gives and
// about whom, and a routine that gives something else is not "word for word"
// the one that does not.
func TestTheFoldedRoutineSaysWhatAStateGives(t *testing.T) {
	rb, err := readback.Of([]byte(outcomeSite(t, "caught-lying", "other-topic")))
	require.NoError(t, err)
	folded := strings.Join(strings.Fields(rb.FoldedText()), " ")
	require.Contains(t, folded, "Entering `alert`, it notes `caught-lying` about the one who walked into")
	require.Contains(t, folded, "Entering `alert`, it notes `other-topic` about the one who walked into")
	require.NotContains(t, folded, "word for word those of", "the two routines give different topics")

	// every statement's path is carried by some folded entry
	carried := map[string]bool{}
	walkItems(rb.Folded(), func(it readback.Item) {
		for _, p := range it.Paths {
			carried[p] = true
		}
	})
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineOutcome {
			require.True(t, carried[st.Path], "outcome at %s has no folded entry", st.Path)
		}
	}
}
