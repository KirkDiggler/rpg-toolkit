// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// topic_beat_names_test.go is THE TOPIC BEAT NAMES WHOM THE NOTE IS ABOUT
// (rpg-project#1123, slice rpg-project#1124), as an author writes it: a step on
// `topic` may ask `noted.as:<topic>` and a state entered on it may give a note
// `about: the_one_named`.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// topicStates is a guard that steps on `topic` when the one the note is about
// is barred.
const topicStates = `        in-hall:
          transitions:
            - on: topic
              when: { fact: { name: "noted.as:standing", op: eq, value: barred } }
              to: at-the-door
              reason: barred
        at-the-door: {}
`

// topicOutcome is what entering at-the-door gives.
const topicOutcome = `    outcomes:
      at-the-door:
        - give: { topic: seen-out, to: itself, about: the_one_named }
`

// topicBar is the line that gives `barred`.
const topicBar = "          - id: bar\n" +
	"            say: \"Be gone.\"\n" +
	"            check: [{ ability: persuasion, dc: 10 }]\n" +
	"            failure:\n" +
	"              - say: No.\n" +
	"                give: { topic: standing, as: barred, to: spoken_to, about: speaker }\n"

func topicSite(states string) string { return valueSiteWith(states, topicOutcome, topicBar) }

func TestDungeonspec_TopicBeatNames_AStepOnTopicAsksTheValue(t *testing.T) {
	compiled := loadSource(t, topicSite(topicStates))
	guard := monsterNamed(t, compiled, "goblin-1")
	require.NotNil(t, guard.Behaviors, "the guard compiled with its machine")
}

func TestDungeonspec_TopicBeatNames_AStateEnteredOnTopicNotesTheOneNamed(t *testing.T) {
	compiled := loadSource(t, topicSite(topicStates))
	guard := monsterNamed(t, compiled, "goblin-1")

	require.Equal(t, map[string][]encounter.Consequence{"at-the-door": {{Recognize: &encounter.Recognition{
		Topic: "seen-out", Holder: encounter.RecognitionHolder(encounter.WhoItself),
		About: encounter.RecognitionSubjects(encounter.WhoTheOneNamed),
	}}}}, guard.Outcomes, "the compiled creature carries the outcome")
}

func TestDungeonspec_TopicBeatNames_AValueNothingGivesIsStillRefused(t *testing.T) {
	states := strings.ReplaceAll(topicStates, "value: barred", "value: scolded")
	errs := refusals(t, topicSite(states))
	requireRefusedAt(t, errs, "behaviorDefinitions.captain-policy.recipe.states.in-hall.transitions[0].when",
		`asks whether "standing" is noted as "scolded", and nothing gives it that value`)
}
