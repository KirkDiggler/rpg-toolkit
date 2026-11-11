// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// vouch_internal_test.go is the two tools of rpg-project#929 on the hold
// hall: a line's answer grants a record in the words an outcome uses (a
// recognised member vouching, a straggler making their own case), and a
// routine wakes on a topic its member comes to hold.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// recognizeOn is a recognition of the topic, said to, about subjects, heard
// by the witnessing carriers when heard.
func recognizeOn(about RecognitionSubjects, heard bool) Consequence {
	r := &Recognition{Topic: admittedTopic, Holder: RecognitionSpokenTo, About: about}
	if heard {
		r.HeardBy = RecognitionCarriersWhoWitnessed
	}
	return Consequence{Recognize: r}
}

// atTheAdmittedGate reads the gate's state at admitted: a line that needs it
// is offered only once the gate has let someone in.
var atTheAdmittedGate = TriggerInteraction{Interaction: hailGate, In: []string{"admitted"}}

// vouchLine is a plain line, offered to a speaker the guard recognises once
// the gate has admitted, whose answer recognises the party he sees.
func vouchLine() Option {
	return Option{
		ID: "vouch", Say: "They're with us.",
		Requires: []Trigger{atTheAdmittedGate, TriggerRecognized{Topic: admittedTopic}},
		Success:  []Answer{{Weight: 1, Say: "Go on through."}},
		Grants:   []RowGrant{{Row: 0, Consequence: recognizeOn(RecognitionPartyInSight, true)}},
	}
}

// stragglerLine is a checked line, offered to a speaker the guard does NOT
// recognise once the gate has admitted, whose success recognises the speaker
// alone and whose failure is a refusal.
func stragglerLine() Option {
	return Option{
		ID: "straggler", Say: "I fell behind.",
		Check:    []CheckApproach{{Ability: "persuasion", DC: 13}},
		Requires: []Trigger{atTheAdmittedGate, TriggerNot{Inner: TriggerRecognized{Topic: admittedTopic}}},
		Success:  []Answer{{Weight: 1, Say: "Catch them up."}},
		Failure:  []Answer{{Weight: 1, Say: "Back the way you came."}},
		Grants:   []RowGrant{{Row: 0, Consequence: recognizeOn(RecognitionSpeaker, true)}},
	}
}

// gateGuard is a recognising guard who carries the claim, the vouch and the
// straggler's line.
func gateGuard(t *testing.T, id MemberID, at spatial.Position) MemberInput {
	t.Helper()
	g := recognisingGuard(t, id, at, true)
	g.Options = append(g.Options, vouchLine(), stragglerLine())
	return g
}

// say has who say a line to the guard at its offered revision.
func say(t *testing.T, enc *Encounter, who, guard MemberID, option string, beaten bool) (*SpeakOutput, error) {
	t.Helper()
	in := &SpeakInput{
		Actor: who, Target: guard, OptionID: option, Roller: wakeRollsOne{}, Range: 3,
		Revision: enc.interactions[hailGate].revision,
	}
	if option == "straggler" {
		in.Skill, in.Beaten, in.DC = "persuasion", beaten, 13
		in.Total = map[bool]int{true: 18, false: 4}[beaten]
	}
	return enc.Speak(context.Background(), in)
}

// offers reports whether the guard offers who the line.
func offers(t *testing.T, enc *Encounter, who, guard MemberID, option string) bool {
	t.Helper()
	offered, err := enc.OptionsFor(who, guard)
	require.NoError(t, err)
	for _, o := range offered {
		if o.ID == option {
			return true
		}
	}
	return false
}

// strangerHall is the hold hall with a guard who sees two cells, alice
// beside him, and bob and carol of the party far down the yard.
func strangerHall(t *testing.T, members ...MemberInput) *Encounter {
	t.Helper()
	in := recognisingSetup(t, true, append([]MemberInput{
		gateGuard(t, postedGuard, holdNorthWest),
		{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 4}},
		{ID: "carol", Kind: KindPlayer, Position: spatial.Position{X: 9, Y: 4}},
	}, members...)...)
	in.Sight = shortSighted{member: postedGuard, cells: 2}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	require.True(t, enc.recognizes(postedGuard, "alice", admittedTopic), "precondition: alice is admitted")
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "precondition: bob was out of his sight")
	return enc
}

func TestAVouchRecognisesTheStragglerInTheGuardsSight(t *testing.T) {
	enc := strangerHall(t)
	require.True(t, offers(t, enc, "alice", postedGuard, "vouch"), "the vouch is offered to whom he recognises")
	require.False(t, offers(t, enc, "bob", postedGuard, "vouch"), "and not to a stranger")

	// Bob comes up the yard into the guard's sight; carol stays out of it.
	placeAt(t, enc, "bob", spatial.Position{X: 3, Y: 1})
	_, err := say(t, enc, "alice", postedGuard, "vouch", true)
	require.NoError(t, err)
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "the straggler in his sight is recognised")
	require.False(t, enc.recognizes(postedGuard, "carol", admittedTopic), "one out of it is not")
	require.Equal(t, "admitted", enc.interactions[hailGate].blob.State, "the gate's conversation did not move")

	// THE GRANT REPEATS: carol comes up later, and a second vouch admits her;
	// alice's own record is confirmed, never written new.
	before, err := enc.intelLog.AboutOn(sightMember(postedGuard), sightMember("alice"), perception.Topic(admittedTopic))
	require.NoError(t, err)
	placeAt(t, enc, "bob", spatial.Position{X: 8, Y: 4})
	placeAt(t, enc, "carol", spatial.Position{X: 3, Y: 1})
	_, err = say(t, enc, "alice", postedGuard, "vouch", true)
	require.NoError(t, err)
	require.True(t, enc.recognizes(postedGuard, "carol", admittedTopic), "a second straggler is vouched for later")
	after, err := enc.intelLog.AboutOn(sightMember(postedGuard), sightMember("alice"), perception.Topic(admittedTopic))
	require.NoError(t, err)
	require.Equal(t, before.Observed, after.Observed, "re-granting keeps when he first recognised her")
}

func TestAVouchIsSpokenAtTheGate(t *testing.T) {
	enc := strangerHall(t)
	placeAt(t, enc, "alice", spatial.Position{X: 9, Y: 2})
	_, err := say(t, enc, "alice", postedGuard, "vouch", true)
	require.Error(t, err, "from down the yard the vouch cannot be said to him")
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic))
}

func TestAStragglerMakesTheirOwnCase(t *testing.T) {
	t.Run("success recognises the speaker alone", func(t *testing.T) {
		enc := strangerHall(t)
		placeAt(t, enc, "alice", spatial.Position{X: 8, Y: 3})
		placeAt(t, enc, "bob", holdSouthEast)
		placeAt(t, enc, "carol", spatial.Position{X: 3, Y: 1})
		require.True(t, offers(t, enc, "bob", postedGuard, "straggler"), "offered to whom he does not recognise")
		require.False(t, offers(t, enc, "alice", postedGuard, "straggler"), "and not to whom he does")
		revision := enc.interactions[hailGate].revision
		_, err := say(t, enc, "bob", postedGuard, "straggler", true)
		require.NoError(t, err)
		require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "the straggler is recognised")
		require.False(t, enc.recognizes(postedGuard, "carol", admittedTopic), "the member beside them in his sight is not")
		require.Equal(t, revision, enc.interactions[hailGate].revision, "a grant that moves no state moves no revision")
	})

	t.Run("failure changes nothing", func(t *testing.T) {
		enc := strangerHall(t)
		placeAt(t, enc, "bob", holdSouthEast)
		revision := enc.interactions[hailGate].revision
		_, err := say(t, enc, "bob", postedGuard, "straggler", false)
		require.NoError(t, err)
		require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "a missed case is a refusal")
		require.Equal(t, "admitted", enc.interactions[hailGate].blob.State)
		require.Equal(t, revision, enc.interactions[hailGate].revision)
		require.True(t, offers(t, enc, "bob", postedGuard, "straggler"), "and the case may be made again")
	})
}

func TestAVouchIsHeardByTheOtherCarriers(t *testing.T) {
	second := MemberID("gate-2")
	enc := strangerHall(t, gateGuard(t, second, spatial.Position{X: 2, Y: 0}))
	placeAt(t, enc, "bob", spatial.Position{X: 3, Y: 1})
	_, err := say(t, enc, "alice", postedGuard, "vouch", true)
	require.NoError(t, err)
	held, err := enc.intelLog.AboutOn(sightMember(second), sightMember("bob"), perception.Topic(admittedTopic))
	require.NoError(t, err, "the other carrier who witnessed it holds the record")
	require.Equal(t, perception.Channel(ChannelHeard), held.Channel)
}

func TestALineGrantSurvivesTheRecord(t *testing.T) {
	enc := strangerHall(t)
	placeAt(t, enc, "bob", spatial.Position{X: 3, Y: 1})
	_, err := say(t, enc, "alice", postedGuard, "vouch", true)
	require.NoError(t, err)
	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: postedSetup(shortSighted{member: postedGuard, cells: 2}).Capabilities})
	require.NoError(t, err)
	require.True(t, loaded.recognizes(postedGuard, "bob", admittedTopic))
	require.True(t, offers(t, loaded, "bob", postedGuard, "vouch"), "the saved line still carries its grant's effect")
	var vouch Option
	for _, o := range loaded.members[postedGuard].Options {
		if o.ID == "vouch" {
			vouch = o
		}
	}
	require.Equal(t, vouchLine().Grants, vouch.Grants, "the grant is saved with the line")
}

// topicRecipe is a posted guard who goes in once he holds the topic about
// anybody, and a dozer who listens for nothing until it is woken.
func topicRecipe(t *testing.T) *machine.Recipe {
	t.Helper()
	held := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineTopic(string(admittedTopic)), Op: machine.OpEq, Value: true}}
	r := holdRecipe(t)
	posted := r.States["posted"]
	posted.Transitions = append([]machine.Transition{{On: beatTopic, When: held, To: "inside", Reason: "admitted; goes in"}},
		posted.Transitions...)
	r.States["posted"] = posted
	r.States["inside"] = machine.State{Transitions: []machine.Transition{pickTime}}
	return r
}

func TestARoutineWakesOnATopicItComesToHold(t *testing.T) {
	g := gateGuard(t, postedGuard, holdNorthWest)
	g.Behaviors.Instances[0].Instance.Recipe = *topicRecipe(t)
	in := recognisingSetup(t, true, g)
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	require.Equal(t, "posted", stateOf(enc, postedGuard), "precondition: posted")

	admit(t, enc, postedGuard)
	// The plain claim pays no round; the topic waits for the next one, as a
	// learned fact does, and a save between keeps it.
	require.Equal(t, "posted", stateOf(enc, postedGuard), "nothing moves before time does")
	enc = reload(t, enc, in.Capabilities)
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "inside", stateOf(enc, postedGuard), "coming to hold the topic, he goes in")
	_, holds := enc.memberHolds(postedGuard, nil)
	require.False(t, holds, "and holds nothing there")

	loaded := reload(t, enc, in.Capabilities)
	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Equal(t, "inside", stateOf(loaded, postedGuard), "after a load he is still inside, nothing fired again")
}

// reload saves the run and loads it back.
func reload(t *testing.T, enc *Encounter, capabilities Capabilities) *Encounter {
	t.Helper()
	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: capabilities})
	require.NoError(t, err)
	return loaded
}

func TestATopicReadsTheHoldersOwnHoldings(t *testing.T) {
	g := gateGuard(t, postedGuard, holdNorthWest)
	g.Behaviors.Instances[0].Instance.Recipe = *topicRecipe(t)
	enc, err := NewEncounter(recognisingSetup(t, true, g))
	require.NoError(t, err)
	// Another member coming to hold the topic is nothing to him.
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, "alice", []MemberID{"alice"}, admittedTopic, ChannelJudgement, recognitionLine{}))
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "posted", stateOf(enc, postedGuard), "the truth of someone else's record never wakes him")
	require.NoError(t, enc.holdTopic(reach, postedGuard, []MemberID{"alice"}, admittedTopic, ChannelHeard, recognitionLine{}))
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "inside", stateOf(enc, postedGuard), "his own record, heard, does")
}

func TestALineGrantIsRefusedWhereItCannotLand(t *testing.T) {
	for name, edit := range map[string]func(*Option){
		"a row the line does not have": func(o *Option) { o.Grants[0].Row = 3 },
		"a consequence of no kind":     func(o *Option) { o.Grants[0].Consequence = Consequence{} },
		"another subject word":         func(o *Option) { o.Grants[0].Consequence.Recognize.About = "anyone" },
		"heard by on a line that names no interaction": func(o *Option) {
			o.Requires = []Trigger{TriggerRecognized{Topic: admittedTopic}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := gateGuard(t, postedGuard, holdNorthWest)
			vouch := vouchLine()
			edit(&vouch)
			g.Options = []Option{hailClaim()[0], vouch}
			_, err := NewEncounter(recognisingSetup(t, true, g))
			require.ErrorIs(t, err, ErrBadMember)
		})
	}

	t.Run("on a prop's line", func(t *testing.T) {
		line := vouchLine()
		line.Requires = nil
		line.Grants[0].Consequence.Recognize.HeardBy = ""
		require.ErrorIs(t, validatePropOptions("altar", "test:props:altar", []Option{line}), ErrNoField)
	})

	t.Run("a topic only a line grants is one a requirement may name", func(t *testing.T) {
		g := gateGuard(t, postedGuard, holdNorthWest)
		vouch := vouchLine()
		vouch.Grants[0].Consequence.Recognize.Topic = "vouched"
		g.Options = append(g.Options, Option{ID: "hello", Say: "Hello.", Requires: []Trigger{TriggerRecognized{Topic: "vouched"}}})
		g.Options[len(g.Options)-3] = vouch
		_, err := NewEncounter(recognisingSetup(t, true, g))
		require.NoError(t, err)
	})
}
