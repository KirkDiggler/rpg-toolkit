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
	guard := func(id MemberID, at spatial.Position) MemberInput {
		g := gateGuard(t, id, at)
		for i := range g.Options {
			if g.Options[i].ID == "vouch" {
				g.Options[i].Loudness = loudness
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
