// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// seen_anyone_test.go is SEEN FIRES FOR ANYONE WHO COMES INTO SIGHT AND A STEP
// READS ITS STANCE TO THEM (rpg-project#1123, slice rpg-project#1125), as an
// author writes it: a step on `seen` may ask `stance`, which is one of
// hostile, neutral, allied or none, and a state entered on `seen` may give a
// note `about: the_one_named`.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

const seenStancePath = "behaviorDefinitions.captain-policy.recipe.states.in-hall.transitions[0].when.fact"

// seenStates is a guard who sounds the alarm on seeing an enemy.
func seenStates(on, value string) string {
	return `        in-hall:
          transitions:
            - on: ` + on + `
              when: { fact: { name: stance, op: eq, value: ` + value + ` } }
              to: alarm
              reason: an enemy
        alarm: {}
`
}

func seenSite(states, outcomes string) string { return valueSiteWith(states, outcomes, topicBar) }

func TestDungeonspec_SeenAnyone_AStepOnSeenAsksTheStance(t *testing.T) {
	compiled := loadSource(t, seenSite(seenStates("seen", "hostile"), ""))
	guard := monsterNamed(t, compiled, "goblin-1")
	require.NotNil(t, guard.Behaviors, "the guard compiled with its machine")
}

func TestDungeonspec_SeenAnyone_AStanceNoneOfTheFourIsRefused(t *testing.T) {
	errs := refusals(t, seenSite(seenStates("seen", "angry"), ""))
	requireRefusedAt(t, errs, seenStancePath, "`stance` is hostile, neutral, allied or none, and this names \"angry\"")
}

func TestDungeonspec_SeenAnyone_AStanceOnTimeIsRefused(t *testing.T) {
	errs := refusals(t, seenSite(seenStates("time", "hostile"), ""))
	requireRefusedAt(t, errs, seenStancePath, "`stance` asks about the one this beat is about, and `time` is about nobody: "+
		"it is read on heard, seen, topic, barred and approached")
}

func TestDungeonspec_SeenAnyone_AStateEnteredOnSeenNotesTheOneNamed(t *testing.T) {
	states := strings.ReplaceAll(seenStates("seen", "hostile"), "alarm: {}", "spotted: {}")
	states = strings.ReplaceAll(states, "to: alarm", "to: spotted")
	outcomes := `    outcomes:
      spotted:
        - give: { topic: spotted, to: itself, about: the_one_named }
`
	guard := monsterNamed(t, loadSource(t, seenSite(states, outcomes)), "goblin-1")

	require.Equal(t, map[string][]encounter.Consequence{"spotted": {{Recognize: &encounter.Recognition{
		Topic: "spotted", Holder: encounter.RecognitionHolder(encounter.WhoItself),
		About: encounter.RecognitionSubjects(encounter.WhoTheOneNamed),
	}}}}, guard.Outcomes, "the compiled creature carries the outcome")
}
