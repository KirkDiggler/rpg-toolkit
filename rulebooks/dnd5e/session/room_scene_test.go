// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type RoomScenePublicAtlasSuite struct{ suite.Suite }

func TestRoomScenePublicAtlasSuite(t *testing.T) { suite.Run(t, new(RoomScenePublicAtlasSuite)) }

func publicRoomWorld(t *testing.T) *encounter.EncounterData {
	t.Helper()
	scene := &encounter.RoomScenePresentation{
		Version:   1,
		Frame:     encounter.RoomSceneFrame{HorizontalPlane: "world-xz", VerticalAxis: "world-y-up", DistanceUnit: "world-scene-unit", HexRadius: 1, FootprintFrame: "owner-local-xz"},
		Workspace: encounter.RoomSceneWorkspace{HexRadius: 6, HorizontalLimit: 12},
		Scene: encounter.RoomVisualScene{Version: 1, ID: "room", Name: "Room", Groups: []encounter.RoomSceneGroup{{Kind: encounter.RoomSceneKindGroup, ID: "altar", Label: "Altar", Transform: encounter.RoomSceneTransform{X: -1.25, Y: 1.5}}}, Items: []encounter.RoomSceneItem{
			{Kind: encounter.RoomSceneKindProp, ID: "base", Label: "Base", AssetRef: "props:base", Transform: encounter.RoomSceneTransform{X: -2, Y: 0, Z: 1}},
			{Kind: encounter.RoomSceneKindProp, ID: "lantern", Label: "Lantern", AssetRef: "props:lantern", SupportID: "base", ParentID: "altar", Transform: encounter.RoomSceneTransform{X: 1.25, Y: 0.5, Z: -0.75, RotationY: 0.125}, HeightScale: floatPtr(1.75), PointLight: &encounter.RoomSceneLight{Enabled: true, Offset: encounter.RoomSceneOffset{X: -0.2, Y: 0.3, Z: 0.4}, Color: "#abcdef", Intensity: 2, Range: 4}},
		}},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field:   encounter.FieldInput{RoomScene: scene, Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{{ID: "room", Name: "Room", Cells: []spatial.Position{{X: 0, Y: 0}, {X: 1, Y: 0}}, Archetype: "hall", Lighting: &encounter.Lighting{Intensity: 0.8}}}},
		Members: []encounter.MemberInput{{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}}}, Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	data := enc.ToData()
	return &data
}

func (s *RoomScenePublicAtlasSuite) TestPreviewLiveAndReloadPreserveFullSceneAndMechanicalAtlas() {
	ctx := context.Background()
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters, Characters: testCharacters(), Events: session.DiscardEvents{}})
	s.Require().NoError(err)
	world := publicRoomWorld(s.T())
	preview, err := mgr.AtlasOf(ctx, &session.AtlasOfInput{World: world})
	s.Require().NoError(err)
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: world})
	s.Require().NoError(err)
	live, err := mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(preview.RoomSceneJSON, live.RoomSceneJSON)
	s.Equal(preview.Grid, live.Grid)
	s.Equal(preview.Layout, live.Layout)
	s.Equal(preview.Cells, live.Cells)
	s.Equal(preview.Props, live.Props)
	s.Equal(preview.Regions, live.Regions)
	s.Equal(preview.Boundaries, live.Boundaries)
	s.Equal(preview.Doorways, live.Doorways)
	s.Equal(preview.Segments, live.Segments)
	s.Equal(preview.Sealed, live.Sealed)
	s.Equal(preview.Exits, live.Exits)
	s.Equal(preview.Start, live.Start)
	var decoded encounter.RoomScenePresentation
	s.Require().NoError(json.Unmarshal([]byte(live.RoomSceneJSON), &decoded))
	s.Equal("base", decoded.Scene.Items[1].SupportID)
	s.Equal(-0.75, decoded.Scene.Items[1].Transform.Z)
	s.Equal(1.75, *decoded.Scene.Items[1].HeightScale)
	worldJSON, _ := json.Marshal(encounters.byID["world"])
	var worldReload encounter.EncounterData
	s.Require().NoError(json.Unmarshal(worldJSON, &worldReload))
	encounters.byID["world"] = &worldReload
	sessJSON, _ := json.Marshal(sessions.byID["sess"])
	var sessReload session.SessionData
	s.Require().NoError(json.Unmarshal(sessJSON, &sessReload))
	sessions.byID["sess"] = &sessReload
	fresh, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters, Characters: testCharacters(), Events: session.DiscardEvents{}})
	s.Require().NoError(err)
	reloaded, err := fresh.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.True(reflect.DeepEqual(live, reloaded))
	saved := reloaded.RoomSceneJSON
	reloaded.RoomSceneJSON = "overwritten"
	again, err := fresh.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(saved, again.RoomSceneJSON)
}

func floatPtr(v float64) *float64 { return &v }
