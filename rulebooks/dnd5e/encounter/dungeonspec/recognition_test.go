// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// recognition_test.go is the authored half of ADMISSION IS RECOGNITION
// (rpg-project#928): an interaction's `outcomes`, the `{ recognized }`
// predicate on a creature's line and on a post's `pass`, each compiled from
// the castle and each refused at the author's own path when it says a word
// the slice does not have.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// castleWith is the castle with one authored fragment rewritten, required to
// be there exactly once.
func castleWith(t *testing.T, old, replacement string) string {
	t.Helper()
	source := castleSource(t)
	require.Equal(t, 1, strings.Count(source, old), "the castle says %q once", old)

	return strings.Replace(source, old, replacement, 1)
}

// theOutcome is the front gate's authored outcome, as the castle writes it.
const theOutcome = `        - recognize:
            topic: admitted
            holder: spoken_to
            about: party_in_sight
            heard_by: carriers_who_witnessed
`

func TestDungeonspec_AnOutcomeCompilesOntoItsInteraction(t *testing.T) {
	c := loadSource(t, castleSource(t))
	require.Len(t, c.Field.Interactions, 1, "the castle has its one front gate")
	require.Equal(t, map[string][]encounter.Consequence{"admitted": {{Recognize: &encounter.Recognition{
		Topic: "admitted", Holder: encounter.RecognitionSpokenTo, About: encounter.RecognitionPartyInSight,
		HeardBy: encounter.RecognitionCarriersWhoWitnessed,
	}}}}, c.Field.Interactions[0].Outcomes)

	var line *encounter.Option
	for _, o := range monsterNamed(t, c, "captain").Options {
		if o.ID == "the-way-in" {
			line = &o
		}
	}
	require.NotNil(t, line, "the captain carries the line for whom he recognises")
	require.Equal(t, []encounter.Trigger{encounter.TriggerRecognized{Topic: "admitted"}}, line.Requires)
}

func TestDungeonspec_APassStillTakesAFact(t *testing.T) {
	c := loadSource(t, castleWith(t, `            pass: { recognized: admitted }
            at:
              - q: -4
                r: 8
`, `            pass: password
            at:
              - q: -4
                r: 8
`))
	for _, p := range monsterNamed(t, c, "captain").Posts {
		require.Equal(t, []encounter.Trigger{encounter.TriggerFact{Fact: "password"}}, p.Pass, "a bare word is the password it always was")
	}
}

func TestDungeonspec_RecognitionIsRefusedAtTheAuthorsPath(t *testing.T) {
	const outcome = "interactions.front-gate.outcomes"
	tests := []struct {
		name, old, replacement, path, message string
	}{
		{
			name: "a holder that is not the one spoken to", old: "holder: spoken_to", replacement: "holder: everyone",
			path: outcome + ".admitted[0].recognize.holder", message: "the holder is `spoken_to`, the creature the line was said to",
		},
		{
			name: "subjects that are not the party in sight", old: "about: party_in_sight", replacement: "about: the_town",
			path: outcome + ".admitted[0].recognize.about", message: "the subjects are `party_in_sight`, the speaker's party the holder sees",
		},
		{
			name: "hearers that are not the witnessing carriers", old: "heard_by: carriers_who_witnessed", replacement: "heard_by: the_town",
			path: outcome + ".admitted[0].recognize.heard_by", message: "the hearers are `carriers_who_witnessed`, or omit it for nobody",
		},
		{
			name: "an outcome on a state the recipe lacks", old: "      admitted:\n" + theOutcome, replacement: "      welcomed:\n" + theOutcome,
			path: outcome + ".welcomed", message: `"welcomed" is not a state of this interaction's recipe`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, castleWith(t, test.old, test.replacement)), test.path, test.message)
		})
	}

	t.Run("a line asking for a topic nothing grants", func(t *testing.T) {
		errs := refusals(t, castleWith(t, "            requires:\n              recognized: admitted\n",
			"            requires:\n              recognized: welcome\n"))
		require.NotEmpty(t, errs)
		require.True(t, strings.HasSuffix(errs[0].Path, ".requires.recognized"), errs[0].Path)
		require.Contains(t, errs[0].Message, `"welcome" is a topic no interaction's`)
	})

	t.Run("a pass asking for a topic nothing grants", func(t *testing.T) {
		errs := refusals(t, strings.ReplaceAll(castleSource(t), "pass: { recognized: admitted }", "pass: { recognized: welcome }"))
		require.NotEmpty(t, errs)
		require.True(t, strings.HasSuffix(errs[0].Path, ".pass.recognized"), errs[0].Path)
	})

	t.Run("a pass on an interaction's state", func(t *testing.T) {
		errs := refusals(t, strings.ReplaceAll(castleSource(t), "pass: { recognized: admitted }",
			"pass: { interaction: { id: front-gate, in: [admitted] } }"))
		require.NotEmpty(t, errs)
		require.True(t, strings.HasSuffix(errs[0].Path, ".pass"), errs[0].Path)
	})
}
