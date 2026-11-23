// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// topic_beat_names_internal_test.go pins THE TOPIC BEAT NAMES WHOM THE NOTE IS
// ABOUT (rpg-project#1123, slice rpg-project#1124): a creature that comes to
// hold a note, or whose note changes value, hears `topic` once for each
// subject whose note changed, and the beat names that subject. Internal,
// because the landed beats and the intel store are the composition's own.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const tbSeenOut = TopicID("seen-out")

// tbBobIn is a second member of the party, beside alice.
func tbBobIn() MemberInput {
	return MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 2}}
}

// tbDoorHall is a hall whose guard steps on `topic` when `noted.as:standing`
// is `barred` about the one the beat names, to at-the-door, where he notes
// `seen-out` about that one.
func tbDoorHall(t *testing.T) (*Encounter, *SetupInput) {
	t.Helper()
	barred := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNotedAs(string(nvTopic)), Op: machine.OpEq, Value: "barred"}}
	g := soGuardIn(t, soNotes("at-the-door", tbSeenOut),
		nvLine("warn", nvTopic, "warned"), nvLine("bar", nvTopic, "barred"))
	g.Behaviors.Instances[0].Instance.Recipe = *recipe(t, "calm", map[string]machine.State{
		"calm":        {Transitions: []machine.Transition{{On: beatTopic, When: barred, To: "at-the-door", Reason: "barred"}}},
		"at-the-door": {},
	})
	in := postedSetup(everyoneSeesTheWholeMap{}, g, tbBobIn())
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return enc, in
}

// tbSay has the actor say a checked line to the guard and miss.
func tbSay(t *testing.T, enc *Encounter, actor MemberID, option string) {
	t.Helper()
	_, err := enc.Speak(context.Background(), &SpeakInput{
		Range: 10, Actor: actor, Target: soGuard, OptionID: option, Roller: wakeRollsOne{},
		Skill: "persuasion", Beaten: false, DC: soCheckDC, Total: 4,
	})
	require.NoError(t, err)
}

// tbHold has the guard hold nvTopic about the subjects as `as`, at once and
// without the world's pass that a spoken line runs, so the beats it lands are
// still in enc.landed (a Speak runs its own pass and drains them).
func tbHold(t *testing.T, enc *Encounter, as string, subjects ...MemberID) {
	t.Helper()
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, soGuard, subjects, nvTopic, ChannelJudgement,
		recognitionLine{speaker: "alice", spokenTo: soGuard, as: as}))
}

// tbTopicBeats is the `topic` beats landed on the guard since landed held
// `since` beats, as the ones they name, in order.
func tbTopicBeats(enc *Encounter, since int) []MemberID {
	var out []MemberID
	for _, b := range enc.landed[since:] {
		if b.Beat == beatTopic && b.Member == soGuard {
			out = append(out, b.Subject)
		}
	}

	return out
}

// tbNotedAbout is whether the guard holds topic about subject.
func tbNotedAbout(enc *Encounter, subject MemberID, topic TopicID) bool {
	_, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember(subject), perceptionTopic(topic))

	return err == nil
}

func TestTopicBeatTheNoteWakesHimAndHeKnowsWhom(t *testing.T) {
	enc, _ := tbDoorHall(t)

	tbSay(t, enc, "bob", "warn")
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "calm", stateOf(enc, soGuard), "bob was only warned: he has not stepped")

	tbSay(t, enc, "alice", "bar")
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "at-the-door", stateOf(enc, soGuard), "alice was barred: the note woke him")
	require.True(t, tbNotedAbout(enc, "alice", tbSeenOut), "and he noted seen-out about alice, the one the note is about")
	require.False(t, tbNotedAbout(enc, "bob", tbSeenOut), "and not about bob")
}

func TestTopicBeatOneBeatForEachOneWhoseNoteChanged(t *testing.T) {
	enc, _ := tbDoorHall(t)
	before := len(enc.landed)

	tbHold(t, enc, "warned", "alice", "bob")

	require.Equal(t, []MemberID{"alice", "bob"}, tbTopicBeats(enc, before),
		"one give about two people lands two topic beats, naming alice then bob")
}

func TestTopicBeatAConfirmedNoteIsNoNewsForThatOne(t *testing.T) {
	enc, _ := tbDoorHall(t)
	tbHold(t, enc, "warned", "alice")
	require.NoError(t, enc.spendWorldAction("alice"))
	before := len(enc.landed)

	tbHold(t, enc, "warned", "alice", "bob")

	require.Equal(t, []MemberID{"bob"}, tbTopicBeats(enc, before),
		"alice is already warned: only bob's note is news")
}

func TestTopicBeatAReplacedValueNamesTheOneItWasReplacedFor(t *testing.T) {
	enc, _ := tbDoorHall(t)
	tbHold(t, enc, "warned", "alice", "bob")
	require.NoError(t, enc.spendWorldAction("alice"))
	before := len(enc.landed)

	tbHold(t, enc, "barred", "alice")

	require.Equal(t, []MemberID{"alice"}, tbTopicBeats(enc, before),
		"barred about alice alone replaced her value only: one beat, naming alice")
}

func TestTopicBeatTheWordsAreReadOnTopic(t *testing.T) {
	noted := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNoted("standing"), Op: machine.OpEq, Value: true}}
	notedAs := &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNotedAs("standing"), Op: machine.OpEq, Value: "barred"}}

	require.Empty(t, GuardWordsRefusal(beatTopic, noted), "`noted:` is read on topic")
	require.Empty(t, GuardWordsRefusal(beatTopic, notedAs), "`noted.as:` is read on topic")

	refusal := GuardWordsRefusal(beatFact, noted)
	require.True(t, strings.HasSuffix(refusal, "it is read on heard, seen, topic, barred and approached"), refusal)
}

func TestTopicBeatSavedBetweenTheGiveAndTheWake(t *testing.T) {
	enc, in := tbDoorHall(t)
	tbSay(t, enc, "bob", "warn")
	require.NoError(t, enc.spendWorldAction("alice"))
	tbHold(t, enc, "barred", "alice")
	require.Len(t, enc.landed, 1, "precondition: the beat is landed and waits for the world")

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
	require.NoError(t, err)
	require.Equal(t, "calm", stateOf(loaded, soGuard), "precondition: the beat waits for the world to move on")

	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Equal(t, "at-the-door", stateOf(loaded, soGuard), "the saved beat still names alice, and woke him")
	require.True(t, tbNotedAbout(loaded, "alice", tbSeenOut), "he noted seen-out about alice")
	require.False(t, tbNotedAbout(loaded, "bob", tbSeenOut), "and not about bob")
}

func TestTopicBeatTheWordsTableSaysTopicNamesTheOneTheNoteIsAbout(t *testing.T) {
	topic, ok := BeatWordOf("topic")
	require.True(t, ok)
	require.True(t, topic.Names, "the topic beat names someone")
	require.Equal(t, "the one the note is about", topic.About, "and says whom")
}
