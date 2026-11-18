// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// heard_line_test.go is a line heard moves an NPC's own machine
// (rpg-project#1000) on the noise hall, with the speech hall's facets
// (speech_test.go): alice speaks to the goblin in room a; `near` stands in
// room a and sees her; `deaf-eyed` stands in room a and sees nobody (it
// catches the words unseeing); `next-door`, in room b off the doorway,
// only notices a voice; `far` is two rooms off and perceives nothing.

const (
	lineBob      encounter.MemberID = "line-bob"
	lineNear     encounter.MemberID = "near"
	lineDeafEyed encounter.MemberID = "deaf-eyed"
	lineNextDoor encounter.MemberID = "next-door"
	lineFar      encounter.MemberID = "far"
)

// The goblin's lines: a checked claim, a plain greeting, a plain line whose
// answer gives the speaker the holding `admitted`, and a checked case whose
// failure row gives them `turned-away`.
const (
	lineClaim = "claim"
	lineHello = "hello"
	lineVouch = "vouch-me"
	lineCase  = "case"
	lineAside = "aside"
)

// lineOptions are the goblin's four lines.
func lineOptions() []encounter.Option {
	give := func(topic encounter.TopicID) encounter.Consequence {
		return encounter.Consequence{Recognize: &encounter.Recognition{
			Topic: topic, Holder: encounter.RecognitionSpokenTo, About: encounter.RecognitionSpeaker,
		}}
	}
	persuade := []encounter.CheckApproach{{Ability: "persuasion", DC: 10}}
	return []encounter.Option{
		{ID: lineClaim, Say: "We're expected.", Check: persuade,
			Success: []encounter.Answer{{Weight: 1, Say: "Go on."}}, Failure: []encounter.Answer{{Weight: 1, Say: "Liar."}}},
		{ID: lineHello, Say: "Evening.", Success: []encounter.Answer{{Weight: 1, Say: "Evening."}}},
		{ID: lineAside, Say: "A word.", Success: []encounter.Answer{{Weight: 1, Say: "Not here.", Loudness: "whisper"}}},
		{ID: lineVouch, Say: "Remember me.", Success: []encounter.Answer{{Weight: 1, Say: "I will."}},
			Grants: []encounter.RowGrant{{Row: 0, Consequence: give("admitted")}}},
		{ID: lineCase, Say: "I fell behind.", Check: persuade,
			Success: []encounter.Answer{{Weight: 1, Say: "Fine."}}, Failure: []encounter.Answer{{Weight: 1, Say: "Off with you."}},
			Grants: []encounter.RowGrant{{Row: 0, Failure: true, Consequence: give("turned-away")}}},
	}
}

// fact is one guard fact leaf.
func fact(name string, value any) machine.Guard {
	return machine.Guard{Fact: &machine.FactLeaf{Name: machine.FactName(name), Op: machine.OpEq, Value: value}}
}

// on is a transition on a beat, when every guard given holds, to a state.
func on(beat machine.Beat, to string, when ...machine.Guard) machine.Transition {
	t := machine.Transition{On: beat, To: to, Reason: to}
	switch len(when) {
	case 0:
	case 1:
		t.When = &when[0]
	default:
		t.When = &machine.Guard{All: when}
	}
	return t
}

// ears is a recipe that starts `quiet` and moves on the first transition
// whose guard holds; each state it moves to hears nothing more, or, when
// again, listens for `heard` and moves to `again`.
func ears(t *testing.T, again bool, from ...machine.Transition) *encounter.BehaviorInput {
	t.Helper()
	ref, err := core.ParseString("test:behaviors:ears")
	require.NoError(t, err)
	states := map[string]machine.State{"quiet": {Transitions: from}, "again": {}}
	for _, tr := range from {
		if tr.To != "" && tr.To != "again" {
			states[tr.To] = machine.State{}
			if again {
				states[tr.To] = machine.State{Transitions: []machine.Transition{on("heard", "again")}}
			}
		}
	}
	return &encounter.BehaviorInput{Instances: []encounter.BoundBehavior{{
		Instance: behavior.InstanceSpec{ID: "ears", Recipe: machine.Recipe{Ref: *ref, Initial: "quiet", States: states}},
	}}}
}

// lineHall is the hall: every NPC runs the recipe, the goblin carries the
// lines, and the sight ranges are the ones given over [sightRanges]'s.
func lineHall(t *testing.T, behaviors *encounter.BehaviorInput, sight sightRanges, goblinBlocks ...string) *encounter.Encounter {
	t.Helper()
	npc := func(id encounter.MemberID, col, row int) encounter.MemberInput {
		m := at(id, encounter.KindMonster, col, row)
		m.Behaviors = behaviors
		return m
	}
	goblinIn := npc(goblin, 6, 5)
	goblinIn.Options = lineOptions()
	nearIn := npc(lineNear, 3, 3)
	nearIn.Options = lineOptions()
	goblinIn.BlockingStates = goblinBlocks
	sheets := sheetFacts{alice: {SpeedFeet: 30}, lineBob: {SpeedFeet: 30}}
	for _, id := range []encounter.MemberID{goblin, lineNear, lineDeafEyed, lineNextDoor, lineFar} {
		sheets[id] = encounter.SheetFacts{SpeedFeet: 30, PassivePerception: 10}
	}
	setup := noiseHallSetup(encounter.DoorIsOpen(), encounter.DoorIsOpen(), sheets, passDriver{},
		at(alice, encounter.KindPlayer, 5, 5), at(lineBob, encounter.KindPlayer, 5, 6), goblinIn,
		nearIn, npc(lineDeafEyed, 2, 1), npc(lineNextDoor, 12, 1), npc(lineFar, 25, 8),
	)
	if sight == nil {
		sight = sightRanges{}
	}
	if _, set := sight[lineDeafEyed]; !set {
		sight[lineDeafEyed] = 0
	}
	setup.Sight = sight
	enc, err := encounter.NewEncounter(setup)
	require.NoError(t, err)
	return enc
}

// sayTo has who say a line to the goblin; beaten matters only on a check.
func sayTo(t *testing.T, enc *encounter.Encounter, who encounter.MemberID, line string, beaten bool) {
	t.Helper()
	in := &encounter.SpeakInput{Range: speakReach, Actor: who, Target: goblin, OptionID: line, Roller: rollsLowest{}}
	if line == lineClaim || line == lineCase {
		in.Skill, in.Beaten, in.DC = "persuasion", beaten, 10
		in.Total = map[bool]int{true: 15, false: 4}[beaten]
	}
	_, err := enc.Speak(context.Background(), in)
	require.NoError(t, err, "%s says %q", who, line)
}

// stateOf is a member's machine state, read off the saved run.
func stateOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) string {
	t.Helper()
	for _, m := range enc.ToData().Members {
		if m.ID != id {
			continue
		}
		require.NotNil(t, m.Behaviors, "%s runs a machine", id)
		for _, instance := range m.Behaviors.Progress.Instances {
			if instance.ID == m.Behaviors.BodyInstance {
				return instance.Blob.State
			}
		}
	}
	require.FailNowf(t, "no machine", "%s", id)
	return ""
}

// states is every listener's state, by id.
func states(t *testing.T, enc *encounter.Encounter) map[encounter.MemberID]string {
	t.Helper()
	out := map[encounter.MemberID]string{}
	for _, id := range []encounter.MemberID{goblin, lineNear, lineDeafEyed, lineNextDoor, lineFar} {
		out[id] = stateOf(t, enc, id)
	}
	return out
}

// TestALineIsHeardAtOnce: everyone who caught the words, and one who only
// noticed the voice, steps on `heard` inside Speak, with no raise; one out
// of earshot does not.
func TestALineIsHeardAtOnce(t *testing.T) {
	enc := lineHall(t, ears(t, false, on("heard", "heard")), nil)
	before := enc.ToData().Clock.HighWater
	sayTo(t, enc, alice, lineHello, true)
	require.Equal(t, before, enc.ToData().Clock.HighWater, "a plain line raises nothing")
	require.Equal(t, map[encounter.MemberID]string{
		goblin: "heard", lineNear: "heard", lineDeafEyed: "heard", lineNextDoor: "heard", lineFar: "quiet",
	}, states(t, enc))
}

// TestALineIsHeardOnlyOnce: a line heard at once is not also landed for the
// next raise. Alice's plain aside is answered in a whisper only she
// catches, so nothing else of it can reach the room: after it, nothing of
// hers waits for a listener, and the raise steps nobody a second time.
func TestALineIsHeardOnlyOnce(t *testing.T) {
	enc := lineHall(t, ears(t, true, on("heard", "once")), nil)
	sayTo(t, enc, alice, lineAside, true)
	require.Equal(t, "once", stateOf(t, enc, lineNear), "heard at once")
	for _, b := range enc.ToData().Landed {
		require.False(t, b.Beat == "heard" && b.Subject == alice, "nothing of alice's line waits for the raise: %+v", b)
	}
	_, err := aRound(enc)
	require.NoError(t, err)
	got := states(t, enc)
	for _, id := range []encounter.MemberID{goblin, lineNear, lineDeafEyed, lineNextDoor} {
		require.Equal(t, "once", got[id], "%s is stepped once for the line", id)
	}
}

// TestALineHeardSaysWhetherTheClaimHeld: `heard.claim` is held or failed for
// one who caught the words of a checked line; a plain line and a voice only
// noticed are neither.
func TestALineHeardSaysWhetherTheClaimHeld(t *testing.T) {
	claims := func(t *testing.T) *encounter.BehaviorInput {
		return ears(t, false,
			on("heard", "held", fact("heard.claim", "held")),
			on("heard", "failed", fact("heard.claim", "failed")),
			on("heard", "no-claim"),
		)
	}
	for _, tc := range []struct {
		name, line      string
		beaten          bool
		caught, noticed string
	}{
		{"a beaten claim", lineClaim, true, "held", "no-claim"},
		{"a missed claim", lineClaim, false, "failed", "no-claim"},
		{"a plain line", lineHello, true, "no-claim", "no-claim"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc := lineHall(t, claims(t), nil)
			sayTo(t, enc, alice, tc.line, tc.beaten)
			got := states(t, enc)
			for _, id := range []encounter.MemberID{goblin, lineNear, lineDeafEyed} {
				require.Equal(t, tc.caught, got[id], "%s caught the words", id)
			}
			require.Equal(t, tc.noticed, got[lineNextDoor], "a voice only noticed carries no claim")
		})
	}
}

// TestHoldsAsksAboutTheOneHeard: `holds:admitted` is what the goblin holds
// about the one it heard and saw — not anybody. Bob is admitted; alice's
// claim does not move him, bob's does. The line that gives the holding is
// heard before its own grant, so it moves nothing either.
func TestHoldsAsksAboutTheOneHeard(t *testing.T) {
	enc := lineHall(t, ears(t, false, on("heard", "knows", fact("holds:admitted", true))), nil)
	sayTo(t, enc, lineBob, lineVouch, true)
	require.Equal(t, "quiet", stateOf(t, enc, goblin), "the line that gives the holding is heard before it is given")
	sayTo(t, enc, alice, lineHello, true)
	require.Equal(t, "quiet", stateOf(t, enc, goblin), "he holds `admitted` about bob, not about alice")
	sayTo(t, enc, lineBob, lineHello, true)
	require.Equal(t, "knows", stateOf(t, enc, goblin), "about bob, whom he heard and saw")
}

// TestHoldsNeedsTheSpeakerSeen: one who catches the words without seeing
// who said them holds nothing about them, whatever he holds about the
// speaker: sight names, a heard voice names nobody. Near holds `admitted`
// about bob; the lamp goes out on him, and bob's line to the goblin, in
// the same room, is a voice from nobody.
func TestHoldsNeedsTheSpeakerSeen(t *testing.T) {
	sight := sightRanges{}
	enc := lineHall(t, ears(t, false, on("heard", "knows", fact("holds:admitted", true))), sight)
	_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: 10, Actor: lineBob, Target: lineNear, OptionID: lineVouch, Roller: rollsLowest{},
	})
	require.NoError(t, err)
	sight[lineNear] = 0
	sayTo(t, enc, lineBob, lineHello, true)
	require.Equal(t, "quiet", stateOf(t, enc, lineNear), "he cannot tell who spoke")
	delete(sight, lineNear)
	sayTo(t, enc, lineBob, lineHello, true)
	require.Equal(t, "knows", stateOf(t, enc, lineNear), "control: seen again, it is bob")
}

// TestALinesOwnFailureRowIsNotReadByItsOwnBeat: the case's failure row
// gives `turned-away`; the guard hears the line before its grant, so the
// first miss finds him holding nothing and only the second moves him.
func TestALinesOwnFailureRowIsNotReadByItsOwnBeat(t *testing.T) {
	enc := lineHall(t, ears(t, false,
		on("heard", "again", fact("heard.claim", "failed"), fact("holds:turned-away", true)),
	), nil)
	sayTo(t, enc, alice, lineCase, false)
	require.Equal(t, "quiet", stateOf(t, enc, goblin), "the first miss: he held nothing when he heard it")
	sayTo(t, enc, alice, lineCase, false)
	require.Equal(t, "again", stateOf(t, enc, goblin), "the second: he holds `turned-away` about her now")
}

// TestAnNPCsAnswerStillLandsAtTheNextRaise: alice's plain line is heard at
// once and raises nothing; the goblin's answer is heard as a creature's
// speech always is, at the next raise.
func TestAnNPCsAnswerStillLandsAtTheNextRaise(t *testing.T) {
	enc := lineHall(t, ears(t, true, on("heard", "once")), nil)
	sayTo(t, enc, alice, lineHello, true)
	require.Equal(t, "once", stateOf(t, enc, lineNear), "alice's plain line, at once; the answer not yet")
	require.NotEmpty(t, enc.ToData().Landed, "the goblin's answer waits for the raise")
	_, err := aRound(enc)
	require.NoError(t, err)
	require.Equal(t, "again", stateOf(t, enc, lineNear), "the answer, at the raise")
}

// TestTheNextVerbReadsTheNewState: the goblin blocks only while `quiet`; a
// line heard moves him and, with no time passing, his cell is no longer
// held against alice.
func TestTheNextVerbReadsTheNewState(t *testing.T) {
	enc := lineHall(t, ears(t, false, on("heard", "heard")), nil, "quiet")
	cell := encounter.CellAtInput{Cell: whereIs(t, enc, goblin), Mover: alice}
	before, err := enc.CellAt(cell)
	require.NoError(t, err)
	require.Equal(t, encounter.PassageBlocked, before.Passage, "precondition: quiet, he holds his cell")
	sayTo(t, enc, alice, lineHello, true)
	after, err := enc.CellAt(cell)
	require.NoError(t, err)
	require.NotEqual(t, encounter.PassageBlocked, after.Passage, "heard, he no longer does")
}
