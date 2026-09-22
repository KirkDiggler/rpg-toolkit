// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// legacy_room_scene_key_test.go is the ONE place this package still spells
// the key it used to write (rpg-project#479, R2).
//
// Between rpg-toolkit#1753 and this change, every encounter record carried
// the World Builder's whole visual scene under `room_scene`, and every save
// rewrote it. The scene is content now: it is served to the player by
// dungeon key and the engine holds none of it. THERE IS NO MIGRATION — the
// ruling is that a loaded record simply ignores the key.
//
// "Go's encoding/json ignores unknown keys" is the mechanism, and it is
// exactly the kind of claim that is true until some field, tag or custom
// unmarshaler quietly makes it false. So it is PROVEN here rather than
// asserted: one blob with the key, one without, loaded through the same
// seam, and the two encounters are the same encounter.

// legacyRoomScene is a presentation of the shape this package used to write:
// the coordinate frame, the workspace, and a scene with an item carrying its
// transform, its asset, its height scale and a point light. Every one of
// these words is now the World Builder's, and none of them has a Go type
// here any more.
const legacyRoomScene = `"room_scene":{"version":1,` +
	`"coordinateFrame":{"horizontalPlane":"world-xz","verticalAxis":"world-y-up",` +
	`"distanceUnit":"world-scene-unit","hexRadius":1,"footprintFrame":"owner-local-xz"},` +
	`"workspace":{"hexRadius":6,"horizontalLimit":12},` +
	`"scene":{"version":1,"id":"scene-1","name":"Workshop","items":[` +
	`{"kind":"prop","id":"table","label":"Table","assetRef":"dnd5e:props:torture-table",` +
	`"transform":{"x":-2.25,"y":0,"z":1.3,"rotationY":0.37},"parentId":"furniture",` +
	`"heightScale":1.5,"pointLight":{"enabled":true,"offset":{"x":0,"y":0.5,"z":0},` +
	`"color":"#ff9d52","intensity":1.1,"range":2.6}}],` +
	`"groups":[{"kind":"group","id":"furniture","label":"Furniture",` +
	`"transform":{"x":-2.175,"y":0.6,"z":1.275,"rotationY":0.37}}]}},`

// TestARecordSavedWithTheOldPresentationKeyStillLoads is the loaded-record
// half of R2: no migration, no refusal, and no trace of the key in what the
// record becomes.
func TestARecordSavedWithTheOldPresentationKeyStillLoads(t *testing.T) {
	setup := &encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsTransparent(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("room-1-region", 0, 0, 5, 5)},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "leave", Trigger: encounter.TriggerExternal{}}},
	}
	fresh, err := encounter.NewEncounter(setup)
	require.NoError(t, err)

	plain, err := json.Marshal(fresh.ToData())
	require.NoError(t, err)

	// The same record as a build that still carried the presentation would
	// have written it: the key sits inside the field object, where FieldData
	// used to declare it.
	require.Contains(t, string(plain), `"field":{`)
	legacy := []byte(strings.Replace(string(plain), `"field":{`, `"field":{`+legacyRoomScene, 1))
	require.NotEqual(t, string(plain), string(legacy), "the fixture actually carries the old key")

	load := func(blob []byte) *encounter.Encounter {
		var data encounter.EncounterData
		require.NoError(t, json.Unmarshal(blob, &data))
		enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Sight:     everyoneSeesTheWholeMap{},
			Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Data: data,
		})
		require.NoError(t, err, "a record carrying the old key loads")

		return enc
	}

	fromLegacy, fromPlain := load(legacy), load(plain)

	// SAME ENCOUNTER. Compared as the record each one saves, which is the
	// whole observable state: a difference anywhere — a cell, a member, the
	// clock, a leftover key — shows up here.
	legacySaved, err := json.Marshal(fromLegacy.ToData())
	require.NoError(t, err)
	plainSaved, err := json.Marshal(fromPlain.ToData())
	require.NoError(t, err)
	require.JSONEq(t, string(plainSaved), string(legacySaved),
		"a record saved with the presentation loads to the same encounter as one without")

	// And the key does not come back out. The engine never writes it again,
	// so the first save after an upgrade is what drops it from the store.
	require.NotContains(t, string(legacySaved), "room_scene")
	require.NotContains(t, string(legacySaved), "torture-table")

	// The atlas the host projects carries none of it either — this is the
	// seam the scene used to reach the player through.
	atlas, err := fromLegacy.Atlas()
	require.NoError(t, err)
	projected, err := json.Marshal(atlas)
	require.NoError(t, err)
	require.NotContains(t, string(projected), "room_scene")
	require.NotContains(t, string(projected), "torture-table")
}
