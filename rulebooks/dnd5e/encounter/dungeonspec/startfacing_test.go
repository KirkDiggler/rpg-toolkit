// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// startfacing_test.go is the start point's optional direction
// (rpg-project#374). Kirk, walking a dungeon: "we always start looking the
// wrong way."
//
// The whole feature is one authored word, and the tests are about the two
// things a new authored word always has to prove: that the OLD spelling stays
// legal and means what it always meant, and that a word outside the closed
// vocabulary is refused by name rather than snapped to the nearest.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type StartFacingSuite struct {
	suite.Suite
	tomb string
}

func TestStartFacingSuite(t *testing.T) { suite.Run(t, new(StartFacingSuite)) }

func (s *StartFacingSuite) SetupTest() { s.tomb = tombYAML(s.T()) }

// withStart is the shipping tomb with its start line replaced, which is the
// ONE thing every case here changes.
func (s *StartFacingSuite) withStart(line string) string {
	s.Require().Contains(s.tomb, "start: [1, 3]\n", "the tomb spells its start as a bare pair")
	return strings.Replace(s.tomb, "start: [1, 3]\n", line, 1)
}

// atlasOf builds the encounter a compiled dungeon describes and returns the
// map a client would draw — golden_test.go's own compiledAtlas, given a spec
// that came from a string rather than from a file.
func (s *StartFacingSuite) atlasOf(compiled dungeonspec.Compiled) encounter.Atlas {
	s.T().Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		CheckResolver: nothingIsEverFound{}, Witness: nobodyPerceivesAnything{},
		Field:   compiled.Field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	return atlas
}

func (s *StartFacingSuite) decode(raw string) (*dungeonspec.Spec, []dungeonspec.FieldError) {
	spec, err := dungeonspec.Decode([]byte(raw))
	if err != nil {
		var verr *dungeonspec.ValidationError
		if s.ErrorAs(err, &verr) {
			return nil, verr.Errors
		}
		s.Require().NoError(err)
	}
	return spec, dungeonspec.Validate(spec)
}

// TestBothSpellingsMeanTheSameCell is the compatibility claim, and it is the
// reason the bare pair is read by hand rather than migrated: a dungeon
// authored before facings existed is not a dungeon facing north.
func (s *StartFacingSuite) TestBothSpellingsMeanTheSameCell() {
	bare, errs := s.decode(s.withStart("start: [1, 3]\n"))
	s.Require().Empty(errs)
	s.Require().NotNil(bare.Start)
	s.Equal([2]int{1, 3}, bare.Start.At)
	s.Empty(bare.Start.Facing, "silence is not a direction — the author said nothing")

	object, errs := s.decode(s.withStart("start: { at: [1, 3] }\n"))
	s.Require().Empty(errs)
	s.Equal(*bare.Start, *object.Start,
		"the object spelling without a facing IS the bare pair, field for field")

	faced, errs := s.decode(s.withStart("start: { at: [1, 3], facing: e }\n"))
	s.Require().Empty(errs)
	s.Equal([2]int{1, 3}, faced.Start.At, "the cell is untouched by saying which way to look")
	s.Equal("e", faced.Start.Facing)
}

// TestTheFacingReachesTheCompiledDungeonAndTheAtlas walks the word all the way
// through: the authored file, the compiled output a host reads, and the map a
// client draws.
func (s *StartFacingSuite) TestTheFacingReachesTheCompiledDungeonAndTheAtlas() {
	compiled, err := dungeonspec.Load([]byte(s.withStart("start: { at: [1, 3], facing: se }\n")))
	s.Require().NoError(err)

	s.Equal("se", compiled.StartFacing, "the convenience beside the seats")
	s.Require().NotNil(compiled.Field.Start, "and the copy that survives being stored")
	s.Equal("se", compiled.Field.Start.Facing)

	atlas := s.atlasOf(compiled)
	s.Require().NotNil(atlas.Start, "the map says where the dungeon begins")
	s.Equal("se", atlas.Start.Facing)
	// The atlas speaks DUNGEON-ABSOLUTE cells and a seat speaks the AUTHORED
	// offset pair, so the expectation is computed through the field's own
	// conversion rather than echoed back from whichever the code produced.
	s.Equal(encounter.HexCellAt(compiled.Field.Canvas.Orientation, 1, 3), atlas.Start.At,
		"the authored cell, converted exactly once")
	s.Equal(spatialOf(compiled.PartyStart[0].At), atlas.Start.At,
		"and it is where the party's best seat is — one authored fact, two derivations")
}

// TestAStartWithNoFacingCarriesNoFacing is the zero-value claim, asked of the
// whole chain rather than of the decoder alone. A dungeon that says nothing
// must arrive at the client saying nothing, so the camera opens however it
// opened before.
func (s *StartFacingSuite) TestAStartWithNoFacingCarriesNoFacing() {
	compiled, err := dungeonspec.Load([]byte(s.withStart("start: [1, 3]\n")))
	s.Require().NoError(err)
	s.Empty(compiled.StartFacing)
	s.Require().NotNil(compiled.Field.Start)
	s.Empty(compiled.Field.Start.Facing)

	atlas := s.atlasOf(compiled)
	s.Require().NotNil(atlas.Start, "the cell is still authored — only the direction was not")
	s.Empty(atlas.Start.Facing)
}

// TestTheStartRefusesByName is the vocabulary rule and the shape rule, both of
// which this dialect applies to every authored word.
func (s *StartFacingSuite) TestTheStartRefusesByName() {
	s.Run("a direction that does not exist", func() {
		_, errs := s.decode(s.withStart("start: { at: [1, 3], facing: northeast }\n"))
		s.Require().Contains(paths(errs), "start.facing")
		for _, e := range errs {
			if e.Path == "start.facing" {
				s.Contains(e.Message, "northeast", "the refusal quotes the word the author wrote")
				s.Contains(e.Message, "n|ne|e|se|s|sw|w|nw", "and lists the eight that exist")
			}
		}
	})

	s.Run("a direction that is merely the wrong case", func() {
		// Snapping "E" to "e" would be the convenience this dialect refuses
		// everywhere else: a vocabulary with a helpful exception is a
		// vocabulary nobody can predict.
		_, errs := s.decode(s.withStart("start: { at: [1, 3], facing: E }\n"))
		s.Require().Contains(paths(errs), "start.facing")
	})

	s.Run("a key nobody defined", func() {
		_, errs := s.decode(s.withStart("start: { at: [1, 3], face: e }\n"))
		s.Require().NotEmpty(errs, "a typo must not be silently dropped")
		s.Contains(errs[0].Message, "face",
			"named, because a custom unmarshaler bypasses the decoder's own strictness")
	})

	s.Run("an object that never says where", func() {
		_, errs := s.decode(s.withStart("start: { facing: e }\n"))
		s.Require().NotEmpty(errs)
		s.Contains(errs[0].Message, "which cell")
	})

	s.Run("a start that is neither a pair nor an object", func() {
		_, errs := s.decode(s.withStart("start: yonder\n"))
		s.Require().NotEmpty(errs)
		s.Contains(errs[0].Message, "[col,row]")
	})
}

// TestAFacedStartIsStillRefusedOffTheFloor pins that the new word did not
// weaken the old checks: where the party stands is still a geometry question,
// and saying which way they look does not excuse standing in the void.
func (s *StartFacingSuite) TestAFacedStartIsStillRefusedOffTheFloor() {
	_, errs := s.decode(s.withStart("start: { at: [99, 99], facing: e }\n"))
	require.Contains(s.T(), paths(errs), "start")
}

// spatialOf converts a seat's AUTHORED offset pair into the dungeon-absolute
// cell the atlas speaks — the field's own conversion, applied once, so the
// two frames are compared rather than confused.
func spatialOf(authored spatial.Position) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), int(authored.X), int(authored.Y))
}
