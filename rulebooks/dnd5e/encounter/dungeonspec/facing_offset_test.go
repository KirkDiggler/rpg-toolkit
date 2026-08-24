// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// facing_offset_test.go is THE ORIENTATION-AWARE FACING VOCABULARY and the
// visual-only OFFSET (rpg-project#261). Both are optional presentational
// facts a prop may author, carried through Compile and the Atlas
// uninterpreted — this package only checks that the WORD is one the
// dungeon's own orientation actually has, and that the NUDGE is a two-number
// list within [-0.5,0.5]. Neither ever becomes an angle here: that is a
// render concern (ideas/dungeon-builder/prop-facing-offset.md, ADR-0040's
// spirit). dialect_test.go's table covers the pointy-orientation refusals
// (a flat-only name, an out-of-range component, a monster that authors
// either); this file covers what that table's single fixture cannot: the
// OTHER orientation's vocabulary, and the fixture the design asked for.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const facedPropsPath = "testdata/tomb-faced-props.yaml"

type FacingOffsetSuite struct {
	suite.Suite
}

func TestFacingOffsetSuite(t *testing.T) {
	suite.Run(t, new(FacingOffsetSuite))
}

// flatRoom is a minimal flat-top dungeon with one prop authoring the given
// facing, so the OTHER six-name vocabulary (flat shows n|s|ne|nw|se|sw,
// pointy shows e|w|ne|nw|se|sw) has a fixture of its own — the reference
// tomb is pointy, and a wall drawn for one layout is not adjacent under the
// other (dialect_test.go's TestTheSameWallIsARefusalUnderTheOtherLayout), so
// an orientation case needs its own file rather than a tombWith substitution.
func flatRoom(facing string) string {
	return `
version: 2
key: flat-room
orientation: flat
void: opaque
regions:
  - id: room
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0],[2,0]]
      - [[0,1],[1,1],[2,1]]
start: [0, 0]
place:
  - { ref: "dnd5e:props:pillar", at: [1,1], blocks_movement: true, blocks_los: true, facing: ` + facing + ` }
`
}

// TestFacingVocabularyIsOrientationAware — the SIX names each layout has are
// different sets, and a name from the wrong one is an ERROR, never a silent
// snap to the nearest valid one (the design doc's ruling). Pointy's own
// wrong-set refusal is dialect_test.go's "a facing from the wrong
// orientation's set" row; this is flat's half, plus both orientations'
// positive cases in one place.
func (s *FacingOffsetSuite) TestFacingVocabularyIsOrientationAware() {
	s.Run("flat-top accepts its own six", func() {
		for _, f := range []string{"n", "s", "ne", "nw", "se", "sw"} {
			spec, err := dungeonspec.Decode([]byte(flatRoom(f)))
			s.Require().NoError(err)
			s.Empty(dungeonspec.Validate(spec), "facing %q is valid under flat-top", f)
		}
	})
	s.Run("flat-top refuses pointy's e and w", func() {
		for _, f := range []string{"e", "w"} {
			spec, err := dungeonspec.Decode([]byte(flatRoom(f)))
			s.Require().NoError(err)
			errs := dungeonspec.Validate(spec)
			s.Require().Len(errs, 1)
			s.Equal("place[0].facing", errs[0].Path)
			s.Contains(errs[0].Message, "flat-top has no facing")
		}
	})
}

// TestFacedPropsFixture_RoundTripsThroughAtlas is the fixture the design
// asked for: the reference tomb with a few props authoring a facing, an
// offset, or both — Load -> Compile -> Atlas with the authored words and
// values intact at every stage, and every OTHER prop still carrying neither
// (the additive fields change nothing for a prop that says nothing).
func (s *FacingOffsetSuite) TestFacedPropsFixture_RoundTripsThroughAtlas() {
	compiled, atlas := compiledAtlas(s.T(), facedPropsPath)

	byAt := map[spatial.Position]encounter.PropInput{}
	for _, p := range compiled.Field.Props {
		byAt[p.At] = p
	}

	facingOnly := byAt[spatial.Position{X: 1, Y: 1}]
	s.Equal("e", facingOnly.Facing, "facing only")
	s.Equal([2]float64{0, 0}, facingOnly.Offset, "no offset authored")

	offsetOnly := byAt[spatial.Position{X: 10, Y: 1}]
	s.Equal("", offsetOnly.Facing, "offset only")
	s.Equal([2]float64{0.3, 0.3}, offsetOnly.Offset)

	both := byAt[spatial.Position{X: 17, Y: 1}]
	s.Equal("se", both.Facing, "both, at once")
	s.Equal([2]float64{0.2, -0.1}, both.Offset)

	neither := byAt[spatial.Position{X: 8, Y: 2}]
	s.Equal("", neither.Facing, "a prop that says nothing carries nothing")
	s.Equal([2]float64{0, 0}, neither.Offset)

	// The Atlas — built by running the compiled field through a live
	// Encounter — carries the same words and numbers, at the absolute axial
	// cells the map reports rather than the authored offset ones.
	atlasByAt := map[spatial.Position]encounter.AtlasProp{}
	for _, p := range atlas.Props {
		atlasByAt[p.At] = p
	}
	o := encounter.HexesArePointyTop()
	s.Equal("e", atlasByAt[encounter.HexCellAt(o, 1, 1)].Facing, "the atlas carries the same word")
	s.Equal([2]float64{0.3, 0.3}, atlasByAt[encounter.HexCellAt(o, 10, 1)].Offset)
	s.Equal("se", atlasByAt[encounter.HexCellAt(o, 17, 1)].Facing)
	s.Equal([2]float64{0.2, -0.1}, atlasByAt[encounter.HexCellAt(o, 17, 1)].Offset, "and the same numbers")
	s.Equal("", atlasByAt[encounter.HexCellAt(o, 8, 2)].Facing, "and still nothing where nothing was authored")
}
