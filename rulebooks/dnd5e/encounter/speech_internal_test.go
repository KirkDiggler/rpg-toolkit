// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// speech_internal_test.go is perceived speech (rpg-project#933) where it
// meets recognition and a kept roll.

// vouchHall is the hold hall with two gate guards carrying the vouch at the
// loudness given, the second guard seeing as far as given, alice admitted
// and bob come up into the posted guard's sight.
func vouchHall(t *testing.T, loudness string, secondSees int) *Encounter {
	t.Helper()
	return vouchHallWith(t, func(id string) string {
		if id == "vouch" {
			return loudness
		}
		return ""
	}, secondSees)
}

// whisperedHall is [vouchHall] with every line the gate guards offer
// whispered and gate-2 seeing the whole hall.
func whisperedHall(t *testing.T) *Encounter {
	t.Helper()
	return vouchHallWith(t, func(string) string { return "whisper" }, 100)
}

// vouchHallWith is [vouchHall] with each option's loudness named by id.
func vouchHallWith(t *testing.T, loudnessOf func(id string) string, secondSees int) *Encounter {
	t.Helper()
	guard := func(id MemberID, at spatial.Position) MemberInput {
		g := gateGuard(t, id, at)
		for i := range g.Options {
			if l := loudnessOf(g.Options[i].ID); l != "" {
				g.Options[i].Loudness = l
			}
		}
		return g
	}
	in := recognisingSetup(t, true,
		guard(postedGuard, holdNorthWest), guard("gate-2", spatial.Position{X: 2, Y: 0}),
		MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 4}},
	)
	in.Sight = fewSighted{postedGuard: 2, "gate-2": secondSees}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	placeAt(t, enc, "bob", spatial.Position{X: 3, Y: 1})
	return enc
}

func TestTheHeardChannelFollowsTheWords(t *testing.T) {
	t.Run("a whispered vouch is held by nobody who only saw it", func(t *testing.T) {
		enc := vouchHall(t, "whisper", 100)
		placeAt(t, enc, "alice", holdNorthEast)
		_, err := say(t, enc, "alice", postedGuard, "vouch", true)
		require.NoError(t, err)
		require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "the guard it was whispered to")
		_, err = enc.intelLog.AboutOn(sightMember("gate-2"), sightMember("bob"), perception.Topic(admittedTopic))
		require.Error(t, err, "gate-2 saw it said and caught none of it: no heard record")
	})
	t.Run("a spoken vouch is held by a carrier who caught it unseeing", func(t *testing.T) {
		enc := vouchHall(t, "", 0)
		_, err := say(t, enc, "alice", postedGuard, "vouch", true)
		require.NoError(t, err)
		held, err := enc.intelLog.AboutOn(sightMember("gate-2"), sightMember("bob"), perception.Topic(admittedTopic))
		require.NoError(t, err, "gate-2, blind, caught the words in the room")
		require.Equal(t, perception.Channel(ChannelHeard), held.Channel)
	})
}

// TestAKeptRollIsPerceivedAsItWasSaid: bob's case is settled after he walked
// out of the posted guard's sight; the guard's version still names bob and
// carries the words, as he perceived them when they were said, though he
// cannot see bob at the settle.
func TestAKeptRollIsPerceivedAsItWasSaid(t *testing.T) {
	enc, at := walkedAway(t)
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.False(t, reach.Reaches(perception.Sight, postedGuard, "bob"), "precondition: he cannot see bob now")
	require.NoError(t, settleTheCase(t, enc, at, false))

	story, err := enc.Story(&StoryInput{Audience: postedGuard})
	require.NoError(t, err)
	var version map[string]any
	for _, entry := range story {
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		if b["beat"] == BeatSpeech && b["option"] == "straggler" {
			version = b
		}
	}
	require.NotNil(t, version, "the guard was written a version of bob's line")
	require.Equal(t, "bob", version["speaker"], "named, as he saw bob say it")
	require.Contains(t, version, "check", "and the roll with the words")
}

// TestAKeptRollSavedBeforeHearingsReachesOnlyItsTwo pins a scene kept before
// speech was perceived (no [ReservedScene.Heard], no SawTarget): its settle
// is caught by the speaker and the one spoken to alone. Those it names as
// witnesses see the speaker say it and catch nothing — the scene never
// recorded who heard, and a guess would invent a hearer.
func TestAKeptRollSavedBeforeHearingsReachesOnlyItsTwo(t *testing.T) {
	enc := vouchHall(t, "", 2)
	u := utterance{
		speaker: "bob", to: postedGuard, words: "He's with me.",
		scene: &ReservedScene{Where: spatial.Position{X: 3, Y: 1}, Witnesses: []MemberID{postedGuard, "gate-2", "alice"}},
	}
	facets := map[MemberID]facet{}
	for _, f := range enc.perceivedAsSpoken(u) {
		facets[f.member] = f
	}
	require.True(t, facets["bob"].words, "the speaker")
	require.True(t, facets[postedGuard].words, "and the one spoken to")
	for _, id := range []MemberID{"gate-2", "alice"} {
		require.True(t, facets[id].sees, "%s saw it said", id)
		require.False(t, facets[id].words, "%s caught no words the scene never said reached them", id)
		require.False(t, facets[id].noticed, "%s noticed no voice either", id)
		require.False(t, facets[id].seesTo, "nor whom it was said to: SawTarget was never kept")
	}
}

// TestOneWhoOnlySawALineThatGrantsLearnsNothingOfIt holds the read-back's
// sentence — "whoever only sees it said learns neither the words, which line
// it was, whether it worked, nor what it taught" — to the engine, on lines
// whose answer grants a recognition (rpg-project#933). The lines are
// whispered, so the watchers see them said and catch none of it: neither is
// written a recognition, a verdict, a line, a fact, a topic or a deed of it,
// and neither comes to hold its topic; the guard it was said to does. (The
// guard's spoken reply is its own utterance, and its versions are its own.)
func TestOneWhoOnlySawALineThatGrantsLearnsNothingOfIt(t *testing.T) {
	whispered := func(g MemberInput) MemberInput {
		for i := range g.Options {
			g.Options[i].Loudness = "whisper"
		}
		return g
	}
	for _, tc := range []struct {
		name     string
		open     func(t *testing.T) *Encounter
		speaker  MemberID
		option   string
		topic    TopicID
		beaten   bool
		watchers []MemberID
	}{
		{
			name: "a vouch admits",
			open: func(t *testing.T) *Encounter {
				enc := whisperedHall(t)
				placeAt(t, enc, "alice", holdNorthEast)
				return enc
			},
			speaker: "alice", option: "vouch", topic: admittedTopic, beaten: true,
			watchers: []MemberID{"gate-2", "bob"},
		},
		{
			name: "a missed straggler's case turns him away",
			open: func(t *testing.T) *Encounter {
				in := recognisingSetup(t, true,
					whispered(oneTryGuard(t, postedGuard, holdNorthWest)),
					whispered(oneTryGuard(t, "gate-2", spatial.Position{X: 2, Y: 0})),
					MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 4}},
					MemberInput{ID: "carol", Kind: KindPlayer, Position: spatial.Position{X: 9, Y: 4}},
				)
				in.Sight = shortSighted{member: postedGuard, cells: 2}
				enc, err := NewEncounter(in)
				require.NoError(t, err)
				admit(t, enc, postedGuard)
				placeAt(t, enc, "alice", spatial.Position{X: 8, Y: 3})
				placeAt(t, enc, "bob", holdSouthEast)
				placeAt(t, enc, "carol", spatial.Position{X: 3, Y: 1})
				return enc
			},
			speaker: "bob", option: "straggler", topic: turnedAwayTopic, beaten: false,
			watchers: []MemberID{"gate-2", "carol"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc := tc.open(t)
			before := map[MemberID]int{}
			for _, w := range tc.watchers {
				entries, err := enc.Story(&StoryInput{Audience: w})
				require.NoError(t, err)
				before[w] = len(entries)
			}
			_, err := say(t, enc, tc.speaker, postedGuard, tc.option, tc.beaten)
			require.NoError(t, err)
			require.True(t, enc.recognizes(postedGuard, "bob", tc.topic), "precondition: the guard it was said to holds it")

			for _, w := range tc.watchers {
				require.False(t, enc.recognizes(w, "bob", tc.topic), "%s holds nothing of it", w)
				entries, err := enc.Story(&StoryInput{Audience: w})
				require.NoError(t, err)
				saw := false
				for _, e := range entries[before[w]:] {
					var b map[string]any
					require.NoError(t, json.Unmarshal(e.Payload, &b))
					require.NotEqual(t, BeatRecognized, b["beat"], "%s is told no recognition the line granted: %v", w, b)
					if b["beat"] == BeatSpeech {
						if b["speaker"] == string(tc.speaker) {
							saw = true
							require.NotContains(t, b, "words", "%s caught no words of the line", w)
						}
						continue
					}
					for _, key := range []string{"option", "beaten", "fact", "topic", "check", "words"} {
						require.NotContains(t, b, key, "%s: a %s beat tells the line's %s", w, b["beat"], key)
					}
					require.NotEqual(t, "fact", b["word"], "%s: a %s beat says the line taught a fact", w, b["beat"])
				}
				require.True(t, saw, "precondition: %s saw it said", w)
			}
		})
	}
}

// TestALiveLineGrantNamesTheHolderOnlyToWhoSeesThem is the live path of the
// holder rule (rpg-project#933): carol, in the hall and seeing only a cell
// round her, catches alice's vouch and sees bob beside her, not the guard it
// was said to. She is told that bob came to be admitted by someone, never
// who; alice, who sees the guard, is told it is him.
func TestALiveLineGrantNamesTheHolderOnlyToWhoSeesThem(t *testing.T) {
	in := recognisingSetup(t, true,
		gateGuard(t, postedGuard, holdNorthWest),
		MemberInput{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 4}},
		MemberInput{ID: "carol", Kind: KindPlayer, Position: spatial.Position{X: 9, Y: 4}},
	)
	in.Sight = fewSighted{"carol": 1}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	placeAt(t, enc, "alice", holdNorthEast)
	placeAt(t, enc, "bob", spatial.Position{X: 7, Y: 3})
	placeAt(t, enc, "carol", spatial.Position{X: 8, Y: 3})
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.False(t, reach.Reaches(perception.Sight, "carol", postedGuard), "precondition: carol cannot see the guard")
	require.True(t, reach.Reaches(perception.Sight, "carol", "bob"), "precondition: she sees bob")

	before := len(enc.ToData().Log.Entries)
	_, err = say(t, enc, "alice", postedGuard, "vouch", true)
	require.NoError(t, err)
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "precondition: the vouch admitted bob")

	told := map[MemberID][]map[string]any{}
	data := enc.ToData()
	for _, entry := range data.Log.Entries[before:] {
		if entry.Tags["tag"] != BeatRecognized || entry.AudienceRef == 0 {
			continue
		}
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		for _, id := range data.Log.Audiences[entry.AudienceRef-1] {
			told[MemberID(id)] = append(told[MemberID(id)], b)
		}
	}
	caught := false
	story, err := enc.Story(&StoryInput{Audience: "carol"})
	require.NoError(t, err)
	for _, e := range story {
		var v map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &v))
		caught = caught || (v["beat"] == BeatSpeech && v["option"] == "vouch" && v["words"] != nil)
	}
	require.True(t, caught, "precondition: carol caught the vouch's words")
	require.NotEmpty(t, told["carol"], "carol is told the recognition she heard granted")
	for _, b := range told["carol"] {
		require.NotContains(t, b, "holder", "she is told someone holds it, never who: %v", b)
	}
	named := false
	for _, b := range told["alice"] {
		named = named || b["holder"] == string(postedGuard)
	}
	require.True(t, named, "alice, who sees the guard, is told it is him")
}
