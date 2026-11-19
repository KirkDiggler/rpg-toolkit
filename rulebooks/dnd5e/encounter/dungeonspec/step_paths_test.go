// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// step_paths_test.go is A REFUSED STEP IS REPORTED AT ITS OWN PATH
// (rpg-project#997, #1001 item 2): a native recipe's refused step lands at
// `behaviorDefinitions.<id>.recipe.states.<S>.transitions[<k>]`, with the
// leaf when it is about a condition or an act, in the refusal's own
// sentence; a refusal the machine does not place stays on the creature.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stepSite is the small site: one native definition `test`, a step on
// `time` that picks `time` and one on `heard`, run by goblin-1, with old
// replaced by replacement in the recipe.
func stepSite(old, replacement string) string {
	native := `ref: example:behaviors:test
initial: ready
states:
  ready: {transitions: [{on: time, do: [{pick: time}]}, {on: heard, to: ready}]}`
	native = strings.Replace(native, old, replacement, 1)
	root := "behaviorDefinitions:\n  test:\n    recipe:\n      " + strings.ReplaceAll(native, "\n", "\n      ") + "\n    table: {time: [{hold: {}}]}\n"
	return v4With(root, bindingOnGoblin1("        behaviors: [{id: body, use: test}]\n"))
}

const testSteps = "behaviorDefinitions.test.recipe.states.ready.transitions"

func TestStepPaths_ARefusedStepIsReportedAtItsOwnPath(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement, path, message string }{
		{
			name: "a target that is no state (the machine's)", old: "to: ready", replacement: "to: nowhere",
			path: testSteps + "[1]", message: `to "nowhere" is not a state of the recipe`,
		},
		{
			name: "a fact the vocabulary lacks (the machine's, inside the condition)", old: "{on: heard, to: ready}",
			replacement: "{on: heard, to: ready, when: {fact: {name: mood, op: eq, value: sad}}}",
			path:        testSteps + "[1].when.fact", message: `fact "mood" is not in the vocabulary`,
		},
		{
			name: "a claim read off hearing (the encounter's, inside the condition)", old: "{on: time, do: [{pick: time}]}",
			replacement: "{on: time, when: {fact: {name: heard.claim, op: eq, value: failed}}, do: [{pick: time}]}",
			path:        testSteps + "[0].when.fact",
			message:     "`heard.claim` reads what the creature heard, and `time` is not hearing: it is read on `heard` only",
		},
		{
			name: "a pick on a beat that cannot pick (the encounter's, at the act)", old: "{on: heard, to: ready}",
			replacement: "{on: heard, to: ready, do: [{anchor: true}, {pick: time}]}",
			path:        testSteps + "[1].do[1]",
			message:     "selection is only authorized on time, turn, barred or approached, not \"heard\"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, stepSite(tc.old, tc.replacement)), tc.path, tc.message)
		})
	}
}

// Two refused steps are two refusals, each at its own path.
func TestStepPaths_TwoRefusedStepsAreTwoPaths(t *testing.T) {
	errs := refusals(t, stepSite("ready: {transitions: [{on: time, do: [{pick: time}]}, {on: heard, to: ready}]}",
		"ready: {transitions: [{on: time, to: gone, do: [{pick: time}]}, {on: heard, to: nowhere}]}"))
	requireRefusedAt(t, errs, testSteps+"[0]", `to "gone" is not a state of the recipe`)
	requireRefusedAt(t, errs, testSteps+"[1]", `to "nowhere" is not a state of the recipe`)
}

// A refusal the machine does not place — mind/behavior's two picks on one
// step, its own text — stays on the creature that runs the definition.
func TestStepPaths_AnUnplacedRefusalStaysOnTheCreature(t *testing.T) {
	errs := refusals(t, stepSite("do: [{pick: time}]", "do: [{pick: time}, {pick: time}]"))
	requireRefusedAt(t, errs, "room.room.monsterBindings.goblin-1.behaviors[0].use", "requests multiple picks")
}

// The one sentence this slice changes: `approached` is scoped, so a states
// block's `within` on an unscoped beat names it among the beats that have a
// place.
func TestStepPaths_WithinNamesEveryScopedBeat(t *testing.T) {
	errs := refusals(t, v4With("", bindingOnGoblin1("        states:\n          initial: posted\n          posted:\n            transitions:\n"+
		"              - { on: fact, within: 2, to: posted }\n")))
	requireRefusedAt(t, errs, "room.room.monsterBindings.goblin-1.states.posted.transitions[0].within",
		"`fact` happened nowhere, so there is nothing to be within: `within` and `regions` are for heard, deed, seen, barred and approached")
}

// An unknown `initial` is refused once, in the dialect's own words: the
// block stops there, so the compile is never asked the same thing.
func TestStepPaths_AnUnknownInitialIsRefusedOnce(t *testing.T) {
	errs := refusals(t, v4With("", bindingOnGoblin1("        states:\n          initial: nowhere\n          posted:\n            transitions:\n"+
		"              - { on: heard, to: posted }\n")))
	at := 0
	for _, e := range errs {
		if e.Path == "room.room.monsterBindings.goblin-1.states.initial" {
			at++
			require.Contains(t, e.Message, `"nowhere" is not a state this block declares`)
		}
	}
	require.Equal(t, 1, at, "%v", errs)
}
