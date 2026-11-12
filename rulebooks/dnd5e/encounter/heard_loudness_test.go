// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/sound"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// heard_loudness_test.go is a routine that listens for a raised voice
// (rpg-project#933, the operator's ruling on rpg-project#930: guards react
// to shouts, not talk): `heard: { kind: voice, loudness: shout }`.

// shoutOnly is a guard's table: walk toward a voice raised to a shout or
// louder, else hold.
func shoutOnly() encounter.Table {
	return encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{
			Kind: behavior.Name{Literal: encounter.SoundVoice}, Within: 2, Loudness: string(encounter.LoudnessShout),
		}}}, Toward: &encounter.Selector{Word: encounter.SelectorSound}},
		{Weight: 1, Hold: true},
	}}
}

// crierHall is the noise hall with a crier in room a saying one line every
// round at a loudness, and a shout-only guard across the same room.
func crierHall(t *testing.T, loudness string) *encounter.Encounter {
	t.Helper()
	return tableHall(t,
		sheetFacts{noiseAlice: {SpeedFeet: 30}, hCrier: {SpeedFeet: 30}, noiseKeen: {SpeedFeet: 30, PassivePerception: 10}},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hCrier, 7, 0, encounter.Table{encounter.AnswerTime: {{Weight: 1, Say: "Who's there?", Loudness: loudness}}}),
		tabled(noiseKeen, 1, 1, shoutOnly()),
	)
}

// TestAShoutOnlyGuardIgnoresTalkAndAnswersAShout: in one room, a guard
// whose row listens for a shout stands while a creature talks every round,
// and walks toward the same line shouted. A scream is louder than a shout.
func TestAShoutOnlyGuardIgnoresTalkAndAnswersAShout(t *testing.T) {
	for _, tc := range []struct {
		loudness string
		walks    bool
	}{
		{"", false}, {"speak", false}, {"whisper", false}, {"shout", true}, {"scream", true},
	} {
		t.Run("said at "+tc.loudness, func(t *testing.T) {
			enc := crierHall(t, tc.loudness)
			start := whereIs(t, enc, noiseKeen)
			for range 3 {
				_, err := aRound(enc)
				require.NoError(t, err)
			}
			if tc.walks {
				require.NotEqual(t, start, whereIs(t, enc, noiseKeen), "a raised voice draws him")
				require.Less(t, enc.Distance(whereIs(t, enc, noiseKeen), cellAt(7, 0)), enc.Distance(start, cellAt(7, 0)),
					"toward where it was shouted")
				return
			}
			require.Equal(t, start, whereIs(t, enc, noiseKeen), "talk in the room leaves him where he stands")
		})
	}
}

// TestAShoutOnlyGuardIgnoresAPlayersSpokenLineAndAnswersAShoutedOne: a
// player's line is a voice too; spoken, the guard in the room holds; the
// same line shouted draws him.
func TestAShoutOnlyGuardIgnoresAPlayersSpokenLineAndAnswersAShoutedOne(t *testing.T) {
	for _, tc := range []struct {
		loudness string
		walks    bool
	}{{"", false}, {"shout", true}} {
		t.Run("said at "+tc.loudness, func(t *testing.T) {
			goblinIn := at(goblin, encounter.KindMonster, 6, 5)
			goblinIn.Options = speechOptions(tc.loudness, "whisper")
			enc := tableHall(t,
				sheetFacts{alice: {SpeedFeet: 30}, goblin: {SpeedFeet: 30}, noiseKeen: {SpeedFeet: 30, PassivePerception: 10}},
				at(alice, encounter.KindPlayer, 5, 5), goblinIn, tabled(noiseKeen, 1, 1, shoutOnly()),
			)
			start := whereIs(t, enc, noiseKeen)
			_, err := enc.Speak(t.Context(), &encounter.SpeakInput{
				Range: speakReach, Actor: alice, Target: goblin, OptionID: "hello", Skill: "persuasion",
				Beaten: true, DC: 10, Total: 15, Roller: rollsLowest{},
			})
			require.NoError(t, err)
			_, err = aRound(enc)
			require.NoError(t, err)
			if tc.walks {
				require.NotEqual(t, start, whereIs(t, enc, noiseKeen), "her shout draws him")
				return
			}
			require.Equal(t, start, whereIs(t, enc, noiseKeen), "her spoken line does not")
		})
	}
}

// TestAHeldVoiceKeepsItsLoudnessThroughASave: the guard's held voice carries
// its loudness, and the blob carries it back.
func TestAHeldVoiceKeepsItsLoudnessThroughASave(t *testing.T) {
	enc := crierHall(t, "shout")
	_, err := aRound(enc)
	require.NoError(t, err)

	held := func(enc *encounter.Encounter) []sound.Sound {
		holdings, err := enc.View(&encounter.ViewInput{Member: noiseKeen})
		require.NoError(t, err)
		sounds, err := sound.Held(holdings)
		require.NoError(t, err)
		return sounds
	}
	before := held(enc)
	require.Contains(t, loudnessesOf(before), "shout", "precondition: he holds the shout with its loudness")

	setup := noiseHallSetup(encounter.DoorIsOpen(), encounter.DoorIsOpen(), nil, tableDriver())
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: enc.ToData(), Capabilities: setup.Capabilities})
	require.NoError(t, err)
	require.Equal(t, before, held(loaded), "the same sounds, loudness included, after a save and a load")
}

func loudnessesOf(sounds []sound.Sound) []string {
	out := make([]string, 0, len(sounds))
	for _, s := range sounds {
		out = append(out, s.Loudness)
	}
	return out
}
