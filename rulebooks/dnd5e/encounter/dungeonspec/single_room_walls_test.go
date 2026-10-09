// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// single_room_walls_test.go is A STRUCTURAL WALL IN THE SINGLE-ROOM DIALECT
// (rpg-project#169; single-room-play.md §3), through the real paths:
//
//   - the web's own schema DECODES (testdata/world-builder-v4-walls.yaml),
//     with the appearance carried and never read;
//   - the authored BLOCKER, not the visible line, supplies the extent, and
//     the openings are subtracted AFTER the blocker's own offset (review
//     correction #3: shifted blockers must not refill a doorway);
//   - the source XZ frame maps to the canonical plane with the sign pinned
//     against endpoints computed by hand, not round-tripped through the
//     lowering;
//   - the compiled contributors stand where they should, proved through the
//     REAL encounter: a crossing through a gap succeeds, a crossing through
//     a solid span is refused, sight obeys the two independent flags, and a
//     save/load rebuilds the same facts.
//
// Nothing here constructs a second collider or reads a rendered mesh. The
// walls reach the engine as the same [encounter.PlacedPropInput] a table and a
// footprint door already are, which is why every gameplay assertion below is
// an EXISTING encounter query.

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SingleRoomWallSuite struct {
	suite.Suite
}

func TestSingleRoomWallSuite(t *testing.T) {
	suite.Run(t, new(SingleRoomWallSuite))
}

const roomWallFixture = "testdata/world-builder-v4-walls.yaml"

// scenePerFoot is k computed here rather than borrowed: one scene unit is
// FeetPerCell/sqrt(3) feet, and a test that read the package's own constant
// could not catch a change to it.
func scenePerFoot() float64 { return encounter.FeetPerCell / math.Sqrt(3) }

func (s *SingleRoomWallSuite) baseSpec() *dungeonspec.SingleRoomSpec {
	raw, err := os.ReadFile(roomWallFixture)
	s.Require().NoError(err)
	out, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Require().NotNil(out.Spec)

	return out.Spec
}

func (s *SingleRoomWallSuite) encoded(spec *dungeonspec.SingleRoomSpec) []byte {
	raw, err := yaml.Marshal(spec)
	s.Require().NoError(err)

	return raw
}

// compiledWith replaces the fixture's wall list and compiles the result through
// the public Load, so every geometry assertion below reads the real lowering.
func (s *SingleRoomWallSuite) compiledWith(walls []dungeonspec.RoomWall) dungeonspec.Compiled {
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls = walls
	compiled, err := dungeonspec.Load(s.encoded(spec))
	s.Require().NoError(err)

	return compiled
}

// compiledWithShiftedFloor moves the fixture's floor out of a wall's way and
// compiles it, so a geometry witness can use exact endpoints without the wall
// landing on a cell the party stands on.
func (s *SingleRoomWallSuite) compiledWithShiftedFloor(walls []dungeonspec.RoomWall, dq, dr int) dungeonspec.Compiled {
	spec := s.baseSpec()
	for i := range spec.Room.Gameplay.WalkableHexes {
		spec.Room.Gameplay.WalkableHexes[i].Q += dq
		spec.Room.Gameplay.WalkableHexes[i].R += dr
	}
	spec.Room.Gameplay.PartyStart.Q += dq
	spec.Room.Gameplay.PartyStart.R += dr
	// Widen the floor bound so the moved cells are still inside it. The
	// workspace node is presentation content the decoder carries opaquely, so
	// this is a test-only edit of the authored radius.
	var workspace yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte("{hexRadius: 30, horizontalLimit: 12}"), &workspace))
	s.Require().NotEmpty(workspace.Content)
	spec.Room.Workspace = *workspace.Content[0]
	spec.Room.Gameplay.Walls = walls
	compiled, err := dungeonspec.Load(s.encoded(spec))
	s.Require().NoError(err)

	return compiled
}

// wallBlocks builds a wall's independent blocker declaration.
func wallBlocks(movement, sight bool, width, depth, offsetX, offsetZ float64) dungeonspec.RoomPropDeclaration {
	return dungeonspec.RoomPropDeclaration{
		BlocksMovement:    &movement,
		BlocksLineOfSight: &sight,
		Footprint: dungeonspec.RoomFootprint{
			Width: width, Depth: depth, OffsetX: offsetX, OffsetZ: offsetZ,
		},
	}
}

// roomWall builds one authored wall from a line in scene XZ units.
func roomWall(id string, sx, sz, ex, ez float64, blocker dungeonspec.RoomPropDeclaration, openings ...dungeonspec.RoomWallOpening) dungeonspec.RoomWall {
	return dungeonspec.RoomWall{
		ID:    id,
		Label: id,
		Line: dungeonspec.RoomWallLine{
			Start: dungeonspec.RoomWallPoint{X: sx, Z: sz},
			End:   dungeonspec.RoomWallPoint{X: ex, Z: ez},
		},
		Appearance: dungeonspec.RoomWallAppearance{
			AssetRef: "dnd5e:env:test:wall", Height: 3, Thickness: 0.3, Elevation: 0,
		},
		Blocker:  blocker,
		Openings: openings,
	}
}

// generatedSpans is the compiled placed contributors whose ids are generated
// wall spans, in the order the field carries them.
func generatedSpans(field encounter.FieldInput) []encounter.PlacedPropInput {
	var out []encounter.PlacedPropInput
	for _, p := range field.Placed {
		if strings.HasPrefix(string(p.ID), "wall/") {
			out = append(out, p)
		}
	}

	return out
}

// wallEncounter builds the real encounter the compiled field runs, with the
// capabilities every one of these geometry scenes supplies.
func (s *SingleRoomWallSuite) wallEncounter(compiled dungeonspec.Compiled) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: axial(0, 0)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sheets:     zeroSheets{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc
}

func (s *SingleRoomWallSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(),
		Capabilities: encounter.Capabilities{
			Sheets:     zeroSheets{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return loaded
}

func (s *SingleRoomWallSuite) sightAcross(enc *encounter.Encounter) bool {
	canvas, err := enc.Canvas()
	s.Require().NoError(err)

	return canvas.IsLineOfSightBlocked(axial(0, 0), axial(4, 0))
}

// --- The schema decodes, and the appearance is carried unread ---

func (s *SingleRoomWallSuite) TestTheWebSchemaDecodesAndRoundTrips() {
	spec := s.baseSpec()
	s.Require().Len(spec.Room.Gameplay.Walls, 1)
	wall := spec.Room.Gameplay.Walls[0]
	s.Equal("long-wall", wall.ID)
	s.Equal("Long wall", wall.Label)
	s.Equal(dungeonspec.RoomWallPoint{X: 0, Z: 3}, wall.Line.Start)
	s.Equal(dungeonspec.RoomWallPoint{X: 20, Z: 3}, wall.Line.End)
	s.Equal("dnd5e:env:test:wall", wall.Appearance.AssetRef)
	s.Equal(3.0, wall.Appearance.Height)
	s.Require().NotNil(wall.Blocker.BlocksMovement)
	s.Require().NotNil(wall.Blocker.BlocksLineOfSight)
	s.True(*wall.Blocker.BlocksMovement)
	s.True(*wall.Blocker.BlocksLineOfSight)
	s.Equal(30.0, wall.Blocker.Footprint.Width, "the wall rectangle is not clamped to the prop limit")
	s.Empty(wall.Openings)

	// The authored bytes survive a re-emit: the decode is round-trip stable
	// and the wall's own words are still in the document.
	once := s.encoded(spec)
	s.Contains(string(once), "walls:")
	s.Contains(string(once), "assetRef: dnd5e:env:test:wall")
	second, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: once})
	s.Require().NoError(err)
	s.Equal(string(once), string(s.encoded(second.Spec)), "the decode is round-trip stable")
}

func (s *SingleRoomWallSuite) TestAbsentWallsKeepTheOldBytesAndTheOldField() {
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	s.Require().NoError(err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Empty(decoded.Spec.Room.Gameplay.Walls)
	s.NotContains(string(s.encoded(decoded.Spec)), "\n        walls:",
		"a room with no walls emits no walls key")

	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	s.Empty(generatedSpans(compiled.Field), "a room with no walls generates no spans")
}

func (s *SingleRoomWallSuite) TestAnEmptyWallListIsAbsent() {
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{}
	s.NotContains(string(s.encoded(spec)), "\n        walls:")
}

// --- The blocker supplies the extent; openings are subtracted after it ---

func (s *SingleRoomWallSuite) TestALongWallCompilesWithoutThePropClamp() {
	// A twenty-unit line with a thirty-unit blocker: the authored extent is
	// kept exactly, far beyond the twelve-unit prop clamp.
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("long-wall", 0, 3, 20, 3, wallBlocks(true, true, 30, 0.5, 0, 0)),
	})
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 1)
	s.Equal(encounter.PropID("wall/long-wall/span/0"), spans[0].ID)
	s.True(spans[0].BlocksMovement)
	s.True(spans[0].BlocksLineOfSight)
	s.InDelta(30*scenePerFoot(), spans[0].Placement.Footprint.Box.D, 1e-9,
		"the blocker width is the span's length and is not clamped")
	s.InDelta(0.5*scenePerFoot(), spans[0].Placement.Footprint.Box.W, 1e-9)
	// The span's centre sits at L/2 along start→end: scene (10,3).
	s.InDelta(10*scenePerFoot(), spans[0].Placement.Origin.X, 1e-9)
	s.InDelta(3*scenePerFoot(), spans[0].Placement.Origin.Y, 1e-9)
	s.InDelta(0, spans[0].Placement.Facing, 1e-9, "a +X line faces east")
}

func (s *SingleRoomWallSuite) TestBlockerOffsetPrecedesOpeningSubtraction() {
	// Visible line [0,10]; independent blocker [-1,11]; opening [6,8]. The
	// blocking spans are [-1,6] and [8,11] — the offset extent is defined
	// FIRST, then the gap is cut, so nothing is pushed back into the doorway.
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("cut-wall", 0, 3, 10, 3, wallBlocks(true, true, 12, 0.5, 0, 0.5),
			dungeonspec.RoomWallOpening{ID: "opening-1", Position: 7, Width: 2}),
	})
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 2)
	s.Equal("wall/cut-wall/span/0", string(spans[0].ID))
	s.Equal("wall/cut-wall/span/1", string(spans[1].ID))
	s.InDelta(7*scenePerFoot(), spans[0].Placement.Footprint.Box.D, 1e-9, "span [-1,6]")
	s.InDelta(2.5*scenePerFoot(), spans[0].Placement.Origin.X, 1e-9, "centre at 2.5 along the line")
	s.InDelta(3*scenePerFoot(), spans[1].Placement.Footprint.Box.D, 1e-9, "span [8,11]")
	s.InDelta(9.5*scenePerFoot(), spans[1].Placement.Origin.X, 1e-9, "centre at 9.5 along the line")
	s.InDelta(0.5*scenePerFoot(), spans[0].Placement.LocalOffset.Y, 1e-9,
		"the transverse offsetZ survives on the span's own local offset")

	// The gap is clear: the independently computed centre of the removed
	// corridor touches neither span. The offset puts the blocker's line at
	// scene z = 3.5, so the gap point is measured there.
	gap := spatial.Point{X: 7 * scenePerFoot(), Y: 3.5 * scenePerFoot()}
	for _, span := range spans {
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: span.Placement, From: gap, To: gap,
		})
		s.Require().NoError(err)
		s.False(trace.Contact, "no blocking span reaches into the doorway: %s", span.ID)
	}
}

func (s *SingleRoomWallSuite) TestAnOpeningCoveringTheWholeLineKeepsTheBlockerBeyondIt() {
	// The visible line is entirely open; the blocker extends past it on both
	// sides and those extensions remain.
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("open-wall", 0, 3, 10, 3, wallBlocks(true, true, 12, 0.5, 0, 0),
			dungeonspec.RoomWallOpening{ID: "whole", Position: 5, Width: 10}),
	})
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 2)
	s.InDelta(1*scenePerFoot(), spans[0].Placement.Footprint.Box.D, 1e-9, "[-1,0] survives")
	s.InDelta(1*scenePerFoot(), spans[1].Placement.Footprint.Box.D, 1e-9, "[10,11] survives")
}

func (s *SingleRoomWallSuite) TestTwoTouchingOpeningsLeaveNoPhantomSpan() {
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("touching-wall", 0, 3, 10, 3, wallBlocks(true, true, 10, 0.5, 0, 0),
			dungeonspec.RoomWallOpening{ID: "first", Position: 2, Width: 4},
			dungeonspec.RoomWallOpening{ID: "second", Position: 6, Width: 4}),
	})
	// Visible [0,10], blocker [0,10], openings [0,4] and [4,8]: one span [8,10].
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 1)
	s.Equal("wall/touching-wall/span/0", string(spans[0].ID))
	s.InDelta(2*scenePerFoot(), spans[0].Placement.Footprint.Box.D, 1e-9)
	s.InDelta(9*scenePerFoot(), spans[0].Placement.Origin.X, 1e-9)
}

// --- The source XZ frame maps to the canonical plane, sign and all ---

func (s *SingleRoomWallSuite) TestDiagonalWallPinsTheAxisAndSign() {
	// Line (0,0)→(6,8): L=10, dir=(0.6,0.8), a noncentral opening at 7. The
	// corners below are computed from the SOURCE geometry by hand, not read
	// back out of the lowering. The floor is moved away so the witness can
	// use the plan's exact endpoints.
	compiled := s.compiledWithShiftedFloor([]dungeonspec.RoomWall{
		roomWall("diagonal", 0, 0, 6, 8, wallBlocks(true, true, 10, 0.4, 0, 0.5),
			dungeonspec.RoomWallOpening{ID: "gap", Position: 7, Width: 2}),
	}, -20, 0)
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 2)

	span := spans[0] // relative [0,6], centre distance 3
	k := scenePerFoot()
	s.InDelta(1.8*k, span.Placement.Origin.X, 1e-9, "origin on the line at distance 3")
	s.InDelta(2.4*k, span.Placement.Origin.Y, 1e-9)
	s.InDelta(math.Atan2(8, 6)*180/math.Pi, span.Placement.Facing, 1e-9,
		"canonical facing is atan2(dz,dx) — the positive-z sign")
	s.InDelta(6*k, span.Placement.Footprint.Box.D, 1e-9)
	s.InDelta(0.4*k, span.Placement.Footprint.Box.W, 1e-9)
	s.InDelta(0.5*k, span.Placement.LocalOffset.Y, 1e-9)

	// Independent centre: line start + dir*3 + across*offsetZ, in scene
	// (across = (-dirZ, dirX)), then scene→canonical (x*k, z*k).
	dirX, dirZ := 0.6, 0.8
	centre := spatial.Point{
		X: (0 + dirX*3 - dirZ*0.5) * k,
		Y: (0 + dirZ*3 + dirX*0.5) * k,
	}
	trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
		Placement: span.Placement, From: centre, To: centre,
	})
	s.Require().NoError(err)
	s.True(trace.Contact, "the independently computed span centre lies inside the compiled box")

	// The gap centre (distance 7 along the line) lies in neither span.
	gapCentre := spatial.Point{X: (dirX * 7) * k, Y: (dirZ * 7) * k}
	for _, candidate := range spans {
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: candidate.Placement, From: gapCentre, To: gapCentre,
		})
		s.Require().NoError(err)
		s.False(trace.Contact, "%s does not reach the gap", candidate.ID)
	}
}

func (s *SingleRoomWallSuite) TestDiagonalWallSignForNegativeZ() {
	// The same magnitude with dz < 0: the facing must flip sign, which is the
	// witness a reversed-line bug fails.
	compiled := s.compiledWithShiftedFloor([]dungeonspec.RoomWall{
		roomWall("diagonal-down", 0, 0, 6, -8, wallBlocks(true, true, 10, 0.4, 0, 0.5),
			dungeonspec.RoomWallOpening{ID: "gap", Position: 7, Width: 2}),
	}, -20, 0)
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 2)
	k := scenePerFoot()
	s.InDelta(1.8*k, spans[0].Placement.Origin.X, 1e-9)
	s.InDelta(-2.4*k, spans[0].Placement.Origin.Y, 1e-9)
	s.InDelta(math.Atan2(-8, 6)*180/math.Pi, spans[0].Placement.Facing, 1e-9)
}

// --- The real encounter: movement, sight, save/load ---

// boundaryWallX is the canonical-plane x of the shared edge between axial
// cells (0,0) and (1,0) — five feet from the first centre — expressed in scene
// units. A thin wall standing there lies between the two centres.
func boundaryWallX(canonicalX float64) float64 { return canonicalX / scenePerFoot() }

func (s *SingleRoomWallSuite) TestSolidSpanRefusesCrossingAndGapOffersIt() {
	boundary := boundaryWallX(2.5)
	for _, tc := range []struct {
		name    string
		opening bool
	}{
		{name: "solid", opening: false},
		{name: "gap", opening: true},
	} {
		s.Run(tc.name, func() {
			openings := []dungeonspec.RoomWallOpening{}
			if tc.opening {
				openings = append(openings, dungeonspec.RoomWallOpening{ID: "gap", Position: 2, Width: 2})
			}
			compiled := s.compiledWith([]dungeonspec.RoomWall{
				roomWall("boundary", boundary, -2, boundary, 2, wallBlocks(true, true, 4, 0.2, 0, 0), openings...),
			})
			enc := s.wallEncounter(compiled)

			_, err := enc.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})
			if tc.opening {
				s.Require().NoError(err, "a step through the doorway succeeds")
				s.False(s.sightAcross(enc), "and sight crosses the gap")
			} else {
				s.Require().ErrorIs(err, encounter.ErrBadPlacement, "a solid span refuses the crossing")
				s.True(s.sightAcross(enc), "and blocks sight")
			}
		})
	}
}

func (s *SingleRoomWallSuite) TestTheTwoFlagsChooseWhatBlocks() {
	boundary := boundaryWallX(2.5)
	cases := []struct {
		name     string
		movement bool
		sight    bool
	}{
		{name: "both", movement: true, sight: true},
		{name: "movement-only", movement: true, sight: false},
		{name: "sight-only", movement: false, sight: true},
		{name: "neither", movement: false, sight: false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			compiled := s.compiledWith([]dungeonspec.RoomWall{
				roomWall("boundary", boundary, -2, boundary, 2, wallBlocks(tc.movement, tc.sight, 4, 0.2, 0, 0)),
			})
			enc := s.wallEncounter(compiled)

			_, err := enc.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})
			if tc.movement {
				s.Require().ErrorIs(err, encounter.ErrBadPlacement)
			} else {
				s.Require().NoError(err, "a movement-transparent wall does not refuse a step")
			}
			s.Equal(tc.sight, s.sightAcross(enc), "sight follows its own flag and only its own flag")
		})
	}
}

func (s *SingleRoomWallSuite) TestSaveAndLoadRetainsGeneratedFootprintsAndAnswers() {
	boundary := boundaryWallX(2.5)
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("boundary", boundary, -2, boundary, 2, wallBlocks(true, true, 4, 0.2, 0, 0),
			dungeonspec.RoomWallOpening{ID: "gap", Position: 2, Width: 2}),
	})
	enc := s.wallEncounter(compiled)
	reloaded := s.reload(enc)

	// The generated span survives the save, with its id and both flags.
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 2)
	_, err := reloaded.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})
	s.Require().NoError(err, "the reloaded gap still offers the crossing")
	s.False(s.sightAcross(reloaded), "and the reloaded gap still lets sight through")
}

func (s *SingleRoomWallSuite) TestOffCenterWallBlocksSightAcrossAndReload() {
	// A wall a foot and a half off the shared edge: the straight crossing is
	// on the wall's interior, but neither cell centre is covered. Sight must
	// not borrow the neighbouring centre to see past it (rpg-toolkit#1913),
	// before or after a save/load.
	offCentre := boundaryWallX(1.5)
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("off-centre", offCentre, -2, offCentre, 2, wallBlocks(true, true, 4, 0.2, 0, 0)),
	})
	enc := s.wallEncounter(compiled)
	s.True(s.sightAcross(enc), "an off-centre wall still blocks the straight lane")

	_, err := enc.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)

	reloaded := s.reload(enc)
	s.True(s.sightAcross(reloaded), "and the save keeps the same answer")
}

func (s *SingleRoomWallSuite) TestAppearanceDoesNotChangeGeometry() {
	plain := roomWall("a", 0, 3, 10, 3, wallBlocks(true, false, 8, 0.4, 1, 0.25),
		dungeonspec.RoomWallOpening{ID: "gap", Position: 4, Width: 2})
	dressed := plain
	dressed.Appearance = dungeonspec.RoomWallAppearance{
		AssetRef: "a-completely-different-asset", Height: 40, Thickness: 9, Elevation: -12,
	}
	first := generatedSpans(s.compiledWith([]dungeonspec.RoomWall{plain}).Field)
	second := generatedSpans(s.compiledWith([]dungeonspec.RoomWall{dressed}).Field)
	s.Require().Len(first, 2)
	s.Require().Len(second, 2)
	for i := range first {
		s.Equal(first[i].Placement, second[i].Placement, "appearance never reaches a footprint")
		s.Equal(first[i].BlocksMovement, second[i].BlocksMovement)
		s.Equal(first[i].BlocksLineOfSight, second[i].BlocksLineOfSight)
	}
}

func (s *SingleRoomWallSuite) TestIndependentOverlappingBlockerSurvives() {
	// A pre-existing authored prop whose rectangle OVERLAPS the wall span
	// keeps its own identity and its own flags: the wall does not replace it
	// and it does not replace the wall.
	spec := s.baseSpec()
	spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{
		"bench": wallBlocks(false, false, 2, 2, 0, 0),
	}
	// The bench sits on the wall's line but north of every cell centre, so it
	// overlaps the blocker without closing a cell the party stands on.
	benchX := boundaryWallX(2.5)
	benchZ := 4 / scenePerFoot()
	spec.Room.Scene = sceneNode(s.T(), "version: 1\nid: scene-1\nname: Walls Room\nitems:\n  - {id: bench, transform: {x: "+
		fmtFloat(benchX)+", z: "+fmtFloat(benchZ)+", rotationY: 0}}\ngroups: []\n")
	boundary := boundaryWallX(2.5)
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("boundary", boundary, -2, boundary, 2, wallBlocks(true, true, 4, 0.2, 0, 0)),
	}
	compiled, err := dungeonspec.Load(s.encoded(spec))
	s.Require().NoError(err)
	flags := map[string]encounter.PlacedPropInput{}
	for _, p := range compiled.Field.Placed {
		flags[string(p.ID)] = p
	}
	bench, benchOK := flags["bench"]
	span, spanOK := flags["wall/boundary/span/0"]
	s.Require().True(benchOK, "the authored prop survives the wall compile")
	s.Require().True(spanOK, "and it did not collide with the span")
	s.False(bench.BlocksMovement, "the bench keeps its own flags")
	s.False(bench.BlocksLineOfSight)
	s.True(span.BlocksMovement, "the wall keeps its own flags")
	s.True(span.BlocksLineOfSight)
}

// fmtFloat renders a scene coordinate the way a YAML number should read.
func fmtFloat(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// --- Refusals ---

func (s *SingleRoomWallSuite) TestCanonicalWallBoundsRefuseAtTheAuthoredPath() {
	limit := float64(1<<30) / scenePerFoot()
	cases := []struct {
		name    string
		wall    dungeonspec.RoomWall
		refused bool
	}{
		{"below canonical origin ceiling", roomWall("bounded", limit-20, 3, limit-10, 3, wallBlocks(false, false, 10, 0.2, 0, 0)), false},
		{"above canonical origin ceiling", roomWall("bounded", limit+10, 3, limit+20, 3, wallBlocks(false, false, 10, 0.2, 0, 0)), true},
		{"canonical longitudinal extent", roomWall("bounded", 0, 3, 10, 3, wallBlocks(false, false, 4e8, 0.2, 0, 0)), true},
		{"canonical transverse extent", roomWall("bounded", 0, 3, 10, 3, wallBlocks(false, false, 10, 4e8, 0, 0)), true},
		{"canonical lateral offset", roomWall("bounded", 0, 3, 10, 3, wallBlocks(false, false, 10, 0.2, 0, 4e8)), true},
		{"derived origin combines valid source values", roomWall("bounded", 2e8, 3, 2e8+10, 3, wallBlocks(false, false, 10, 0.2, 2e8, 0)), true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			spec := s.baseSpec()
			spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{tc.wall}
			compiled, err := dungeonspec.Load(s.encoded(spec))
			if tc.refused {
				s.Require().Error(err)
				s.Contains(err.Error(), "room.room.walls[0]")
				s.Contains(err.Error(), "representable")
				s.Empty(compiled.Field.Placed, "no partial field on refusal")
				return
			}
			s.Require().NoError(err)
			// The wall compiles to its identity-only presence entry PLUS its one
			// blocking span. Find each by id rather than trusting list order:
			// the presence carries the raw wall id and asserts no blocking, the
			// span carries the generated id and the authored flags.
			presence, present := placedByID(compiled.Field, tc.wall.ID)
			s.Require().True(present, "the wall's raw identity is present")
			s.False(presence.BlocksMovement, "identity-only presence never blocks movement")
			s.False(presence.BlocksLineOfSight, "identity-only presence never blocks sight")
			span, spanned := placedByID(compiled.Field, "wall/"+tc.wall.ID+"/span/0")
			s.Require().True(spanned, "the blocking span is present")
			s.LessOrEqual(math.Abs(span.Placement.Origin.X), float64(1<<30))
			s.LessOrEqual(math.Abs(presence.Placement.Origin.X), float64(1<<30))
		})
	}
}

func (s *SingleRoomWallSuite) TestWallsUnderAThirdVersionRootRefuseByName() {
	spec := s.baseSpec()
	spec.Version = 3
	_, err := dungeonspec.Load(s.encoded(spec))
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.walls")
	s.Contains(err.Error(), "require root version 4")
}

func (s *SingleRoomWallSuite) TestWallGeometryRefusalsCarryTheirPaths() {
	cases := []struct {
		name    string
		mutate  func(*dungeonspec.RoomWall)
		path    string
		message string
	}{
		{"empty id", func(w *dungeonspec.RoomWall) { w.ID = "" }, "room.room.walls[0].id", "nonempty"},
		{"zero length", func(w *dungeonspec.RoomWall) { w.Line.End = w.Line.Start }, "room.room.walls[0].line", "finite positive length"},
		{"nonfinite endpoint", func(w *dungeonspec.RoomWall) { w.Line.Start.X = math.NaN() }, "room.room.walls[0].line.start.x", "finite"},
		{"zero width", func(w *dungeonspec.RoomWall) { w.Blocker.Footprint.Width = 0 }, "room.room.walls[0].blocker.footprint.width", "positive"},
		{"negative depth", func(w *dungeonspec.RoomWall) { w.Blocker.Footprint.Depth = -1 }, "room.room.walls[0].blocker.footprint.depth", "positive"},
		{"nonfinite offset", func(w *dungeonspec.RoomWall) { w.Blocker.Footprint.OffsetX = math.Inf(1) }, "room.room.walls[0].blocker.footprint.offsetX", "finite"},
		{"opening outside the line", func(w *dungeonspec.RoomWall) {
			w.Openings = []dungeonspec.RoomWallOpening{{ID: "gap", Position: 19, Width: 4}}
		}, "room.room.walls[0].openings[0]", "fit within the line"},
		{"opening with no width", func(w *dungeonspec.RoomWall) {
			w.Openings = []dungeonspec.RoomWallOpening{{ID: "gap", Position: 4, Width: 0}}
		}, "room.room.walls[0].openings[0].width", "positive"},
		{"nonfinite opening position", func(w *dungeonspec.RoomWall) {
			w.Openings = []dungeonspec.RoomWallOpening{{ID: "gap", Position: math.NaN(), Width: 1}}
		}, "room.room.walls[0].openings[0].position", "finite"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			spec := s.baseSpec()
			tc.mutate(&spec.Room.Gameplay.Walls[0])
			_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: s.encoded(spec)})
			s.Require().Error(err)
			var validation *dungeonspec.ValidationError
			s.Require().ErrorAs(err, &validation)
			found := false
			for _, defect := range validation.Errors {
				if defect.Path == tc.path {
					found = true
					s.Contains(defect.Message, tc.message)
				}
			}
			s.True(found, "a defect at %s: %+v", tc.path, validation.Errors)
		})
	}
}

func (s *SingleRoomWallSuite) TestDuplicateAndCollidingWallIDsRefuse() {
	s.Run("two walls share an id", func() {
		spec := s.baseSpec()
		spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
			roomWall("dup", 0, 3, 10, 3, wallBlocks(true, true, 4, 0.2, 0, 0)),
			roomWall("dup", 0, 5, 10, 5, wallBlocks(true, true, 4, 0.2, 0, 0)),
		}
		_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: s.encoded(spec)})
		s.Require().Error(err)
		s.Contains(err.Error(), "room.room.walls[1].id")
		s.Contains(err.Error(), "duplicate or colliding id")
	})

	s.Run("an opening id collides with another wall", func() {
		spec := s.baseSpec()
		spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
			roomWall("first", 0, 3, 10, 3, wallBlocks(true, true, 4, 0.2, 0, 0),
				dungeonspec.RoomWallOpening{ID: "second", Position: 2, Width: 1}),
			roomWall("second", 0, 5, 10, 5, wallBlocks(true, true, 4, 0.2, 0, 0)),
		}
		_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: s.encoded(spec)})
		s.Require().Error(err)
		s.Contains(err.Error(), "room.room.walls[0].openings[0].id")
		s.Contains(err.Error(), "duplicate or colliding id")
	})

	s.Run("overlapping openings refuse", func() {
		spec := s.baseSpec()
		spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
			roomWall("overlap", 0, 3, 10, 3, wallBlocks(true, true, 4, 0.2, 0, 0),
				dungeonspec.RoomWallOpening{ID: "a", Position: 4, Width: 4},
				dungeonspec.RoomWallOpening{ID: "b", Position: 6, Width: 4}),
		}
		_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: s.encoded(spec)})
		s.Require().Error(err)
		s.Contains(err.Error(), "room.room.walls[0].openings[1]")
		s.Contains(err.Error(), "overlaps")
	})
}

func (s *SingleRoomWallSuite) TestUnknownStructuralKeyIsNamedAtItsPath() {
	raw, err := os.ReadFile(roomWallFixture)
	s.Require().NoError(err)
	mutated := strings.Replace(string(raw),
		"        label: Long wall\n",
		"        label: Long wall\n        mystery: true\n", 1)
	s.Require().NotEqual(string(raw), mutated)
	_, err = dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: []byte(mutated)})
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.walls[0].mystery")
}

func (s *SingleRoomWallSuite) TestAGeneratedSpanIDCollidingWithAPropRefusesByPath() {
	spec := s.baseSpec()
	spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Walls Room
items:
  - {id: wall/a/span/0, transform: {x: 2, z: 3, rotationY: 0}}
groups: []
`)
	spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{
		"wall/a/span/0": wallBlocks(false, false, 1, 1, 0, 0),
	}
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("a", 0, 3, 10, 3, wallBlocks(true, true, 4, 0.2, 0, 0)),
	}
	_, err := dungeonspec.Load(s.encoded(spec))
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.walls[0]")
	s.Contains(err.Error(), "wall/a/span/0")
	s.Contains(err.Error(), "collides")
}

// --- Stable wall presence: identity, not obstruction ---

func (s *SingleRoomWallSuite) TestPresenceEntryIsIdentityOnlyAndCannotRefillAnOpening() {
	boundary := boundaryWallX(2.5)
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("boundary", boundary, -2, boundary, 2, wallBlocks(true, true, 4, 0.2, 0, 0),
			dungeonspec.RoomWallOpening{ID: "gap", Position: 2, Width: 2}),
	})

	presence, ok := placedByID(compiled.Field, "boundary")
	s.Require().True(ok, "the raw wall id is a placed contributor")
	s.False(presence.BlocksMovement, "identity-only presence never blocks movement")
	s.False(presence.BlocksLineOfSight, "identity-only presence never blocks sight")
	// The presence rectangle is the COMPLETE authored blocker, so it spans the
	// opening the spans left clear.
	s.InDelta(4*scenePerFoot(), presence.Placement.Footprint.Box.D, 1e-9,
		"presence carries the complete blocker, not the cut spans")

	// The opening's midpoint sits INSIDE the presence rectangle and inside NO
	// blocking span — the identity covers it without ever filling it.
	gapPoint := spatial.Point{X: boundary * scenePerFoot(), Y: 0}
	trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: presence.Placement, From: gapPoint, To: gapPoint})
	s.Require().NoError(err)
	s.True(trace.Contact, "the presence rectangle does span the opening")
	spans := generatedSpans(compiled.Field)
	s.Require().Len(spans, 2)
	for _, span := range spans {
		s.True(span.BlocksMovement, "the linked span keeps the authored flags")
		s.True(span.BlocksLineOfSight)
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: span.Placement, From: gapPoint, To: gapPoint})
		s.Require().NoError(err)
		s.False(trace.Contact, "no blocking span reaches into the opening")
	}

	// The real encounter still walks the gap and sees through it: the presence
	// covers the crossing without refilling it.
	enc := s.wallEncounter(compiled)
	_, err = enc.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})
	s.Require().NoError(err, "the identity-only presence does not close the doorway")
	s.False(s.sightAcross(enc), "and it does not obstruct the lane")
}

func (s *SingleRoomWallSuite) TestZeroSpanWallRetainsItsPresence() {
	// A blocker entirely inside its own opening: the subtraction leaves no
	// blocking span, but the wall's identity presence survives.
	compiled := s.compiledWith([]dungeonspec.RoomWall{
		roomWall("open", 0, 0, 10, 0, wallBlocks(true, true, 2, 0.5, 0, 0),
			dungeonspec.RoomWallOpening{ID: "whole", Position: 5, Width: 10}),
	})
	s.Empty(generatedSpans(compiled.Field), "no blocker span survives the opening")
	presence, ok := placedByID(compiled.Field, "open")
	s.Require().True(ok, "the wall still has its identity entry")
	s.False(presence.BlocksMovement)
	s.False(presence.BlocksLineOfSight)
	s.InDelta(2*scenePerFoot(), presence.Placement.Footprint.Box.D, 1e-9, "the blocker's own width")
}

func (s *SingleRoomWallSuite) TestPresenceMatchesTheAuthoredBlockerAndAppearanceNeverChangesIt() {
	// Line length 10, blocker 30 wide, 0.5 deep, offsetX 3 and offsetZ 0.25:
	// the long extent is kept, and the blocker centre is L/2+offsetX = 8 along
	// the line at scene (8,3).
	plain := roomWall("extent", 0, 3, 10, 3, wallBlocks(true, true, 30, 0.5, 3, 0.25))
	presence, ok := placedByID(s.compiledWith([]dungeonspec.RoomWall{plain}).Field, "extent")
	s.Require().True(ok)
	k := scenePerFoot()
	s.InDelta(30*k, presence.Placement.Footprint.Box.D, 1e-9, "the long extent is kept, not clamped")
	s.InDelta(0.5*k, presence.Placement.Footprint.Box.W, 1e-9, "the authored depth")
	s.InDelta(3*k, presence.Placement.LocalOffset.X, 1e-9, "the authored longitudinal offsetX survives")
	s.InDelta(0.25*k, presence.Placement.LocalOffset.Y, 1e-9, "the authored lateral offsetZ survives")
	s.InDelta(5*k, presence.Placement.Origin.X, 1e-9, "the pose is the wall midpoint")
	s.InDelta(3*k, presence.Placement.Origin.Y, 1e-9)
	s.InDelta(8*k, presence.Placement.Origin.X+presence.Placement.LocalOffset.X, 1e-9,
		"the blocker centre sits at L/2+offsetX = 8 along the line")
	s.InDelta(0, presence.Placement.Facing, 1e-9)

	dressed := plain
	dressed.Appearance = dungeonspec.RoomWallAppearance{AssetRef: "other", Height: 40, Thickness: 9, Elevation: -12}
	other, ok := placedByID(s.compiledWith([]dungeonspec.RoomWall{dressed}).Field, "extent")
	s.Require().True(ok)
	s.Equal(presence.Placement, other.Placement, "appearance never reaches the presence rectangle")
}

func (s *SingleRoomWallSuite) TestARawWallIdentityCollidingWithAGeneratedSpanRefuses() {
	// A wall id that is another wall's generated span id gives two
	// contributors one name; the compile refuses it by path.
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("b", 0, 3, 10, 3, wallBlocks(true, true, 4, 0.2, 0, 0)),
		roomWall("wall/b/span/0", 0, 5, 10, 5, wallBlocks(true, true, 4, 0.2, 0, 0)),
	}
	_, err := dungeonspec.Load(s.encoded(spec))
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.walls")
	s.Contains(err.Error(), "wall/b/span/0")
	s.Contains(err.Error(), "collides")
}

func (s *SingleRoomWallSuite) TestAConcealmentOfAZeroSpanWallStillConcealsItsPresence() {
	// A wall whose blocker subtracts to no span has only its presence entry;
	// naming the wall still hides that identity.
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("open", 0, 0, 10, 0, wallBlocks(true, true, 2, 0.5, 0, 0),
			dungeonspec.RoomWallOpening{ID: "whole", Position: 5, Width: 10}),
	}
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{
		"vault": {
			Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 15}},
			Cells:  []dungeonspec.RoomCell{{Q: 4, R: 0}},
			Props:  []string{"open"},
		},
	}
	compiled, err := dungeonspec.Load(s.encoded(spec))
	s.Require().NoError(err)
	s.Require().Len(compiled.Concealments, 1)
	s.Equal([]string{"open"}, compiled.Concealments[0].Props,
		"the presence is concealed even when the wall has no spans")
}

func (s *SingleRoomWallSuite) TestAConcealmentNamingAnUnknownWallRefuses() {
	// A wall id that names no wall this room authors is still an unknown ref,
	// refused by name rather than silently ignored.
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls = nil
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{
		"vault": {
			Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 15}},
			Props:  []string{"long-wall"},
		},
	}
	_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: s.encoded(spec)})
	s.Require().Error(err)
	s.Contains(err.Error(), "concealments.vault.props[0]")
	s.Contains(err.Error(), "declare it under propDeclarations")
}

// sceneNode parses a scene mapping into the node a room source carries.
func sceneNode(t require.TestingT, src string) yaml.Node {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &root))
	require.NotEmpty(t, root.Content)

	return *root.Content[0]
}
