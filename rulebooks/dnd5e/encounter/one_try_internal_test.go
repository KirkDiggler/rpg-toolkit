// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// one_try_internal_test.go is a missed line may grant a record
// (rpg-project#932, ruled 2026-11-12: "1 try and the option should be
// gone"). A checked line's failure row grants as a success row does — the
// same consequence, the same live sight, the same Speak — and only when the
// line is missed. The straggler's case grants `turned-away` about the speaker
// on a miss and requires that the guard does not hold it, so one failed try
// takes the case away from that speaker and nobody else.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// turnedAwayTopic is what a guard comes to hold about a straggler he turned
// away.
const turnedAwayTopic TopicID = "turned-away"

// oneTryLine is the straggler's case with one try: a miss grants
// `turned-away` about the speaker, heard by the witnessing carriers, and the
// case is offered only to a speaker the guard does not hold it about.
func oneTryLine() Option {
	o := stragglerLine()
	o.Requires = append(o.Requires, TriggerNot{Inner: TriggerRecognized{Topic: turnedAwayTopic}})
	turned := recognizeOn(RecognitionSpeaker, true)
	turned.Recognize.Topic = turnedAwayTopic
	o.Grants = append(o.Grants, RowGrant{Row: 0, Failure: true, Consequence: turned})
	return o
}

// oneTryGuard is a gate guard whose straggler's case is [oneTryLine].
func oneTryGuard(t *testing.T, id MemberID, at spatial.Position) MemberInput {
	t.Helper()
	g := gateGuard(t, id, at)
	for i, o := range g.Options {
		if o.ID == "straggler" {
			g.Options[i] = oneTryLine()
		}
	}
	return g
}

// oneTryHall is [strangerHall] with both guards giving one try, bob and
// carol of the party in the first guard's sight and nobody admitted but
// alice.
func oneTryHall(t *testing.T) *Encounter {
	t.Helper()
	in := recognisingSetup(t, true,
		oneTryGuard(t, postedGuard, holdNorthWest),
		oneTryGuard(t, "gate-2", spatial.Position{X: 2, Y: 0}),
		MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 4}},
		MemberInput{ID: "carol", Kind: KindPlayer, Position: spatial.Position{X: 9, Y: 4}},
	)
	in.Sight = shortSighted{member: postedGuard, cells: 2}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "precondition: bob was out of his sight")
	placeAt(t, enc, "alice", spatial.Position{X: 8, Y: 3})
	placeAt(t, enc, "bob", holdSouthEast)
	placeAt(t, enc, "carol", spatial.Position{X: 3, Y: 1})
	return enc
}

func TestAMissedLineGrantsItsFailureRow(t *testing.T) {
	enc := oneTryHall(t)
	require.True(t, offers(t, enc, "bob", postedGuard, "straggler"), "precondition: offered once")
	revision := enc.interactions[hailGate].revision

	_, err := say(t, enc, "bob", postedGuard, "straggler", false)
	require.NoError(t, err)
	require.True(t, enc.recognizes(postedGuard, "bob", turnedAwayTopic), "the guard holds that he turned bob away")
	held, err := enc.intelLog.AboutOn(sightMember("gate-2"), sightMember("bob"), perception.Topic(turnedAwayTopic))
	require.NoError(t, err, "the other carrier who witnessed it holds it too")
	require.Equal(t, perception.Channel(ChannelHeard), held.Channel)
	require.False(t, enc.recognizes(postedGuard, "carol", turnedAwayTopic), "about the speaker alone, not the one beside him")
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "and the success row's grant did not land")
	require.Equal(t, revision, enc.interactions[hailGate].revision, "a grant that moves no state moves no revision")

	require.False(t, offers(t, enc, "bob", postedGuard, "straggler"), "one try: the case is gone for bob")
	require.False(t, offers(t, enc, "bob", "gate-2", "straggler"), "on both guards")
	require.True(t, offers(t, enc, "carol", postedGuard, "straggler"), "and still offered to another stranger")

	// A friend's vouch still lets him in.
	placeAt(t, enc, "alice", spatial.Position{X: 3, Y: 2})
	_, err = say(t, enc, "alice", postedGuard, "vouch", true)
	require.NoError(t, err)
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "vouched for, the turned-away straggler is admitted")
}

func TestALandedLineGrantsNoFailureRow(t *testing.T) {
	enc := oneTryHall(t)
	_, err := say(t, enc, "bob", postedGuard, "straggler", true)
	require.NoError(t, err)
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "a won case admits")
	require.False(t, enc.recognizes(postedGuard, "bob", turnedAwayTopic), "and grants nothing a miss would")
}

func TestAFailureGrantSurvivesTheRecord(t *testing.T) {
	enc := oneTryHall(t)
	_, err := say(t, enc, "bob", postedGuard, "straggler", false)
	require.NoError(t, err)
	loaded := reload(t, enc, postedSetup(shortSighted{member: postedGuard, cells: 2}).Capabilities)
	require.True(t, loaded.recognizes(postedGuard, "bob", turnedAwayTopic), "the record is kept")
	require.False(t, offers(t, loaded, "bob", postedGuard, "straggler"), "and the case stays gone after a load")
	for _, o := range loaded.members[postedGuard].Options {
		if o.ID == "straggler" {
			require.Equal(t, oneTryLine().Grants, o.Grants, "the failure row's grant is saved with the line")
		}
	}
}

func TestAFailureGrantIsRefusedWhereItCannotLand(t *testing.T) {
	for name, edit := range map[string]func(*Option){
		"a failure row the line does not have": func(o *Option) {
			*o = oneTryLine()
			o.Grants[1].Row = 1
		},
		"a failure grant on a plain line, which cannot be missed": func(o *Option) {
			*o = vouchLine()
			o.Grants[0].Failure = true
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := gateGuard(t, postedGuard, holdNorthWest)
			var line Option
			edit(&line)
			g.Options = []Option{hailClaim()[0], line}
			_, err := NewEncounter(recognisingSetup(t, true, g))
			require.ErrorIs(t, err, ErrBadMember)
		})
	}
}

// TestARoutineOnATopicIgnoresAnotherTopic: the captain's routine wakes on
// `topic` and is guarded by `topic:admitted`, so a guard coming to hold
// `turned-away` about someone moves nothing.
func TestARoutineOnATopicIgnoresAnotherTopic(t *testing.T) {
	g := gateGuard(t, postedGuard, holdNorthWest)
	g.Behaviors.Instances[0].Instance.Recipe = *topicRecipe(t)
	enc, err := NewEncounter(recognisingSetup(t, true, g))
	require.NoError(t, err)
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, postedGuard, []MemberID{"alice"}, turnedAwayTopic, ChannelJudgement, recognitionLine{}))
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "posted", stateOf(enc, postedGuard), "turned-away is not admitted: he stays")
}
