// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// state_outcomes_internal_test.go pins A CREATURE'S MACHINE GIVES A NOTE ON
// ENTERING A STATE (rpg-project#989, rpg-project#1103): the engine half of
// "a state does things once on entering". A guard whose machine steps into a
// state that has [MemberInput.Outcomes] notes the topic about the one the beat
// names, inside the same verb, told to the guard alone. Internal, because the
// intel store, the machine blob and the landed beats are the composition's own.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	soGuard    = MemberID("so-guard")
	soBlind    = MemberID("so-blind")
	soBystand  = MemberID("so-bystander")
	soLied     = TopicID("lied")
	soLine     = "claim"
	soCheckDC  = 10
	soBeatNote = "recognized"
)

// perceptionTopic is a topic as the intel store spells it.
func perceptionTopic(t TopicID) perception.Topic { return perception.Topic(t) }

// soSight is a sight model with a range per member and no limit for anybody
// it does not name.
type soSight map[MemberID]int

func (s soSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 1000
		if cells, ok := s[id]; ok {
			out[id] = cells
		}
	}

	return out, nil
}

// soClaimIs is the guard fact "the claim just heard was <verdict>".
func soClaimIs(verdict string) *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: factHeardClaim, Op: machine.OpEq, Value: verdict}}
}

// soRecipe is a listener that goes calm → wary on a failed claim, stays wary
// (and re-enters it) on the next failed claim, and goes back to calm on a
// claim that held.
func soRecipe(t *testing.T) *machine.Recipe {
	t.Helper()

	return recipe(t, "calm", map[string]machine.State{
		"calm": {Transitions: []machine.Transition{
			{On: beatHeard, When: soClaimIs(ClaimFailed), To: "wary", Reason: "lied to"},
		}},
		"wary": {Transitions: []machine.Transition{
			{On: beatHeard, When: soClaimIs(ClaimFailed), To: "wary", Reason: "lied to again"},
			{On: beatHeard, When: soClaimIs(ClaimHeld), To: "calm", Reason: "convinced"},
		}},
	})
}

// soNote is the one consequence a state may give: the creature notes the
// topic about the one the beat names.
func soNote(topic TopicID) Consequence {
	return Consequence{Recognize: &Recognition{
		Topic: topic, Holder: RecognitionHolder(WhoItself), About: RecognitionSubjects(WhoTheOneNamed),
	}}
}

// soNotes is [soNote] on the state given.
func soNotes(state string, topic TopicID) map[string][]Consequence {
	return map[string][]Consequence{state: {soNote(topic)}}
}

// soListener is a garrison creature at (col,row) running r with outcomes.
func soListener(t *testing.T, id MemberID, col, row float64, r *machine.Recipe, outcomes map[string][]Consequence) MemberInput {
	t.Helper()
	m := listener(id, col, row, r)
	m.Faction = postedGarrison
	m.Outcomes = outcomes

	return m
}

// soGuardIn is the guard the party speaks to: it carries the checked claim.
func soGuardIn(t *testing.T, outcomes map[string][]Consequence, extra ...Option) MemberInput {
	t.Helper()
	g := soListener(t, soGuard, postedPost.X, postedPost.Y, soRecipe(t), outcomes)
	g.Name = "Gate guard"
	g.Options = append([]Option{{
		ID: soLine, Say: "We're expected.", Check: []CheckApproach{{Ability: "persuasion", DC: soCheckDC}},
		Success: []Answer{{Weight: 1, Say: "Go on."}}, Failure: []Answer{{Weight: 1, Say: "Liar."}},
	}}, extra...)

	return g
}

// soHall opens the posted hall with the sight given.
func soHall(t *testing.T, sight Sight, members ...MemberInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(postedSetup(sight, members...))
	require.NoError(t, err)

	return enc
}

// soClaim has alice make her claim to the guard; beaten says whether it held.
func soClaim(t *testing.T, enc *Encounter, beaten bool) {
	t.Helper()
	total := 4
	if beaten {
		total = 15
	}
	_, err := enc.Speak(context.Background(), &SpeakInput{
		Range: 10, Actor: "alice", Target: soGuard, OptionID: soLine, Roller: wakeRollsOne{},
		Skill: "persuasion", Beaten: beaten, DC: soCheckDC, Total: total,
	})
	require.NoError(t, err)
}

// soHoldings is how many records holder holds on topic, any subject.
func soHoldings(t *testing.T, enc *Encounter, holder MemberID, topic TopicID) int {
	t.Helper()
	held, err := enc.intelLog.OnTopic(sightMember(holder), perceptionTopic(topic))
	if err != nil {
		return 0
	}

	return len(held)
}

// soToldNotes is the recognized beats one member is told, decoded.
func soToldNotes(t *testing.T, enc *Encounter, audience MemberID) []map[string]any {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: audience})
	require.NoError(t, err)
	var out []map[string]any
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == soBeatNote {
			out = append(out, beat)
		}
	}

	return out
}

// TestEnteringAStateNotesTheOneItHeard: a failed claim steps the guard from
// calm to wary; entering wary, he notes `lied` about the one he heard.
func TestEnteringAStateNotesTheOneItHeard(t *testing.T) {
	enc := soHall(t, everyoneSeesTheWholeMap{}, soGuardIn(t, soNotes("wary", soLied)))
	require.False(t, enc.recognizes(soGuard, "alice", soLied), "precondition: he holds nothing yet")

	soClaim(t, enc, false)

	require.Equal(t, "wary", stateOf(enc, soGuard))
	require.True(t, enc.recognizes(soGuard, "alice", soLied), "entering wary, he noted that she lied")
	held, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(soLied))
	require.NoError(t, err)
	var why recognitionTestimony
	require.NoError(t, json.Unmarshal(held.Payload, &why))
	require.Equal(t, recognitionTestimony{State: "wary"}, why,
		"the testimony names the state entered, and no interaction and no option")
}

// TestEnteringAStateOnABumpNotesTheWalker: the `barred` path runs through
// selectDecision; the one who walked into the guard is the one he notes.
func TestEnteringAStateOnABumpNotesTheWalker(t *testing.T) {
	g := bumpGuard(t)
	g.Outcomes = soNotes("challenged", "bumped")
	enc := postedHall(t, g)
	require.False(t, enc.recognizes(postedGuard, "alice", "bumped"))

	bumped(t, enc, postedPost, postedGuard)

	require.Equal(t, "challenged", stateOf(enc, postedGuard))
	require.True(t, enc.recognizes(postedGuard, "alice", "bumped"), "he noted the one who walked into him")
}

// TestAHeardLineFromSomeoneUnseenNotesNobody: the same step with the speaker
// out of the listener's sight. The words are caught and the state is entered,
// but the beat names nobody, so the note is written about nobody.
func TestAHeardLineFromSomeoneUnseenNotesNobody(t *testing.T) {
	enc := soHall(t, soSight{soBlind: 0},
		soGuardIn(t, soNotes("wary", soLied)),
		soListener(t, soBlind, 3, 2, soRecipe(t), soNotes("wary", soLied)),
	)

	soClaim(t, enc, false)

	require.Equal(t, "wary", stateOf(enc, soBlind), "he caught the words, so his machine stepped")
	require.Zero(t, soHoldings(t, enc, soBlind, soLied), "but he saw nobody, so he noted nobody")
	require.True(t, enc.recognizes(soGuard, "alice", soLied), "control: the one who saw her noted her")
}

// TestAStepBackIntoTheSameStateKeepsTheFirstNote: entering wary again does
// the outcomes again; the record he already holds is kept, not doubled.
func TestAStepBackIntoTheSameStateKeepsTheFirstNote(t *testing.T) {
	enc := soHall(t, everyoneSeesTheWholeMap{}, soGuardIn(t, soNotes("wary", soLied)))

	soClaim(t, enc, false)
	first, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(soLied))
	require.NoError(t, err)
	soClaim(t, enc, false)

	require.Equal(t, "wary", stateOf(enc, soGuard))
	require.Equal(t, 1, soHoldings(t, enc, soGuard, soLied), "one record, not two")
	again, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember("alice"), perceptionTopic(soLied))
	require.NoError(t, err)
	require.Equal(t, first.Payload, again.Payload, "the first one is the one kept")
	require.Len(t, soToldNotes(t, enc, soGuard), 2, "each entering is told, the second as a confirmation")
}

// TestTheFirstStateGivesNothingAtSetup: outcomes on the initial state, which
// a transition also steps into so it loads. Nothing enters it at setup.
func TestTheFirstStateGivesNothingAtSetup(t *testing.T) {
	enc := soHall(t, everyoneSeesTheWholeMap{}, soGuardIn(t, soNotes("calm", soLied)))

	require.Equal(t, "calm", stateOf(enc, soGuard))
	require.Zero(t, soHoldings(t, enc, soGuard, soLied), "the first state is entered by nothing")
	require.Empty(t, soToldNotes(t, enc, soGuard))

	soClaim(t, enc, false) // calm → wary: nothing for wary
	soClaim(t, enc, true)  // wary → calm: now he enters calm
	require.Equal(t, "calm", stateOf(enc, soGuard))
	require.True(t, enc.recognizes(soGuard, "alice", soLied), "entered by a step, calm gives its note")
}

// TestAMachinesNoteIsToldToItsTakerAlone: the beat that tells the note is
// addressed to the guard; the speaker and a bystander are not told.
func TestAMachinesNoteIsToldToItsTakerAlone(t *testing.T) {
	enc := soHall(t, everyoneSeesTheWholeMap{},
		soGuardIn(t, soNotes("wary", soLied)),
		soListener(t, soBystand, 3, 2, postedRecipe(t), nil),
	)

	soClaim(t, enc, false)

	told := soToldNotes(t, enc, soGuard)
	require.Len(t, told, 1, "the guard is told")
	require.Equal(t, string(soLied), told[0]["topic"])
	require.Equal(t, []any{"alice"}, told[0]["subjects"])
	require.Empty(t, soToldNotes(t, enc, "alice"), "the speaker is not told what he noted")
	require.Empty(t, soToldNotes(t, enc, soBystand), "nor is a bystander")
}

// TestALineMayRequireATopicOnlyAMachineGives: the guard's only giver of
// `lied` is his own state outcome, and a line he offers requires it. The site
// loads. I wrote the line to need the topic: it is not offered before the step
// and is offered after it. A second line needing `not` it is the reverse.
func TestALineMayRequireATopicOnlyAMachineGives(t *testing.T) {
	plead := Option{ID: "plead", Say: "Please.", Requires: []Trigger{TriggerRecognized{Topic: soLied}},
		Success: []Answer{{Weight: 1, Say: "Hm."}}}
	fresh := Option{ID: "fresh", Say: "Hello.", Requires: []Trigger{TriggerNot{Inner: TriggerRecognized{Topic: soLied}}},
		Success: []Answer{{Weight: 1, Say: "Hello."}}}
	enc := soHall(t, everyoneSeesTheWholeMap{}, soGuardIn(t, soNotes("wary", soLied), plead, fresh))

	offered := func() []string {
		got, err := enc.OptionsFor("alice", soGuard)
		require.NoError(t, err)
		var ids []string
		for _, o := range got {
			ids = append(ids, o.ID)
		}

		return ids
	}
	require.ElementsMatch(t, []string{soLine, "fresh"}, offered(), "before the step: not yet noted")

	soClaim(t, enc, false)

	require.ElementsMatch(t, []string{soLine, "plead"}, offered(), "after the step: noted")

	// Control: with no machine giving the topic, the same line is refused.
	bare := soGuardIn(t, nil, plead)
	_, err := NewEncounter(postedSetup(everyoneSeesTheWholeMap{}, bare))
	require.Error(t, err, "nothing grants lied")
	require.Contains(t, err.Error(), "lied")
}

// TestStateOutcomesSurviveSaveAndLoad: the outcomes are saved with the
// member and with a member still waiting, and a loaded run still notes.
func TestStateOutcomesSurviveSaveAndLoad(t *testing.T) {
	waiting := soListener(t, soBlind, 5, 2, soRecipe(t), soNotes("wary", soLied))
	waiting.Arrives = TriggerRound{Round: 99}
	enc := soHall(t, everyoneSeesTheWholeMap{}, soGuardIn(t, soNotes("wary", soLied)), waiting)

	data := enc.ToData()
	var saved *MemberData
	for i := range data.Members {
		if data.Members[i].ID == soGuard {
			saved = &data.Members[i]
		}
	}
	require.NotNil(t, saved)
	require.Equal(t, soNotes("wary", soLied), saved.Outcomes, "saved with the member")
	require.Len(t, data.Reserve, 1)
	require.Equal(t, soNotes("wary", soLied), data.Reserve[0].Outcomes, "and with the member still waiting")

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	var back EncounterData
	require.NoError(t, json.Unmarshal(raw, &back))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: back, Capabilities: postedSetup(everyoneSeesTheWholeMap{}).Capabilities})
	require.NoError(t, err)
	require.Equal(t, soNotes("wary", soLied), loaded.ToData().Reserve[0].Outcomes, "a waiting member keeps them through a load")

	soClaimOn := func(e *Encounter) {
		_, err := e.Speak(context.Background(), &SpeakInput{
			Range: 10, Actor: "alice", Target: soGuard, OptionID: soLine, Roller: wakeRollsOne{},
			Skill: "persuasion", Beaten: false, DC: soCheckDC, Total: 4,
		})
		require.NoError(t, err)
	}
	soClaimOn(loaded)
	require.True(t, loaded.recognizes(soGuard, "alice", soLied), "the loaded guard still notes on entering")
}

// TestStateOutcomesAreRefused: every refusal, from the one judge, at install,
// at the producer's check and on load; each sentence names the state.
func TestStateOutcomesAreRefused(t *testing.T) {
	give := func(r Recognition) map[string][]Consequence {
		return map[string][]Consequence{"wary": {{Recognize: &r}}}
	}
	good := Recognition{Topic: soLied, Holder: "itself", About: "the_one_named"}
	with := func(f func(r *Recognition)) map[string][]Consequence {
		r := good
		f(&r)

		return give(r)
	}
	one := func(c Consequence) map[string][]Consequence { return map[string][]Consequence{"wary": {c}} }
	timed := recipe(t, "calm", map[string]machine.State{
		"calm": {Transitions: []machine.Transition{{On: beatTime, To: "wary"}}},
		"wary": {Transitions: []machine.Transition{{On: beatTime, To: "calm"}}},
	})
	oneWay := recipe(t, "calm", map[string]machine.State{
		"calm": {Transitions: []machine.Transition{{On: beatHeard, To: "wary"}}},
		"wary": {},
	})
	for _, tc := range []struct {
		name     string
		recipe   *machine.Recipe
		outcomes map[string][]Consequence
		sentence string
	}{
		{"a state the recipe lacks", soRecipe(t), soNotes("asleep", soLied),
			`state outcome "asleep" is not a state of its recipe`},
		{"a state with no consequence", soRecipe(t), map[string][]Consequence{"wary": {}},
			`state outcome "wary" names no consequence`},
		{"a state nothing steps into", oneWay, soNotes("calm", soLied),
			`state outcome "calm" is entered by no transition of its recipe, so its outcomes could never run`},
		{"a status", soRecipe(t), one(Consequence{ApplyStatus: &StatusGrant{Status: json.RawMessage(`{}`), To: "speaker"}}),
			`state outcome "wary" consequence 0 gives a status: a line's row gives a status; a machine's state gives nobody one yet`},
		{"a told record", soRecipe(t), one(Consequence{Tells: "secret"}),
			`state outcome "wary" consequence 0 tells a record: a line's row tells whoever caught its answer; a state has no answer`},
		{"a relocation", soRecipe(t), one(Consequence{Relocate: &RelocateGrant{}}),
			`state outcome "wary" consequence 0 relocates: a line's row moves the speaker; a state moves nobody yet`},
		{"an arrest", soRecipe(t), one(Consequence{Arrest: &Arrest{}}),
			`state outcome "wary" consequence 0 arrests: an arrest follows a conversation entering a state, never a machine's`},
		{"a jail", soRecipe(t), one(Consequence{Jail: &Jail{}}),
			`state outcome "wary" consequence 0 jails: a jail follows a conversation entering a state, never a machine's`},
		{"a scene move", soRecipe(t), one(Consequence{SceneMove: &SceneMove{}}),
			`state outcome "wary" consequence 0 moves the scene: a scene move follows a conversation entering a state, never a machine's`},
		{"no kind", soRecipe(t), one(Consequence{}),
			`state outcome "wary" consequence 0 is of no kind: the one kind is recognize`},
		{"no topic", soRecipe(t), with(func(r *Recognition) { r.Topic = "" }),
			`state outcome "wary" consequence 0 recognizes on no topic`},
		{"another holder", soRecipe(t), with(func(r *Recognition) { r.Holder = RecognitionSpokenTo }),
			`state outcome "wary" consequence 0 holder "spoken_to": the holder is "itself"`},
		{"another subject", soRecipe(t), with(func(r *Recognition) { r.About = RecognitionSpeaker }),
			`state outcome "wary" consequence 0 about "speaker": the subject is "the_one_named"`},
		{"someone else hearing it", soRecipe(t), with(func(r *Recognition) { r.HeardBy = RecognitionCarriersWhoWitnessed }),
			`state outcome "wary" consequence 0 heard_by "carriers_who_witnessed": a state's note has no line, so nobody else hears it`},
		{"the one named on a beat that names nobody", timed, soNotes("wary", soLied),
			"state outcome \"wary\" consequence 0 notes the one the beat names, and the recipe enters \"wary\" on `time`, " +
				"which names nobody: it is read on heard, seen, topic, barred and approached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStateOutcomes(tc.recipe, tc.outcomes)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.sentence)

			member := soListener(t, soGuard, 3, 2, tc.recipe, tc.outcomes)
			_, err = NewEncounter(postedSetup(everyoneSeesTheWholeMap{}, member))
			require.ErrorIs(t, err, ErrBadMember, "install refuses it")
			require.Contains(t, err.Error(), tc.sentence)

			_, err = CheckBehaviors(&CheckBehaviorsInput{Behaviors: *member.Behaviors, Outcomes: tc.outcomes})
			require.ErrorIs(t, err, ErrBadMember, "the producer's check refuses it")
			require.Contains(t, err.Error(), tc.sentence)
		})
	}

	t.Run("outcomes on a member with no recipe", func(t *testing.T) {
		err := validateStateOutcomes(nil, soNotes("wary", soLied))
		require.Error(t, err)
		require.Contains(t, err.Error(), "name states of no recipe")
		bare := MemberInput{ID: soGuard, Kind: KindMonster, Position: spatial.Position{X: 3, Y: 2}, Outcomes: soNotes("wary", soLied)}
		_, err = NewEncounter(postedSetup(everyoneSeesTheWholeMap{}, bare))
		require.ErrorIs(t, err, ErrBadMember)
	})

	t.Run("a refused load", func(t *testing.T) {
		enc := soHall(t, everyoneSeesTheWholeMap{}, soGuardIn(t, soNotes("wary", soLied)))
		data := enc.ToData()
		for i := range data.Members {
			if data.Members[i].ID == soGuard {
				data.Members[i].Outcomes = soNotes("asleep", soLied)
			}
		}
		_, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: postedSetup(everyoneSeesTheWholeMap{}).Capabilities})
		require.ErrorIs(t, err, ErrInvalidData)
		require.True(t, strings.Contains(err.Error(), `"asleep"`), err.Error())
	})
}

// TestAHeardNoiseLandedForTheRaiseNotesNobody: a clang lands on the listener
// carrying its maker, but a landed sound names nobody, so the state it enters
// notes nobody: the beat's own naming decides, not the maker it carries.
func TestAHeardNoiseLandedForTheRaiseNotesNobody(t *testing.T) {
	ear := listener(soBystand, 3, 1, postedRecipe(t))
	ear.Outcomes = soNotes("alert", soLied)
	enc, _ := earsHall(t, ear)
	require.NoError(t, enc.makeSound(noiseAttack, "alice", uint64(enc.clock.ToData().HighWater)))
	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, "alert", stateOf(enc, soBystand), "he heard it and stepped")
	require.Zero(t, soHoldings(t, enc, soBystand, soLied), "but a sound names nobody, so he noted nobody")
}
