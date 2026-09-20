// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// single_room_doors_test.go is WHAT A DOOR MAY SAY IN THE SINGLE-ROOM DIALECT
// (rpg-project#485; rpg-toolkit#1850), in dialect_test.go's shape: every case
// is the shipping v4 site with ONE thing changed, and every refusal is checked
// for the PATH it points at and the WORDS an author reads there — because the
// World Builder draws each one on the field it names.
//
// The sentences are spelled out here rather than compared to the constants
// that produce them: a test that asserts a constant equals itself proves that
// the constant exists, which is not the claim.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SingleRoomDoorSuite struct {
	suite.Suite
	site string
}

func TestSingleRoomDoorSuite(t *testing.T) {
	suite.Run(t, new(SingleRoomDoorSuite))
}

func (s *SingleRoomDoorSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/world-builder-v4-site.yaml")
	s.Require().NoError(err)
	s.site = string(raw)
}

// theDoorBinding and theDoorDeclaration are the two blocks most cases swap
// out, quoted exactly as the fixture writes them.
const (
	theDoorBinding     = "      cellar-door: {closed: true}\n"
	theDoorDeclaration = "      cellar-door:\n" +
		"        blocksMovement: false\n" +
		"        blocksLineOfSight: false\n" +
		"        footprint: {width: 0.35, depth: 1.8, offsetX: 0, offsetZ: 0}\n"
	theDoorID = encounter.DoorID("front-room-site/cellar-door")
)

func (s *SingleRoomDoorSuite) siteWith(old, repl string) []byte {
	s.Require().Contains(s.site, old, "siteWith: anchor not present in the fixture")

	return []byte(strings.Replace(s.site, old, repl, 1))
}

// defects decodes the source and returns every defect it earned.
func (s *SingleRoomDoorSuite) defects(raw []byte) []dungeonspec.FieldError {
	_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().Error(err, "this source was expected to be refused")
	var verr *dungeonspec.ValidationError
	s.Require().ErrorAs(err, &verr)

	return verr.Errors
}

// TestTheShippingSiteCompilesItsDoor is the acceptance case: everything below
// is this file with one thing changed.
//
// The door reaches the field TWICE and that is the design, not a duplicate:
// as the placed prop the map draws and reports in its atlas, asserting NO
// blocking of its own, and as the door whose state decides what its rectangle
// closes. Both carry the SAME rectangle, because both come from the one
// authored footprint through the one adapter.
func (s *SingleRoomDoorSuite) TestTheShippingSiteCompilesItsDoor() {
	compiled, err := dungeonspec.Load([]byte(s.site))
	s.Require().NoError(err)

	s.Require().Len(compiled.Field.Doors, 1)
	door := compiled.Field.Doors[0]
	s.Equal(theDoorID, door.ID, "the id is v2's minting: <dungeon key>/<item id>")
	s.Equal(encounter.DoorClosed, door.State.Kind())
	s.Empty(door.Edges, "a single-room door stands in no crossing")
	s.Require().NotNil(door.Placement)

	var placed *encounter.PlacedPropInput
	for i := range compiled.Field.Placed {
		if compiled.Field.Placed[i].ID == "cellar-door" {
			placed = &compiled.Field.Placed[i]
		}
	}
	s.Require().NotNil(placed, "the door is still a placed prop, under the ITEM id the web joins by")
	s.False(placed.BlocksMovement, "and it asserts no blocking of its own — the state decides")
	s.False(placed.BlocksLineOfSight)
	s.Equal(placed.Placement.Origin, door.Placement.Origin, "one authored footprint, one adapter")
	s.Equal(placed.Placement.Footprint.Box.W, door.Placement.Footprint.Box.W)
	s.Equal(placed.Placement.Footprint.Box.D, door.Placement.Footprint.Box.D)
}

// TestTheClosedLeafSealsTheGoblinOff is the fixture's own gameplay claim,
// through the engine: the goblin's cell has exactly one way in, the leaf
// stands across it, and opening the door is what offers it.
//
// THIS IS THE FIXTURE, NOT A CONSTRUCTED FIELD. footprintdoors_test.go proves
// the gate on geometry chosen to make every case reachable; this proves the
// authored door in the shipping file actually lands where its author put it —
// the two halves of "the picture is right" and "the picture means something".
func (s *SingleRoomDoorSuite) TestTheClosedLeafSealsTheGoblinOff() {
	compiled, err := dungeonspec.Load([]byte(s.site))
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: nothingIsEverFound{}, Witness: nobodyPerceivesAnything{},
		Field:   compiled.Field,
		Members: []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: axial(1, 0)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Step(&encounter.StepInput{Member: "walker", To: axial(2, 0)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrDoorShut)
	s.Contains(err.Error(), string(theDoorID))

	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: theDoorID})
	s.Require().NoError(err)

	_, err = enc.Step(&encounter.StepInput{Member: "walker", To: axial(2, 0)})
	s.Require().NoError(err, "the cell the shut leaf refused is the one they walk into")
}

// TestADoorNeedsAFootprint: the geometry is the prop declaration's, so a
// binding without one names a door that is nowhere.
func (s *SingleRoomDoorSuite) TestADoorNeedsAFootprint() {
	errs := s.defects(s.siteWith(theDoorDeclaration, ""))
	s.Require().Len(errs, 1)
	s.Equal(dungeonspec.FieldError{
		Path:    "room.room.doorBindings.cellar-door",
		Message: "a door needs a footprint: declare it under propDeclarations",
	}, errs[0])
}

// TestADoorInsideAnArrangementIsRefusedByName is R5: a stamped door has no
// remap yet, and an id that names an arrangement template is told so rather
// than being sent to declare a prop it cannot declare.
func (s *SingleRoomDoorSuite) TestADoorInsideAnArrangementIsRefusedByName() {
	withArrangement := s.siteWith("    arrangementDeclarations: {}\n",
		"    arrangementDeclarations:\n"+
			"      barricade:\n"+
			"        cellar-door:\n"+
			"          blocksMovement: false\n"+
			"          blocksLineOfSight: false\n"+
			"          footprint: {width: 0.35, depth: 1.8, offsetX: 0, offsetZ: 0}\n")
	errs := s.defects(swapOnce(s.T(), withArrangement, theDoorDeclaration, ""))

	s.Require().Len(errs, 1)
	s.Equal("room.room.doorBindings.cellar-door", errs[0].Path)
	s.Contains(errs[0].Message, "a door inside an arrangement is not something this build stamps yet")
	s.Contains(errs[0].Message, "declare the door on a placed item of its own")
}

// TestStateWinsOverTheBlockingFlags is R3 and rpg-toolkit#1846's collision:
// a flag set TRUE on a door item would be a wall that never opens, because
// nothing consults door state to clear one.
//
// FALSE IS NOT REFUSED, and that is the point of the pair below: the World
// Builder seeds every fresh declaration with `blocksMovement: false`, and "a
// fresh declaration asserts nothing" is exactly what a door wants said.
func (s *SingleRoomDoorSuite) TestStateWinsOverTheBlockingFlags() {
	for _, flag := range []string{"blocksMovement", "blocksLineOfSight"} {
		s.Run(flag, func() {
			raw := s.siteWith(theDoorDeclaration,
				strings.Replace(theDoorDeclaration, flag+": false", flag+": true", 1))
			errs := s.defects(raw)
			s.Require().Len(errs, 1)
			s.Equal("room.room.propDeclarations.cellar-door."+flag, errs[0].Path,
				"drawn on the flag itself, not on the binding")
			s.Contains(errs[0].Message, "a door's state decides what it blocks")
			s.Contains(errs[0].Message, "say `closed` under doorBindings")
		})
	}

	// And the shipped seeding — both flags false — is legal.
	_, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: []byte(s.site)})
	s.Require().NoError(err)
}

// TestConcealedIsRefusedInThisDialect is R2: the word means something, it is
// simply not built here, so it is refused as itself at its own path rather
// than as a key nobody has heard of.
func (s *SingleRoomDoorSuite) TestConcealedIsRefusedInThisDialect() {
	errs := s.defects(s.siteWith(theDoorBinding,
		"      cellar-door: {closed: true, concealed: [{ability: perception, dc: 15}]}\n"))

	s.Require().Len(errs, 1)
	s.Equal("room.room.doorBindings.cellar-door.concealed", errs[0].Path)
	s.True(strings.HasPrefix(errs[0].Message, "concealed is not something a single room can declare yet:"),
		"the `at:` refusal's own shape: what is wrong, then what to write instead")
}

// TestTheLockIsTheOneCheckGrammar: `locked` is v2's [DoorSpec.Locked] — the
// same nil-vs-empty law and the same sentences, at this dialect's paths.
func (s *SingleRoomDoorSuite) TestTheLockIsTheOneCheckGrammar() {
	s.Run("a lock with no way through", func() {
		errs := s.defects(s.siteWith(theDoorBinding, "      cellar-door: {closed: true, locked: []}\n"))
		s.Require().Len(errs, 1)
		s.Equal(dungeonspec.FieldError{
			Path:    "room.room.doorBindings.cellar-door.locked",
			Message: "this locked door needs at least one way through it — an ability and a DC",
		}, errs[0], "the sentence a v2 door earns, at a single room's path")
	})

	s.Run("a route with nothing to beat", func() {
		errs := s.defects(s.siteWith(theDoorBinding,
			"      cellar-door: {closed: true, locked: [{ability: str, dc: 0}]}\n"))
		s.Require().Len(errs, 1)
		s.Equal("room.room.doorBindings.cellar-door.locked[0].dc", errs[0].Path)
		s.Contains(errs[0].Message, "has nothing to beat")
	})

	s.Run("a route with no ability", func() {
		errs := s.defects(s.siteWith(theDoorBinding,
			"      cellar-door: {closed: true, locked: [{dc: 15}]}\n"))
		s.Require().Len(errs, 1)
		s.Equal("room.room.doorBindings.cellar-door.locked[0].ability", errs[0].Path)
	})

	s.Run("a lock compiles to a locked door", func() {
		compiled, err := dungeonspec.Load(s.siteWith(theDoorBinding,
			"      cellar-door: {closed: true, locked: [{ability: str, dc: 15}]}\n"))
		s.Require().NoError(err)
		s.Require().Len(compiled.Field.Doors, 1)
		s.Equal(encounter.DoorLocked, compiled.Field.Doors[0].State.Kind())
		lock, locked := compiled.Field.Doors[0].State.Lock()
		s.Require().True(locked)
		s.Equal([]encounter.CheckApproach{{Ability: "str", DC: 15}}, lock.Approaches)
	})
}

// TestAnAbsentClosedIsAnOpenDoorway: `closed` is a STATE and absence is one
// of its values — [DoorSpec.Closed]'s own rule, not a missing answer the way
// a declaration's two flags would be.
func (s *SingleRoomDoorSuite) TestAnAbsentClosedIsAnOpenDoorway() {
	compiled, err := dungeonspec.Load(s.siteWith(theDoorBinding, "      cellar-door: {}\n"))
	s.Require().NoError(err)
	s.Require().Len(compiled.Field.Doors, 1)
	s.Equal(encounter.DoorOpen, compiled.Field.Doors[0].State.Kind())
}

// TestANullBindingIsRefused: absence means "this item is no door" and null
// means "the author deleted the state", and reading both as the zero value
// would lose that — the distinction every optional key in this document keeps.
func (s *SingleRoomDoorSuite) TestANullBindingIsRefused() {
	errs := s.defects(s.siteWith(theDoorBinding, "      cellar-door:\n"))
	s.Require().Len(errs, 1)
	s.Equal("room.room.doorBindings.cellar-door", errs[0].Path)
	s.Equal("must not be null", errs[0].Message)
}

// TestAnUnknownDoorKeyIsNamedAtItsPath is rpg-project#481 R2 reaching the new
// block: a typo inside a binding is refused where it sits, listing the keys a
// door state does take.
func (s *SingleRoomDoorSuite) TestAnUnknownDoorKeyIsNamedAtItsPath() {
	errs := s.defects(s.siteWith(theDoorBinding, "      cellar-door: {closed: true, shut: true}\n"))
	s.Require().Len(errs, 1)
	s.Equal(dungeonspec.FieldError{
		Path:    "room.room.doorBindings.cellar-door.shut",
		Message: `"shut" is not a key this build reads: they are closed, concealed, locked`,
	}, errs[0])
}

// TestABindingNamingNothingLiveIsRefusedByThePropPath: a door must have a
// prop declaration, and a declaration may not outlive the thing it names — so
// the dangling binding is refused by the sentence that already existed for
// exactly this mistake, once, at the declaration's own path.
func (s *SingleRoomDoorSuite) TestABindingNamingNothingLiveIsRefusedByThePropPath() {
	errs := s.defects(s.siteWith("      - id: cellar-door\n", "      - id: cellar-door-renamed\n"))

	s.Require().Len(errs, 1, "one mistake, one defect — not one per declaration kind")
	s.Equal(dungeonspec.FieldError{
		Path:    "room.room.propDeclarations.cellar-door",
		Message: "must name a live scene prop",
	}, errs[0])
}

// swapOnce replaces exactly one occurrence, failing the test when the anchor
// is not there — a silently unmutated fixture is a case that proves nothing.
func swapOnce(t require.TestingT, hay []byte, old, repl string) []byte {
	require.Contains(t, string(hay), old, "swapOnce: anchor not present")

	return []byte(strings.Replace(string(hay), old, repl, 1))
}

// axial is the fixture's authored axial cell in the frame the engine speaks —
// the same conversion [dungeonspec.CompileSingleRoom] runs.
func axial(q, r int) spatial.Position {
	cube := spatial.CubeCoordinate{X: q, Y: -q - r, Z: r}

	return cube.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
}
