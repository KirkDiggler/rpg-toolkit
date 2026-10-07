// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// A down beat told before beats carried the fallen member's kind is still a
// fall, and its kind comes from the roster. White-box because no verb writes
// the old shape any more: the beat is appended as a saved world would hold it.
func TestAnOldDownBeatTakesItsKindFromTheRoster(t *testing.T) {
	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: UnobservedEquipment{}, Sheets: zeroSheets{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("crypt", 0, 0, 12, 12)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 0, Y: 10}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)
	baseline, err := enc.NextStorySeq()
	require.NoError(t, err)

	_, err = enc.appendBeat(&record.AppendInput{
		Audience: enc.rosterIDs(), Tags: map[string]string{"tag": "outcome"},
		Payload: []byte(`{"beat":"down","member":"goblin"}`),
	})
	require.NoError(t, err)

	settled, err := enc.Settlement(&SettlementInput{FromSeq: baseline})
	require.NoError(t, err)
	require.Len(t, settled.Falls, 1)
	require.Equal(t, MemberID("goblin"), settled.Falls[0].Member)
	require.Equal(t, KindMonster, settled.Falls[0].Kind, "the roster answers the kind the old beat did not carry")
}
