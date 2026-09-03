// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// memberdown_internal_test.go pins the one-close law from the inside: a
// sight refresh can close the encounter mid-verb (TriggerMemberDown fires in
// noticeDown), and the same verb's later reached-position scan must report
// that close rather than fire a second one — one outcome, one ended beat
// (Copilot round on rpg-toolkit#1241).

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// somebodyDown is a Standing capability the test can flip mid-scene.
type somebodyDown struct {
	down []MemberID
}

func (s *somebodyDown) Standing([]MemberID) ([]MemberID, error) {
	return s.down, nil
}

func (s *somebodyDown) Assess(members []MemberID) (*ParticipationAssessment, error) {
	return testAssessmentFromDown(members, s.down), nil
}

func TestAClosedEncounterIsNotClosedAgainByAnArrival(t *testing.T) {
	stairs := spatial.Position{X: 1, Y: 1}
	standing := &somebodyDown{}

	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: FieldInput{
			Canvas:  CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
			Regions: []RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: "g1", Kind: KindMonster, Position: spatial.Position{X: 7, Y: 7}},
		},
		Endings: []EndingInput{
			{Key: "stairs", Trigger: TriggerReachedPosition{Position: stairs}},
			{Key: "boss-down", Trigger: TriggerMemberDown{Member: "g1"}},
		},
	})
	require.NoError(t, err)

	// Close it the member-down way: the consult notices the body.
	standing.down = []MemberID{"g1"}
	_, err = enc.Pump(&PumpInput{})
	require.NoError(t, err)
	require.NotNil(t, enc.outcome, "control: the consult closed the run")
	require.Equal(t, "boss-down", enc.outcome.Ending)
	firstAt := enc.outcome.At

	// The scan that would have double-closed: the member stands on the
	// reached-position ending's cell in the same pass, and the scan reports
	// the EXISTING close instead of firing "stairs" over it.
	fired, err := enc.firedReachedPosition(enc.members["alice"], stairs, firstAt+1)
	require.NoError(t, err)
	require.NotNil(t, fired, "the verb still reports the close on its output")
	require.Equal(t, "boss-down", fired.Ending, "whichever trigger fired it — not the arrival's")
	require.Equal(t, firstAt, fired.At, "the first close, not a rewrite")

	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	endeds := 0
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == "ended" {
			endeds++
		}
	}
	require.Equal(t, 1, endeds, "one close, one ended beat")
}
