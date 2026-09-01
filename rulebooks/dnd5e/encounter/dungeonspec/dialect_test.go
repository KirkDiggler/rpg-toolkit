// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// dialect_test.go is WHAT THE FILE MAY SAY (rpg-project#256, design §2).
//
// Every case below is the shipping tomb with one thing changed, so each
// refusal differs from a VALID file by exactly the thing it is about — and
// each refusal is checked for WHERE it points, because the builder draws every
// FieldError on the canvas at the path it names.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

type DialectSuite struct {
	suite.Suite
	tomb string
}

func TestDialectSuite(t *testing.T) {
	suite.Run(t, new(DialectSuite))
}

func (s *DialectSuite) SetupTest() { s.tomb = tombYAML(s.T()) }

func (s *DialectSuite) tombWith(old, new string) string {
	s.Require().Contains(s.tomb, old, "tombWith: anchor not present in the tomb")
	return strings.Replace(s.tomb, old, new, 1)
}

// validate decodes and validates, returning every defect's path.
func (s *DialectSuite) validate(raw string) []dungeonspec.FieldError {
	spec, err := dungeonspec.Decode([]byte(raw))
	if err != nil {
		var verr *dungeonspec.ValidationError
		s.Require().ErrorAs(err, &verr)
		return verr.Errors
	}
	return dungeonspec.Validate(spec)
}

func paths(errs []dungeonspec.FieldError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Path)
	}
	return out
}

// TestTheShippingTombDecodes is the acceptance case: everything below is about
// a file that differs from this one by one thing.
func (s *DialectSuite) TestTheShippingTombDecodes() {
	spec, err := dungeonspec.Decode([]byte(s.tomb))
	s.Require().NoError(err)
	s.Empty(dungeonspec.Validate(spec))

	s.Equal(2, spec.Version)
	s.Equal("reference-tomb", spec.Key)
	s.Equal("The Reference Tomb", spec.Name)
	s.Equal("pointy", spec.Orientation)
	s.Equal("opaque", spec.Void)
	s.Require().NotNil(spec.Start)
	s.Equal([2]int{1, 3}, *spec.Start)
	s.Len(spec.Regions, 3)
	s.Len(spec.Regions[0].Cells, 8, "eight rows")
	s.Len(spec.Regions[0].Cells[0], 6, "six cells each")
	s.Len(spec.Walls, 28)
	s.Len(spec.Doors, 2)
	s.Len(spec.Place, 18)
}

// TestDecode_RefusesVersion1 — the deleted dialect is refused by name, not
// parsed hopefully. A version-1 file fails twice over: its keys are unknown,
// and its version is not this one. Both land on the author.
func (s *DialectSuite) TestDecode_RefusesVersion1() {
	v1 := `
version: 1
key: reference-tomb
void: opaque
orientation: pointy
height: 8
start: [1, 3]
rooms:
  - id: entrance
    width: 6
connectors: []
`
	errs := s.validate(v1)
	s.Require().NotEmpty(errs)
	joined := strings.Join(paths(errs), " ")
	s.Contains(joined, "line", "unknown keys are refused at the line the decoder found them")
	var messages []string
	for _, e := range errs {
		messages = append(messages, e.Message)
	}
	all := strings.Join(messages, "\n")
	s.Contains(all, "height")
	s.Contains(all, "rooms")
	s.Contains(all, "connectors")

	// And with only the version wrong, it is the version that is named.
	errs = s.validate(s.tombWith("version: 2", "version: 1"))
	s.Require().Len(errs, 1)
	s.Equal("version", errs[0].Path)
	s.Contains(errs[0].Message, "version 1")
	s.Contains(errs[0].Message, "speaks 2")
}

// TestADungeonMustBeOneDocumentOfKnownKeys — the decoder's own refusals.
func (s *DialectSuite) TestADungeonMustBeOneDocumentOfKnownKeys() {
	s.Run("empty", func() {
		errs := s.validate("")
		s.Require().Len(errs, 1)
		s.Contains(errs[0].Message, "empty")
	})
	s.Run("two documents", func() {
		errs := s.validate(s.tomb + "\n---\n" + s.tomb)
		s.Require().Len(errs, 1)
		s.Contains(errs[0].Message, "one document")
	})
	s.Run("an unknown key", func() {
		errs := s.validate(s.tombWith("void: opaque", "void: opaque\nheight: 8"))
		s.Require().Len(errs, 1)
		s.Contains(errs[0].Message, "height")
		s.Contains(errs[0].Path, "line")
	})
}

// TestValidate_PathsNameTheThing — the table: every refusal names the YAML
// path of the thing that is wrong, because that is where the builder draws it.
// TestWallObjectFormIsStrict — the wall object form keeps Decode's own
// strictness even though a custom unmarshaler bypasses KnownFields: an
// unknown key and an edgeless object are refusals naming the line, not facts
// silently dropped.
func (s *DialectSuite) TestWallObjectFormIsStrict() {
	_, err := dungeonspec.Decode([]byte(s.tombWith("  - [[5,1],[6,0]]", "  - { between: [[5,1],[6,0]], hieght: 2 }")))
	s.Require().Error(err, "a typo'd key is refused, not dropped")
	s.Contains(err.Error(), "hieght")

	_, err = dungeonspec.Decode([]byte(s.tombWith("  - [[5,1],[6,0]]", "  - { height: 2 }")))
	s.Require().Error(err, "a wall object with no edge is not a wall")
	s.Contains(err.Error(), "between")
}

func (s *DialectSuite) TestValidate_PathsNameTheThing() {
	for _, tc := range []struct {
		name string
		old  string
		new  string
		path string
		says string
	}{
		{"a cell in two regions", "      - [[6,0],[7,0],[8,0],[9,0],[10,0],[11,0],[12,0],[13,0],[14,0],[15,0]]",
			"      - [[6,0],[7,0],[8,0],[5,0],[10,0],[11,0],[12,0],[13,0],[14,0],[15,0]]",
			"regions[1].cells[0][3]", `already painted in region "entrance"`},
		{"a cell painted twice in one region", "      - [[6,0],[7,0],[8,0],[9,0],[10,0],[11,0],[12,0],[13,0],[14,0],[15,0]]",
			"      - [[6,0],[7,0],[8,0],[6,0],[10,0],[11,0],[12,0],[13,0],[14,0],[15,0]]",
			"regions[1].cells[0][3]", "painted twice"},
		{"a wall between cells that do not touch", "  - [[5,3],[6,4]]", "  - [[5,3],[6,5]]",
			"walls[7]", "not adjacent under pointy"},
		{"a wall off the floor", "  - [[5,0],[6,0]]", "  - [[5,0],[5,-1]]",
			"walls[0]", "not floor"},
		{"a wall listed twice", "  - [[5,1],[6,0]]", "  - [[6,0],[5,0]]",
			"walls[1]", "already listed at walls[0]"},
		{"a door edge that is also a wall", "edges: [[[5,4],[6,4]]]", "edges: [[[5,3],[6,3]]]",
			"doors[0].edges[0]", "also a wall (walls[6])"},
		{"a prop without blocks_los", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true }`,
			"place[0].blocks_los", "there is no default"},
		{"a monster that says what it blocks", `at: [11,3], targeting: lowest-health }`, `at: [11,3], targeting: lowest-health, blocks_los: true }`,
			"place[8].blocks_los", "not a prop"},
		{"a facing that is not a compass word", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, facing: north }`,
			"place[0].facing", "not a compass direction"},
		{"an offset height above the ceiling", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, offset: [0.1, 0, 3.5] }`,
			"place[0].offset", "outside [0,3]"},
		{"an offset height below the floor", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, offset: [0.1, 0, -0.2] }`,
			"place[0].offset", "outside [0,3]"},
		{"an offset of four numbers", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, offset: [0.1, 0, 0, 0] }`,
			"place[0].offset", "must be [x,y] or [x,y,height]"},
		{"a wall height below standard", "  - [[5,1],[6,0]]", "  - { between: [[5,1],[6,0]], height: 0.5 }",
			"walls[1].height", "outside [1,3]"},
		{"a wall height above the ceiling", "  - [[5,1],[6,0]]", "  - { between: [[5,1],[6,0]], height: 3.5 }",
			"walls[1].height", "outside [1,3]"},
		{"an offset component out of range", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, offset: [0.6, 0] }`,
			"place[0].offset", "outside [-0.5,0.5]"},
		{"an offset that is not two numbers", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, offset: [0.1] }`,
			"place[0].offset", "must be [x,y]"},
		{"a monster with an authored facing", `at: [11,3], targeting: lowest-health }`, `at: [11,3], targeting: lowest-health, facing: n }`,
			"place[8].facing", "cannot declare an authored facing"},
		{"a monster with an authored offset", `at: [11,3], targeting: lowest-health }`, `at: [11,3], targeting: lowest-health, offset: [0.1, 0.1] }`,
			"place[8].offset", "cannot declare an authored offset"},
		{"a prop with an explicitly empty offset list", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, offset: [] }`,
			"place[0].offset", "must be [x,y]"},
		{"a monster with an explicitly empty offset list", `at: [11,3], targeting: lowest-health }`, `at: [11,3], targeting: lowest-health, offset: [] }`,
			"place[8].offset", "cannot declare an authored offset"},
		{"start on void", "start: [1, 3]", "start: [30, 3]", "start", "not floor"},
		{"start on a prop", "start: [1, 3]", "start: [1, 1]", "start", "already stands"},
		{"start missing", "start: [1, 3]\n", "", "start", "does not say"},
		{"an intensity of 1.2", "lighting: { intensity: 0.6 }", "lighting: { intensity: 1.2 }",
			"regions[0].lighting.intensity", "outside [0,1]"},
		{"a lighting block with no intensity", "lighting: { intensity: 0.6 }", "lighting: {}",
			"regions[0].lighting.intensity", "does not say"},
		{"no lighting at all", "    lighting: { intensity: 0.6 }\n", "", "regions[0].lighting", "no default"},
		{"a missing archetype", "    archetype: crypt\n    lighting: { intensity: 0.6 }", "    lighting: { intensity: 0.6 }",
			"regions[0].archetype", "no archetype"},
		{"a key that is not a slug", "key: reference-tomb", "key: Reference Tomb", "key", "[a-z0-9-]"},
		{"an unknown void", "void: opaque", "void: fog", "void", "fog"},
		{"an unknown orientation", "orientation: pointy", "orientation: sideways", "orientation", "sideways"},
		{"a ref this compiler cannot place", `ref: "dnd5e:props:brazier", at: [1,1]`, `ref: "dnd5e:traps:pit", at: [1,1]`,
			"place[0].ref", "cannot place"},
		{"a prop on void", `ref: "dnd5e:props:brazier", at: [1,1]`, `ref: "dnd5e:props:brazier", at: [40,1]`,
			"place[0].at", "not floor"},
		{"two things on one cell", `ref: "dnd5e:props:brazier", at: [1,6]`, `ref: "dnd5e:props:brazier", at: [1,1]`,
			"place[1].at", "same cell"},
		{"a targeting word this build does not know", "targeting: lowest-health }", "targeting: lowest-helth }",
			"place[8].targeting", "lowest-helth"},
		{"a boss that is a prop", `at: [1,1], blocks_movement: true, blocks_los: false }`, `at: [1,1], blocks_movement: true, blocks_los: false, boss: true }`,
			"place[0].boss", "not a monster"},
		{"a lock approach nothing has to beat", "locked: [{ ability: dex, dc: 12 }]", "locked: [{ ability: dex, dc: 0 }]",
			"doors[1].locked[0].dc", "nothing to beat"},
		{"a lock approach with no ability", "locked: [{ ability: dex, dc: 12 }]", "locked: [{ dc: 12 }]",
			"doors[1].locked[0].ability", "ability"},
		{"a locked door with no approaches", "locked: [{ ability: dex, dc: 12 }]", "locked: []",
			"doors[1].locked", "at least one way through"},
		{"a concealed door with no find approach", "locked: [{ ability: dex, dc: 12 }]",
			"locked: [{ ability: dex, dc: 12 }]\n    concealed: []",
			"doors[1].concealed", "at least one way to find it"},
		{"a find approach with no ability", "locked: [{ ability: dex, dc: 12 }]",
			"locked: [{ ability: dex, dc: 12 }]\n    concealed: [{ dc: 15 }]",
			"doors[1].concealed[0].ability", "ability"},
		{"a find approach nothing has to beat", "locked: [{ ability: dex, dc: 12 }]",
			"locked: [{ ability: dex, dc: 12 }]\n    concealed: [{ ability: perception, dc: 0 }]",
			"doors[1].concealed[0].dc", "nothing to beat"},
		{"a door with no edges", "edges: [[[5,4],[6,4]]]", "edges: []", "doors[0].edges", "no edges"},
		{"a door with no id", "  - id: entrance-hall\n", "  - id: \"\"\n", "doors[0].id", "no id"},
	} {
		s.Run(tc.name, func() {
			errs := s.validate(s.tombWith(tc.old, tc.new))
			s.Require().NotEmpty(errs, "the defect must be found")
			s.Equal([]string{tc.path}, paths(errs), "exactly one defect, at the thing that is wrong")
			s.Contains(errs[0].Message, tc.says)
		})
	}

	s.Run("an empty region", func() {
		empty := s.tomb
		start := strings.Index(empty, "    cells:\n      - [[0,0]")
		end := strings.Index(empty, "  - id: hall")
		empty = empty[:start] + "    cells: []\n" + empty[end:]
		errs := s.validate(empty)
		// Every wall and prop in the entrance is off the floor now too, so
		// the first defect is the one that caused the rest.
		s.Require().NotEmpty(errs)
		s.Equal("regions[0].cells", errs[0].Path)
		s.Contains(errs[0].Message, "no cells")
	})

	s.Run("two bosses in one region", func() {
		one := s.tombWith(`at: [13,5], targeting: lowest-health }`, `at: [13,5], targeting: lowest-health, boss: true }`)
		s.Empty(s.validate(one), "one boss per REGION: a skeleton boss in the hall is legal beside the captain in the tomb")

		two := strings.Replace(one, `at: [11,3], targeting: lowest-health }`, `at: [11,3], targeting: lowest-health, boss: true }`, 1)
		errs := s.validate(two)
		s.Equal([]string{"place[9].boss"}, paths(errs), "the second boss in the hall is the one refused")
		s.Contains(errs[0].Message, `region "hall" already names`)
	})
}

// TestEveryDefectIsReported — all of them, not the first, because the builder
// draws every one.
func (s *DialectSuite) TestEveryDefectIsReported() {
	broken := s.tombWith("void: opaque", "void: fog")
	broken = strings.Replace(broken, "start: [1, 3]", "start: [30, 3]", 1)
	broken = strings.Replace(broken, "lighting: { intensity: 0.6 }", "lighting: { intensity: 2 }", 1)

	errs := s.validate(broken)
	s.Equal([]string{"void", "regions[0].lighting.intensity", "start"}, paths(errs))

	_, err := dungeonspec.Load([]byte(broken))
	s.Require().ErrorIs(err, dungeonspec.ErrBadSpec)
	s.Contains(err.Error(), "void: ")
	s.Contains(err.Error(), "start: ")
}

// TestAWrongOrientationIsTheOnlyThingReported — every geometric check needs
// the layout, so under an unknown one they are skipped rather than piled on.
func (s *DialectSuite) TestAWrongOrientationIsTheOnlyThingReported() {
	errs := s.validate(s.tombWith("orientation: pointy", "orientation: sideways"))
	s.Equal([]string{"orientation"}, paths(errs))
}

// TestTheSameWallIsARefusalUnderTheOtherLayout — the discriminator, in the
// file's own terms (the rpg-toolkit#1141/#1150 lesson: one formula per
// orientation, not a swapped pair). The tomb's seam walls are drawn for
// pointy-top. Under flat-top the 14 straight crossings still touch, and of
// the 14 staggered ones exactly the 7 that lean UP ([5,r]-[6,r-1]) stop
// touching while the 7 that lean DOWN ([5,r]-[6,r+1]) still do — because
// under odd-q the odd column 5 staggers down, not up. A symmetric mistake in
// either formula would refuse all 14 or none.
func (s *DialectSuite) TestTheSameWallIsARefusalUnderTheOtherLayout() {
	errs := s.validate(s.tombWith("orientation: pointy", "orientation: flat"))
	s.Require().NotEmpty(errs)
	for _, e := range errs {
		s.True(strings.HasPrefix(e.Path, "walls["), "only walls are refused: %s", e.Path)
		s.Contains(e.Message, "not adjacent under flat")
	}
	s.Equal([]string{"walls[1]", "walls[5]", "walls[8]", "walls[12]", "walls[15]", "walls[19]", "walls[22]", "walls[26]"}, paths(errs))
}
