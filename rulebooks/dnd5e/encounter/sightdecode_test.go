// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sightdecode_test.go is ADR-0041's own half of this module: the composition
// decodes its own encoding rather than handing session (or any other caller)
// bytes to unmarshal itself (rpg-toolkit#1157).

// TestDecodeSightPayloadRoundTripsASightPayload pins the ordinary case: bytes
// this package itself produced decode back to the position they encoded.
func TestDecodeSightPayloadRoundTripsASightPayload(t *testing.T) {
	encoded, err := json.Marshal(encounter.SightPayload{X: 10, Y: 3})
	require.NoError(t, err)

	pos, ok := encounter.DecodeSightPayload(encoded)
	require.True(t, ok, "a payload this package encoded must decode")
	require.Equal(t, spatial.Position{X: 10, Y: 3}, pos)
}

// TestDecodeSightPayloadRefusesWhatItDidNotEncode pins the failure case:
// bytes that are not a SightPayload come back refused rather than
// zero-valued and silently accepted, which would let a caller mistake "not
// sight" for "sight at the origin".
func TestDecodeSightPayloadRefusesWhatItDidNotEncode(t *testing.T) {
	_, ok := encounter.DecodeSightPayload([]byte("not a sight payload"))
	require.False(t, ok)

	_, ok = encounter.DecodeSightPayload(nil)
	require.False(t, ok)
}

// TestDecodeSightPayloadAgreesWithAViewsOwnPayload is item 5 of #1157's
// testable cases: the typed decode and the raw payload a caller could
// unmarshal itself must name the same cell, on a real percept the
// composition produced from geometry rather than a literal this test
// invented.
func TestDecodeSightPayloadAgreesWithAViewsOwnPayload(t *testing.T) {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
			{ID: core.EntityID("skeleton-1"), Kind: encounter.KindMonster, Room: "hall",
				Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	view, err := enc.View(&encounter.ViewInput{Member: alice})
	require.NoError(t, err)

	var holding *intel.Holding
	for i := range view {
		if view[i].Subject == intel.Subject("skeleton-1") {
			holding = &view[i]
		}
	}
	require.NotNil(t, holding, "alice must hold a sighting of the skeleton to test against")

	var raw encounter.SightPayload
	require.NoError(t, json.Unmarshal(holding.Payload, &raw))

	pos, ok := encounter.DecodeSightPayload(holding.Payload)
	require.True(t, ok)
	require.Equal(t, spatial.Position{X: raw.X, Y: raw.Y}, pos,
		"the typed decode must name the same cell as the raw payload")
	require.Equal(t, spatial.Position{X: 5, Y: 6}, pos, "and that cell is really where the skeleton stands")
}
