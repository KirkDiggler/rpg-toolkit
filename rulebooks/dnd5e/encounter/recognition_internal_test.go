// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// recognition_internal_test.go is ADMISSION IS RECOGNITION (rpg-project#928)
// on the hold hall: a claim said to a posted guard moves the front gate to
// `admitted`, its outcome makes him recognise the party he sees, and his
// post's `{ recognized }` pass lets the recognised through. The castle's own
// routes are castletest's; these are the mechanism's.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// admittedTopic is the hold hall's recognition topic.
const admittedTopic = TopicID("admitted")

// recognizedPass is the pass every recognising post here carries.
var recognizedPass = []Trigger{TriggerRecognized{Topic: admittedTopic}}

// admitting is the front gate's outcome: entering `admitted`, the guard
// spoken to recognises the party he sees, and, when heard, every other
// carrier who witnessed it does too.
func admitting(heard bool) map[string][]Consequence {
	r := &Recognition{Topic: admittedTopic, Holder: RecognitionSpokenTo, About: RecognitionPartyInSight}
	if heard {
		r.HeardBy = RecognitionCarriersWhoWitnessed
	}

	return map[string][]Consequence{"admitted": {{Recognize: r}}}
}

// recognisingGuard is a gate guard posted on the leaf's crossing cell,
// both ways, letting through whom he recognises; carrier says whether he
// carries the front gate's claim.
func recognisingGuard(t *testing.T, id MemberID, at spatial.Position, carrier bool) MemberInput {
	t.Helper()
	g := setHolder(t, id, at, holdLeaf, PostBothWays, "", at)
	for state, p := range g.Posts {
		p.Pass = recognizedPass
		g.Posts[state] = p
	}
	if carrier {
		g.Options = hailClaim()
	}

	return g
}

// recognisingSetup is the sided hall with the leaf open, alice beside the
// guard on the yard side, the front gate admitting as heard says, and the
// members given.
func recognisingSetup(t *testing.T, heard bool, members ...MemberInput) *SetupInput {
	t.Helper()
	in := sidedSetup(holdLeafDoor(DoorIsOpen()), holdSouthEast, members...)
	in.Field.Interactions = []InteractionInput{{ID: hailGate, Recipe: hailRecipe(), Outcomes: admitting(heard)}}

	return in
}

// recognisingHall opens [recognisingSetup].
func recognisingHall(t *testing.T, heard bool, members ...MemberInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(recognisingSetup(t, heard, members...))
	require.NoError(t, err)

	return enc
}

// admit has alice say the claim to the guard, which admits.
func admit(t *testing.T, enc *Encounter, guard MemberID) {
	t.Helper()
	_, err := enc.Speak(context.Background(), &SpeakInput{
		Actor: "alice", Target: guard, OptionID: "guest", Roller: wakeRollsOne{},
		Revision: enc.interactions[hailGate].revision,
	})
	require.NoError(t, err)
	require.Equal(t, "admitted", enc.interactions[hailGate].blob.State, "precondition: the claim admitted")
}

// recognizedBeats is every [BeatRecognized] beat on the record.
func recognizedBeats(t *testing.T, enc *Encounter) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, entry := range enc.ToData().Log.Entries {
		if entry.Tags["tag"] != BeatRecognized {
			continue
		}
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		out = append(out, b)
	}

	return out
}

func TestAnAdmissionIsTheGuardRecognisingWhomHeSees(t *testing.T) {
	bob := MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 7, Y: 3}}
	porter := MemberInput{ID: "porter", Kind: KindMonster, Faction: postedGarrison, Position: spatial.Position{X: 8, Y: 3}, Behaviors: ordinaryBehavior(holds())}
	enc := recognisingHall(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true), bob, porter)

	require.Equal(t, "Gate guard bars the way", heldStop(t, enc, holdNorthEast), "before the claim, barred")
	admit(t, enc, postedGuard)

	require.True(t, enc.recognizes(postedGuard, "alice", admittedTopic), "the speaker is recognised")
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "and the party member he sees")
	require.False(t, enc.recognizes(postedGuard, "porter", admittedTopic), "not a creature of another side")
	beats := recognizedBeats(t, enc)
	require.NotEmpty(t, beats)
	require.Equal(t, string(postedGuard), beats[0]["holder"])
	require.Equal(t, string(admittedTopic), beats[0]["topic"])
	require.Equal(t, ChannelJudgement, beats[0]["channel"])
	require.ElementsMatch(t, []any{"alice", "bob"}, beats[0]["subjects"])

	steps(t, enc, holdNorthEast, "recognised, she crosses the held opening")
	require.Equal(t, "posted", stateOf(enc, postedGuard), "while he stays posted")
}

func TestAGuardDoesNotRecogniseWhomHeDoesNotSee(t *testing.T) {
	// The guard sees two cells: alice beside him, not bob across the yard,
	// who sees him. Bob is not recognised, and is barred.
	bob := MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 7, Y: 3}}
	in := recognisingSetup(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true), bob)
	in.Sight = shortSighted{member: postedGuard, cells: 2}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)

	require.True(t, enc.recognizes(postedGuard, "alice", admittedTopic))
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "a member out of his sight is not recognised")
	placeAt(t, enc, "alice", spatial.Position{X: 7, Y: 4})
	placeAt(t, enc, "bob", holdSouthEast)
	out, err := enc.Step(&StepInput{Member: "bob", To: holdNorthEast})
	requireStopped(t, out, err, "and is barred")
}

// hideFrom stands a party member sneaking past the posted guard, the verdict
// beaten, and runs a sight pass so his stored testimony agrees.
func hideFrom(t *testing.T, enc *Encounter, who MemberID) {
	t.Helper()
	enc.members[who].Sneaking = true
	enc.setSneakVerdict(who, postedGuard, SneakVerdictData{Beaten: true, InZone: true})
	_, _, err := enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
}

func TestRecognitionIsJudgedOnLiveSight(t *testing.T) {
	bob := MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 7, Y: 3}}

	t.Run("a sneaking speaker who wins the claim is recognised", func(t *testing.T) {
		enc := recognisingHall(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true), bob)
		hideFrom(t, enc, "alice")
		stored, err := enc.currentlySees(postedGuard, "alice")
		require.NoError(t, err)
		require.False(t, stored, "precondition: his last sight pass holds her hidden")
		admit(t, enc, postedGuard)
		require.True(t, enc.recognizes(postedGuard, "alice", admittedTopic), "speaking showed her, and he recognises her")
		steps(t, enc, holdNorthEast, "and lets her through")
	})

	t.Run("a member who went hidden after the last sight pass is not", func(t *testing.T) {
		enc := recognisingHall(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true), bob)
		stored, err := enc.currentlySees(postedGuard, "bob")
		require.NoError(t, err)
		require.True(t, stored, "precondition: his last sight pass holds bob seen")
		enc.members["bob"].Sneaking = true
		enc.setSneakVerdict("bob", postedGuard, SneakVerdictData{Beaten: true, InZone: true})
		admit(t, enc, postedGuard)
		require.True(t, enc.recognizes(postedGuard, "alice", admittedTopic))
		require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "hidden now, bob is not recognised")
	})
}

func TestARecognitionIsToldOnlyWhatEachReaderPerceives(t *testing.T) {
	// Carol, the guard's fellow, sees two cells: alice speaking near her,
	// not bob across the yard. The guard sees both.
	bob := MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 4}}
	carol := MemberInput{ID: "carol", Kind: KindMonster, Faction: postedGarrison, Behaviors: ordinaryBehavior(holds()), Position: spatial.Position{X: 3, Y: 2}}
	in := recognisingSetup(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true), bob, carol)
	in.Sight = shortSighted{member: "carol", cells: 2}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "precondition: the guard recognised bob")

	told := func(reader MemberID) [][]any {
		t.Helper()
		entries, err := enc.Story(&StoryInput{Audience: reader})
		require.NoError(t, err)
		var out [][]any
		for _, e := range entries {
			if e.Tags["tag"] != BeatRecognized {
				continue
			}
			var b map[string]any
			require.NoError(t, json.Unmarshal(e.Payload, &b))
			out = append(out, b["subjects"].([]any))
		}
		return out
	}
	require.Equal(t, [][]any{{"alice", "bob"}}, told(postedGuard), "the holder is told everyone he recognised")
	require.Equal(t, [][]any{{"alice"}}, told("carol"), "carol is told only alice, whom she sees")
}

func TestEveryHolderOfTheDoorMustRecogniseTheWalker(t *testing.T) {
	second := MemberID("gate-2")
	secondAt := spatial.Position{X: 2, Y: 0}

	t.Run("a holder who holds nothing about her still bars her", func(t *testing.T) {
		enc := recognisingHall(t, true,
			recognisingGuard(t, postedGuard, holdNorthWest, true), recognisingGuard(t, second, secondAt, false))
		admit(t, enc, postedGuard)
		require.True(t, enc.recognizes(postedGuard, "alice", admittedTopic))
		require.False(t, enc.recognizes(second, "alice", admittedTopic), "precondition: gate-2 carries no line of the gate, so heard nothing")
		heldStop(t, enc, holdNorthEast)
	})

	t.Run("a carrier who witnessed the admission lets the recognised by", func(t *testing.T) {
		enc := recognisingHall(t, true,
			recognisingGuard(t, postedGuard, holdNorthWest, true), recognisingGuard(t, second, secondAt, true))
		admit(t, enc, postedGuard)
		require.True(t, enc.recognizes(second, "alice", admittedTopic), "gate-2 heard the admission")
		held, err := enc.intelLog.AboutOn(sightMember(second), sightMember("alice"), perception.Topic(admittedTopic))
		require.NoError(t, err)
		require.Equal(t, perception.Channel(ChannelHeard), held.Channel, "through the heard channel")
		steps(t, enc, holdNorthEast, "both holders recognise her: she crosses")
	})

	t.Run("not heard unless the outcome says so", func(t *testing.T) {
		enc := recognisingHall(t, false,
			recognisingGuard(t, postedGuard, holdNorthWest, true), recognisingGuard(t, second, secondAt, true))
		admit(t, enc, postedGuard)
		require.False(t, enc.recognizes(second, "alice", admittedTopic))
	})
}

func TestARecognitionSurvivesTheRecordAndIsGrantedOnce(t *testing.T) {
	in := recognisingSetup(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true))
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	granted := len(recognizedBeats(t, enc))
	require.Positive(t, granted)

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
	require.NoError(t, err, "a named-topic record under the guard's sight identity loads")
	require.True(t, loaded.recognizes(postedGuard, "alice", admittedTopic), "the recognition holds after a reload")
	require.Equal(t, granted, len(recognizedBeats(t, loaded)), "and nothing was granted again on load")
	steps(t, loaded, holdNorthEast, "and lets her through")
	require.Equal(t, granted, len(recognizedBeats(t, loaded)), "nor by walking on")
}

func TestARecognitionLeavesSightAlone(t *testing.T) {
	enc := recognisingHall(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true))
	observer := sightMember(postedGuard)
	positions := map[MemberID]spatial.Position{"alice": {X: 6, Y: 3}}
	read := func() (any, any, any, any) {
		t.Helper()
		intel, err := enc.memberIntel(postedGuard)
		require.NoError(t, err)
		held, err := enc.intelLog.Held(observer)
		require.NoError(t, err)
		headings, err := enc.headingsSeenBy(observer, positions)
		require.NoError(t, err)
		sees, err := enc.currentlySees(postedGuard, "alice")
		require.NoError(t, err)

		return intel, held, headings, sees
	}
	beforeIntel, beforeHeld, beforeHeadings, beforeSees := read()

	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, postedGuard, []MemberID{"alice"}, admittedTopic, ChannelJudgement, recognitionLine{
		interaction: hailGate, state: "admitted", speaker: "alice", spokenTo: postedGuard,
	}))
	require.True(t, enc.recognizes(postedGuard, "alice", admittedTopic), "precondition: the record is there")

	afterIntel, afterHeld, afterHeadings, afterSees := read()
	require.Equal(t, beforeIntel, afterIntel, "his sight testimony is unchanged")
	require.Equal(t, beforeHeld, afterHeld, "the default topic reads the same")
	require.Equal(t, beforeHeadings, afterHeadings, "the heading he would hold is unchanged")
	require.Equal(t, beforeSees, afterSees, "whether he sees her is unchanged")
}

func TestARecognitionIsRefusedWhereItCannotBeMet(t *testing.T) {
	t.Run("a pass on a topic nothing grants", func(t *testing.T) {
		g := recognisingGuard(t, postedGuard, holdNorthWest, false)
		for state, p := range g.Posts {
			p.Pass = []Trigger{TriggerRecognized{Topic: "nobody-grants-this"}}
			g.Posts[state] = p
		}
		_, err := NewEncounter(recognisingSetup(t, false, g))
		require.ErrorIs(t, err, ErrBadMember)
	})

	t.Run("a line on a topic nothing grants", func(t *testing.T) {
		g := recognisingGuard(t, postedGuard, holdNorthWest, true)
		g.Options = append(g.Options, Option{ID: "hall", Say: "Which way?", Requires: []Trigger{TriggerRecognized{Topic: "nobody-grants-this"}}})
		_, err := NewEncounter(recognisingSetup(t, false, g))
		require.ErrorIs(t, err, ErrBadMember)
	})

	t.Run("a pass on an interaction's state", func(t *testing.T) {
		g := recognisingGuard(t, postedGuard, holdNorthWest, true)
		for state, p := range g.Posts {
			p.Pass = []Trigger{TriggerInteraction{Interaction: hailGate, In: []string{"admitted"}}}
			g.Posts[state] = p
		}
		_, err := NewEncounter(recognisingSetup(t, false, g))
		require.ErrorIs(t, err, ErrBadMember)
	})

	t.Run("an outcome's words", func(t *testing.T) {
		for name, r := range map[string]Recognition{
			"no topic":       {Holder: RecognitionSpokenTo, About: RecognitionPartyInSight},
			"another holder": {Topic: admittedTopic, Holder: "everyone", About: RecognitionPartyInSight},
			"other subjects": {Topic: admittedTopic, Holder: RecognitionSpokenTo, About: "anyone"},
			"other hearers":  {Topic: admittedTopic, Holder: RecognitionSpokenTo, About: RecognitionPartyInSight, HeardBy: "the town"},
		} {
			_, err := CheckInteraction(&InteractionInput{ID: hailGate, Recipe: hailRecipe(), Outcomes: map[string][]Consequence{"admitted": {{Recognize: &r}}}})
			require.ErrorIs(t, err, ErrNoField, name)
		}
		_, err := CheckInteraction(&InteractionInput{ID: hailGate, Recipe: hailRecipe(), Outcomes: map[string][]Consequence{"nowhere": admitting(false)["admitted"]}})
		require.ErrorIs(t, err, ErrNoField, "a state the recipe lacks")
		_, err = CheckInteraction(&InteractionInput{ID: hailGate, Recipe: hailRecipe(), Outcomes: map[string][]Consequence{"admitted": {{}}}})
		require.ErrorIs(t, err, ErrNoField, "a consequence of no kind")
		checked, err := CheckInteraction(&InteractionInput{ID: hailGate, Recipe: hailRecipe(), Outcomes: admitting(true)})
		require.NoError(t, err)
		require.Equal(t, []TopicID{admittedTopic}, checked.Topics)
	})

	t.Run("a prop's line, said to something that holds nothing", func(t *testing.T) {
		err := validatePropOptions("altar", "test:props:altar", []Option{{
			ID: "pray", Say: "Pray.", Requires: []Trigger{TriggerNot{Inner: TriggerRecognized{Topic: admittedTopic}}},
		}})
		require.ErrorIs(t, err, ErrNoField)
	})

	t.Run("an ending that waits on it", func(t *testing.T) {
		in := recognisingSetup(t, false, recognisingGuard(t, postedGuard, holdNorthWest, true))
		in.Endings = append(in.Endings, EndingInput{Key: "known", Trigger: TriggerRecognized{Topic: admittedTopic}})
		_, err := NewEncounter(in)
		require.ErrorIs(t, err, ErrNoEnding)
	})
}

// shortSighted is everyone seeing the whole map but one member, who sees
// only a few cells.
type shortSighted struct {
	member MemberID
	cells  int
}

// Sight answers the one member's few cells, and the whole map for the rest.
func (s shortSighted) Sight(members []MemberID) (map[MemberID]int, error) {
	out, _ := everyoneSeesTheWholeMap{}.Sight(members)
	if _, ok := out[s.member]; ok {
		out[s.member] = s.cells
	}

	return out, nil
}
