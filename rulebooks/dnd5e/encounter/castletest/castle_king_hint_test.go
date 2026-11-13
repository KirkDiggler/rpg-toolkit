// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_king_hint_test.go is rpg-project#936: the gate guard's answer to a
// stranger asking after their friends — the hint that a friend can speak for
// them — is SHOUTED. It reaches the next room through the gate's open
// doorway, so a friend inside, out of sight in the yard, catches the words
// without the name; two rooms off, in the great hall, nobody catches
// anything. These read each member's own story, which is what a player is
// shown at `party: { sharing: own }` (the session's merge is the only thing
// the dial changes).

// theHint is gate-1's shouted answer to `friends-inside`.
const theHint = "The troupe? Aye, they’re inside. If one of them comes back and speaks for you, in you go."

// inTheYard is courtyard floor out of both gate guards' sight, one room in
// from the gatehouse through the gate's open doorway.
var inTheYard = spatial.Position{X: -12, Y: 2}

// hintVersion is one member's version of the hint, as their story holds it.
type hintVersion struct {
	Utterance string `json:"utterance"`
	Speaker   string `json:"speaker"`
	To        string `json:"to"`
	Words     string `json:"words"`
	Loudness  string `json:"loudness"`
	Hops      *int   `json:"hops"`
}

// hintIn is member's versions of the hint after seq from.
func (s *kingScene) hintIn(member encounter.MemberID, from uint64) []hintVersion {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: member})
	require.NoError(s.t, err)
	var out []hintVersion
	for _, e := range story {
		if e.Seq < from {
			continue
		}
		var b struct {
			Beat string `json:"beat"`
			hintVersion
		}
		require.NoError(s.t, json.Unmarshal(e.Payload, &b))
		if b.Beat == encounter.BeatSpeech && b.Words == theHint {
			out = append(out, b.hintVersion)
		}
	}
	return out
}

// askAfterFriends brings the straggler to the gatehouse and has them ask
// gate-1 after their friends, returning the record's sequence before it.
func (s *kingScene) askAfterFriends(gatehouse spatial.Position) uint64 {
	s.t.Helper()
	s.walkMember(straggler, gatehouse, false)
	require.True(s.t, s.offersTo(straggler, kingGate1, friendsLine), "precondition: gate-1 offers the straggler the line")
	from := s.enc.ToData().Log.NextSeq
	require.NoError(s.t, s.speakAs(straggler, kingGate1, friendsLine, true))
	return from
}

// TestMadKingTheHintIsShoutedToTheYard: the hero, admitted, stands in the
// yard out of both gate guards' sight. The straggler asks after their friends
// and gate-1 shouts the hint: the hero catches its words with no speaker and
// no one spoken to, from one room off. The hero walks back and vouches with
// the straggler in gate-1's sight, and the straggler is let through.
//
// Who else catches it, and what they do: the shout's voice reaches every
// creature within earshot, and none of them acts on it in the round after —
// gate-1 keeps his post, gate-2, the hall's guard, the barracks' sergeant and
// turnkey keep their states, the two reserves wait, and the throne room's
// keep guards hold no voice at all. (Run against the same scene with the
// answer at `speak`, every creature stands and moves exactly as here: the
// only difference the shout makes to them is the voice the hall's guard now
// holds.)
func TestMadKingTheHintIsShoutedToTheYard(t *testing.T) {
	t.Parallel()
	s := admittedWithAStraggler(t)
	gatehouse := s.at(hero)
	s.walkMember(hero, inTheYard, false)
	require.Equal(t, encounter.RegionID("region-courtyard"), s.regionOf(s.at(hero)), "precondition: the hero is in the yard")
	require.False(t, s.seesLive(kingGate1, hero), "precondition: out of gate-1's sight")
	require.False(t, s.seesLive(kingGate2, hero), "precondition: and gate-2's")
	require.False(t, s.seesLive(hero, kingGate1), "precondition: and she cannot see him")

	from := s.askAfterFriends(gatehouse)

	got := s.hintIn(hero, from)
	require.Len(t, got, 1, "the hero catches the shouted hint")
	require.Equal(t, "", got[0].Speaker, "from someone she did not see")
	require.Equal(t, "", got[0].To, "said to nobody she saw")
	require.Equal(t, "shout", got[0].Loudness)
	require.NotNil(t, got[0].Hops)
	require.Equal(t, 1, *got[0].Hops, "one room off")
	require.Len(t, s.hintIn(straggler, from), 1, "the straggler, spoken to, catches it too")

	// NOTHING THE SHOUT IS CAUGHT BY ACTS ON IT, over the round after it.
	s.aRoundPasses()
	s.onHisPost(kingGate1, "gate-1 keeps his post")
	for id, want := range map[encounter.MemberID]string{
		kingGate2: "ready", "hall-1": "ready", "sergeant": "ready", "turnkey": "ready",
		"reserve-1": "waiting", "reserve-2": "waiting", kingKeep1: "ready", kingKeep2: "ready",
	} {
		require.Equal(t, want, machineState(t, s.enc, id), "%s keeps its state", id)
	}
	for _, id := range []encounter.MemberID{"hall-1", "sergeant", "turnkey", "reserve-1", "reserve-2"} {
		require.True(t, s.heardKind(id, encounter.SoundVoice), "%s holds the voice", id)
	}
	for _, id := range []encounter.MemberID{kingKeep1, kingKeep2} {
		require.False(t, s.heardKind(id, encounter.SoundVoice), "%s, in the throne room, holds none", id)
	}

	// The hero walks back and vouches with the straggler in gate-1's sight.
	s.walkMember(hero, gatehouse, false)
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	require.True(t, s.recognizes(kingGate1, straggler), "the vouch admits the straggler")
	require.Empty(t, s.heldAgainst(straggler), "and the gate lets them through")
}

// theHallWithItsDoor is the admitted scene with the hero beside the captain
// in the great hall — two rooms in from the gatehouse — and the hall's door
// as given.
func theHallWithItsDoor(t *testing.T, state encounter.DoorStateKind) *kingScene {
	t.Helper()
	admittedScene.once.Do(buildTheAdmittedScene)
	require.NoError(t, admittedScene.err)
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(admittedScene.inTheHall, &data))
	found := false
	for i := range data.Doors {
		if data.Doors[i].ID == "the-mad-king/hall-wall-door" {
			data.Doors[i].State, found = string(state), true
		}
	}
	require.True(t, found, "precondition: the hall's door")
	blob, err := json.Marshal(data)
	require.NoError(t, err)
	s := loadTheScene(t, blob)
	require.Equal(t, encounter.RegionID("region-great-hall"), s.regionOf(s.at(hero)), "precondition: the hero is in the great hall")
	return s
}

// gateCell is where the hero stood at the gate in the admitted scene, where
// the straggler comes to ask.
func gateCell(t *testing.T) spatial.Position {
	t.Helper()
	return admittedWithAStraggler(t).at(hero)
}

// speechSince is every speech version in member's story after seq from.
func (s *kingScene) speechSince(member encounter.MemberID, from uint64) []string {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: member, AfterSeq: from})
	require.NoError(s.t, err)
	var out []string
	for _, e := range story {
		var b struct {
			Beat string `json:"beat"`
		}
		require.NoError(s.t, json.Unmarshal(e.Payload, &b))
		if b.Beat == encounter.BeatSpeech {
			out = append(out, string(e.Payload))
		}
	}
	return out
}

// TestMadKingTheHintDoesNotReachTheGreatHall: the hero in the great hall,
// two rooms in from the gatehouse. Behind its CLOSED door she is shown
// nothing of the shout at all. With the door standing open (as the captain
// leaves it) the words still do not reach her — a shout carries one room —
// and she may notice only a voice two rooms off, by passive Perception,
// with neither words nor a name.
func TestMadKingTheHintDoesNotReachTheGreatHall(t *testing.T) {
	t.Parallel()
	gate := gateCell(t)

	t.Run("behind the closed door, nothing", func(t *testing.T) {
		t.Parallel()
		s := theHallWithItsDoor(t, encounter.DoorClosed)
		from := s.askAfterFriends(gate)
		require.Empty(t, s.speechSince(hero, from), "no version of anything said at the gate")
	})
	t.Run("through the open door, a voice and no words", func(t *testing.T) {
		t.Parallel()
		s := theHallWithItsDoor(t, encounter.DoorOpen)
		from := s.askAfterFriends(gate)
		require.Empty(t, s.hintIn(hero, from), "two rooms off, no words")
		for _, v := range s.speechSince(hero, from) {
			var b hintVersion
			require.NoError(t, json.Unmarshal([]byte(v), &b))
			require.Empty(t, b.Words, "no words: %s", v)
			require.Empty(t, b.Speaker, "no name: %s", v)
		}
	})
}

// regionOf is the authored region a cell is in.
func (s *kingScene) regionOf(cell spatial.Position) encounter.RegionID {
	s.t.Helper()
	id, ok := s.enc.RegionAt(cell)
	require.True(s.t, ok, "%v is floor", cell)
	return id
}
