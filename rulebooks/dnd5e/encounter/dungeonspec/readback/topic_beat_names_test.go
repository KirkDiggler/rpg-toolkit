// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// topic_beat_names_test.go is THE READ-BACK SAYS WHOM A `topic` STEP ASKS
// ABOUT (rpg-project#1123, slice rpg-project#1124): the one the note is about,
// with the same ask connections a `noted.as:` step on `heard` carries.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

func TestTopicBeatNames_AStepOnTopicSaysWhomItAsksAbout(t *testing.T) {
	topicStep := strings.Replace(valueStep, "- on: heard", "- on: topic", 1)
	topicStep = strings.Replace(topicStep, "                all:\n"+
		"                  - { fact: { name: heard.claim, op: eq, value: failed } }\n"+
		"                  - { fact: { name: \"noted.as:standing\", op: eq, value: warned } }\n",
		"                all:\n"+
			"                  - { fact: { name: \"noted.as:standing\", op: eq, value: warned } }\n", 1)
	site := strings.Replace(valueSite(), valueStep, topicStep, 1)
	rb, err := readback.Of([]byte(site))
	require.NoError(t, err)

	var found []readback.Statement
	for _, st := range rb.Statements() {
		for _, c := range st.Connects {
			if !notePointer(st) && c.Kind == "topic" && c.ID == "standing" && c.Role == "asks" && strings.HasPrefix(st.Path, "behaviorDefinitions.") &&
				strings.Contains(st.Path, "in-hall") {
				found = append(found, st)
				break
			}
		}
	}
	require.NotEmpty(t, found, "the `topic` step's `noted.as:standing` asks the topic")
	st := found[0]
	require.Contains(t, st.Sentence, "the one the note is about", "the sentence says whom the step asks about")
	require.Equal(t, []readback.Connection{{Kind: "topic", ID: "standing", Role: "asks", Is: "warned"}}, topicEdges(st, "asks"))
	var about []string
	for _, c := range st.Connects {
		if c.Kind == "who" && c.Role == "about" && c.At == "standing" {
			about = append(about, c.ID)
		}
	}
	require.Equal(t, []string{"the_one_named"}, about, "whom it is about")
}
