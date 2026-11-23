// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// state_outcomes_test.go is A STATE'S OUTCOMES, WRITTEN (rpg-project#989,
// rpg-project#1104): a behaviour definition's `outcomes` says what its machine
// gives on entering a state, once, for every creature whose body it is. The
// engine half is rpg-project#1103; this is the author's half.

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const (
	outcomesTopic   = "caught-lying"
	outcomesCaptain = encounter.MemberID("goblin-1")
	outcomesVisitor = encounter.MemberID("visitor")
	outcomesDef     = "captain-policy"
	outcomesAt      = "behaviorDefinitions." + outcomesDef + ".outcomes"
)

// outcomesStates is the definition's states: ready steps to back-to-gate on a
// claim that failed.
const outcomesStates = `        ready:
          transitions:
            - on: heard
              when: { fact: { name: heard.claim, op: eq, value: failed } }
              to: back-to-gate
              reason: lied to
        back-to-gate: {}
`

// outcomesGive is the one thing a state may give, as an author writes it.
const outcomesGive = `        - give:
            topic: caught-lying
            to: itself
            about: the_one_named
`

// outcomesBlock is the definition's `outcomes` key on back-to-gate.
const outcomesBlock = "    outcomes:\n      back-to-gate:\n" + outcomesGive

// outcomesSiteWith is a small site of two creatures: the first (goblin-1) takes
// a claim and runs captain-policy as its body, the second runs it too. The
// states and the outcomes are the caller's, written at their own indentation.
func outcomesSiteWith(states, outcomes, lines string) string {
	definitions := "behaviorDefinitions:\n  " + outcomesDef + ":\n    recipe:\n" +
		"      ref: site:behaviors:captain-policy\n      initial: ready\n      states:\n" + states + outcomes
	gameplay := "    monsterBindings:\n" +
		"      goblin-1:\n        behaviorState: policy\n        behaviors: [{ id: policy, use: " + outcomesDef + " }]\n" +
		"        options:\n" +
		"          - { id: claim, say: \"We're expected.\", check: [{ ability: persuasion, dc: 10 }], failure: [{ say: Liar. }] }\n" +
		lines +
		"      goblin-2:\n        behaviorState: policy\n        behaviors: [{ id: policy, use: " + outcomesDef + " }]\n"
	return v4With(definitions, gameplay)
}

// outcomesSite is the site as an author writes it.
func outcomesSite() string { return outcomesSiteWith(outcomesStates, outcomesBlock, "") }

// theNote is what the engine is handed for the topic.
func theNote() map[string][]encounter.Consequence {
	return map[string][]encounter.Consequence{"back-to-gate": {{Recognize: &encounter.Recognition{
		Topic: outcomesTopic, Holder: encounter.RecognitionHolder(encounter.WhoItself),
		About: encounter.RecognitionSubjects(encounter.WhoTheOneNamed),
	}}}}
}

func TestDungeonspec_AStatesOutcomesCompileOntoTheCreatureWhoseBodyRunsIt(t *testing.T) {
	compiled := loadSource(t, outcomesSite())

	for _, id := range []string{"goblin-1", "goblin-2"} {
		require.Equal(t, theNote(), monsterNamed(t, compiled, id).Outcomes, "%s runs the definition as its body", id)
	}
}

func TestDungeonspec_AStatesNoteIsTakenInPlay(t *testing.T) {
	compiled := loadSource(t, outcomesSite())
	members := []encounter.MemberInput{{ID: outcomesVisitor, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
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
	require.False(t, noted(t, enc, outcomesCaptain, outcomesVisitor), "precondition: he holds nothing yet")

	_, err = enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: 10, Actor: outcomesVisitor, Target: outcomesCaptain, OptionID: "claim", Skill: "persuasion",
		Beaten: false, DC: 10, Total: 4, Roller: rollsOne{},
	})
	require.NoError(t, err)

	require.True(t, noted(t, enc, outcomesCaptain, outcomesVisitor),
		"a failed claim stepped him into back-to-gate, and entering it he noted that she lied")
}

// noted reports whether holder holds the topic about subject, as stored.
func noted(t *testing.T, enc *encounter.Encounter, holder, subject encounter.MemberID) bool {
	t.Helper()
	held := enc.ToData().Perception.Intel.Holdings[core.EntityID("member|"+string(holder))][intel.Subject("member|"+string(subject))]
	for topic := range held {
		if string(topic) == outcomesTopic {
			return true
		}
	}
	return false
}

func TestDungeonspec_ALineMayRequireATopicOnlyAStateGives(t *testing.T) {
	const again = "          - { id: again, say: \"Again.\", requires: { noted: caught-lying } }\n"

	t.Run("a line's requires", func(t *testing.T) {
		require.Empty(t, refusals(t, outcomesSiteWith(outcomesStates, outcomesBlock, again)))
	})
	t.Run("the same file without the outcome is refused, naming the topic", func(t *testing.T) {
		requireRefusedAt(t, refusals(t, outcomesSiteWith(outcomesStates, "", again)),
			"room.room.monsterBindings.goblin-1.options[1].requires.noted", `"caught-lying"`)
	})
	t.Run("a machine's guard", func(t *testing.T) {
		states := strings.Replace(outcomesStates, "        back-to-gate: {}\n", `        back-to-gate:
          transitions:
            - on: time
              when: { fact: { name: "noted.anyone:caught-lying", op: eq, value: true } }
              to: ready
`, 1)
		require.Empty(t, refusals(t, outcomesSiteWith(states, outcomesBlock, "")))
		requireRefusedAt(t, refusals(t, outcomesSiteWith(states, "", "")),
			outcomesAt[:len("behaviorDefinitions."+outcomesDef)]+".recipe.states.back-to-gate.transitions[0].when", `"caught-lying"`)
	})
}

func TestDungeonspec_AStatesOutcomesAreRefusedWhereTheyAreWrong(t *testing.T) {
	giveWith := func(to, about, extra string) string {
		return "    outcomes:\n      back-to-gate:\n        - give:\n            topic: caught-lying\n            to: " + to +
			"\n            about: " + about + "\n" + extra
	}
	at := outcomesAt + ".back-to-gate[0]"
	cases := []struct{ name, states, outcomes, path, sentence string }{
		{
			name: "a state the recipe lacks", states: outcomesStates,
			outcomes: "    outcomes:\n      nowhere:\n" + outcomesGive, path: outcomesAt + ".nowhere",
			sentence: "\"nowhere\" is not a state of this definition's recipe",
		},
		{
			name: "an empty list", states: outcomesStates,
			outcomes: "    outcomes:\n      back-to-gate: []\n", path: outcomesAt + ".back-to-gate",
			sentence: "an outcome lists what follows entering the state: write at least one `give`",
		},
		{
			name: "a state nothing steps into", states: outcomesStates + "        idle: {}\n",
			outcomes: "    outcomes:\n      idle:\n" + outcomesGive, path: outcomesAt + ".idle",
			sentence: "nothing steps into \"idle\", so what it gives could never happen",
		},
		{
			name: "two kinds", states: outcomesStates,
			outcomes: outcomesBlock + "          arrest: { cell: room-1-region, area: { regions: [room-1-region] } }\n", path: at,
			sentence: "a consequence is one kind, not two",
		},
		{
			name: "an arrest", states: outcomesStates,
			outcomes: "    outcomes:\n      back-to-gate:\n        - arrest: { cell: room-1-region, area: { regions: [room-1-region] } }\n", path: at,
			sentence: "an arrest follows a conversation entering a state, never a machine's",
		},
		{
			name: "a jail", states: outcomesStates,
			outcomes: "    outcomes:\n      back-to-gate:\n        - jail: { words: Away with him. }\n", path: at,
			sentence: "a jail follows a conversation entering a state, never a machine's",
		},
		{
			name: "no kind", states: outcomesStates,
			outcomes: "    outcomes:\n      back-to-gate:\n        - {}\n", path: at,
			sentence: "a state's consequence is `give: { topic, to: itself, about: the_one_named }`",
		},
		{
			name: "no topic", states: outcomesStates,
			outcomes: strings.Replace(outcomesBlock, "            topic: caught-lying\n", "", 1), path: at + ".give.topic",
			sentence: "a give names the topic given: write `topic: <topic>`",
		},
		{
			name: "a taker that is not itself", states: outcomesStates,
			outcomes: giveWith("spoken_to", "the_one_named", ""), path: at + ".give.to",
			sentence: "a state's note is given `to: itself`, the creature whose machine stepped",
		},
		{
			name: "a subject that is not the one the beat names", states: outcomesStates,
			outcomes: giveWith("itself", "speaker", ""), path: at + ".give.about",
			sentence: "the subjects are `the_one_named`, the one the beat names, or `party_in_sight`, the party of the one the beat names that it sees, or `the_gone`, the party of the one the beat names that it saw and does not see now",
		},
		{
			name: "heard_by", states: outcomesStates,
			outcomes: giveWith("itself", "the_one_named", "            heard_by: carriers_who_witnessed\n"), path: at + ".give.heard_by",
			sentence: "a state's note has no line, so nobody else hears it: omit heard_by",
		},
		{
			name: "the one the beat names, on a beat that names nobody",
			states: strings.Replace(outcomesStates, "on: heard\n              when: { fact: { name: heard.claim, op: eq, value: failed } }",
				"on: time", 1),
			outcomes: outcomesBlock, path: at + ".give.about",
			sentence: "\"back-to-gate\" is entered on `time`, which names nobody, so `the_one_named` has no one to be about: it is read on heard, seen, topic, barred and approached",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, outcomesSiteWith(tc.states, tc.outcomes, "")), tc.path, tc.sentence)
		})
	}

	// `sceneMove` is retired: the decoder refuses it at its line, before any
	// state's outcome is checked, so it is not a row of the table above.
	t.Run("a scene move is retired", func(t *testing.T) {
		requireLoadRefused(t, outcomesSiteWith(outcomesStates,
			"    outcomes:\n      back-to-gate:\n        - sceneMove: { leave: gone }\n", ""), sceneMoveRetired)
	})

	t.Run("a sequence has none", func(t *testing.T) {
		raw := string(readFixture(t, "testdata/behavior-patrol-consumer.yaml"))
		const sequence = "    end: repeat\n"
		require.Equal(t, 1, strings.Count(raw, sequence))
		source := strings.Replace(raw, sequence, sequence+"    outcomes:\n      walk:\n"+outcomesGive, 1)
		requireRefusedAt(t, refusals(t, source), "behaviorDefinitions.patrol.outcomes",
			"outcomes are written on a recipe's states; a sequence has none")
	})

	t.Run("a definition with outcomes that is not the body", func(t *testing.T) {
		source := strings.Replace(outcomesSite(), "      goblin-2:\n        behaviorState: policy\n        behaviors: [{ id: policy, use: "+outcomesDef+" }]\n",
			"      goblin-2:\n        behaviorState: calm\n        behaviors: [{ id: calm, use: calm }, { id: policy, use: "+outcomesDef+" }]\n", 1)
		source = strings.Replace(source, "behaviorDefinitions:\n", "behaviorDefinitions:\n  calm:\n    recipe:\n"+
			"      ref: site:behaviors:calm\n      initial: still\n      states:\n        still: {}\n", 1)
		requireRefusedAt(t, refusals(t, source), "room.room.monsterBindings.goblin-2.behaviors[1].use",
			"`"+outcomesDef+"` gives on entering a state, and only the machine that moves the creature does that: "+
				"name it as `behaviorState`, or remove its outcomes")
	})

	t.Run("apply_status is not a key of a state's consequence", func(t *testing.T) {
		source := strings.Replace(outcomesBlock, "        - give:", "        - apply_status: { status: held, to: speaker }\n          give:", 1)
		_, err := dungeonspec.Load([]byte(outcomesSiteWith(outcomesStates, source, "")))
		require.Error(t, err)
		t.Log(err)
		require.Contains(t, err.Error(), "apply_status")
	})
}
