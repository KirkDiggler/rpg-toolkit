// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// heard_words_test.go is a line heard moves an NPC's own machine
// (rpg-project#1000), as content: the castle's captain writes `heard.claim`
// and `holds:admitted` on his `in-hall` step, and each way they cannot work
// is refused when the castle is loaded, at the step, in the engine's words.

import (
	"strings"
	"testing"
)

// theLiedToStep is the captain's new step, as the castle writes it.
const theLiedToStep = `            - on: heard
              when:
                all:
                  - fact:
                      name: heard.claim
                      op: eq
                      value: failed
                  - fact:
                      name: holds:admitted
`

// liedToAt is that step's guard's path.
const liedToAt = "behaviorDefinitions.captain-policy.recipe.states.in-hall.transitions[1].when"

func TestHeardWords_TheCaptainsStepIsRefusedWhereItCannotWork(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement, path, sentence string }{
		{
			name: "a claim that is neither held nor failed", old: theLiedToStep,
			replacement: strings.Replace(theLiedToStep, "value: failed", "value: maybe", 1), path: liedToAt,
			sentence: "`heard.claim` is held or failed, and this names \"maybe\"",
		},
		{
			name: "a claim read on a beat that is not hearing", old: theLiedToStep,
			replacement: strings.Replace(theLiedToStep, "- on: heard", "- on: seen", 1), path: liedToAt,
			sentence: "`heard.claim` reads what the creature heard, and `seen` is not hearing: it is read on `heard` only",
		},
		{
			name: "holds read on a beat that names nobody", old: "                  name: holds.anyone:admitted\n",
			replacement: "                  name: holds:admitted\n",
			path:        "behaviorDefinitions.captain-policy.recipe.states.ready.transitions[2].when",
			sentence: "`holds:admitted` asks about the one this beat is about, and `topic` is about nobody: " +
				"it is read on heard, seen, barred and approached",
		},
		{
			name: "holds of a topic nothing gives", old: theLiedToStep,
			replacement: strings.Replace(theLiedToStep, "name: holds:admitted", "name: holds:welcome", 1), path: liedToAt,
			sentence: "\"welcome\" is a topic no interaction's `outcomes` and no line's success or failure row gives, " +
				"so nobody could ever come to hold it",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, castleWith(t, tc.old, tc.replacement)), tc.path, tc.sentence)
		})
	}
}
