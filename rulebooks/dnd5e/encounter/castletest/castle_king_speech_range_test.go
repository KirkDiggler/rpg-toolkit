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

// castle_king_speech_range_test.go is rpg-project#953 on the castle's gate:
// a line spoken at `speak` carries [encounter.SpeechCarryFeet] through the
// gate's OPEN doorway into the next room, measured along the way through the
// doorway; a closed gate stops it. Played at the session's sight
// ([hintScene]), the hero WALKED ([kingScene.walkedTo]).

var (
	// justInsideTheGate is courtyard floor one step inside the open gate,
	// where the hero stood to vouch in the operator's run.
	justInsideTheGate = spatial.Position{X: -3, Y: 6}
	// gate2InTheRun is gatehouse floor two cells from [justInsideTheGate],
	// where gate-2 stood in the operator's run.
	gate2InTheRun = spatial.Position{X: -2, Y: 8}
	// behindTheGate is courtyard floor just behind the gate.
	behindTheGate = spatial.Position{X: -3, Y: 5}
	// deepInTheYard is courtyard floor more than the carry from the
	// straggler beside gate-1, even in a straight line.
	deepInTheYard = spatial.Position{X: 4, Y: 2}
)

// theGateWith is [hintScene]'s moment east of the gate with gate-2 and the
// straggler stood as the operator's run had them and the gate as given.
func theGateWith(t *testing.T, gate encounter.DoorStateKind) *kingScene {
	t.Helper()
	hintScene.once.Do(buildTheHintScene)
	require.NoError(t, hintScene.err, "the admitted scene at the session's sight")
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(hintScene.moments[momentEastOfTheGate], &data))
	found := false
	for i := range data.Doors {
		if data.Doors[i].ID == string(kingGateDoor) {
			data.Doors[i].State, found = string(gate), true
		}
	}
	require.True(t, found, "precondition: the gate")
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return placedIn(t, raw, map[encounter.MemberID]spatial.Position{kingGate2: gate2InTheRun, straggler: besideGate1}, "",
		hintScene.compiled, hintScene.host)
}

// caughtFrom is the line ids member caught the words of after seq from.
func (s *kingScene) caughtFrom(member encounter.MemberID, from uint64) []string {
	s.t.Helper()
	var out []string
	for _, v := range s.speechSince(member, from) {
		var b struct {
			Option string `json:"option"`
			Words  string `json:"words"`
		}
		require.NoError(s.t, json.Unmarshal([]byte(v), &b))
		if b.Words != "" && b.Option != "" {
			out = append(out, b.Option)
		}
	}
	return out
}

// TestMadKingAVouchFromInsideTheGateCarries is the operator's run
// (rpg-project#953): the hero, walked one step inside the open gate, vouches
// to gate-1; gate-2 and the straggler in the gatehouse, two cells off, catch
// the words; gate-2 recognises the straggler, and the straggler walks
// through.
func TestMadKingAVouchFromInsideTheGateCarries(t *testing.T) {
	t.Parallel()
	s := theGateWith(t, encounter.DoorOpen)
	s.walkedTo(justInsideTheGate)
	require.Equal(t, encounter.RegionID("region-courtyard"), s.regionOf(s.at(hero)), "precondition: the hero is in the yard")
	require.Equal(t, encounter.RegionID("region-gatehouse"), s.regionOf(s.at(kingGate2)), "precondition: gate-2 is in the gatehouse")

	from := s.enc.ToData().Log.NextSeq
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	require.Contains(t, s.caughtFrom(kingGate2, from), vouchLine, "gate-2 catches the vouch through the open gate")
	require.Contains(t, s.caughtFrom(straggler, from), vouchLine, "and so does the straggler")
	require.True(t, s.recognizes(kingGate2, straggler), "gate-2 recognises the straggler")
	require.Empty(t, s.heldAgainst(straggler), "and the gate lets them through")
	s.walkMember(straggler, behindTheGate, false)
	require.Equal(t, encounter.RegionID("region-courtyard"), s.regionOf(s.at(straggler)), "the straggler walks into the yard")
}

// TestMadKingSpeechThroughTheGateHasARange: the straggler beside gate-1 asks
// after their friends, spoken. The hero walked just behind the open gate
// catches the words; walked deep into the yard, beyond the carry, does not;
// and behind the CLOSED gate, however near, catches nothing.
func TestMadKingSpeechThroughTheGateHasARange(t *testing.T) {
	t.Parallel()
	t.Run("just behind the open gate", func(t *testing.T) {
		t.Parallel()
		s := theGateWith(t, encounter.DoorOpen)
		s.walkedTo(behindTheGate)
		require.Contains(t, s.caughtFrom(hero, s.askAfterFriends()), friendsLine)
	})
	t.Run("deep in the yard", func(t *testing.T) {
		t.Parallel()
		s := theGateWith(t, encounter.DoorOpen)
		s.walkedTo(deepInTheYard)
		require.Greater(t, s.enc.Distance(s.at(straggler), s.at(hero)), float64(encounter.CellsFromFeet(encounter.SpeechCarryFeet)),
			"precondition: beyond the carry even in a straight line")
		require.NotContains(t, s.caughtFrom(hero, s.askAfterFriends()), friendsLine)
	})
	t.Run("behind the closed gate", func(t *testing.T) {
		t.Parallel()
		s := theGateWith(t, encounter.DoorClosed)
		s.walkedTo(behindTheGate)
		require.Empty(t, s.caughtFrom(hero, s.askAfterFriends()), "a closed gate stops the words")
	})
}
