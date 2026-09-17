// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

// room_scene_internal_test.go pins the projection's own law at the one seam
// that hands a full scene to one member (issue #1753): a field that carries
// both a room scene presentation and concealed structure is refused by name
// at [Encounter.AtlasFor], even though construction refuses the combination
// first — the output boundary does not trust the input boundary it sits
// behind. A concealed legacy field (placed contributors, no scene) still
// projects exactly as it always did.

// sceneStubResolver is the CheckResolver these scenes install: nothing is
// ever found, which is all a construction about the refusal needs.
type sceneStubResolver struct{}

func (sceneStubResolver) ResolveCheck(in *ResolveCheckInput) (*ResolveCheckOutput, error) {
	return &ResolveCheckOutput{Beaten: false, Applied: in.Approaches[0]}, nil
}

// sceneStubWitness is the Witness these scenes install: nobody perceives.
type sceneStubWitness struct{}

func (sceneStubWitness) Perceivers(*PerceiversInput) ([]MemberID, error) { return nil, nil }

// stubPresentation is a minimal valid presentation for the internal seams.
func stubPresentation() *RoomScenePresentation {
	return &RoomScenePresentation{
		Version:   1,
		Frame:     RoomSceneFrame{HorizontalPlane: "world-xz", VerticalAxis: "world-y-up", DistanceUnit: "world-scene-unit", HexRadius: 1, FootprintFrame: "owner-local-xz"},
		Workspace: RoomSceneWorkspace{HexRadius: 6, HorizontalLimit: 12},
		Scene:     RoomVisualScene{Version: 1, ID: "scene", Name: "Scene"},
	}
}

func TestProjectionRefusesRoomSceneBesideConcealedStructure(t *testing.T) {
	in := FieldInput{
		Canvas: CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
		Regions: []RegionInput{{
			ID: "vault", Name: "vault", Archetype: "crypt", Lighting: &Lighting{Intensity: 1},
			Cells:     []spatial.Position{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}},
			Concealed: true,
		}},
	}
	e, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: sceneStubResolver{}, Witness: sceneStubWitness{},
		Field:   in,
		Members: []MemberInput{{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 0}}},
		Endings: []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)
	require.True(t, e.world.conceals(), "the fixture is a concealed field")

	// Inject the combination construction refuses — the state the output
	// boundary must never transmit, however it came to exist.
	e.field.roomScene = copyRoomScene(stubPresentation())

	_, err = e.AtlasFor("alice")
	require.Error(t, err, "a full scene beside hidden space is not a member projection")
	require.Contains(t, err.Error(), "room scene presentation")

	// The author's whole-truth atlas is not a member projection and still
	// answers; the presentation rides it as construction truth.
	full, err := e.Atlas()
	require.NoError(t, err)
	require.NotNil(t, full.RoomScene)

	// And with the presentation absent, the concealed legacy field projects
	// exactly as it always did: withheld placed geometry, no scene, no error.
	e.field.roomScene = nil
	atlas, err := e.AtlasFor("alice")
	require.NoError(t, err)
	require.Nil(t, atlas.RoomScene)
}

func TestCopyRoomSceneIsIsolatedAndFaithful(t *testing.T) {
	p := stubPresentation()
	p.Scene.Items = []RoomSceneItem{{
		Kind: RoomSceneKindProp, ID: "table", Label: "Table", AssetRef: "dnd5e:props:table",
		HeightScale: &[]float64{1.5}[0],
		PointLight:  &RoomSceneLight{Enabled: true, Color: "#ff9d52", Intensity: 1.1, Range: 2.6},
	}}
	p.Scene.Groups = []RoomSceneGroup{{Kind: RoomSceneKindGroup, ID: "g", Label: "G"}}

	out := copyRoomScene(p)
	require.NotSame(t, p, out)
	require.NotSame(t, &p.Scene.Items[0], &out.Scene.Items[0])
	require.NotSame(t, p.Scene.Items[0].HeightScale, out.Scene.Items[0].HeightScale)
	require.NotSame(t, p.Scene.Items[0].PointLight, out.Scene.Items[0].PointLight)
	require.NotSame(t, &p.Scene.Groups[0], &out.Scene.Groups[0])
	require.Equal(t, *p, *out, "doubles and values are carried exactly")

	// Mutating either side reaches nothing.
	p.Scene.Items[0].Transform.X = 99
	p.Scene.Items[0].HeightScale = nil
	p.Scene.Items[0].PointLight.Intensity = 19
	out.Scene.Groups[0].Label = "changed"
	require.Equal(t, 1.5, *out.Scene.Items[0].HeightScale)
	require.Equal(t, 1.1, out.Scene.Items[0].PointLight.Intensity)
	require.Equal(t, "G", p.Scene.Groups[0].Label)

	// Nil stays nil, and an authored empty list stays an empty list.
	require.Nil(t, copyRoomScene(nil))
	empty := stubPresentation()
	empty.Scene.Items = []RoomSceneItem{}
	empty.Scene.Groups = []RoomSceneGroup{}
	emptyOut := copyRoomScene(empty)
	require.NotNil(t, emptyOut.Scene.Items)
	require.NotNil(t, emptyOut.Scene.Groups)
	require.Empty(t, emptyOut.Scene.Items)
}
