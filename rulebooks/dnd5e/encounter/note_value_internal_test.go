// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// note_value_internal_test.go pins A NOTE CARRIES A VALUE (rpg-project#1007,
// slice rpg-project#1112): a give may carry a plain string, a newer value
// replaces the older, the same value only confirms, and a line's requirement
// or a post's pass may ask for a value. Internal, because the intel store and
// its payload are the composition's own.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

const (
	nvTopic = TopicID("standing")
	nvSaid  = "nv-said"
)

// nvLine is a checked line whose failure row gives nvTopic as `as`, noted by
// the one spoken to about the speaker.
func nvLine(id string, topic TopicID, as string) Option {
	return Option{
		ID: id, Say: "Hear me.", Check: []CheckApproach{{Ability: "persuasion", DC: soCheckDC}},
		Success: []Answer{{Weight: 1, Say: "Go on."}}, Failure: []Answer{{Weight: 1, Say: "No."}},
		Grants: []RowGrant{{Row: 0, Failure: true, Consequence: Consequence{Recognize: &Recognition{
			Topic: topic, Holder: RecognitionSpokenTo, About: RecognitionSpeaker, As: as,
		}}}},
	}
}

// nvAsk is a plain line the guard offers only when its requirement holds.
func nvAsk(id string, requires ...Trigger) Option {
	return Option{ID: id, Say: "Well?", Requires: requires, Success: []Answer{{Weight: 1, Say: "Hm."}}}
}

// nvSay has alice say a checked line to the guard and miss.
func nvSay(t *testing.T, enc *Encounter, option string) {
	t.Helper()
	_, err := enc.Speak(context.Background(), &SpeakInput{
		Range: 10, Actor: "alice", Target: soGuard, OptionID: option, Roller: wakeRollsOne{},
		Skill: "persuasion", Beaten: false, DC: soCheckDC, Total: 4,
	})
	require.NoError(t, err)
}

// nvHeld is the value the guard holds on topic about alice, and whether he holds it.
func nvHeld(t *testing.T, enc *Encounter, topic TopicID) (string, bool) {
	t.Helper()
	held, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(topic))
	if err != nil {
		return "", false
	}
	var why recognitionTestimony
	require.NoError(t, json.Unmarshal(held.Payload, &why))

	return why.As, true
}

// nvOffered is the ids of the lines the guard offers alice.
func nvOffered(t *testing.T, enc *Encounter) []string {
	t.Helper()
	got, err := enc.OptionsFor("alice", soGuard)
	require.NoError(t, err)
	var ids []string
	for _, o := range got {
		ids = append(ids, o.ID)
	}

	return ids
}

// nvGuard is the soGuard with the lines given besides its own claim.
func nvGuard(t *testing.T, lines ...Option) MemberInput {
	t.Helper()

	return soGuardIn(t, nil, lines...)
}

func TestNoteValueAFailureRowGivesATopicWithAValue(t *testing.T) {
	enc := soHall(t, everyoneSeesTheWholeMap{}, nvGuard(t, nvLine(nvSaid, nvTopic, "warned")))

	nvSay(t, enc, nvSaid)

	got, held := nvHeld(t, enc, nvTopic)
	require.True(t, held, "the one spoken to noted the topic about the speaker")
	require.Equal(t, "warned", got, "and holds it with the value the row gave")
}

func TestNoteValueANewerValueReplacesAndTheSameValueConfirms(t *testing.T) {
	enc := soHall(t, everyoneSeesTheWholeMap{}, nvGuard(t,
		nvLine("warn", nvTopic, "warned"), nvLine("bar", nvTopic, "barred"), nvLine("warn-again", nvTopic, "warned")))

	nvSay(t, enc, "warn")
	first, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(nvTopic))
	require.NoError(t, err)

	nvSay(t, enc, "warn-again")
	again, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(nvTopic))
	require.NoError(t, err)
	require.Equal(t, first.Payload, again.Payload, "the same value: the first record is kept, payload and all")
	require.Equal(t, 1, soHoldings(t, enc, soGuard, nvTopic), "one record, not two")

	nvSay(t, enc, "bar")
	got, held := nvHeld(t, enc, nvTopic)
	require.True(t, held)
	require.Equal(t, "barred", got, "a different value replaces the held one")
	require.Equal(t, 1, soHoldings(t, enc, soGuard, nvTopic), "still one record about her")
}

func TestNoteValueTheValuelessGiveStillKeepsTheFirstRecord(t *testing.T) {
	enc := soHall(t, everyoneSeesTheWholeMap{}, nvGuard(t, nvLine("a", nvTopic, ""), nvLine("b", nvTopic, "")))
	nvSay(t, enc, "a")
	first, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(nvTopic))
	require.NoError(t, err)

	nvSay(t, enc, "b")

	again, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(nvTopic))
	require.NoError(t, err)
	require.Equal(t, first.Payload, again.Payload, "two empty values are the same: the first record is kept")
	var why recognitionTestimony
	require.NoError(t, json.Unmarshal(again.Payload, &why))
	require.Equal(t, "a", why.Option, "and it still names the line that first gave it")
}

func TestNoteValueARequirementAsksForAValueOrForAny(t *testing.T) {
	anyOne := TriggerRecognized{Topic: nvTopic}
	asWarned := TriggerRecognized{Topic: nvTopic, As: "warned"}
	asBarred := TriggerRecognized{Topic: nvTopic, As: "barred"}
	enc := soHall(t, everyoneSeesTheWholeMap{}, nvGuard(t,
		nvLine("warn", nvTopic, "warned"), nvLine("bar", nvTopic, "barred"),
		nvAsk("any", anyOne), nvAsk("not-any", TriggerNot{Inner: anyOne}),
		nvAsk("warned", asWarned), nvAsk("not-warned", TriggerNot{Inner: asWarned}),
		nvAsk("barred", asBarred), nvAsk("not-barred", TriggerNot{Inner: asBarred}),
	))
	require.ElementsMatch(t, []string{soLine, "warn", "bar", "not-any", "not-warned", "not-barred"}, nvOffered(t, enc),
		"nothing noted: no requirement is met, every `not` is")

	nvSay(t, enc, "warn")
	require.ElementsMatch(t, []string{soLine, "warn", "bar", "any", "warned", "not-barred"}, nvOffered(t, enc),
		"noted warned: any and warned are met; barred is not, and its `not` is")

	nvSay(t, enc, "bar")
	require.ElementsMatch(t, []string{soLine, "warn", "bar", "any", "barred", "not-warned"}, nvOffered(t, enc),
		"replaced by barred: the answers turn over")
}

// nvPostedHall is the sided hall with a guard whose post lets through only
// the walker noted as `ask` on nvTopic; the guard carries one plain line per
// value, each giving it on success.
func nvPostedHall(t *testing.T, ask string) *Encounter {
	t.Helper()
	g := recognisingGuard(t, postedGuard, holdNorthWest, false)
	for state, p := range g.Posts {
		p.Pass = []Trigger{TriggerRecognized{Topic: nvTopic, As: ask}}
		g.Posts[state] = p
	}
	for _, as := range []string{"warned", "barred"} {
		g.Options = append(g.Options, Option{
			ID: "note-" + as, Say: "Noted.", Success: []Answer{{Weight: 1, Say: "Hm."}},
			Grants: []RowGrant{{Row: 0, Consequence: Consequence{Recognize: &Recognition{
				Topic: nvTopic, Holder: RecognitionSpokenTo, About: RecognitionSpeaker, As: as,
			}}}},
		})
	}

	return recognisingHall(t, false, g)
}

func TestNoteValueAPostsPassAsksForAValue(t *testing.T) {
	note := func(t *testing.T, enc *Encounter, as string) {
		t.Helper()
		_, err := enc.Speak(context.Background(), &SpeakInput{
			Actor: "alice", Target: postedGuard, OptionID: "note-" + as, Roller: wakeRollsOne{}, Range: 3,
		})
		require.NoError(t, err)
	}

	t.Run("the walker noted with the value is let through", func(t *testing.T) {
		enc := nvPostedHall(t, "warned")
		note(t, enc, "warned")
		steps(t, enc, holdNorthEast, "noted warned, she crosses")
	})

	t.Run("the walker noted with another value is barred", func(t *testing.T) {
		enc := nvPostedHall(t, "warned")
		note(t, enc, "barred")
		out, err := enc.Step(&StepInput{Member: "alice", To: holdNorthEast})
		requireStopped(t, out, err, "noted barred, she is stopped")
	})

	t.Run("a newer value takes the pass away", func(t *testing.T) {
		enc := nvPostedHall(t, "warned")
		note(t, enc, "warned")
		note(t, enc, "barred")
		out, err := enc.Step(&StepInput{Member: "alice", To: holdNorthEast})
		requireStopped(t, out, err, "replaced by barred, she is stopped")
	})
}

// TestNoteValueAReplacedValueLandsNoTopicBeat pins that only a FIRST note
// lands the `topic` beat on a listening holder; a value that replaces one
// lands none. The next slice may change this (rpg-project#1007).
func TestNoteValueAReplacedValueLandsNoTopicBeat(t *testing.T) {
	held := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineTopic(string(nvTopic)), Op: machine.OpEq, Value: true}}
	g := nvGuard(t, nvLine("warn", nvTopic, "warned"), nvLine("bar", nvTopic, "barred"))
	g.Behaviors.Instances[0].Instance.Recipe = *recipe(t, "calm", map[string]machine.State{
		"calm":   {Transitions: []machine.Transition{{On: beatTopic, When: held, To: "first", Reason: "a topic"}}},
		"first":  {Transitions: []machine.Transition{{On: beatTopic, When: held, To: "second", Reason: "another topic"}}},
		"second": {},
	})
	enc := soHall(t, everyoneSeesTheWholeMap{}, g)

	nvSay(t, enc, "warn")
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "first", stateOf(enc, soGuard), "a first note lands the topic beat on a holder that listens")

	nvSay(t, enc, "bar")
	require.NoError(t, enc.spendWorldAction("alice"))
	got, _ := nvHeld(t, enc, nvTopic)
	require.Equal(t, "barred", got, "the value did change")
	require.Equal(t, "first", stateOf(enc, soGuard), "but a replaced value lands no beat of its own")
}

// TestNoteValueAFirstLieNeverCountsAsTheSecond pins the order (F6): in one
// Speak a listening machine hears the line BEFORE the line's own failure row
// gives its note, so a guard who steps on `heard` when the claim failed and he
// has noted the speaker finds nothing noted the first time.
func TestNoteValueAFirstLieNeverCountsAsTheSecond(t *testing.T) {
	noted := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNoted(string(nvTopic)), Op: machine.OpEq, Value: true}}
	r := recipe(t, "calm", map[string]machine.State{
		"calm": {Transitions: []machine.Transition{
			{On: beatHeard, When: &machine.Guard{All: []machine.Guard{*soClaimIs(ClaimFailed), *noted}}, To: "wary", Reason: "lied again"},
		}},
		"wary": {},
	})
	g := soListener(t, soGuard, postedPost.X, postedPost.Y, r, nil)
	g.Name = "Gate guard"
	g.Options = []Option{nvLine(soLine, nvTopic, "warned")}
	g.Options[0].Check = []CheckApproach{{Ability: "persuasion", DC: soCheckDC}}
	enc := soHall(t, everyoneSeesTheWholeMap{}, g)

	nvSay(t, enc, soLine)
	_, held := nvHeld(t, enc, nvTopic)
	require.True(t, held, "the first lie is noted")
	require.Equal(t, "calm", stateOf(enc, soGuard), "but he heard it before he noted it: not stepped")

	nvSay(t, enc, soLine)
	require.Equal(t, "wary", stateOf(enc, soGuard), "the second lie finds it noted: stepped")
}

func TestNoteValueRefusals(t *testing.T) {
	nvOutcome := func(topic TopicID, as string) map[string][]Consequence {
		return map[string][]Consequence{"wary": {{Recognize: &Recognition{
			Topic: topic, Holder: RecognitionHolder(WhoItself), About: RecognitionSubjects(WhoTheOneNamed), As: as,
		}}}}
	}
	refusesRow := func(t *testing.T, g MemberInput, sentence string) {
		t.Helper()
		_, err := NewEncounter(postedSetup(everyoneSeesTheWholeMap{}, g))
		require.Error(t, err)
		require.Contains(t, err.Error(), sentence)
	}

	t.Run("a value on an engine's own topic", func(t *testing.T) {
		for _, topic := range []TopicID{TopicArrested, TopicWanted, TopicJailed} {
			sentence := `gives "` + string(topic) + `" as "x": "` + string(topic) + `" is the engine's own topic and has no value`

			t.Run("a line's row "+string(topic), func(t *testing.T) {
				refusesRow(t, nvGuard(t, nvLine("x", topic, "x")), sentence)
			})

			t.Run("a state's outcome "+string(topic), func(t *testing.T) {
				err := validateStateOutcomes(soRecipe(t), nvOutcome(topic, "x"))
				require.Error(t, err)
				require.Contains(t, err.Error(), sentence)
			})

			t.Run("a conversation's outcome "+string(topic), func(t *testing.T) {
				err := validateOutcomes(hailGate, map[string]bool{"wary": true}, map[string][]Consequence{"wary": {{Recognize: &Recognition{
					Topic: topic, Holder: RecognitionSpokenTo, About: RecognitionPartyInSight, As: "x",
				}}}})
				require.Error(t, err)
				require.Contains(t, err.Error(), sentence)
			})
		}
	})

	t.Run("a topic given with a value and without", func(t *testing.T) {
		sentence := `the topic "standing" is given as "warned" in one place and with no value in another: ` +
			`a topic has a value everywhere it is given, or nowhere`
		refusesRow(t, nvGuard(t, nvLine("with", nvTopic, "warned"), nvLine("without", nvTopic, "")), sentence)
		// a machine's state gives it too
		g := soGuardIn(t, nvOutcome(nvTopic, ""), nvLine("with", nvTopic, "warned"))
		refusesRow(t, g, sentence)
		// two different values are fine
		_, err := NewEncounter(postedSetup(everyoneSeesTheWholeMap{},
			nvGuard(t, nvLine("w", nvTopic, "warned"), nvLine("b", nvTopic, "barred"))))
		require.NoError(t, err)
	})

	t.Run("a requirement for a value nothing gives", func(t *testing.T) {
		sentence := `asks whether "standing" is noted as "exiled", and nothing gives it that value`
		ask := TriggerRecognized{Topic: nvTopic, As: "exiled"}
		refusesRow(t, nvGuard(t, nvLine("with", nvTopic, "warned"), nvAsk("ask", ask)), sentence)
		refusesRow(t, nvGuard(t, nvLine("with", nvTopic, "warned"), nvAsk("ask", TriggerNot{Inner: ask})), sentence)
		refusesRow(t, nvGuard(t, nvLine("plain", nvTopic, ""), nvAsk("ask", ask)), sentence)

		g := recognisingGuard(t, postedGuard, holdNorthWest, false)
		g.Options = []Option{nvLine("with", nvTopic, "warned")}
		for state, p := range g.Posts {
			p.Pass = []Trigger{ask}
			g.Posts[state] = p
		}
		_, err := NewEncounter(recognisingSetup(t, false, g))
		require.Error(t, err)
		require.Contains(t, err.Error(), sentence)
	})
}

func TestNoteValueSurvivesSaveAndLoad(t *testing.T) {
	asWarned := TriggerRecognized{Topic: nvTopic, As: "warned"}
	in := postedSetup(everyoneSeesTheWholeMap{}, nvGuard(t,
		nvLine("warn", nvTopic, "warned"), nvAsk("warned", asWarned), nvAsk("any", TriggerRecognized{Topic: nvTopic})))
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	nvSay(t, enc, "warn")

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
	require.NoError(t, err)

	got, held := nvHeld(t, loaded, nvTopic)
	require.True(t, held)
	require.Equal(t, "warned", got, "the note keeps its value")
	require.ElementsMatch(t, []string{soLine, "warn", "warned", "any"}, nvOffered(t, loaded), "a loaded run reads the value")

	var saved []Trigger
	for _, o := range loaded.members[soGuard].Options {
		if o.ID == "warned" || o.ID == "any" {
			saved = append(saved, o.Requires...)
		}
	}
	require.ElementsMatch(t, []Trigger{asWarned, TriggerRecognized{Topic: nvTopic}}, saved,
		"the requirement keeps its value, and a requirement saved with none loads as before")

	// A saved requirement with no `as` carries no key.
	valueless, err := json.Marshal(triggerDataFrom(TriggerRecognized{Topic: nvTopic}))
	require.NoError(t, err)
	require.NotContains(t, string(valueless), `"as"`)
}
