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
			name: "a holder that is not the one spoken to", old: theOutcome, replacement: strings.Replace(theOutcome, "holder: spoken_to", "holder: everyone", 1),
			path: outcome + ".admitted[0].recognize.holder", message: "the holder is `spoken_to`, the creature the line was said to",
		},
		{
			name: "subjects that are not the party in sight", old: theOutcome, replacement: strings.Replace(theOutcome, "about: party_in_sight", "about: the_town", 1),
			path: outcome + ".admitted[0].recognize.about", message: "the subjects are `party_in_sight`, the speaker's party the holder sees, or `speaker`, the speaker alone",
		},
		{
			name: "hearers that are not the witnessing carriers", old: theOutcome, replacement: strings.Replace(theOutcome, "heard_by: carriers_who_witnessed", "heard_by: the_town", 1),
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

// theStragglersSuccess is gate-1's straggler line's success row, as the castle
// writes it (rpg-project#929).
const theStragglersSuccess = `              - say: Fell behind, did you? Go on, catch them up.
                recognize:
                  topic: admitted
                  holder: spoken_to
                  about: speaker
                  heard_by: carriers_who_witnessed
`

func TestDungeonspec_ALinesAnswerGrantsARecord(t *testing.T) {
	c := loadSource(t, castleSource(t))
	for _, id := range []string{"gate-1", "gate-2"} {
		var vouch, straggler, friends *encounter.Option
		for _, o := range monsterNamed(t, c, id).Options {
			switch o.ID {
			case "vouch":
				vouch = &o
			case "straggler":
				straggler = &o
			case "friends-inside":
				friends = &o
			}
			require.NotEqual(t, "guest", o.ID, "the guest claim is the captain's alone")
		}
		require.NotNil(t, vouch, "%s carries the vouch", id)
		require.Equal(t, []encounter.RowGrant{{Row: 0, Consequence: encounter.Consequence{Recognize: &encounter.Recognition{
			Topic: "admitted", Holder: encounter.RecognitionSpokenTo, About: encounter.RecognitionPartyInSight,
			HeardBy: encounter.RecognitionCarriersWhoWitnessed,
		}}}}, vouch.Grants)
		require.NotNil(t, straggler, "%s carries the straggler's case", id)
		require.Equal(t, encounter.RecognitionSpeaker, straggler.Grants[0].Consequence.Recognize.About)
		require.True(t, straggler.Checked(), "the straggler's case is checked")
		require.False(t, vouch.Checked(), "the vouch is plain")

		// ONE TRY (rpg-project#932): a miss grants `turned-away` about the
		// speaker, heard, and the case asks that the guard not hold it.
		require.Equal(t, encounter.RowGrant{Row: 0, Failure: true, Consequence: encounter.Consequence{Recognize: &encounter.Recognition{
			Topic: "turned-away", Holder: encounter.RecognitionSpokenTo, About: encounter.RecognitionSpeaker,
			HeardBy: encounter.RecognitionCarriersWhoWitnessed,
		}}}, straggler.Grants[1])
		require.Contains(t, straggler.Requires, encounter.Trigger(encounter.TriggerNot{Inner: encounter.TriggerRecognized{Topic: "turned-away"}}))
		require.NotNil(t, friends, "%s answers a stranger asking after their friends", id)
		require.Empty(t, friends.Grants, "and grants nothing")
		require.Nil(t, friends.Advance, "and advances nothing")
		require.False(t, friends.Checked(), "it is plain speech")
	}
}

func TestDungeonspec_ALinesGrantIsRefusedAtTheAuthorsPath(t *testing.T) {
	gate1 := "room.room.monsterBindings.gate-1.options[1]"

	t.Run("on a failure row it compiles as the miss's grant", func(t *testing.T) {
		c := loadSource(t, castleSource(t))
		for _, o := range monsterNamed(t, c, "gate-1").Options {
			if o.ID == "straggler" {
				require.True(t, o.Grants[1].Failure, "the failure row's grant lands on a miss")
			}
		}
	})

	t.Run("on a plain line's failure row", func(t *testing.T) {
		errs := refusals(t, strings.Replace(castleSource(t), "              - say: The troupe? Aye, they’re inside. If one of them comes back and speaks for you, in you go.\n",
			"              - say: The troupe? Aye, they’re inside. If one of them comes back and speaks for you, in you go.\n            failure:\n              - say: No.\n                recognize: { topic: turned-away, holder: spoken_to, about: speaker }\n", 1))
		require.NotEmpty(t, errs, "a plain line cannot be missed, so it has no failure row to grant on")
		require.True(t, strings.HasSuffix(errs[0].Path, ".failure"), errs[0].Path)
	})

	t.Run("heard_by on a line that names no interaction", func(t *testing.T) {
		text := castleWith(t, "            say: Where will the feast be, captain?\n            requires:\n              recognized: admitted\n            success:\n              - say: In the great hall. You’re expected.\n",
			"            say: Where will the feast be, captain?\n            requires:\n              recognized: admitted\n            success:\n              - say: In the great hall. You’re expected.\n                recognize: { topic: admitted, holder: spoken_to, about: speaker, heard_by: carriers_who_witnessed }\n")
		errs := refusals(t, text)
		require.NotEmpty(t, errs)
		require.True(t, strings.HasSuffix(errs[0].Path, ".success[0].recognize.heard_by"), errs[0].Path)
	})

	t.Run("an unknown subject word", func(t *testing.T) {
		text := strings.Replace(castleSource(t), theStragglersSuccess,
			strings.Replace(theStragglersSuccess, "about: speaker", "about: anyone", 1), 1)
		requireExactDefect(t, refusals(t, text), gate1+".success[0].recognize.about",
			"the subjects are `party_in_sight`, the speaker's party the holder sees, or `speaker`, the speaker alone")
	})

	t.Run("on a table row", func(t *testing.T) {
		errs := refusals(t, castleWith(t, "        - say: Not one step further without my say.\n",
			"        - say: Not one step further without my say.\n          recognize: { topic: admitted, holder: spoken_to, about: speaker }\n"))
		require.NotEmpty(t, errs)
		require.Equal(t, "recognize is only on a line's success or failure row: a table row grants nothing", errs[0].Message)
	})

	t.Run("a guard reading a topic nothing grants", func(t *testing.T) {
		errs := refusals(t, castleWith(t, "                  name: topic:admitted\n", "                  name: topic:welcome\n"))
		require.NotEmpty(t, errs, "a routine cannot wait on a topic nobody could ever hold")
	})
}
