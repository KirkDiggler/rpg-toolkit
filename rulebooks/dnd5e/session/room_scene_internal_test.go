package session

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/stretchr/testify/require"
)

func validRoomScene() *encounter.RoomScenePresentation {
	return &encounter.RoomScenePresentation{
		Version:   1,
		Frame:     encounter.RoomSceneFrame{HorizontalPlane: "world-xz", VerticalAxis: "world-y-up", DistanceUnit: "world-scene-unit", HexRadius: 1, FootprintFrame: "owner-local-xz"},
		Workspace: encounter.RoomSceneWorkspace{HexRadius: 6, HorizontalLimit: 12},
		Scene:     encounter.RoomVisualScene{Version: 1, ID: "room", Name: "Room", Items: []encounter.RoomSceneItem{}, Groups: []encounter.RoomSceneGroup{}},
	}
}

func TestProjectAtlasRoomSceneIsCanonicalAndComplete(t *testing.T) {
	in := validRoomScene()
	in.Scene.Groups = []encounter.RoomSceneGroup{{Kind: encounter.RoomSceneKindGroup, ID: "g", Label: "group", Transform: encounter.RoomSceneTransform{X: -1.25, Y: 2.5}}}
	in.Scene.Items = []encounter.RoomSceneItem{{Kind: encounter.RoomSceneKindProp, ID: "p", Label: "prop", AssetRef: "asset", ParentID: "g", Transform: encounter.RoomSceneTransform{X: 1.5, Y: 0.25, Z: 3.75}, HeightScale: ptr(1.5), PointLight: &encounter.RoomSceneLight{Enabled: true, Color: "#ffffff", Intensity: 2, Range: 4}}}
	out, err := projectAtlas(*atlasWithScene(in))
	require.NoError(t, err)
	require.NotEmpty(t, out.RoomSceneJSON)
	require.Contains(t, out.RoomSceneJSON, `"items":[`)
	require.Contains(t, out.RoomSceneJSON, `"groups":[`)
	var got encounter.RoomScenePresentation
	require.NoError(t, json.Unmarshal([]byte(out.RoomSceneJSON), &got))
	require.Equal(t, float64(1.5), got.Scene.Items[0].Transform.X)
	require.Equal(t, "g", got.Scene.Items[0].ParentID)
	require.Equal(t, -1.25, got.Scene.Groups[0].Transform.X)
	in.Scene.Items[0].Transform.X = 99
	require.Equal(t, 1.5, got.Scene.Items[0].Transform.X)
}

func TestProjectAtlasValidEmptyRoomSceneRetainsArrays(t *testing.T) {
	out, err := projectAtlas(*atlasWithScene(validRoomScene()))
	require.NoError(t, err)
	require.Contains(t, out.RoomSceneJSON, `"items":[]`)
	require.Contains(t, out.RoomSceneJSON, `"groups":[]`)
}

func TestProjectAtlasNilRoomSceneIsAbsent(t *testing.T) {
	out, err := projectAtlas(encounter.Atlas{Orientation: encounter.HexesArePointyTop()})
	require.NoError(t, err)
	require.Empty(t, out.RoomSceneJSON)
}

func TestProjectAtlasRoomSceneRejectsInvalidWithoutPartialAtlas(t *testing.T) {
	bad := validRoomScene()
	bad.Version = 99
	bad.Scene.Items = []encounter.RoomSceneItem{{Kind: encounter.RoomSceneKindProp, ID: "p", AssetRef: "a", Transform: encounter.RoomSceneTransform{X: math.NaN()}}}
	out, err := projectAtlas(*atlasWithScene(bad))
	require.ErrorIs(t, err, ErrInvalidWorld)
	require.Empty(t, out.RoomSceneJSON)
	require.Empty(t, out.Cells)
}

func atlasWithScene(scene *encounter.RoomScenePresentation) *encounter.Atlas {
	a := &encounter.Atlas{Orientation: encounter.HexesArePointyTop(), RoomScene: scene}
	return a
}

func ptr(v float64) *float64 { return &v }
