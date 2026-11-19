// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// noted_words_test.go is THE NOTED WORDS (rpg-project#1005, as ruled): what
// a creature has in mind about a person is a topic, asked with `noted` and
// given with `give`; carrying a prop is `carries`; no predicate says
// `holds` any more; a state's post is `post`; a machine asks about anybody
// with `noted.anyone:`. Every old word is
// refused when content is loaded, in a sentence naming the new one, and a
// word that lived only in an unreleased build is refused naming its key.

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

// givesWelcome is a goblin line whose success gives `welcome` about the
// speaker: a topic the document gives.
const givesWelcome = "          - { id: hi, say: \"Hi.\", success: [{ say: Fine., " +
	"give: { topic: welcome, to: spoken_to, about: speaker } }] }\n"

func TestNotedWords_AnOldWordIsRefusedNamingTheNewOne(t *testing.T) {
	t.Run("recognized on a line is now noted", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("requires: { recognized: welcome }"),
			"`recognized` is now `noted`: write { noted: <topic> }")
	})
	t.Run("recognized under a not is now noted", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("requires: { not: { recognized: welcome } }"), "`recognized` is now `noted`")
	})
	t.Run("recognize on a line's row is now give", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("success: [{ say: Fine., recognize: { topic: welcome, holder: spoken_to, about: speaker } }]"),
			"`recognize` is now `give`: write give: { topic: <topic>, to: spoken_to, "+
				"about: party_in_sight|speaker, heard_by: carriers_who_witnessed }")
	})
	t.Run("a give's holder is now to", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("success: [{ say: Fine., give: { topic: welcome, holder: spoken_to, about: speaker } }]"),
			"`holder` is now `to`: write give: { topic, to: spoken_to, about }")
	})
	t.Run("a state's holds is now post", func(t *testing.T) {
		requireLoadRefused(t, v4With("", "    monsterBindings:\n      goblin-1:\n        states:\n          initial: posted\n"+
			"          posted:\n            holds: { door: front-door }\n"),
			"a state's `holds` is now `post`: write post: { door: <door id>, at, against, pass }")
	})
}

// The words that lived only in v0.221 and v0.222 never reached a released
// api: each is refused naming its key and its line, nothing more.
func TestNotedWords_AnUnreleasedWordIsRefusedByName(t *testing.T) {
	t.Run("a give's holding", func(t *testing.T) {
		requireLoadRefused(t, goblinLine("success: [{ say: Fine., give: { holding: welcome, to: spoken_to, about: speaker } }]"),
			"options[0].success[0].give.holding: \"holding\" is not a key this build reads")
	})
	t.Run("a guard's holds.anyone", func(t *testing.T) {
		requireLoadRefused(t, castleWith(t, "                  name: noted.anyone:admitted\n", "                  name: holds.anyone:admitted\n"),
			"states.ready.transitions[2].when.fact: fact \"holds.anyone:admitted\" is not in the vocabulary")
	})
	t.Run("a guard's holds", func(t *testing.T) {
		requireLoadRefused(t, castleWith(t, "                      name: noted:admitted\n", "                      name: holds:admitted\n"),
			"states.in-hall.transitions[1].when.all[1].fact: fact \"holds:admitted\" is not in the vocabulary")
	})
}

func TestNotedWords_AnInteractionsRecognizeIsNowGive(t *testing.T) {
	requireLoadRefused(t, castleWith(t, theOutcome, strings.Replace(theOutcome, "- give:", "- recognize:", 1)),
		"`recognize` is now `give`")
}

func TestNotedWords_AGuardsTopicIsNowNotedAnyone(t *testing.T) {
	errs := refusals(t, castleWith(t, "                  name: noted.anyone:admitted\n", "                  name: topic:admitted\n"))
	requireExactDefect(t, errs, "behaviorDefinitions.captain-policy.recipe.states.ready.transitions[2].when",
		"`topic:admitted` is now `noted.anyone:admitted`: a machine asks whether the creature has noted \"admitted\" "+
			"about anyone with `noted.anyone:<topic>`")
}

func TestNotedWords_ASequenceBodysTopicIsNowNotedAnyone(t *testing.T) {
	raw, err := os.ReadFile("testdata/behavior-patrol-consumer.yaml")
	require.NoError(t, err)
	const step = "- {on: time, do: [{pick: move}]}"
	require.Equal(t, 1, strings.Count(string(raw), step))
	source := strings.Replace(string(raw), step,
		"- {on: time, when: {fact: {name: \"topic:welcome\", op: eq, value: true}}, do: [{pick: move}]}", 1)
	requireRefusedAt(t, refusals(t, source), "behaviorDefinitions.patrol.body",
		"`topic:welcome` is now `noted.anyone:welcome`")
}

func TestNotedWords_NotedAsksAboutAPersonCarriesAThing(t *testing.T) {
	t.Run("a prop named by noted is carried", func(t *testing.T) {
		requireExactDefect(t, refusals(t, goblinLine("requires: { noted: table }")), goblinAt+"[0].requires.noted",
			"`noted` asks what the creature spoken to has noted about the speaker, and \"table\" is a prop: "+
				"a speaker carrying it is `carries: table`")
	})
	t.Run("an id that is both a prop and a topic is refused for the doubt", func(t *testing.T) {
		source := optionsOnGoblin("          - { id: hi, say: \"Hi.\", success: [{ say: Fine., " +
			"give: { topic: table, to: spoken_to, about: speaker } }] }\n" +
			"          - { id: again, say: \"Again.\", requires: { noted: table } }\n")
		requireExactDefect(t, refusals(t, source), goblinAt+"[1].requires.noted",
			"\"table\" is both a prop and a topic something gives, so `noted: table` leaves a reader unsure which "+
				"is meant (a prop carried is `carries: table`): give the topic a name no prop has")
	})
	t.Run("a topic nothing gives", func(t *testing.T) {
		requireExactDefect(t, refusals(t, goblinLine("requires: { noted: welcome }")), goblinAt+"[0].requires.noted",
			"\"welcome\" is a topic no interaction's `outcomes` and no line's row gives, "+
				"so nobody could ever have noted it about anyone")
	})
	t.Run("a topic given on the goblin's own line is asked for", func(t *testing.T) {
		require.Empty(t, refusals(t, optionsOnGoblin(givesWelcome+"          - { id: again, say: \"Again.\", requires: { noted: welcome } }\n")))
	})
	t.Run("holds of a topic is pointed at noted", func(t *testing.T) {
		requireExactDefect(t, refusals(t, optionsOnGoblin(givesWelcome+"          - { id: again, say: \"Again.\", requires: { holds: welcome } }\n")),
			goblinAt+"[1].requires.holds", "`holds` is retired here: a topic the one spoken to has noted is `noted: welcome`")
	})
	t.Run("holds of a prop is pointed at carries", func(t *testing.T) {
		requireExactDefect(t, refusals(t, goblinLine("requires: { holds: table }")), goblinAt+"[0].requires.holds",
			"`holds` is retired here: a prop the speaker carries is `carries: table`")
	})
	t.Run("holds of neither names both", func(t *testing.T) {
		requireExactDefect(t, refusals(t, goblinLine("requires: { holds: lantern }")), goblinAt+"[0].requires.holds",
			"`holds` is retired here: a prop the speaker carries is `carries: <prop>`; a topic the one spoken to has noted is `noted: <topic>`")
	})
	t.Run("holds on a post's pass", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, strings.Replace(castleSource(t), "pass: { noted: admitted }", "pass: { holds: admitted }", 1)),
			"room.room.monsterBindings.gate-1.posts.alert.pass.holds",
			"`holds` is retired here: a topic the one spoken to has noted is `noted: admitted`")
	})
	t.Run("holds where nothing speaks", func(t *testing.T) {
		// Neither word that replaced it is legal in an ending: neither is suggested.
		requireExactDefect(t, refusals(t, v4With("endings:\n  - { id: rich, when: { holds: table } }", "")), "endings[0].when.holds",
			"`holds` is retired, and nothing here speaks: a prop carried and a topic noted are only asked by a line's requires "+
				"(`carries`, `noted`) or a post's pass (`noted`)")
	})
}

func TestNotedWords_APropNotesNothingAboutAnybody(t *testing.T) {
	const at = "room.room.propBindings.table.options[0]"
	propLine := func(rest string) string {
		return v4With("", "    propBindings:\n      table:\n        options:\n"+
			"          - { id: look, do: You look., "+rest+" }\n")
	}
	t.Run("asking what it has noted", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, propLine("requires: { noted: welcome }")), at+".requires.noted",
			"a prop notes nothing about anybody, so a line said to one cannot ask what it has noted")
	})
	t.Run("a prop's line naming a prop by noted is pointed at carries", func(t *testing.T) {
		requireExactDefect(t, refusals(t, propLine("requires: { noted: table }")), at+".requires.noted",
			"`noted` asks what the creature spoken to has noted about the speaker, and \"table\" is a prop: "+
				"a speaker carrying it is `carries: table`")
	})
	t.Run("giving it a topic", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, propLine("success: [{ give: { topic: welcome, to: spoken_to, about: speaker } }]")),
			at+".success[0].give", "a prop notes nothing about anybody, so a line said to one cannot give it a topic")
	})
}
