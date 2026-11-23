// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// note_value_test.go is AN AUTHOR WRITES A NOTE'S VALUE (rpg-project#1007,
// rpg-project#1114): `give` takes `as`, `noted` takes `{ topic, as }`, and a
// recipe's `noted.as:<topic>` counts as asking that topic. The engine half is
// rpg-project#1112 and #1113; this is the author's file.

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

const (
	standing        = "standing"
	standingGuard   = encounter.MemberID("goblin-1")
	standingVisitor = encounter.MemberID("visitor")
)

// valueStep is the step both states carry: a checked line that failed, with the
// guard already warned, sends the machine back to the gate.
const valueStep = `          transitions:
            - on: heard
              when:
                all:
                  - { fact: { name: heard.claim, op: eq, value: failed } }
                  - { fact: { name: "noted.as:standing", op: eq, value: warned } }
              to: back-to-gate
              reason: lied twice
`

// valueStates is the definition's two states.
const valueStates = "        in-hall:\n" + valueStep + "        back-to-gate:\n" + valueStep

// valueOutcome is what entering back-to-gate gives, as an author writes it.
const valueOutcome = `    outcomes:
      back-to-gate:
        - give: { topic: standing, as: barred, to: itself, about: the_one_named }
`

// valueLine is `lie`: said when nothing is noted, it gives a value on failure.
const valueLie = "          - id: lie\n" +
	"            say: \"We're expected.\"\n" +
	"            check: [{ ability: persuasion, dc: 10 }]\n" +
	"            requires: { not: { noted: standing } }\n" +
	"            failure:\n" +
	"              - say: Liar.\n" +
	"                give: { topic: standing, as: warned, to: spoken_to, about: speaker }\n"

// valueLieAgain is `lie-again`: said only to one who has warned.
const valueLieAgain = "          - id: lie-again\n" +
	"            say: \"Truly.\"\n" +
	"            check: [{ ability: persuasion, dc: 10 }]\n" +
	"            requires: { noted: { topic: standing, as: warned } }\n" +
	"            failure:\n" +
	"              - say: Liar again.\n"

// valueSiteWith is a small site of two creatures; the states, the outcomes and
// the lines are the caller's, written at their own indentation.
func valueSiteWith(states, outcomes, lines string) string {
	definitions := "behaviorDefinitions:\n  captain-policy:\n    recipe:\n" +
		"      ref: site:behaviors:captain-policy\n      initial: in-hall\n      states:\n" + states + outcomes
	gameplay := "    monsterBindings:\n" +
		"      goblin-1:\n        behaviorState: policy\n        behaviors: [{ id: policy, use: captain-policy }]\n" +
		"        options:\n" + lines +
		"      goblin-2:\n        behaviorState: policy\n        behaviors: [{ id: policy, use: captain-policy }]\n"

	return v4With(definitions, gameplay)
}

// valueSite is the two-strikes site as an author writes it.
func valueSite() string { return valueSiteWith(valueStates, valueOutcome, valueLie+valueLieAgain) }

// offeredIDs is the ids of the lines the guard offers the visitor now.
func offeredIDs(t *testing.T, enc *encounter.Encounter) []string {
	t.Helper()
	offered, err := enc.OptionsFor(standingVisitor, standingGuard)
	require.NoError(t, err)
	ids := []string{}
	for _, o := range offered {
		ids = append(ids, o.ID)
	}

	return ids
}

func TestDungeonspec_NoteValue_TwoStrikesCompiledAndPlayed(t *testing.T) {
	compiled := loadSource(t, valueSite())
	guard := monsterNamed(t, compiled, "goblin-1")

	// COMPILED: the values are on the engine's fields.
	require.Equal(t, map[string][]encounter.Consequence{"back-to-gate": {{Recognize: &encounter.Recognition{
		Topic: standing, As: "barred", Holder: encounter.RecognitionHolder(encounter.WhoItself),
		About: encounter.RecognitionSubjects(encounter.WhoTheOneNamed),
	}}}}, guard.Outcomes, "a state's give carries its value")

	byID := map[string]encounter.Option{}
	for _, o := range guard.Options {
		byID[o.ID] = o
	}
	require.Len(t, byID["lie"].Grants, 1)
	require.Equal(t, "warned", byID["lie"].Grants[0].Consequence.Recognize.As, "a line's give carries its value")
	require.Equal(t, []encounter.Trigger{encounter.TriggerRecognized{Topic: standing, As: "warned"}}, byID["lie-again"].Requires,
		"a line's ask carries its value")
	require.Equal(t, []encounter.Trigger{encounter.TriggerNot{Inner: encounter.TriggerRecognized{Topic: standing}}}, byID["lie"].Requires,
		"a plain noted asks whatever the value")

	// PLAYED.
	members := []encounter.MemberInput{{ID: standingVisitor, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At, Faction: encounter.FactionID(m.Faction),
			Behaviors: m.Behaviors, Options: m.Options, Outcomes: m.Outcomes,
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sheets:        hearingSheets{},
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        encounter.BehaviorDriver{},
			Roller:        rollsOne{},
			CheckResolver: nothingIsEverFound{},
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	say := func(line string) {
		_, speakErr := enc.Speak(context.Background(), &encounter.SpeakInput{
			Range: 10, Actor: standingVisitor, Target: standingGuard, OptionID: line, Skill: "persuasion",
			Beaten: false, DC: 10, Total: 4, Roller: rollsOne{},
		})
		require.NoError(t, speakErr)
	}

	require.Equal(t, []string{"lie"}, offeredIDs(t, enc), "nothing is noted yet, so only the first lie is offered")
	say("lie")
	require.Equal(t, []string{"lie-again"}, offeredIDs(t, enc),
		"after the first lie he has warned her: the second lie is offered and the first is not")
	say("lie-again")
	require.Empty(t, offeredIDs(t, enc), "after the second lie he has barred her: neither is offered")
}

func TestDungeonspec_NoteValue_APostsPassAsksAValue(t *testing.T) {
	const give = "            topic: caught-lying\n            to: itself\n            about: the_one_named\n"
	const pass = "              - not:\n                  noted: caught-lying\n            at:\n"
	source := castleWith(t, give, give+"            as: barred\n")
	require.Equal(t, 1, strings.Count(source, pass), "the captain's gate pass says it once")
	source = strings.Replace(source, pass, "              - not:\n                  noted: { topic: caught-lying, as: barred }\n            at:\n", 1)

	c := loadSource(t, source)
	var found bool
	for _, post := range monsterNamed(t, c, "captain").Posts {
		for _, req := range post.Pass {
			if not, ok := req.(encounter.TriggerNot); ok {
				if r, ok := not.Inner.(encounter.TriggerRecognized); ok && r.Topic == "caught-lying" {
					require.Equal(t, "barred", r.As, "the pass asks the value")
					found = true
				}
			}
		}
	}
	require.True(t, found, "the captain's gate post asks not-barred")
}

func TestDungeonspec_NoteValue_WhatIsWrittenTodayMeansWhatItMeant(t *testing.T) {
	c := loadSource(t, castleSource(t))
	for _, m := range c.Monsters {
		for _, cs := range m.Outcomes {
			for _, c := range cs {
				if c.Recognize != nil {
					require.Empty(t, c.Recognize.As, "a give with no `as` has no value")
				}
			}
		}
		for _, o := range m.Options {
			for _, g := range o.Grants {
				if g.Consequence.Recognize != nil {
					require.Empty(t, g.Consequence.Recognize.As, "a give with no `as` has no value")
				}
			}
			for _, r := range o.Requires {
				if r, ok := r.(encounter.TriggerRecognized); ok {
					require.Empty(t, r.As, "a plain `noted` asks for no value")
				}
			}
		}
	}
	// the plain word still decodes to its topic, and says itself back.
	var spec dungeonspec.PredicateSpec
	require.NoError(t, yamlInto("noted: admitted", &spec))
	require.Equal(t, "admitted", spec.Noted)
	require.Empty(t, spec.NotedAs)
	require.Equal(t, "{ noted: admitted }", spec.String())
}

func TestDungeonspec_NoteValue_AValuedNotedReadsBackAsWritten(t *testing.T) {
	var spec dungeonspec.PredicateSpec
	require.NoError(t, yamlInto("noted: { topic: standing, as: warned }", &spec))
	require.Equal(t, "standing", spec.Noted)
	require.Equal(t, "warned", spec.NotedAs)
	require.Equal(t, "{ noted: { topic: standing, as: warned } }", spec.String())

	var topicOnly dungeonspec.PredicateSpec
	require.NoError(t, yamlInto("noted: { topic: standing }", &topicOnly))
	require.Equal(t, "{ noted: standing }", topicOnly.String(), "a mapping with no value is the plain word")
}

func TestDungeonspec_NoteValue_TheFileIsRefusedWhereItIsWrong(t *testing.T) {
	at := "behaviorDefinitions.captain-policy.outcomes.back-to-gate[0].give.as"
	lieAt := "room.room.monsterBindings.goblin-1.options[0].failure[0].give.as"

	t.Run("a value on the engine's own topic", func(t *testing.T) {
		site := valueSiteWith(valueStates, strings.Replace(valueOutcome, "topic: standing, as: barred", "topic: wanted, as: barred", 1), valueLie+valueLieAgain)
		requireRefusedAt(t, refusals(t, site), at, `gives "wanted" as "barred": "wanted" is the engine's own topic and has no value`)
		line := strings.Replace(valueLie, "topic: standing, as: warned", "topic: jailed, as: warned", 1)
		requireRefusedAt(t, refusals(t, valueSiteWith(valueStates, valueOutcome, line+valueLieAgain)), lieAt,
			`gives "jailed" as "warned": "jailed" is the engine's own topic and has no value`)
	})

	t.Run("a topic given with a value in one place and with none in another", func(t *testing.T) {
		plain := strings.Replace(valueLieAgain, "failure:\n              - say: Liar again.\n",
			"failure:\n              - say: Liar again.\n                give: { topic: standing, to: spoken_to, about: speaker }\n", 1)
		errs := refusals(t, valueSiteWith(valueStates, valueOutcome, valueLie+plain))
		requireRefusedAt(t, errs, lieAt, `the topic "standing" is given as "warned" in one place and with no value in another`,
			"a topic has a value everywhere it is given, or nowhere")
	})

	t.Run("a value nothing gives, on a line", func(t *testing.T) {
		line := strings.Replace(valueLieAgain, "as: warned", "as: scolded", 1)
		requireRefusedAt(t, refusals(t, valueSiteWith(valueStates, valueOutcome, valueLie+line)),
			"room.room.monsterBindings.goblin-1.options[1].requires.noted",
			`asks whether "standing" is noted as "scolded", and nothing gives it that value`)
	})

	t.Run("a value nothing gives, in a machine step", func(t *testing.T) {
		states := strings.ReplaceAll(valueStates, "value: warned", "value: scolded")
		errs := refusals(t, valueSiteWith(states, valueOutcome, valueLie+valueLieAgain))
		requireRefusedAt(t, errs, "behaviorDefinitions.captain-policy.recipe.states.in-hall.transitions[0].when",
			`asks whether "standing" is noted as "scolded", and nothing gives it that value`)
	})

	t.Run("a topic a step asks as a value and nothing gives", func(t *testing.T) {
		states := strings.ReplaceAll(valueStates, "noted.as:standing", "noted.as:bogus")
		errs := refusals(t, valueSiteWith(states, valueOutcome, valueLie+valueLieAgain))
		requireRefusedAt(t, errs, "behaviorDefinitions.captain-policy.recipe.states.in-hall.transitions[0].when",
			`"bogus" is a topic no interaction's`)
	})

	t.Run("a value nothing gives, on a post", func(t *testing.T) {
		const give = "            topic: caught-lying\n            to: itself\n            about: the_one_named\n"
		const pass = "              - not:\n                  noted: caught-lying\n            at:\n"
		source := castleWith(t, give, give+"            as: barred\n")
		source = strings.Replace(source, pass, "              - not:\n                  noted: { topic: caught-lying, as: forgiven }\n            at:\n", 1)
		errs := refusals(t, source)
		require.NotEmpty(t, errs)
		var found bool
		for _, e := range errs {
			if strings.HasSuffix(e.Path, ".pass[1].not.noted") {
				require.Contains(t, e.Message, `asks whether "caught-lying" is noted as "forgiven", and nothing gives it that value`)
				found = true
			}
		}
		require.True(t, found, "no refusal at the post's pass: %v", errs)
	})

	t.Run("an unknown key in the noted mapping", func(t *testing.T) {
		var spec dungeonspec.PredicateSpec
		err := yamlInto("noted: { topic: standing, by: x }", &spec)
		require.Error(t, err)
		require.Contains(t, err.Error(), "line 1")
		require.Contains(t, err.Error(), "by")
		require.Contains(t, err.Error(), "a noted takes topic, as")
	})

	t.Run("a noted mapping with no topic", func(t *testing.T) {
		var spec dungeonspec.PredicateSpec
		err := yamlInto("noted: { as: warned }", &spec)
		require.Error(t, err)
		require.Contains(t, err.Error(), "topic")
	})

	t.Run("an unknown key on a give names the keys a give takes, now with as", func(t *testing.T) {
		var give dungeonspec.GiveSpec
		err := yaml.Unmarshal([]byte("{ topic: standing, whom: x }"), &give)
		require.Error(t, err)
		require.Contains(t, err.Error(), "a give takes topic, as, to, about, heard_by")
	})
}

// yamlInto decodes one predicate written as a one-line mapping.
func yamlInto(source string, into *dungeonspec.PredicateSpec) error {
	return yaml.Unmarshal([]byte(source), into)
}

func TestDungeonspec_NoteValue_TheReadBackSaysAValuedGivesValueAndListsNothingUnsaid(t *testing.T) {
	rb, err := readback.Of([]byte(valueSite()))
	require.NoError(t, err)
	for _, un := range rb.NotYetSaid {
		require.NotContains(t, un.Path, "give.as", "a line's valued give says its `as`: %v", un)
	}
	require.Contains(t, rb.Text(), "is given the topic `standing` as `warned`", "the value is said, not dropped")
}
