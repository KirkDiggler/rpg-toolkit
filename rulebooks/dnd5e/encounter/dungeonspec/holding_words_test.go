// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// holding_words_test.go is THE HOLDING WORDS (rpg-project#1005, ruled on
// rpg-project#987): what a creature holds about a person is asked with
// `holds` and given with `give`, carrying a thing is `carries`, a state's
// post is `post`, and a machine asks about anybody with `holds.anyone:`.
// Every old word is refused when content is loaded, in a sentence naming
// the new one — and `holds` naming a thing, the key's old meaning, is
// pointed at `carries`.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// goblinLine is one creature line on the goblin, with the rest of an option
// written by the caller.
func goblinLine(rest string) string {
	return optionsOnGoblin("          - { id: hi, say: \"Hi.\", " + rest + " }\n")
}

// requireLoadRefused asserts the source is refused at load, in the sentence.
func requireLoadRefused(t *testing.T, source, sentence string) {
	t.Helper()
	_, err := dungeonspec.Load([]byte(source))
	require.Error(t, err)
	require.Contains(t, err.Error(), sentence)
}

const goblinAt = "room.room.monsterBindings.goblin-1.options"

func TestHoldingWords_AnOldWordIsRefusedNamingTheNewOne(t *testing.T) {
	t.Run("recognized on a line is now holds", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("requires: { recognized: welcome }"),
			"`recognized` is now `holds`: write { holds: <holding> }")
	})
	t.Run("recognized under a not is now holds", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("requires: { not: { recognized: welcome } }"),
			"`recognized` is now `holds`")
	})
	t.Run("recognize on a line's row is now give", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("success: [{ say: Fine., recognize: { topic: welcome, holder: spoken_to, about: speaker } }]"),
			"`recognize` is now `give`: write give: { holding: <holding>, to: spoken_to, "+
				"about: party_in_sight|speaker, heard_by: carriers_who_witnessed }")
	})
	t.Run("a give's topic is now holding", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("success: [{ say: Fine., give: { topic: welcome, to: spoken_to, about: speaker } }]"),
			"`topic` is now `holding`: write give: { holding: <holding>, to, about }")
	})
	t.Run("a give's holder is now to", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("success: [{ say: Fine., give: { holding: welcome, holder: spoken_to, about: speaker } }]"),
			"`holder` is now `to`: write give: { holding, to: spoken_to, about }")
	})
	t.Run("a state's holds is now post", func(t *testing.T) {
		requireLoadRefused(t, v4With("", "    monsterBindings:\n      goblin-1:\n        states:\n          initial: posted\n"+
			"          posted:\n            holds: { door: front-door }\n"),
			"a state's `holds` is now `post`: write post: { door: <door id>, at, against, pass }")
	})
}

func TestHoldingWords_AnInteractionsRecognizeIsNowGive(t *testing.T) {
	requireLoadRefused(t, castleWith(t, theOutcome, strings.Replace(strings.Replace(theOutcome,
		"- give:", "- recognize:", 1), "holding:", "topic:", 1)),
		"`recognize` is now `give`")
}

func TestHoldingWords_AGuardsTopicIsNowHoldsAnyone(t *testing.T) {
	errs := refusals(t, castleWith(t, "                  name: holds.anyone:admitted\n", "                  name: topic:admitted\n"))
	requireExactDefect(t, errs, "behaviorDefinitions.captain-policy.recipe.states.ready.transitions[2].when",
		"`topic:admitted` is now `holds.anyone:admitted`: a machine asks whether the creature holds \"admitted\" "+
			"about anyone with `holds.anyone:<holding>`")
}

func TestHoldingWords_ASequenceBodysTopicIsNowHoldsAnyone(t *testing.T) {
	raw, err := os.ReadFile("testdata/behavior-patrol-consumer.yaml")
	require.NoError(t, err)
	const step = "- {on: time, do: [{pick: move}]}"
	require.Equal(t, 1, strings.Count(string(raw), step))
	source := strings.Replace(string(raw), step,
		"- {on: time, when: {fact: {name: \"topic:welcome\", op: eq, value: true}}, do: [{pick: move}]}", 1)
	requireRefusedAt(t, refusals(t, source), "behaviorDefinitions.patrol.body",
		"`topic:welcome` is now `holds.anyone:welcome`")
}

func TestHoldingWords_HoldsAsksAboutAPersonNotAThing(t *testing.T) {
	t.Run("a prop named by holds is carried", func(t *testing.T) {
		requireExactDefect(t, refusals(t, goblinLine("requires: { holds: table }")), goblinAt+"[0].requires.holds",
			"`holds` asks what the creature spoken to holds about the speaker, and \"table\" is a prop: "+
				"a speaker carrying it is `carries: table`")
	})
	t.Run("an id that is both a prop and a holding reads two ways", func(t *testing.T) {
		source := optionsOnGoblin("          - { id: hi, say: \"Hi.\", success: [{ say: Fine., " +
			"give: { holding: table, to: spoken_to, about: speaker } }] }\n" +
			"          - { id: again, say: \"Again.\", requires: { holds: table } }\n")
		requireExactDefect(t, refusals(t, source), goblinAt+"[1].requires.holds",
			"\"table\" is both a prop and a holding something gives, so `holds: table` reads two ways: "+
				"the creature spoken to holds \"table\" about the speaker, or (the old meaning) the speaker carries "+
				"the prop \"table\" — write `carries: table` for the prop, or give the holding a name no prop has")
	})
	t.Run("a holding nothing gives", func(t *testing.T) {
		requireExactDefect(t, refusals(t, goblinLine("requires: { holds: welcome }")), goblinAt+"[0].requires.holds",
			"\"welcome\" is a holding no interaction's `outcomes` and no line's row gives, "+
				"so nobody could ever hold it about anyone")
	})
	t.Run("a holding given on the goblin's own line is asked for", func(t *testing.T) {
		source := optionsOnGoblin("          - { id: hi, say: \"Hi.\", success: [{ say: Fine., " +
			"give: { holding: welcome, to: spoken_to, about: speaker } }] }\n" +
			"          - { id: again, say: \"Again.\", requires: { holds: welcome } }\n")
		require.Empty(t, refusals(t, source))
	})
}

func TestHoldingWords_APropHoldsNothingAboutAnybody(t *testing.T) {
	const at = "room.room.propBindings.table.options[0]"
	propLine := func(rest string) string {
		return v4With("", "    propBindings:\n      table:\n        options:\n"+
			"          - { id: look, do: You look., "+rest+" }\n")
	}
	t.Run("asking what it holds", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, propLine("requires: { holds: welcome }")), at+".requires.holds",
			"a prop holds nothing about anybody, so a line said to one cannot ask what it holds")
	})
	t.Run("a prop's line naming a prop by holds is pointed at carries", func(t *testing.T) {
		// Legal under the old meaning (the speaker carries the table), so
		// the author is told the new word, not that a prop holds nothing.
		requireExactDefect(t, refusals(t, propLine("requires: { holds: table }")), at+".requires.holds",
			"`holds` asks what the creature spoken to holds about the speaker, and \"table\" is a prop: "+
				"a speaker carrying it is `carries: table`")
	})
	t.Run("giving it a holding", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, propLine("success: [{ give: { holding: welcome, to: spoken_to, about: speaker } }]")),
			at+".success[0].give", "a prop holds nothing about anybody, so a line said to one cannot give it a holding")
	})
}
