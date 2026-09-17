// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

// room_scene_persistence_test.go is the durable boundary for the complete
// typed room scene presentation (issue #1753): one real v3 source, through
// public Load, an actual NewEncounter, ToData/JSON, LoadEncounter, and back
// out through Atlas and AtlasFor — with the real CellAt/Step/Route/Canvas
// facts asked on both sides, so a reload answers what the saved run
// answered.
//
// The suite proves five things the parent's virtual overlay only sketches:
// the presentation survives every carrier with its doubles, empty lists and
// pointer leaves intact; no caller-owned pointer or returned snapshot can
// alias another; malformed and unsupported presentations fail closed at
// BOTH construction seams; a loaded member standing inside a footprint is
// refused before a live encounter is returned; and every legacy shape —
// fields without a scene, concealed fields, the atlas member projection's
// withheld placed geometry — answers exactly as it did before.
type RoomScenePersistenceSuite struct {
	suite.Suite
}

func TestRoomScenePersistenceSuite(t *testing.T) {
	suite.Run(t, new(RoomScenePersistenceSuite))
}

// --- fixtures ---

// v3CompiledSlid is the reference room with its declared table slid onto the
// centre of walkable cell (0,1) — plane point (√3/2, 1.5) in source units —
// before compile: the placed geometry follows the scene transform, the start
// and the monster stay clear, and the floor carries one covered centre.
func (s *RoomScenePersistenceSuite) v3CompiledSlid() dungeonspec.Compiled {
	return s.v3CompiledEdited(func(spec *dungeonspec.SingleRoomSpec) {
		spec.Room.Scene.Items[0].Transform.X = math.Sqrt(3) / 2
		spec.Room.Scene.Items[0].Transform.Z = 1.5
	})
}

// v3CompiledEdited decodes the committed v3 source, edits it at the source
// level, and compiles it the way a host does.
func (s *RoomScenePersistenceSuite) v3CompiledEdited(edit func(*dungeonspec.SingleRoomSpec)) dungeonspec.Compiled {
	raw, err := os.ReadFile("dungeonspec/testdata/world-builder-v3.yaml")
	s.Require().NoError(err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	spec := decoded.Spec
	if edit != nil {
		edit(spec)
	}
	compiled, err := dungeonspec.Load(yamlMarshal(s, spec))
	s.Require().NoError(err)

	return compiled
}

// basePresentation is a valid hand-built presentation: the approved frame,
// a workspace preset, one prop with both pointer leaves and one group.
func (s *RoomScenePersistenceSuite) basePresentation() *encounter.RoomScenePresentation {
	return &encounter.RoomScenePresentation{
		Version: 1,
		Frame: encounter.RoomSceneFrame{
			HorizontalPlane: "world-xz", VerticalAxis: "world-y-up",
			DistanceUnit: "world-scene-unit", HexRadius: 1, FootprintFrame: "owner-local-xz",
		},
		Workspace: encounter.RoomSceneWorkspace{HexRadius: 6, HorizontalLimit: 12},
		Scene: encounter.RoomVisualScene{
			Version: 1, ID: "scene-1", Name: "Hall",
			Items: []encounter.RoomSceneItem{{
				Kind: encounter.RoomSceneKindProp, ID: "table", Label: "Table",
				AssetRef:    "dnd5e:props:table",
				Transform:   encounter.RoomSceneTransform{X: -2.25, Y: 0, Z: 1.3, RotationY: 0.37},
				HeightScale: floatPtr(1.5), ParentID: "furniture",
				PointLight: &encounter.RoomSceneLight{
					Enabled: true, Offset: encounter.RoomSceneOffset{X: 0, Y: 0.5, Z: 0},
					Color: "#ff9d52", Intensity: 1.1, Range: 2.6,
				},
			}},
			Groups: []encounter.RoomSceneGroup{{
				Kind: encounter.RoomSceneKindGroup, ID: "furniture", Label: "Furniture",
				Transform: encounter.RoomSceneTransform{X: -2.175, Y: 0.6, Z: 1.275, RotationY: 0.37},
			}},
		},
	}
}

// presentationField paints the 5x5 fixture hall with the given presentation
// and an optional seed for the rest of the field.
func (s *RoomScenePersistenceSuite) presentationField(
	p *encounter.RoomScenePresentation, seed func(*encounter.FieldInput),
) encounter.FieldInput {
	field := placedField()
	field.RoomScene = p
	if seed != nil {
		seed(&field)
	}

	return field
}

// boot constructs through the real Setup seam.
func (s *RoomScenePersistenceSuite) boot(
	field encounter.FieldInput, members ...encounter.MemberInput,
) *encounter.Encounter {
	game, err := s.tryBoot(field, members...)
	s.Require().NoError(err)

	return game
}

func (s *RoomScenePersistenceSuite) tryBoot(
	field encounter.FieldInput, members ...encounter.MemberInput,
) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

// bootConcealed constructs a concealed field through the real Setup seam,
// with the two concealment capabilities it requires.
func (s *RoomScenePersistenceSuite) bootConcealed(
	field encounter.FieldInput, members ...encounter.MemberInput,
) *encounter.Encounter {
	game, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return game
}

// reloadFrom saves the encounter as bytes and loads the bytes back — the
// public JSON round trip a host's storage performs, not an in-memory pass.
func (s *RoomScenePersistenceSuite) reloadFrom(data encounter.EncounterData) *encounter.Encounter {
	wire, err := json.Marshal(data)
	s.Require().NoError(err)
	var back encounter.EncounterData
	s.Require().NoError(json.Unmarshal(wire, &back))
	restored, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      back,
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	return restored
}

// tryReloadFrom is reloadFrom without the NoError: the trust boundary's
// refusals are themselves the subject.
func (s *RoomScenePersistenceSuite) tryReloadFrom(data encounter.EncounterData) (*encounter.Encounter, error) {
	wire, err := json.Marshal(data)
	s.Require().NoError(err)
	var back encounter.EncounterData
	s.Require().NoError(json.Unmarshal(wire, &back))

	return encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      back,
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
}

// presentationJSON renders any presentation carrier as the JSON it would be
// compared by: a pointer marshals as its value, so two carriers agree when
// and only when their content does.
func (s *RoomScenePersistenceSuite) presentationJSON(v any) string {
	b, err := json.Marshal(v)
	s.Require().NoError(err)

	return string(b)
}

// floatPtr is the presentation's optional heightScale.
func floatPtr(v float64) *float64 { return &v }

// --- the complete boundary ---

func (s *RoomScenePersistenceSuite) TestCompleteV3SceneSurvivesRuntimePersistenceAndReload() {
	compiled := s.v3CompiledSlid()
	// The declared table stands on the centre of walkable cell (0,1), so the
	// floor carries a real covered centre for the step and fold facts to
	// refuse; the start and the monster cell stay clear.
	expected := s.presentationJSON(compiled.Field.RoomScene)
	regionCells := compiled.Field.Regions[0].Cells

	game := s.boot(compiled.Field,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At})

	// THE AUTHOR'S INPUT IS THEIRS: editing the scene they handed in must
	// not reach a running field (construction snapshot).
	compiled.Field.RoomScene.Scene.Items[0].Transform.X = 99
	compiled.Field.RoomScene.Scene.Items[1].PointLight.Color = "#123456"

	atlas, err := game.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(atlas.RoomScene), "constructor must snapshot the presentation")
	s.Require().Len(atlas.Placed, 1, "the declared table is a placed contributor")
	s.Equal("table", atlas.Placed[0].ID)
	s.Empty(atlas.Props, "a placed footprint is never a legacy cell prop")
	s.Len(atlas.Cells, len(regionCells), "the presentation invents no floor")

	// COPY-OUT: mutating one atlas's scene reaches nothing.
	view := atlas.RoomScene
	view.Scene.Items[0].Transform.X = 99
	view.Scene.Items[1].PointLight.Range = 99
	again, err := game.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(again.RoomScene), "atlas must copy out")

	// The member-scoped answer carries the same scene on the supported
	// combination (one unconcealed region): the scene is construction
	// truth, the same for every member.
	memberAtlas, err := game.AtlasFor(alice)
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(memberAtlas.RoomScene))

	// PERSISTENCE: ToData carries the same presentation.
	data := game.ToData()
	s.Equal(expected, s.presentationJSON(data.Field.RoomScene))

	// THE REAL JSON ROUND TRIP, through LoadEncounter.
	restored := s.reloadFrom(data)

	// THE BLOB IS THE CALLER'S once saved: mutating it after the load must
	// not reach the reloaded encounter, and a second ToData must not alias
	// the first.
	data.Field.RoomScene.Scene.Items[1].PointLight.Intensity = 19
	s.Equal(expected, s.presentationJSON(game.ToData().Field.RoomScene),
		"two ToData calls must not alias one scene")

	restoredAtlas, err := restored.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(restoredAtlas.RoomScene), "load must snapshot")
	restoredForMember, err := restored.AtlasFor(alice)
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(restoredForMember.RoomScene))

	// NO ACTOR PREVIEW MARKERS: the compiler injected no seats, no roster,
	// nothing the author did not draw — the scene is exactly the source's.
	s.Require().Len(restoredAtlas.RoomScene.Scene.Items, 2)
	s.Require().Len(restoredAtlas.RoomScene.Scene.Groups, 1)

	// THE FACTS: every cell answers identically live and reloaded.
	s.assertFactsAgree(game, restored, regionCells, alice)
}

// assertFactsAgree compares the real movement and sight answers two
// encounters give about the same floor: the CellAt fold cell by cell, the
// handed-out canvas's placement and sight answers for every cell and every
// ordered pair, one Step refusal and one Route. The geometry is never
// re-derived here — the two encounters agreeing IS the fact under test.
func (s *RoomScenePersistenceSuite) assertFactsAgree(
	live, reloaded *encounter.Encounter, authored []spatial.Position, mover encounter.MemberID,
) {
	for _, authoredCell := range authored {
		cell := cellAt(int(authoredCell.X), int(authoredCell.Y))
		before := live.CellAt(encounter.CellAtInput{Cell: cell, Mover: mover})
		after := reloaded.CellAt(encounter.CellAtInput{Cell: cell, Mover: mover})
		s.Equal(before.Passage, after.Passage, "cell [%g,%g] passage", cell.X, cell.Y)
		s.Equal(before.Cost, after.Cost, "cell [%g,%g] cost", cell.X, cell.Y)
		s.Equal(contribWords(before), contribWords(after), "cell [%g,%g] contributors", cell.X, cell.Y)
	}

	// One Step onto a covered centre is refused by the placement's name, on
	// both sides of the save; one ordinary step succeeds on both.
	covered, coveredFound := s.firstBlocked(live, authored)
	s.Require().True(coveredFound, "fixture must have a covered cell")
	_, err := live.Step(&encounter.StepInput{Member: mover, To: covered})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table")
	_, err = reloaded.Step(&encounter.StepInput{Member: mover, To: covered})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table", "the reloaded refusal names the placement")

	// A route across the floor reads the same flood before and after: the
	// mover walks from where they stand (Route reads the canvas) toward the
	// far corner of the floor.
	target := cellAt(int(authored[len(authored)-1].X), int(authored[len(authored)-1].Y))
	outBefore, err := live.Route(encounter.RouteInput{Mover: mover, Policy: encounter.MoveToward, Anchor: target, Budget: 6})
	s.Require().NoError(err)
	outAfter, err := reloaded.Route(encounter.RouteInput{Mover: mover, Policy: encounter.MoveToward, Anchor: target, Budget: 6})
	s.Require().NoError(err)
	s.Equal(outBefore.Path, outAfter.Path, "route facts")

	// The handed-out canvas answers identically for every cell and every
	// ordered cell pair.
	liveCanvas, err := live.Canvas()
	s.Require().NoError(err)
	reloadedCanvas, err := reloaded.Canvas()
	s.Require().NoError(err)
	s.Equal(canvasEntityIDs(liveCanvas), canvasEntityIDs(reloadedCanvas))
	liveEntity := canvasEntity(liveCanvas, mover)
	reloadedEntity := canvasEntity(reloadedCanvas, mover)
	for _, authoredFrom := range authored {
		from := cellAt(int(authoredFrom.X), int(authoredFrom.Y))
		s.Equal(liveCanvas.CanPlaceEntity(liveEntity, from), reloadedCanvas.CanPlaceEntity(reloadedEntity, from),
			"placement at [%g,%g]", from.X, from.Y)
		for _, authoredTo := range authored {
			to := cellAt(int(authoredTo.X), int(authoredTo.Y))
			s.Equal(liveCanvas.IsLineOfSightBlocked(from, to), reloadedCanvas.IsLineOfSightBlocked(from, to),
				"sight [%g,%g]->[%g,%g]", from.X, from.Y, to.X, to.Y)
		}
	}
}

// contribWords is a cell fact's contributors as one comparable word each:
// kind, identity and whether it closed the cell, sorted so an unordered
// fold compares deterministically.
func contribWords(fact encounter.CellFact) []string {
	out := make([]string, 0, len(fact.Contribs))
	for _, c := range fact.Contribs {
		out = append(out, fmt.Sprintf("%s:%s:%s:%t", c.Kind, c.ID, c.Ref, c.Blocks))
	}
	sort.Strings(out)

	return out
}

// canvasEntityIDs is the canvas's roster, sorted for one comparable answer.
func canvasEntityIDs(canvas spatial.Room) []string {
	entities := canvas.GetAllEntities()
	out := make([]string, 0, len(entities))
	for id := range entities {
		out = append(out, id)
	}
	sort.Strings(out)

	return out
}

// canvasEntity is the named member's entity as the canvas holds it.
func canvasEntity(canvas spatial.Room, member encounter.MemberID) core.Entity {
	entity, ok := canvas.GetAllEntities()[string(member)]
	if !ok {
		panic("member " + member + " is not on the canvas")
	}

	return entity
}

func (s *RoomScenePersistenceSuite) firstBlocked(enc *encounter.Encounter, authored []spatial.Position) (spatial.Position, bool) {
	for _, authoredCell := range authored {
		cell := cellAt(int(authoredCell.X), int(authoredCell.Y))
		if fact := enc.CellAt(encounter.CellAtInput{Cell: cell, Mover: ""}); fact.Passage == encounter.PassageBlocked {
			return cell, true
		}
	}

	return spatial.Position{}, false
}

// --- nested and pointer isolation, end to end ---

func (s *RoomScenePersistenceSuite) TestPresentationSnapshotsAreIsolatedEndToEnd() {
	p := s.basePresentation()
	expected := s.presentationJSON(p)
	field := s.presentationField(p, nil)
	game := s.boot(field,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})

	// THE CALLER'S OWN POINTERS: the scene they handed in, its pointer
	// leaves, its group list — all of it is theirs to mutate, and none of
	// it reaches a running field.
	p.Scene.Items[0].Transform.X = 99
	*p.Scene.Items[0].HeightScale = 3.7
	p.Scene.Items[0].PointLight.Intensity = 19
	p.Scene.Groups[0].Transform.RotationY = 9

	atlas, err := game.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(atlas.RoomScene), "construction snapshot")
	data := game.ToData()
	s.Equal(expected, s.presentationJSON(data.Field.RoomScene))

	// MUTATING A RETURNED SNAPSHOT reaches nothing: not another atlas, not
	// the persisted blob, not the running field.
	atlas.RoomScene.Scene.Items[0].PointLight.Range = 99
	s.Equal(expected, s.presentationJSON(game.ToData().Field.RoomScene))

	// MUTATING THE BLOB reaches nothing: not the live encounter, not the
	// second save.
	data.Field.RoomScene.Scene.Items[0].HeightScale = floatPtr(4.0)
	s.Equal(expected, s.presentationJSON(game.ToData().Field.RoomScene))
	s.Equal(expected, s.presentationJSON(game.ToData().Field.RoomScene))

	// THE RELOADED ENCOUNTER is isolated the same way, on every carrier.
	restored := s.reloadFrom(game.ToData())
	restoredData := restored.ToData()
	s.Equal(expected, s.presentationJSON(restoredData.Field.RoomScene))
	restoredData.Field.RoomScene.Scene.Items[0].Transform.Z = 42
	s.Equal(expected, s.presentationJSON(restored.ToData().Field.RoomScene))

	restoredAtlas, err := restored.Atlas()
	s.Require().NoError(err)
	restoredAtlas.RoomScene.Scene.Groups[0].Label = "changed"
	again, err := restored.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(again.RoomScene), "reload must copy out")

	memberAtlas, err := restored.AtlasFor(alice)
	s.Require().NoError(err)
	memberAtlas.RoomScene.Scene.Items[0].Transform.Y = 42
	memberAgain, err := restored.AtlasFor(alice)
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(memberAgain.RoomScene), "AtlasFor must copy out")
}

// --- fail closed, at both seams ---

func (s *RoomScenePersistenceSuite) TestInvalidPresentationsAreRefusedAtBothSeams() {
	s.Run("version at construction", func() {
		_, err := s.tryBoot(s.presentationField(s.basePresentation(), func(f *encounter.FieldInput) {
			f.RoomScene.Version = 2
		}))
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "version 2")
	})

	s.Run("a scene defect is named at its path at construction", func() {
		_, err := s.tryBoot(s.presentationField(s.basePresentation(), func(f *encounter.FieldInput) {
			f.RoomScene.Scene.Items[0].ID = ""
		}))
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "scene.items[0].id")
	})

	s.Run("multi-region at construction", func() {
		field := s.presentationField(s.basePresentation(), func(f *encounter.FieldInput) {
			f.Regions = append(f.Regions, rectRegion("annex", 10, 0, 2, 2))
		})
		_, err := s.tryBoot(field)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "carries 2 regions")
	})

	s.Run("concealed region at construction", func() {
		field := s.presentationField(s.basePresentation(), func(f *encounter.FieldInput) {
			f.Regions[0].Concealed = true
		})
		_, err := s.tryBoot(field)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "hall")
	})

	s.Run("concealed doors at construction", func() {
		field := s.presentationField(s.basePresentation(), func(f *encounter.FieldInput) {
			f.Doors = []encounter.DoorInput{{
				ID:        "veil",
				State:     encounter.DoorIsClosed(),
				Edges:     []encounter.DoorEdge{{From: cellAt(0, 1), To: cellAt(1, 1)}},
				Concealed: []encounter.CheckApproach{{Ability: "perception", DC: 12}},
			}}
		})
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Mover: quietMover{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
			Field:   field,
			Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)}},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "room scene presentation")
	})

	s.Run("version at load", func() {
		game := s.boot(s.presentationField(s.basePresentation(), nil),
			encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})
		data := game.ToData()
		data.Field.RoomScene.Version = 2
		restored, err := s.tryReloadFrom(data)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
		s.Nil(restored)
	})

	s.Run("a malformed scene at load is named at its path", func() {
		game := s.boot(s.presentationField(s.basePresentation(), nil),
			encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})
		data := game.ToData()
		data.Field.RoomScene.Scene.Items[0].AssetRef = ""
		restored, err := s.tryReloadFrom(data)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "scene.items[0].assetRef")
		s.Nil(restored)
	})

	s.Run("concealed structure beside a scene at load", func() {
		game := s.bootConcealed(concealField(),
			encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(2, 2)})
		data := game.ToData()
		data.Field.RoomScene = s.basePresentation()
		restored, err := s.tryReloadFrom(data)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "room scene presentation")
		s.Nil(restored)
	})
}

// --- the load trust boundary for member geometry ---

func (s *RoomScenePersistenceSuite) TestLoadRefusesAMemberSavedOntoACoveredCentre() {
	blocked := cellAt(1, 1)
	game := s.boot(placedField(placed("table", coveredBox(5, centreOf(blocked)), true, false)),
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})
	data := game.ToData()
	s.Require().NotNil(data.Members[0].Cell)
	data.Members[0].Cell.X = blocked.X
	data.Members[0].Cell.Y = blocked.Y

	restored, err := s.tryReloadFrom(data)
	s.Require().Error(err, "load is a trust boundary even if normal saves cannot produce it")
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table", "the refusal names the placement that covers the cell")
	s.Nil(restored)
}

// An outcome cell is a finished position, not a live one — but the load
// law for it is the same question asked the same way as for a live member,
// so a closed encounter whose member finished inside a footprint refuses
// too.
func (s *RoomScenePersistenceSuite) TestLoadRefusesAnOutcomeCellUnderAFootprint() {
	blocked := cellAt(1, 1)
	game := s.boot(placedField(placed("table", coveredBox(5, centreOf(blocked)), true, false)),
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})
	data := game.ToData()
	data.Outcome = &encounter.OutcomeData{
		Ending: "withdrawn",
		Members: []encounter.MemberOutcomeData{{
			ID: alice, Cell: &encounter.PositionData{X: blocked.X, Y: blocked.Y},
		}},
	}

	restored, err := s.tryReloadFrom(data)
	s.Require().Error(err, "a finished position inside a footprint is invalid saved state")
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table")
	s.Nil(restored)
}

func (s *RoomScenePersistenceSuite) TestLoadRefusesAReserveSeatUnderAFootprint() {
	blocked := cellAt(1, 1)
	game := s.boot(placedField(placed("table", coveredBox(5, centreOf(blocked)), true, false)),
		encounter.MemberInput{
			ID: goblin, Kind: encounter.KindMonster, Position: cellAt(3, 0),
			Arrives: encounter.TriggerExternal{},
		})
	data := game.ToData()
	s.Require().Len(data.Reserve, 1)

	// The untampered blob loads: the reserve seat is an ordinary standable
	// cell, and reserve loading itself is not what is being refused.
	s.reloadFrom(data)

	data.Reserve[0].Cell.X = blocked.X
	data.Reserve[0].Cell.Y = blocked.Y
	restored, err := s.tryReloadFrom(data)
	s.Require().Error(err, "a seat the arrival can never take is invalid saved state")
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "table")
	s.Nil(restored)
}

// --- empty scenes ---

func (s *RoomScenePersistenceSuite) TestAnEmptySceneSurvivesAsEmpty() {
	p := s.basePresentation()
	p.Scene.Items = []encounter.RoomSceneItem{}
	p.Scene.Groups = []encounter.RoomSceneGroup{}
	expected := s.presentationJSON(p)
	field := s.presentationField(p, nil)
	game := s.boot(field,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})

	data := game.ToData()
	s.Require().NotNil(data.Field.RoomScene)
	s.NotNil(data.Field.RoomScene.Scene.Items, "an authored empty list is an empty list, not absence")
	s.NotNil(data.Field.RoomScene.Scene.Groups)

	// The wire says [] — an empty list — on both the save and the reload.
	var raw map[string]any
	wire, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(wire, &raw))
	persisted := raw["field"].(map[string]any)["room_scene"].(map[string]any)
	s.NotNil(persisted["scene"].(map[string]any)["items"], "the blob carries [] where the scene was empty")

	restored := s.reloadFrom(data)
	restoredAtlas, err := restored.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(restoredAtlas.RoomScene), "empty arrays retained through reload")
}

// --- legacy shapes are unchanged ---

func (s *RoomScenePersistenceSuite) TestLegacyFieldsCarryNoRoomScene() {
	game := s.boot(placedField(),
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)})
	data := game.ToData()
	s.Nil(data.Field.RoomScene, "a field without a presentation persists as absence")

	// The wire omits the key entirely, and the absence survives the round
	// trip as nil — never an empty scene invented at load.
	var raw map[string]any
	wire, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(wire, &raw))
	_, present := raw["field"].(map[string]any)["room_scene"]
	s.False(present, "a legacy blob names no room_scene key")

	restored := s.reloadFrom(data)
	restoredAtlas, err := restored.Atlas()
	s.Require().NoError(err)
	s.Nil(restoredAtlas.RoomScene)
	restoredForMember, err := restored.AtlasFor(alice)
	s.Require().NoError(err)
	s.Nil(restoredForMember.RoomScene)
}

// TestConcealedProjectionWithholdsNewGeometry pins the member projection's
// law for a legacy concealed field that carries placed contributors but no
// room scene: the whole-truth atlas answers, the member atlas withholds the
// new continuous geometry entirely — never an unfiltered slice of it — and
// no scene appears anywhere.
func (s *RoomScenePersistenceSuite) TestConcealedProjectionWithholdsNewGeometry() {
	field := concealField()
	field.Placed = append(field.Placed,
		placed("table", coveredBox(5, centreOf(cellAt(2, 2))), true, false))
	game := s.bootConcealed(field,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(2, 2)})

	full, err := game.Atlas()
	s.Require().NoError(err)
	s.Require().Len(full.Placed, 1, "the author's whole truth carries the placement")
	s.Nil(full.RoomScene)

	memberAtlas, err := game.AtlasFor(alice)
	s.Require().NoError(err)
	s.Empty(memberAtlas.Placed, "placed geometry is withheld from a member projection, never unfiltered")
	s.Nil(memberAtlas.RoomScene)
}

// --- the compiler cases, end to end ---

func (s *RoomScenePersistenceSuite) TestNegativeAndOddRowAxialCellsSurviveTheFullBoundary() {
	raw, err := os.ReadFile("dungeonspec/testdata/world-builder-v3.yaml")
	s.Require().NoError(err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	spec := decoded.Spec
	// Two rows and columns into negative territory, across the odd/even row
	// parity seam the offset frame staggers on.
	shift := dungeonspec.RoomCell{Q: -2, R: -1}
	for i := range spec.Room.Gameplay.WalkableHexes {
		spec.Room.Gameplay.WalkableHexes[i].Q += shift.Q
		spec.Room.Gameplay.WalkableHexes[i].R += shift.R
	}
	spec.Room.Gameplay.PartyStart.Q += shift.Q
	spec.Room.Gameplay.PartyStart.R += shift.R
	for i := range spec.Room.Gameplay.Monsters {
		spec.Room.Gameplay.Monsters[i].Cell.Q += shift.Q
		spec.Room.Gameplay.Monsters[i].Cell.R += shift.R
	}
	compiled, err := dungeonspec.Load(yamlMarshal(s, spec))
	s.Require().NoError(err)
	expected := s.presentationJSON(compiled.Field.RoomScene)

	game := s.boot(compiled.Field,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At})
	restored := s.reloadFrom(game.ToData())

	// The member stands where the SOURCE authored: the atlas reports
	// absolute axial cells, and those are exactly the axial cells the source
	// named — on both sides of the save.
	for _, enc := range []*encounter.Encounter{game, restored} {
		atlas, err := enc.Atlas()
		s.Require().NoError(err)
		s.Require().Len(atlas.Regions, 1)
		got := map[[2]int]bool{}
		for _, c := range atlas.Regions[0].Cells {
			got[[2]int{int(c.X), int(c.Y)}] = true
		}
		want := map[[2]int]bool{}
		for _, c := range spec.Room.Gameplay.WalkableHexes {
			want[[2]int{c.Q, c.R}] = true
		}
		s.Equal(want, got, "floor frame shifted")
		abs := encounter.HexCellAt(compiled.Field.Canvas.Orientation,
			int(compiled.PartyStart[0].At.X), int(compiled.PartyStart[0].At.Y))
		s.Equal([2]int{spec.Room.Gameplay.PartyStart.Q, spec.Room.Gameplay.PartyStart.R},
			[2]int{int(abs.X), int(abs.Y)}, "start frame shifted")
	}

	restoredAtlas, err := restored.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, s.presentationJSON(restoredAtlas.RoomScene), "the shifted scene survived too")
}

// yamlMarshal marshals a decoded spec back to the YAML public Load accepts.
func yamlMarshal(s *RoomScenePersistenceSuite, spec *dungeonspec.SingleRoomSpec) []byte {
	s.T().Helper()
	b, err := yaml.Marshal(spec)
	s.Require().NoError(err)

	return b
}
