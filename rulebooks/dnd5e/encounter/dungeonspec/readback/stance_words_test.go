// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// stance_words_test.go is THE READ-BACK SAYS A STANCE (rpg-project#1123, slice
// rpg-project#1125): a step on `seen` that asks `stance` is said as the one
// the creature saw is its enemy.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

func TestStanceWords_AStepOnSeenSaysTheOneSawIsItsEnemy(t *testing.T) {
	site := strings.ReplaceAll(valueSite(), "on: heard", "on: seen")
	site = strings.ReplaceAll(site, "{ name: heard.claim, op: eq, value: failed }", "{ name: stance, op: eq, value: hostile }")
	rb, err := readback.Of([]byte(site))
	require.NoError(t, err, "a step on seen may ask the stance")

	var sentences []string
	for _, st := range rb.Statements() {
		sentences = append(sentences, st.Sentence)
	}
	require.Condition(t, func() bool {
		for _, s := range sentences {
			if strings.Contains(s, "the one the creature running it saw is its enemy") {
				return true
			}
		}
		return false
	}, "some statement says the one the creature running it saw is its enemy: %v", sentences)
}

func TestStanceWords_AStepOnSeenUnderNotSaysTheNotOfThatSentence(t *testing.T) {
	site := strings.ReplaceAll(valueSite(), "on: heard", "on: seen")
	site = strings.ReplaceAll(site, "{ fact: { name: heard.claim, op: eq, value: failed } }",
		"{ not: { fact: { name: stance, op: eq, value: hostile } } }")
	rb, err := readback.Of([]byte(site))
	require.NoError(t, err, "a step on seen may ask the stance under a not")

	var sentences []string
	for _, st := range rb.Statements() {
		sentences = append(sentences, st.Sentence)
	}
	require.Condition(t, func() bool {
		for _, s := range sentences {
			if strings.Contains(s, "NOT (the one the creature running it saw is its enemy)") {
				return true
			}
		}
		return false
	}, "some statement says the not of the stance sentence: %v", sentences)
}
