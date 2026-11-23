// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// own_hail_words_test.go is THE READ-BACK SAYS A CREATURE'S OWN HAIL
// (rpg-project#1139, slice rpg-project#1140): a step on `approached` that
// requires a `within` is said as someone's step bringing them within the
// creature's own distance, and the conversation's hail rule second.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

func TestOwnHailWords_AStepOnApproachedSaysTheCreaturesOwnDistanceFirst(t *testing.T) {
	site := strings.ReplaceAll(valueSite(), "on: heard", "on: approached")
	site = strings.ReplaceAll(site, "{ fact: { name: heard.claim, op: eq, value: failed } }", "{ scope: { within: 2 } }")
	rb, err := readback.Of([]byte(site))
	require.NoError(t, err, "a creature with no conversation may write its own hail")

	var sentences []string
	for _, st := range rb.Statements() {
		sentences = append(sentences, st.Sentence)
	}
	require.Condition(t, func() bool {
		for _, s := range sentences {
			if strings.Contains(s, "when someone's step brings them within the creature running it's distance (the step's own `within`) "+
				"and the creature running it sees them; or, when the creature running it carries a conversation that declares an approach, "+
				"under that conversation's hail rule") {
				return true
			}
		}
		return false
	}, "some statement says the creature's own hail first and the conversation's second: %v", sentences)
}
