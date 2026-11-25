// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// state_party_test.go is A STATE NOTES ABOUT THE PARTY IT SEES, AND THOSE IT
// SAW WHO ARE GONE (rpg-project#1134, slice rpg-project#1135), as an author
// writes it: a state's give may be `about: party_in_sight` or `about: the_gone`.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// partyOutcome is what entering at-the-door gives: both groups.
const partyOutcome = `    outcomes:
      at-the-door:
        - give: { topic: standing, as: barred, to: itself, about: party_in_sight }
        - give: { topic: standing, as: wanted, to: itself, about: the_gone }
`

func TestDungeonspec_StateParty_BothWordsCompile(t *testing.T) {
	compiled := loadSource(t, valueSiteWith(topicStates, partyOutcome, topicBar))
	guard := monsterNamed(t, compiled, "goblin-1")

	note := func(as, about string) encounter.Consequence {
		return encounter.Consequence{Recognize: &encounter.Recognition{
			Topic: "standing", As: as, Holder: encounter.RecognitionHolder(encounter.WhoItself),
			About: encounter.RecognitionSubjects(about),
		}}
	}
	require.Equal(t, map[string][]encounter.Consequence{"at-the-door": {
		note("barred", string(encounter.RecognitionPartyInSight)), note("wanted", encounter.WhoTheGone),
	}}, guard.Outcomes, "the compiled creature carries both outcomes")
}

func TestDungeonspec_StateParty_ASpeakerIsRefusedAtItsPath(t *testing.T) {
	outcome := strings.Replace(partyOutcome, "about: party_in_sight", "about: speaker", 1)
	errs := refusals(t, valueSiteWith(topicStates, outcome, topicBar))

	requireRefusedAt(t, errs, "behaviorDefinitions.captain-policy.outcomes.at-the-door[0].give.about",
		"the subjects are", "`the_one_named`", "`party_in_sight`", "`the_gone`")
}

func TestDungeonspec_StateParty_OnABeatThatNamesNobodyIsRefused(t *testing.T) {
	states := strings.Replace(topicStates, "on: topic\n              when: { fact: { name: \"noted.as:standing\", op: eq, value: barred } }", "on: time", 1)
	require.NotEqual(t, topicStates, states, "precondition: the step is on time")
	outcome := "    outcomes:\n      at-the-door:\n        - give: { topic: standing, as: wanted, to: itself, about: the_gone }\n"
	errs := refusals(t, valueSiteWith(states, outcome, topicBar))

	requireRefusedAt(t, errs, "behaviorDefinitions.captain-policy.outcomes.at-the-door[0].give.about",
		"which names nobody", "`the_gone` has no one to be about")
}
