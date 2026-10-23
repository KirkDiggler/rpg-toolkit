// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sightdecode_test.go is ADR-0041's own half of this module: the composition
// decodes its own encoding rather than handing session (or any other caller)
// bytes to unmarshal itself (rpg-toolkit#1157).

func TestLocationPayloadRoundTrip(t *testing.T) {
	known := encounter.SightTestimony{State: encounter.LocationKnown, Position: spatial.Position{X: 0, Y: -2}}
	payload, err := encounter.EncodeSightTestimony(known)
	require.NoError(t, err)
	require.JSONEq(t, `{"state":"known","x":0,"y":-2}`, string(payload))

	got, ok := encounter.DecodeSightTestimony(payload)
	require.True(t, ok)
	require.Equal(t, known, got)

	unknown, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationUnknown})
	require.NoError(t, err)
	require.JSONEq(t, `{"state":"unknown"}`, string(unknown))
	gotUnknown, ok := encounter.DecodeSightTestimony(unknown)
	require.True(t, ok)
	require.Equal(t, encounter.SightTestimony{State: encounter.LocationUnknown}, gotUnknown)
}

// TestDecodeSightTestimonyRefusesAnUntaggedPayload: a payload with no state is
// not read as a known location. The untagged form predates every fact but
// position, and a position with no state is "sight at the origin" waiting to
// happen to a payload that merely failed to say what it is.
func TestDecodeSightTestimonyRefusesAnUntaggedPayload(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{name: "coordinates only", payload: `{"x":0,"y":4}`},
		{name: "empty object", payload: `{}`},
		{name: "only x", payload: `{"x":1}`},
		{name: "only y", payload: `{"y":1}`},
		{name: "empty state", payload: `{"state":"","x":1,"y":2}`},
		{name: "the room-bearing dialect", payload: `{"room":"hall","x":2,"y":5}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := encounter.DecodeSightTestimony([]byte(tc.payload))
			require.False(t, ok)
		})
	}
}

func TestDecodeSightTestimonyRejectsMalformedOrContradictoryShapes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{name: "empty", payload: ``},
		{name: "null", payload: `null`},
		{name: "unsupported state", payload: `{"state":"maybe","x":1,"y":2}`},
		{name: "known without x", payload: `{"state":"known","y":2}`},
		{name: "known without y", payload: `{"state":"known","x":1}`},
		{name: "unknown with x", payload: `{"state":"unknown","x":1}`},
		{name: "unknown with y", payload: `{"state":"unknown","y":2}`},
		{name: "unknown with null x", payload: `{"state":"unknown","x":null}`},
		{name: "extra field", payload: `{"state":"known","x":1,"y":2,"extra":true}`},
		{name: "case variant state", payload: `{"State":"unknown"}`},
		{name: "case variant coordinate", payload: `{"state":"known","X":1,"y":2}`},
		{name: "non numeric x", payload: `{"state":"known","x":"1","y":2}`},
		{name: "non numeric y", payload: `{"state":"known","x":1,"y":"2"}`},
		{name: "duplicate state", payload: `{"state":"known","state":"known","x":1,"y":2}`},
		{name: "conflicting state", payload: `{"state":"known","state":"unknown","x":1,"y":2}`},
		{name: "duplicate x", payload: `{"state":"known","x":1,"x":1,"y":2}`},
		{name: "conflicting x", payload: `{"state":"known","x":1,"x":2,"y":2}`},
		{name: "duplicate y", payload: `{"state":"known","x":1,"y":2,"y":2}`},
		{name: "conflicting y", payload: `{"state":"known","x":1,"y":2,"y":3}`},
		{name: "trailing json", payload: `{"state":"known","x":1,"y":2}{}`},
		// A facing key must hold a bearing in [0, 360) (rpg-project#582).
		{name: "null facing", payload: `{"state":"known","x":1,"y":2,"facing":null}`},
		{name: "facing out of range", payload: `{"state":"known","x":1,"y":2,"facing":400}`},
		{name: "negative facing", payload: `{"state":"known","x":1,"y":2,"facing":-1}`},
		{name: "unknown with facing", payload: `{"state":"unknown","facing":90}`},
		// A heading key is a bearing on facing's rule (rpg-project#679).
		{name: "null heading", payload: `{"state":"known","x":1,"y":2,"heading":null}`},
		{name: "heading out of range", payload: `{"state":"known","x":1,"y":2,"heading":360}`},
		{name: "unknown with heading", payload: `{"state":"unknown","heading":90}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := encounter.DecodeSightTestimony([]byte(tc.payload))
			require.False(t, ok)
		})
	}
}

func TestEncodeSightTestimonyRejectsInvalidKnowledge(t *testing.T) {
	for _, knowledge := range []encounter.SightTestimony{
		{State: "maybe", Position: spatial.Position{X: 1, Y: 2}},
		{State: encounter.LocationUnknown, Position: spatial.Position{Y: 1}},
	} {
		_, err := encounter.EncodeSightTestimony(knowledge)
		require.ErrorIs(t, err, encounter.ErrInvalidData)
	}
}

// TestDecodeSightTestimonyAgreesWithAViewsOwnPayload is item 5 of #1157's
// testable cases: the typed decode and the raw payload a caller could
// unmarshal itself must name the same cell, on a real percept the
// composition produced from geometry rather than a literal this test
// invented.
func TestDecodeSightTestimonyAgreesWithAViewsOwnPayload(t *testing.T) {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: core.EntityID("skeleton-1"), Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	view, err := enc.View(&encounter.ViewInput{Member: alice})
	require.NoError(t, err)

	var holding *perception.Holding
	for i := range view {
		if view[i].Subject == "skeleton-1" {
			holding = &view[i]
		}
	}
	require.NotNil(t, holding, "alice must hold a sighting of the skeleton to test against")

	var raw sightPayload
	require.NoError(t, json.Unmarshal(holding.Payload, &raw))

	testimony, ok := encounter.DecodeSightTestimony(holding.Payload)
	require.True(t, ok)
	require.Equal(t, encounter.LocationKnown, testimony.State)
	require.Equal(t, spatial.Position{X: raw.X, Y: raw.Y}, testimony.Position,
		"the typed decode must name the same cell as the raw payload")
	require.Equal(t, cellAt(5, 6), testimony.Position, "and that cell is really where the skeleton stands")
}

// TestLostTestimonyKeepsItsCellAndFacing is rpg-project#668 R3's encoding: a
// lost memory carries everything a known one did, under its own state, and
// never decodes without the cell it was lost at.
func TestLostTestimonyKeepsItsCellAndFacing(t *testing.T) {
	east := 90.0
	lost := encounter.SightTestimony{State: encounter.LocationLost, Position: spatial.Position{X: 3, Y: -1}, Facing: &east}
	payload, err := encounter.EncodeSightTestimony(lost)
	require.NoError(t, err)
	require.JSONEq(t, `{"state":"lost","x":3,"y":-1,"facing":90}`, string(payload))

	got, ok := encounter.DecodeSightTestimony(payload)
	require.True(t, ok)
	require.Equal(t, lost, got)

	_, ok = encounter.DecodeSightTestimony([]byte(`{"state":"lost"}`))
	require.False(t, ok, "a lost memory names where it was lost")
}

// TestLostTestimonyKeepsTheWayItWent is rpg-project#679's encoding: the
// heading rides beside the facing under its own key, and decodes back as it
// was written.
func TestLostTestimonyKeepsTheWayItWent(t *testing.T) {
	east, north := 90.0, 0.0
	lost := encounter.SightTestimony{
		State: encounter.LocationLost, Position: spatial.Position{X: 3, Y: -1}, Facing: &north, Heading: &east,
	}
	payload, err := encounter.EncodeSightTestimony(lost)
	require.NoError(t, err)
	require.JSONEq(t, `{"state":"lost","x":3,"y":-1,"facing":0,"heading":90}`, string(payload))

	got, ok := encounter.DecodeSightTestimony(payload)
	require.True(t, ok)
	require.Equal(t, lost, got)

	_, err = encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationUnknown, Heading: &east})
	require.ErrorIs(t, err, encounter.ErrInvalidData, "an unknown subject has no way it went")
}
