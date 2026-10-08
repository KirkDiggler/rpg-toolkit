// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// single_room_wall_doors_test.go is A DOOR ATTACHED TO A STRUCTURAL WALL
// OPENING IN THE SINGLE-ROOM DIALECT (rpg-project#169, Task 6) — the provider
// counterpart to rpg-dnd5e-web's Task 5 attached-door contract.
//
// The claim under test is narrow and exact: an opening may carry a door record
// that stores ONLY an identity and an appearance, and the engine lowers that
// into the door state it already has — one nonblocking placed entry under the
// raw attached id, one live `<key>/<id>` door sharing the placement resolved
// from the opening. No second collider, no door grammar, no scene-item copy,
// no prop declaration. The state is the existing `doorBindings` grammar, and
// what the door closes is its state's answer and only its state's answer.
//
// The geometry witnesses are Task 5's own numbers computed by hand, not read
// back out of the lowering: (0,0)→(10,0) with an opening at 7 width 2 resolves
// at (7,0); translating by (2,3) moves it to (9,3); rotating +pi/2 about (5,0)
// puts it at (5,2) with the wall's direction turned to +90°. Gameplay is an
// existing encounter query — a closed leaf refuses the crossing and the lane,
// opening it clears only that contribution, and a save keeps both.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"gopkg.in/yaml.v3"
)

type SingleRoomWallDoorSuite struct {
	suite.Suite
}

func TestSingleRoomWallDoorSuite(t *testing.T) {
	suite.Run(t, new(SingleRoomWallDoorSuite))
}

const wallDoorsFixture = "testdata/world-builder-v4-wall-doors.yaml"

const boundDoorID = "wall-doors-room/long-wall-door"

func (s *SingleRoomWallDoorSuite) baseSpec() *dungeonspec.SingleRoomSpec {
	raw, err := os.ReadFile(wallDoorsFixture)
	s.Require().NoError(err)
	out, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Require().NotNil(out.Spec)

	return out.Spec
}

func (s *SingleRoomWallDoorSuite) encoded(spec *dungeonspec.SingleRoomSpec) []byte {
	raw, err := yaml.Marshal(spec)
	s.Require().NoError(err)

	return raw
}

func (s *SingleRoomWallDoorSuite) load(spec *dungeonspec.SingleRoomSpec) dungeonspec.Compiled {
	compiled, err := dungeonspec.Load(s.encoded(spec))
	s.Require().NoError(err)

	return compiled
}

// defects decodes and returns the defects the source earned.
func (s *SingleRoomWallDoorSuite) defects(raw []byte) []dungeonspec.FieldError {
	_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().Error(err, "this source was expected to be refused")
	var verr *dungeonspec.ValidationError
	s.Require().ErrorAs(err, &verr)

	return verr.Errors
}

// requireDefect asserts a defect is present at exactly this path carrying this
// sentence — used where a source mutation leaves more than one true defect (a
// malformed door also strands the binding that named it).
func (s *SingleRoomWallDoorSuite) requireDefect(errs []dungeonspec.FieldError, path, message string) {
	for _, d := range errs {
		if d.Path == path && (message == "" || strings.Contains(d.Message, message)) {
			return
		}
	}
	s.Failf("no defect at %s containing %q", path, message, "defects: %+v", errs)
}

// gapSpec is the fixture with ONE wall standing at the crossing between axial
// (0,0) and (1,0) — the geometry the gameplay assertions below walk — carrying
// an attached door whose state is the binding handed in.
func (s *SingleRoomWallDoorSuite) gapSpec(binding dungeonspec.RoomDoorBinding) *dungeonspec.SingleRoomSpec {
	spec := s.baseSpec()
	boundary := boundaryWallX(2.5)
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("boundary", boundary, -2, boundary, 2, wallBlocks(true, true, 4, 0.2, 0, 0),
			dungeonspec.RoomWallOpening{
				ID: "gap", Position: 2, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "gap-door", AssetRef: "dnd5e:env:test:door"},
			}),
	}
	spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"gap-door": binding}

	return spec
}

func (s *SingleRoomWallDoorSuite) play(compiled dungeonspec.Compiled, members ...encounter.MemberInput) *encounter.Encounter {
	if len(members) == 0 {
		members = []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: axial(0, 0)}}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sheets:    zeroSheets{},
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: encounter.UnobservedEquipment{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: nothingIsEverFound{}, Witness: nobodyPerceivesAnything{},
		Field:   compiled.Field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

func (s *SingleRoomWallDoorSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sheets:    zeroSheets{},
		Data:      enc.ToData(),
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: encounter.UnobservedEquipment{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: nothingIsEverFound{}, Witness: nobodyPerceivesAnything{},
	})
	s.Require().NoError(err)

	return loaded
}

func (s *SingleRoomWallDoorSuite) step(enc *encounter.Encounter) error {
	_, err := enc.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})

	return err
}

func (s *SingleRoomWallDoorSuite) sightAcross(enc *encounter.Encounter) bool {
	canvas, err := enc.Canvas()
	s.Require().NoError(err)

	return canvas.IsLineOfSightBlocked(axial(0, 0), axial(4, 0))
}

// placedByID finds the compiled placed contributor carrying an id.
func placedByID(field encounter.FieldInput, id string) (encounter.PlacedPropInput, bool) {
	for _, p := range field.Placed {
		if string(p.ID) == id {
			return p, true
		}
	}

	return encounter.PlacedPropInput{}, false
}

// doorByID finds the compiled door carrying an id.
func doorByID(doors []encounter.DoorInput, id string) (encounter.DoorInput, bool) {
	for _, d := range doors {
		if d.ID == id {
			return d, true
		}
	}

	return encounter.DoorInput{}, false
}

// doorReadByID finds the live door read carrying an id.
func doorReadByID(doors []encounter.Door, id string) (encounter.Door, bool) {
	for _, d := range doors {
		if d.ID == id {
			return d, true
		}
	}

	return encounter.Door{}, false
}

// doorsListed reports whether a member's own door read carries an id — the
// projection that withholds a concealed door.
func doorsListed(doors []encounter.Door, id encounter.DoorID) bool {
	for _, d := range doors {
		if d.ID == id {
			return true
		}
	}

	return false
}

// --- The web Task 5 contract decodes, carries appearance unread, and round-trips ---

func (s *SingleRoomWallDoorSuite) TestTheWebAttachedDoorDecodes() {
	spec := s.baseSpec()
	s.Require().Len(spec.Room.Gameplay.Walls, 1)
	wall := spec.Room.Gameplay.Walls[0]
	s.Require().Len(wall.Openings, 1)
	opening := wall.Openings[0]
	s.Require().NotNil(opening.Door, "the opening carries an attached door")
	s.Equal("long-wall-door", opening.Door.ID)
	s.Equal("dnd5e:env:test:door", opening.Door.AssetRef, "appearance is carried")

	// A bound door has NO prop declaration and NO scene item: the appearance
	// is the only thing carried, and the opening owns the pose.
	s.Empty(spec.Room.Gameplay.PropDeclarations)
	s.NotContains(s.encoded(spec), "transform")
}

func (s *SingleRoomWallDoorSuite) TestTheAttachedDoorRoundTrips() {
	spec := s.baseSpec()
	once := s.encoded(spec)
	s.Contains(string(once), "door:")
	s.Contains(string(once), "long-wall-door")
	second, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: once})
	s.Require().NoError(err)
	s.Equal(string(once), string(s.encoded(second.Spec)), "the decode is round-trip stable")
}

func (s *SingleRoomWallDoorSuite) TestAbsentDoorKeepsTheOldField() {
	// The room without an attached door compiles to no doors and no synthetic
	// placed entry: absence is a bare opening.
	raw, err := os.ReadFile(roomWallFixture)
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	s.Empty(compiled.Field.Doors, "a wall with no attached door compiles no door")
	for _, p := range compiled.Field.Placed {
		s.NotEqual(string(p.ID), "long-wall-door")
	}
}

// --- The canonical geometry is the opening's, computed by hand ---

func (s *SingleRoomWallDoorSuite) TestAttachedDoorResolvesCenterAndOffsets() {
	spec := s.baseSpec()
	// (0,0)→(10,0), opening at 7 width 2, blocker depth 0.6 and offsetZ 0.5.
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("wall", 0, 0, 10, 0, wallBlocks(true, true, 30, 0.6, 0, 0.5),
			dungeonspec.RoomWallOpening{
				ID: "gap", Position: 7, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "gap-door", AssetRef: "dnd5e:env:test:door"},
			}),
	}
	spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"gap-door": {Closed: true}}
	compiled := s.load(spec)

	placed, ok := placedByID(compiled.Field, "gap-door")
	s.Require().True(ok, "the raw attached id is the placed entry the atlas reports")
	door, ok := doorByID(compiled.Field.Doors, "wall-doors-room/gap-door")
	s.Require().True(ok, "the live door is minted <key>/<id>")

	k := scenePerFoot()
	s.InDelta(7*k, placed.Placement.Origin.X, 1e-9, "the opening's centre on the line")
	s.InDelta(0, placed.Placement.Origin.Y, 1e-9)
	s.InDelta(0, placed.Placement.Facing, 1e-9, "a +X line faces east")
	s.InDelta(2*k, placed.Placement.Footprint.Box.D, 1e-9, "width is the opening's")
	s.InDelta(0.6*k, placed.Placement.Footprint.Box.W, 1e-9, "depth is the blocker's")
	s.InDelta(0.5*k, placed.Placement.LocalOffset.Y, 1e-9, "lateral offset is the blocker's")
	s.False(placed.BlocksMovement, "a door asserts no blocking of its own")
	s.False(placed.BlocksLineOfSight)

	// The two contributions SHARE the one derived placement.
	s.Require().NotNil(door.Placement)
	s.Equal(placed.Placement, *door.Placement, "one authored footprint, one adapter")
	s.Equal(encounter.DoorClosed, door.State.Kind())
}

func (s *SingleRoomWallDoorSuite) TestTranslatedAttachedDoorMatchesTheWitness() {
	spec := s.baseSpec()
	// The original wall translated by (2,3): its opening centre moves to (9,3).
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("wall", 2, 3, 12, 3, wallBlocks(true, true, 30, 0.6, 0, 0.5),
			dungeonspec.RoomWallOpening{
				ID: "gap", Position: 7, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "gap-door", AssetRef: "dnd5e:env:test:door"},
			}),
	}
	spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"gap-door": {}}
	compiled := s.load(spec)

	placed, ok := placedByID(compiled.Field, "gap-door")
	s.Require().True(ok)
	k := scenePerFoot()
	s.InDelta(9*k, placed.Placement.Origin.X, 1e-9, "translation carries the opening")
	s.InDelta(3*k, placed.Placement.Origin.Y, 1e-9)
	s.InDelta(0, placed.Placement.Facing, 1e-9)

	door, ok := doorByID(compiled.Field.Doors, "wall-doors-room/gap-door")
	s.Require().True(ok)
	s.Equal(encounter.DoorOpen, door.State.Kind(), "an empty binding is an OPEN door")
}

func (s *SingleRoomWallDoorSuite) TestRotatedAttachedDoorMatchesTheWitness() {
	spec := s.baseSpec()
	// The original wall rotated +pi/2 in XZ about (5,0): line (5,-5)→(5,5),
	// opening centre (5,2), wall direction turned to +90° canonical facing.
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("wall", 5, -5, 5, 5, wallBlocks(true, true, 30, 0.6, 0, 0),
			dungeonspec.RoomWallOpening{
				ID: "gap", Position: 7, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "gap-door", AssetRef: "dnd5e:env:test:door"},
			}),
	}
	spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"gap-door": {Closed: true}}
	compiled := s.load(spec)

	placed, ok := placedByID(compiled.Field, "gap-door")
	s.Require().True(ok)
	k := scenePerFoot()
	s.InDelta(5*k, placed.Placement.Origin.X, 1e-9)
	s.InDelta(2*k, placed.Placement.Origin.Y, 1e-9, "the rotated opening resolves at (5,2)")
	s.InDelta(90, placed.Placement.Facing, 1e-9, "the wall's direction turned with it")
}

// --- Refusals, each at the author's own path ---

func (s *SingleRoomWallDoorSuite) TestMissingAttachedBindingRefusesAtTheAttachment() {
	spec := s.baseSpec()
	spec.Room.Gameplay.DoorBindings = nil
	errs := s.defects(s.encoded(spec))
	s.Require().Len(errs, 1)
	s.Equal("room.room.walls[0].openings[0].door", errs[0].Path)
	s.Contains(errs[0].Message, "an attached door needs its state under doorBindings")
}

func (s *SingleRoomWallDoorSuite) TestMalformedAttachedDoorRefuses() {
	fixture := s.fixture()
	anchor := "door: {id: long-wall-door, assetRef: 'dnd5e:env:test:door'}"

	s.Run("authored null", func() {
		raw := swapOnce(s.T(), fixture, anchor, "door:")
		errs := s.defects(raw)
		s.requireDefect(errs, "room.room.walls[0].openings[0].door", "must not be null")
	})

	s.Run("no id", func() {
		raw := swapOnce(s.T(), fixture, anchor, "door: {assetRef: 'dnd5e:env:test:door'}")
		errs := s.defects(raw)
		s.requireDefect(errs, "room.room.walls[0].openings[0].door.id", "is required")
		s.requireDefect(errs, "room.room.walls[0].openings[0].door.id", "nonempty")
	})

	s.Run("empty appearance ref", func() {
		spec := s.baseSpec()
		spec.Room.Gameplay.Walls[0].Openings[0].Door.AssetRef = ""
		errs := s.defects(s.encoded(spec))
		s.Require().Len(errs, 1)
		s.Equal("room.room.walls[0].openings[0].door.assetRef", errs[0].Path)
		s.Contains(errs[0].Message, "is required")
	})
}

func (s *SingleRoomWallDoorSuite) TestAnUnknownAttachedDoorKeyRefuses() {
	// A stored transform is the forbidden second pose. It does not exist on
	// the record, so the strict decoder names it with the keys the record does
	// take.
	raw := swapOnce(s.T(), s.fixture(),
		"door: {id: long-wall-door, assetRef: 'dnd5e:env:test:door'}",
		"door: {id: long-wall-door, assetRef: 'dnd5e:env:test:door', transform: {x: 1, z: 2}}")
	errs := s.defects(raw)
	s.Require().Len(errs, 1)
	s.Equal("room.room.walls[0].openings[0].door.transform", errs[0].Path)
	s.Contains(errs[0].Message, `"transform" is not a key this build reads`)
	s.Contains(errs[0].Message, "assetRef, id")
}

// fixture is the authored fixture file's own bytes, for the cases that must
// mutate the SOURCE text an author wrote rather than a re-marshal of it.
func (s *SingleRoomWallDoorSuite) fixture() []byte {
	raw, err := os.ReadFile(wallDoorsFixture)
	s.Require().NoError(err)

	return raw
}

func (s *SingleRoomWallDoorSuite) TestAttachedDoorIDCollisionsRefuse() {
	s.Run("against the wall id", func() {
		spec := s.baseSpec()
		spec.Room.Gameplay.Walls[0].Openings[0].Door.ID = "long-wall"
		spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"long-wall": {}}
		errs := s.defects(s.encoded(spec))
		s.Require().Len(errs, 1)
		s.Equal("room.room.walls[0].openings[0].door.id", errs[0].Path)
		s.Contains(errs[0].Message, "duplicate or colliding id")
	})

	s.Run("against its own opening id", func() {
		// A door id is claimed before its opening's, matching the web's own
		// order — so the collision is named at the OPENING, which is the
		// second claim.
		spec := s.baseSpec()
		spec.Room.Gameplay.Walls[0].Openings[0].Door.ID = "door-gap"
		spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"door-gap": {}}
		errs := s.defects(s.encoded(spec))
		s.Require().Len(errs, 1)
		s.Equal("room.room.walls[0].openings[0].id", errs[0].Path)
		s.Contains(errs[0].Message, "duplicate or colliding id")
	})

	s.Run("against a scene item", func() {
		spec := s.baseSpec()
		spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Wall Doors Room
items:
  - {id: long-wall-door, transform: {x: 1, z: 1, rotationY: 0}}
groups: []
`)
		errs := s.defects(s.encoded(spec))
		s.Require().Len(errs, 1)
		s.Equal("room.room.walls[0].openings[0].door.id", errs[0].Path)
		s.Contains(errs[0].Message, "duplicate or colliding id")
	})

	s.Run("two openings sharing a door id", func() {
		spec := s.baseSpec()
		spec.Room.Gameplay.Walls[0].Openings = []dungeonspec.RoomWallOpening{
			{ID: "a", Position: 4, Width: 2, Door: &dungeonspec.RoomWallDoor{ID: "shared", AssetRef: "dnd5e:env:test:door"}},
			{ID: "b", Position: 8, Width: 2, Door: &dungeonspec.RoomWallDoor{ID: "shared", AssetRef: "dnd5e:env:test:door"}},
		}
		spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"shared": {}}
		errs := s.defects(s.encoded(spec))
		s.Require().Len(errs, 1)
		s.Equal("room.room.walls[0].openings[1].door.id", errs[0].Path)
		s.Contains(errs[0].Message, "duplicate or colliding id")
	})
}

func (s *SingleRoomWallDoorSuite) TestAnAttachedIDThatIsAlsoADeclaredPropRefuses() {
	// "either standalone owner or bound owner, never both": a bound door id
	// that is also a declared prop item is refused at the attachment, because
	// its geometry is the opening's and a prop declaration would give it a
	// second one.
	spec := s.baseSpec()
	spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Wall Doors Room
items:
  - {id: long-wall-door, transform: {x: 1, z: 1, rotationY: 0}}
groups: []
`)
	spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{
		"long-wall-door": wallBlocks(false, false, 0.35, 1.8, 0, 0),
	}
	errs := s.defects(s.encoded(spec))
	s.Require().Len(errs, 1)
	s.Equal("room.room.walls[0].openings[0].door.id", errs[0].Path)
	s.Contains(errs[0].Message, "duplicate or colliding id")
}

func (s *SingleRoomWallDoorSuite) TestAnUnresolvableDoorBindingStillRefuses() {
	// The existing standalone path is untouched: a binding that names no prop
	// and no attached door still asks for a declaration.
	spec := s.baseSpec()
	spec.Room.Gameplay.DoorBindings["ghost-door"] = dungeonspec.RoomDoorBinding{}
	errs := s.defects(s.encoded(spec))
	s.Require().Len(errs, 1)
	s.Equal("room.room.doorBindings.ghost-door", errs[0].Path)
	s.Contains(errs[0].Message, "a door needs a footprint")
}

// --- The real encounter: state, movement, sight, save/load ---

func (s *SingleRoomWallDoorSuite) TestAttachedDoorStateDecidesMovementAndSight() {
	locked := dungeonspec.CheckSpec{{Ability: "str", DC: 15}}
	cases := []struct {
		name    string
		binding dungeonspec.RoomDoorBinding
		kind    encounter.DoorStateKind
		stepErr error
	}{
		{name: "closed", binding: dungeonspec.RoomDoorBinding{Closed: true}, kind: encounter.DoorClosed, stepErr: encounter.ErrDoorShut},
		{name: "locked", binding: dungeonspec.RoomDoorBinding{Locked: locked}, kind: encounter.DoorLocked, stepErr: encounter.ErrLocked},
		{name: "open", binding: dungeonspec.RoomDoorBinding{}, kind: encounter.DoorOpen},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			compiled := s.load(s.gapSpec(tc.binding))
			door, ok := doorByID(compiled.Field.Doors, "wall-doors-room/gap-door")
			s.Require().True(ok)
			s.Equal(tc.kind, door.State.Kind())

			enc := s.play(compiled)
			err := s.step(enc)
			if tc.stepErr != nil {
				s.Require().ErrorIs(err, tc.stepErr, "a shut leaf refuses the crossing")
				s.True(s.sightAcross(enc), "and the lane")
			} else {
				s.Require().NoError(err, "an open door offers the crossing")
				s.False(s.sightAcross(enc), "and lets sight through")
				return
			}

			// Opening clears the one contribution and offers the gap.
			_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: encounter.DoorID("wall-doors-room/gap-door")})
			if tc.kind == encounter.DoorLocked {
				s.Require().ErrorIs(err, encounter.ErrLocked, "a locked door is unlocked first")
				return
			}
			s.Require().NoError(err)
			s.Require().NoError(s.step(enc), "opening the door offers the crossing")
			s.False(s.sightAcross(enc), "and lets sight through")
		})
	}
}

func (s *SingleRoomWallDoorSuite) TestDoorStateOwnsBlockingEvenWhenTheWallFlagsAreFalse() {
	// The wall's independent blocker asserts NO blocking; the closed door
	// still seals the gap, because a door's state — not the wall's flags — is
	// what its rectangle closes.
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Gameplay.Walls[0].Blocker = wallBlocks(false, false, 4, 0.2, 0, 0)
	compiled := s.load(spec)

	enc := s.play(compiled)
	s.Require().ErrorIs(s.step(enc), encounter.ErrDoorShut)
	s.True(s.sightAcross(enc))

	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: encounter.DoorID("wall-doors-room/gap-door")})
	s.Require().NoError(err)
	s.Require().NoError(s.step(enc))
	s.False(s.sightAcross(enc))
}

func (s *SingleRoomWallDoorSuite) TestOpeningClearsOnlyThisContribution() {
	// An independent authored prop overlapping the door's rectangle survives
	// the compile and still blocks after the door is opened — so the open door
	// cleared only its OWN contribution.
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	boundary := boundaryWallX(2.5)
	spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Wall Doors Room
items:
  - {id: bench, transform: {x: `+fmtFloat(boundary)+`, z: 0, rotationY: 0}}
groups: []
`)
	spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{
		"bench": wallBlocks(true, true, 0.4, 0.4, 0, 0),
	}
	compiled := s.load(spec)

	bench, ok := placedByID(compiled.Field, "bench")
	s.Require().True(ok, "the independent blocker survives the door compile")
	s.True(bench.BlocksMovement, "with its own flags")
	_, ok = placedByID(compiled.Field, "gap-door")
	s.Require().True(ok, "and the bound door did not replace it")

	enc := s.play(compiled)
	s.Require().Error(s.step(enc), "the shut door and the bench both close it")

	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: encounter.DoorID("wall-doors-room/gap-door")})
	s.Require().NoError(err)

	err = s.step(enc)
	s.Require().ErrorIs(err, encounter.ErrBadPlacement,
		"the bench's own refusal remains — the door's contribution is gone")
	s.NotErrorIs(err, encounter.ErrDoorShut, "the door is no longer what refuses the crossing")
}

func (s *SingleRoomWallDoorSuite) TestSaveAndLoadRetainsBoundDoorStateAndGeometry() {
	s.Run("a closed door stays shut", func() {
		compiled := s.load(s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true}))
		enc := s.play(compiled)
		reloaded := s.reload(enc)
		s.Require().ErrorIs(s.step(reloaded), encounter.ErrDoorShut)
		s.True(s.sightAcross(reloaded))

		door, ok := doorByID(compiled.Field.Doors, "wall-doors-room/gap-door")
		s.Require().True(ok)
		s.Equal(encounter.DoorClosed, door.State.Kind())
	})

	s.Run("an opened door stays open", func() {
		compiled := s.load(s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true}))
		enc := s.play(compiled)
		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: encounter.DoorID("wall-doors-room/gap-door")})
		s.Require().NoError(err)

		reloaded := s.reload(enc)
		s.Require().NoError(s.step(reloaded), "the reloaded gap still offers the crossing")
		s.False(s.sightAcross(reloaded), "and the reloaded door still lets sight through")

		door, ok := doorReadByID(reloaded.Doors(), "wall-doors-room/gap-door")
		s.Require().True(ok)
		s.Equal(encounter.DoorOpen, door.State.Kind())
	})
}

func (s *SingleRoomWallDoorSuite) TestAtlasIdentityAndLiveDoorStateMatch() {
	compiled := s.load(s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true}))
	enc := s.play(compiled)

	// The observer's own read withholds nothing here (no concealment) and
	// reports the live door under the minted id, sharing the placed entry's
	// geometry.
	doors, err := enc.DoorsFor("walker")
	s.Require().NoError(err)
	s.True(doorsListed(doors, encounter.DoorID("wall-doors-room/gap-door")))

	placed, ok := placedByID(compiled.Field, "gap-door")
	s.Require().True(ok)
	var live *encounter.Door
	for i := range doors {
		if doors[i].ID == "wall-doors-room/gap-door" {
			live = &doors[i]
		}
	}
	s.Require().NotNil(live)
	s.Require().NotNil(live.Placement)
	s.Equal(placed.Placement, *live.Placement)
	s.Equal(encounter.DoorClosed, live.State.Kind())
}

// --- Concealment: the bound door is a placed id, classified as a door ---

func (s *SingleRoomWallDoorSuite) TestConcealmentNamesTheBoundDoorAndWithholdsIt() {
	spec := s.baseSpec()
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{
		"vault": {
			Checks: dungeonspec.CheckSpec{{Ability: "investigation", DC: 12}},
			Cells:  []dungeonspec.RoomCell{{Q: 4, R: 0}},
			Props:  []string{"long-wall-door"},
		},
	}
	compiled := s.load(spec)

	// The author wrote one placed id; the existing binding map classifies it
	// as a door and mints it the way every door is minted.
	s.Require().Len(compiled.Concealments, 1)
	s.Equal([]encounter.DoorID{boundDoorID}, compiled.Concealments[0].Doors)
	s.Equal([]string{"long-wall-door"}, compiled.Concealments[0].Props,
		"one author selection explicitly covers the door's placed representation too")

	enc := s.play(compiled,
		encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: axial(0, 0)},
		encounter.MemberInput{ID: "lurker", Kind: encounter.KindPlayer, Position: axial(4, 0)},
	)

	// An observer who has not found the secret does not get the door's
	// identity or state at all.
	blind, err := enc.DoorsFor("walker")
	s.Require().NoError(err)
	s.False(doorsListed(blind, encounter.DoorID(boundDoorID)),
		"the concealed bound door is withheld from an observer who has not found it")
	blindAtlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	for _, placement := range blindAtlas.Placed {
		s.NotEqual("long-wall-door", placement.ID, "the paired drawn identity is withheld explicitly")
	}

	// Presence pierces from frame one, by the existing engine semantics: the
	// occupant of the hidden cell knows the door too.
	aware, err := enc.DoorsFor("lurker")
	s.Require().NoError(err)
	s.True(doorsListed(aware, encounter.DoorID(boundDoorID)),
		"standing in the secret is knowing the door in it")
}

func (s *SingleRoomWallDoorSuite) TestAConcealmentNamingAWallExpandsToPresenceAndSpansWithoutTheDoor() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{
		"vault": {
			Checks: dungeonspec.CheckSpec{{Ability: "investigation", DC: 12}},
			Cells:  []dungeonspec.RoomCell{{Q: 4, R: 0}},
			Props:  []string{"boundary"},
		},
	}
	compiled := s.load(spec)

	// An authored wall id expands to its raw presence entry AND every span
	// the SAME lowering produced; the attached door's independent id is NOT
	// swept in.
	s.Require().Len(compiled.Concealments, 1)
	s.Equal([]string{"boundary", "wall/boundary/span/0", "wall/boundary/span/1"},
		compiled.Concealments[0].Props)
	s.Empty(compiled.Concealments[0].Doors, "the attached door is selected explicitly, never swept in")

	// The unaware observer loses the wall's presence and both spans, keeps the
	// unlisted attached door, and loses only the concealed floor.
	enc := s.play(compiled)
	blind, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	visible := map[string]bool{}
	for _, p := range blind.Placed {
		visible[p.ID] = true
	}
	s.False(visible["boundary"], "the wall's presence is withheld")
	s.False(visible["wall/boundary/span/0"], "and its first span")
	s.False(visible["wall/boundary/span/1"], "and its second span")
	s.True(visible["gap-door"], "the unlisted attached door stays")

	doors, err := enc.DoorsFor("walker")
	s.Require().NoError(err)
	s.True(doorsListed(doors, encounter.DoorID("wall-doors-room/gap-door")),
		"and its live state too")
	s.NotContains(blind.Cells, axial(4, 0), "the concealed floor is withheld")
}

func (s *SingleRoomWallDoorSuite) TestAGeneratedSpanIDCollidingWithABoundDoorRefuses() {
	// A wall's generated span is derived from the wall id; a bound door whose
	// raw id is that same string would give two contributors one name.
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{
		roomWall("a", 0, 3, 10, 3, wallBlocks(true, true, 4, 0.2, 0, 0),
			dungeonspec.RoomWallOpening{
				ID: "gap", Position: 4, Width: 2,
				Door: &dungeonspec.RoomWallDoor{ID: "wall/a/span/0", AssetRef: "dnd5e:env:test:door"},
			}),
	}
	spec.Room.Gameplay.DoorBindings = map[string]dungeonspec.RoomDoorBinding{"wall/a/span/0": {Closed: true}}
	_, err := dungeonspec.Load(s.encoded(spec))
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.walls[0]")
	s.Contains(err.Error(), "collides")
}

func (s *SingleRoomWallDoorSuite) TestAStaleConcealmentReferenceRefusesByName() {
	// Removing an attachment leaves any concealment that named its door id
	// referring to nothing; that refuses by name rather than silently
	// retargeting.
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls[0].Openings = nil
	spec.Room.Gameplay.DoorBindings = nil
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{
		"vault": {
			Checks: dungeonspec.CheckSpec{{Ability: "investigation", DC: 12}},
			Props:  []string{"long-wall-door"},
		},
	}
	errs := s.defects(s.encoded(spec))
	s.Require().Len(errs, 1)
	s.Equal("concealments.vault.props[0]", errs[0].Path)
	s.Contains(errs[0].Message, "declare it under propDeclarations")
}

func (s *SingleRoomWallDoorSuite) TestAPropBindingOnABoundDoorRetainsItsRefusal() {
	// A bound door is a door, so a prop behavior binding naming it earns the
	// existing refusal a standalone door gets.
	spec := s.baseSpec()
	spec.Room.Gameplay.PropBindings = map[string]dungeonspec.RoomPropBinding{
		"long-wall-door": {Holdable: true},
	}
	errs := s.defects(s.encoded(spec))
	s.Require().Len(errs, 1)
	s.Equal("room.room.propBindings.long-wall-door", errs[0].Path)
	s.Contains(errs[0].Message, "door")
}

// --- Determinism and source immutability ---

func (s *SingleRoomWallDoorSuite) TestCompileIsDeterministicAndLeavesTheSource() {
	spec := s.baseSpec()
	before := s.encoded(spec)

	first := s.load(spec)
	second := s.load(spec)

	s.Equal(string(before), string(s.encoded(spec)), "the decoded spec is untouched by the compile")
	s.Equal(first.Field.Placed, second.Field.Placed, "one id order for the placed list")
	s.Equal(first.Field.Doors, second.Field.Doors, "one id order for the doors")
}
