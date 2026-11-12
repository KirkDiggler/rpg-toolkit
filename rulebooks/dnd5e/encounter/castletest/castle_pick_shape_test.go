// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_pick_shape_test.go is rpg-project#948: the captain's stand-aside has
// a success table and a failure table of different shapes, and a member who
// sees the captain but caught none of the line must not read which it was
// off his pick.

// inTheGreatHall is where a member stands who sees nothing at the gate.
var inTheGreatHall = spatial.Position{X: -6, Y: -16}

// faraway is a member in the great hall.
const faraway = encounter.MemberID("faraway")

// standAsideSeenNotHeard plays the hero's stand-aside, WHISPERED for this
// route so the listener seated off the captain sees it said and catches
// nothing of it, and returns every beat the listener is written by the line
// itself (the beats outside any later act), in order.
func standAsideSeenNotHeard(t *testing.T, beaten bool) []map[string]any {
	t.Helper()
	return standAsideWrites(t, beaten)[listener]
}

// standAsideWrites is [standAsideSeenNotHeard] for every member: each one's
// beats from the line itself, a member in the great hall who sees nothing
// of it among them.
func standAsideWrites(t *testing.T, beaten bool) map[encounter.MemberID][]map[string]any {
	t.Helper()
	s := openTheCastleAs(t, castleHost{at: gatehouseSeat, loudness: map[string]string{"stand-aside": "whisper"},
		players: []encounter.MemberInput{{ID: faraway, Kind: encounter.KindPlayer, Position: inTheGreatHall}}})
	s.walkTo(s.at(kingCaptain))
	s.joinListener()
	before := len(s.enc.ToData().Log.Entries)
	s.said(kingCaptain, "stand-aside", "intimidation", beaten)
	data := s.enc.ToData()
	out := map[encounter.MemberID][]map[string]any{}
	for _, e := range data.Log.Entries[before:] {
		if e.AudienceRef == 0 || e.Tags[encounter.ActTag] != "" {
			continue
		}
		var b map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		for _, who := range data.Log.Audiences[e.AudienceRef-1] {
			out[encounter.MemberID(who)] = append(out[encounter.MemberID(who)], b)
		}
	}
	return out
}

// TestMadKingTheStandAsidesPickTellsAWatcherNothing: the listener's version
// of the captain's pick is the same body, byte for byte, whether the
// stand-aside landed or missed: who answered, and nothing of the die. What
// else differs for her is what she sees happen, or hears said aloud.
func TestMadKingTheStandAsidesPickTellsAWatcherNothing(t *testing.T) {
	t.Parallel()
	won, lost := standAsideSeenNotHeard(t, true), standAsideSeenNotHeard(t, false)
	pick := func(beats []map[string]any) []byte {
		t.Helper()
		var picks [][]byte
		for _, b := range beats {
			if b["beat"] == encounter.BeatAnswered && b["creature"] == string(kingCaptain) {
				raw, err := json.Marshal(b)
				require.NoError(t, err)
				picks = append(picks, raw)
			}
		}
		require.Len(t, picks, 1, "she sees him answer, once")
		return picks[0]
	}
	require.Equal(t, string(pick(won)), string(pick(lost)), "the same pick, landed or missed")
	require.JSONEq(t, `{"beat":"answered","creature":"captain"}`, string(pick(won)), "who answered, and nothing of the die")

	// EVERYTHING ELSE THAT DIFFERS for her is seen or heard: his answer,
	// spoken aloud in her room; on a landing, the world's round and the
	// guards turning cowed (the state beat, as today: rpg-project#948 leaves
	// it for the operator); on a miss, the garrison's stance and the fight.
	differ := map[string]bool{}
	seen := func(beats []map[string]any) map[string]int {
		out := map[string]int{}
		for _, b := range beats {
			raw, err := json.Marshal(b)
			require.NoError(t, err)
			out[string(raw)]++
		}
		return out
	}
	a, b := seen(won), seen(lost)
	for _, beats := range []struct {
		mine, theirs map[string]int
	}{{a, b}, {b, a}} {
		for raw, n := range beats.mine {
			if beats.theirs[raw] != n {
				var body map[string]any
				require.NoError(t, json.Unmarshal([]byte(raw), &body))
				differ[body["beat"].(string)] = true
			}
		}
	}
	var kinds []string
	for k := range differ {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	require.Equal(t, []string{"bubble-formed", "speech", "stance", "state", "tick", "turn-ended"}, kinds,
		"only what she sees happen or hears said differs")
	for _, body := range won {
		if body["beat"] == encounter.BeatSpeech && body["speaker"] == string(hero) {
			require.NotContains(t, body, "words", "she caught none of the line")
		}
	}
}

// TestMadKingAStateChangeReachesTheWholeRun pins the state beat's audience
// today (rpg-project#948): the captain's `ready -> cowed`, with its reason,
// reaches the listener who watched the stand-aside and caught none of it,
// AND a member in the great hall who saw and heard nothing of it — every
// member of the run. Whether it should reach only those who see him is the
// operator's to rule; this is what such a ruling would change.
func TestMadKingAStateChangeReachesTheWholeRun(t *testing.T) {
	t.Parallel()
	writes := standAsideWrites(t, true)
	cowed := map[string]any{"beat": "state", "member": "captain", "from": "ready", "to": "cowed", "reason": "stood aside"}
	for _, who := range []encounter.MemberID{listener, faraway} {
		var states []map[string]any
		for _, b := range writes[who] {
			if b["beat"] == encounter.BeatState && b["member"] == string(kingCaptain) {
				states = append(states, b)
			}
		}
		require.Equal(t, []map[string]any{cowed}, states, "%s is told the change and its reason", who)
	}
	for _, b := range writes[faraway] {
		require.NotEqual(t, encounter.BeatSpeech, b["beat"], "precondition: the great hall perceives none of the line")
		require.NotEqual(t, encounter.BeatAnswered, b["beat"], "nor sees the captain answer")
	}
}
