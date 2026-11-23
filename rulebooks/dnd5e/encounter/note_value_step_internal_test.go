// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// note_value_step_internal_test.go pins A MACHINE STEP READS THE VALUE OF A
// NOTE (rpg-project#1007, slice rpg-project#1113): `noted.as:<topic>` asks
// what this creature has noted, as a value, about the one the beat names, and
// always answers known. Internal, because the intel store and the machine blob
// are the composition's own.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	nsHall = "in-hall"
	nsGate = "back-to-gate"
)

// nsNotedAs is the guard fact `noted.as:standing` equals as.
func nsNotedAs(as string) *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNotedAs(string(nvTopic)), Op: machine.OpEq, Value: as}}
}

// nsTwoStrikes is the guard's recipe: in either state, a lie from someone he
// has already noted as warned sends him back to his gate, where his state
// notes that person as barred.
func nsTwoStrikes(t *testing.T) (*machine.Recipe, map[string][]Consequence) {
	t.Helper()
	again := func(from string) machine.State {
		return machine.State{Transitions: []machine.Transition{{
			On:   beatHeard,
			When: &machine.Guard{All: []machine.Guard{*soClaimIs(ClaimFailed), *nsNotedAs("warned")}},
			To:   nsGate, Reason: "lied again, from " + from,
		}}}
	}
	r := recipe(t, nsHall, map[string]machine.State{nsHall: again(nsHall), nsGate: again(nsGate)})
	outcomes := map[string][]Consequence{nsGate: {{Recognize: &Recognition{
		Topic: nvTopic, Holder: RecognitionHolder(WhoItself), About: RecognitionSubjects(WhoTheOneNamed), As: "barred",
	}}}}

	return r, outcomes
}

// nsGuardWith is the guard running r, with the first lie noting `warned` and
// a second line that gives nothing, so only the note decides.
func nsGuardWith(t *testing.T, r *machine.Recipe, outcomes map[string][]Consequence) MemberInput {
	t.Helper()
	g := soListener(t, soGuard, postedPost.X, postedPost.Y, r, outcomes)
	g.Name = "Gate guard"
	again := nvLine("lie-again", nvTopic, "")
	again.Grants = nil
	g.Options = []Option{nvLine("lie", nvTopic, "warned"), again}

	return g
}

// nsPair is the hall with alice and bob in it and the two-strikes guard.
func nsPair(t *testing.T) (*SetupInput, *Encounter) {
	t.Helper()
	r, outcomes := nsTwoStrikes(t)
	in := postedSetup(everyoneSeesTheWholeMap{}, nsGuardWith(t, r, outcomes),
		MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}})
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return in, enc
}

// nsSay has the speaker say a checked line to the guard and miss.
func nsSay(t *testing.T, enc *Encounter, speaker MemberID, option string) {
	t.Helper()
	_, err := enc.Speak(context.Background(), &SpeakInput{
		Range: 10, Actor: speaker, Target: soGuard, OptionID: option, Roller: wakeRollsOne{},
		Skill: "persuasion", Beaten: false, DC: soCheckDC, Total: 4,
	})
	require.NoError(t, err)
}

// nsHeld is the value the guard holds on the topic about subject.
func nsHeld(t *testing.T, enc *Encounter, subject MemberID) (string, bool) {
	t.Helper()
	held, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember(subject), perceptionTopic(nvTopic))
	if err != nil {
		return "", false
	}
	var why recognitionTestimony
	require.NoError(t, json.Unmarshal(held.Payload, &why))

	return why.As, true
}

func requireTwoStrikes(t *testing.T, enc *Encounter, alsoBeforeBob func(*Encounter) *Encounter) {
	t.Helper()
	nsSay(t, enc, "alice", "lie")
	got, _ := nsHeld(t, enc, "alice")
	require.Equal(t, nsHall, stateOf(enc, soGuard), "a first lie finds nothing noted: he stays in the hall")
	require.Equal(t, "warned", got, "and notes her as warned")

	if alsoBeforeBob != nil {
		enc = alsoBeforeBob(enc)
	}
	nsSay(t, enc, "alice", "lie-again")
	got, _ = nsHeld(t, enc, "alice")
	require.Equal(t, nsGate, stateOf(enc, soGuard), "her second lie finds her warned: he goes back to his gate")
	require.Equal(t, "barred", got, "and his state notes her as barred")

	nsSay(t, enc, "bob", "lie")
	bob, _ := nsHeld(t, enc, "bob")
	alice, _ := nsHeld(t, enc, "alice")
	require.Equal(t, "warned", bob, "the count is the person's: bob is only warned")
	require.Equal(t, "barred", alice, "and alice is still barred")

	nsSay(t, enc, "bob", "lie-again")
	bob, _ = nsHeld(t, enc, "bob")
	require.Equal(t, "barred", bob, "bob's second lie bars him: the step re-entered the state")
}

func TestNoteValueStepTwoStrikesBarsTheSecondLieOfThePersonNotTheFirst(t *testing.T) {
	_, enc := nsPair(t)

	requireTwoStrikes(t, enc, nil)
}

func TestNoteValueStepASavedRunStepsOnTheSecondLie(t *testing.T) {
	in, enc := nsPair(t)

	requireTwoStrikes(t, enc, func(live *Encounter) *Encounter {
		raw, err := json.Marshal(live.ToData())
		require.NoError(t, err)
		var data EncounterData
		require.NoError(t, json.Unmarshal(raw, &data))
		loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
		require.NoError(t, err)

		return loaded
	})
}

func TestNoteValueStepANotOfAnUnnotedValueIsKnownAndApplies(t *testing.T) {
	notWarned := &machine.Guard{Not: nsNotedAs("warned")}
	r := recipe(t, "calm", map[string]machine.State{
		"calm":     {Transitions: []machine.Transition{{On: beatHeard, When: notWarned, To: "unwarned", Reason: "not warned"}}},
		"unwarned": {},
	})
	g := soListener(t, soGuard, postedPost.X, postedPost.Y, r, nil)
	g.Options = []Option{nvLine("lie", nvTopic, "warned")}
	enc := soHall(t, everyoneSeesTheWholeMap{}, g)

	nsSay(t, enc, "alice", "lie")

	require.Equal(t, "unwarned", stateOf(enc, soGuard),
		"nothing was noted yet when he heard it: the value is the empty string, known, and not-warned holds")
}

func TestNoteValueStepNotedStillAnswersYesForANoteWithAValue(t *testing.T) {
	noted := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNoted(string(nvTopic)), Op: machine.OpEq, Value: true}}
	r := recipe(t, "calm", map[string]machine.State{
		"calm": {Transitions: []machine.Transition{
			{On: beatHeard, When: &machine.Guard{All: []machine.Guard{*soClaimIs(ClaimFailed), *noted}}, To: "wary", Reason: "again"},
		}},
		"wary": {},
	})
	g := soListener(t, soGuard, postedPost.X, postedPost.Y, r, nil)
	g.Options = []Option{nvLine("lie", nvTopic, "warned")}
	enc := soHall(t, everyoneSeesTheWholeMap{}, g)

	nsSay(t, enc, "alice", "lie")
	require.Equal(t, "calm", stateOf(enc, soGuard), "the first lie finds nothing noted")
	nsSay(t, enc, "alice", "lie")

	require.Equal(t, "wary", stateOf(enc, soGuard), "`noted:<topic>` is true for a note that carries a value")
}

func TestNoteValueStepABeatThatNamesNobodyIsRefusedBySentence(t *testing.T) {
	const sentence = "`noted.as:standing` asks about the one this beat is about, and `time` is about nobody: " +
		"it is read on heard, seen, barred and approached"

	require.Equal(t, sentence, GuardWordsRefusal(beatTime, nsNotedAs("warned")))
	require.Empty(t, GuardWordsRefusal(beatHeard, nsNotedAs("warned")), "on a beat that names someone it is sound")
	err := ValidateMachine(wordsRecipe(t, machine.Transition{On: beatTime, When: nsNotedAs("warned")}))
	require.Error(t, err)
	require.Contains(t, err.Error(), sentence)
}

func TestNoteValueStepTheWordsTablePublishesTheWord(t *testing.T) {
	word := publishedCondition(t, "noted.as:")

	require.True(t, word.Fact)
	require.Equal(t, "has noted {id} as {value} about them", word.Label)
	require.Equal(t, "has not noted {id} as {value} about them", word.LabelNot)
	require.Equal(t, "{who} {verb} {id} as {value} about {about}", word.Long)
	require.Equal(t, publishedCondition(t, "noted:").On, word.On, "it is read where `noted:` is read")

	places := map[string]bool{}
	for _, w := range MachineWords().Who {
		places[w.Word+"@"+w.Allowed] = true
	}
	require.True(t, places[WhoTheOneNamed+"@"+WhoInNotedAs], "`the_one_named` is a who-word where `noted.as:` is written")
}
