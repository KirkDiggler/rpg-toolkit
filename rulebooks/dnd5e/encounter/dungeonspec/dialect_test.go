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

// theEntranceSeam and theEntranceDoor are the two blocks most cases here swap
// out — the first wall of the tomb and the door standing in it, quoted exactly
// as the file writes them.
const (
	theEntranceSeam = "  - start: { cell: [5,7], offset: [0.25, 0.375] }\n" +
		"    end:   { cell: [6,0], offset: [-0.25, -0.375] }\n" +
		"    name: the entrance seam\n"
	theEntranceDoor = "    at: { cell: [6,4], offset: [-0.25, -0.375] }\n"
	tombSeamName    = "    name: the tomb seam\n"
)

func (s *DialectSuite) tombWith(old, new string) string {
	s.Require().Contains(s.tomb, old, "tombWith: anchor not present in the tomb")
	return strings.Replace(s.tomb, old, new, 1)
}

// tombWithShortcut is the tomb with an extra line drawn across the hall and
// one door on it, listed first so it is doors[0]. A door needs exactly one
// wall through it (F10), so a case that wants a door somewhere new brings its
// wall with it — see compile_test.go's shortcutWall for the line itself.
func (s *DialectSuite) tombWithShortcut(door string) string {
	raw := s.tombWith(tombSeamName, tombSeamName+shortcutWall)
	s.Require().Contains(raw, "doors:\n")

	return strings.Replace(raw, "doors:\n", "doors:\n  - id: shortcut\n"+shortcutAt+door+"\n", 1)
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
	s.Equal(dungeonspec.StartSpec{At: [2]int{1, 3}}, *spec.Start,
		"the bare pair stays legal, and states no facing — every shipping fixture "+
			"still spells it this way (rpg-project#374)")
	s.Len(spec.Regions, 3)
	s.Len(spec.Regions[0].Cells, 8, "eight rows")
	s.Len(spec.Regions[0].Cells[0], 6, "six cells each")
	s.Len(spec.Walls, 2, "two seams, one line each — 28 loose crossings before the line form")
	s.Equal(dungeonspec.PositionSpec{Cell: [2]int{5, 7}, Offset: [2]float64{0.25, 0.375}}, spec.Walls[0].Start)
	s.Equal(dungeonspec.PositionSpec{Cell: [2]int{6, 0}, Offset: [2]float64{-0.25, -0.375}}, spec.Walls[0].End)
	s.Nil(spec.Walls[0].Height, "the tomb authors no height, which is not the same fact as writing 1")
	s.Len(spec.Doors, 2)
	s.Equal(dungeonspec.PositionSpec{Cell: [2]int{6, 4}, Offset: [2]float64{-0.25, -0.375}}, spec.Doors[0].At)
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

// TestThePairFormIsRefusedByName is F4 and F12: the dialect a wall used to be
// written in is DELETED, not deprecated, and a file that speaks it is told so
// in as many words.
//
// Named separately from any other unknown key on purpose. `edges` is not a
// typo — it is last dialect's dungeon, and the difference between "field edges
// not found" and a sentence saying what replaced it is the difference between
// an author who knows what to do and one who does not. There is no migration
// to offer: legacy dungeons are deleted and re-authored (C17).
func (s *DialectSuite) TestThePairFormIsRefusedByName() {
	for _, tc := range []struct{ name, old, new string }{
		{"a bare edge", theEntranceSeam, "  - [[5,0],[6,0]]\n"},
		{"a run of edges", theEntranceSeam, "  - { edges: [[[5,0],[6,0]]], height: 2 }\n"},
		{"one edge in `between`", theEntranceSeam, "  - { between: [[5,0],[6,0]] }\n"},
		{"a door's edges", theEntranceDoor, "    edges: [[[5,4],[6,4]]]\n"},
	} {
		s.Run(tc.name, func() {
			_, err := dungeonspec.Decode([]byte(s.tombWith(tc.old, tc.new)))
			s.Require().Error(err)
			s.Contains(err.Error(), "the deleted pair form", "the form is named")
			s.Contains(err.Error(), "`start` and `end`", "and so is what replaced it")
		})
	}
}

// TestTheLineFormIsStrict — the wall, door and position forms keep Decode's
// own strictness even though a custom unmarshaler bypasses KnownFields: an
// unknown key and a half-written line are refusals naming the line, not facts
// silently dropped.
func (s *DialectSuite) TestTheLineFormIsStrict() {
	for _, tc := range []struct{ name, new, says string }{
		{"a typo'd key on a wall", "  - start: { cell: [5,0], offset: [0.5, 0] }\n" +
			"    end: { cell: [6,0], offset: [-0.5, 0] }\n    hieght: 2\n", "hieght"},
		{"a wall with no end", "  - start: { cell: [5,0], offset: [0.5, 0] }\n", "`start` to `end`"},
		{"a position with no offset", "  - start: { cell: [5,0] }\n" +
			"    end: { cell: [6,0], offset: [-0.5, 0] }\n", "the centre is [0,0], written out"},
		{"a position with no cell", "  - start: { offset: [0.5, 0] }\n" +
			"    end: { cell: [6,0], offset: [-0.5, 0] }\n", "which cell it is named from"},
		{"a typo'd key on a position", "  - start: { cell: [5,0], ofset: [0.5, 0] }\n" +
			"    end: { cell: [6,0], offset: [-0.5, 0] }\n", "ofset"},
	} {
		s.Run(tc.name, func() {
			_, err := dungeonspec.Decode([]byte(s.tombWith(theEntranceSeam, tc.new)))
			s.Require().Error(err)
			s.Contains(err.Error(), tc.says)
		})
	}

	_, err := dungeonspec.Decode([]byte(s.tombWith(theEntranceDoor, "    closed: true\n")))
	s.Require().Error(err, "a door that does not say where it stands is not a door")
	s.Contains(err.Error(), "`at`, one position on a wall")
}

// TestValidate_PathsNameTheThing — the table: every refusal names the YAML
// path of the thing that is wrong, because that is where the builder draws it.
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
		// The wall-shape refusals stand on a THIRD wall added after the two
		// the tomb ships, so each file differs from a valid one by exactly
		// the broken line — breaking a seam would refuse its door beside it,
		// and two refusals for one edit is a worse test of either.
		{"an offset outside the seven", tombSeamName, tombSeamName +
			"  - start: { cell: [8,7], offset: [0.3, 0.4] }\n" +
			"    end:   { cell: [9,0], offset: [-0.25, -0.375] }\n",
			"walls[2].start", "not one of the seven points a wall may stand at"},
		{"a direction off the twelve", tombSeamName, tombSeamName +
			"  - start: { cell: [8,7], offset: [0.25, 0.375] }\n" +
			"    end:   { cell: [10,0], offset: [-0.25, -0.375] }\n",
			"walls[2]", "not one of the twelve directions"},
		{"a wall that stands nowhere", tombSeamName, tombSeamName +
			"  - start: { cell: [8,7], offset: [0.25, 0.375] }\n" +
			"    end:   { cell: [8,7], offset: [0.25, 0.375] }\n",
			"walls[2]", "starts and ends at the same point"},
		{"a wall off the floor entirely", tombSeamName, tombSeamName +
			"  - start: { cell: [40,0], offset: [0.5, 0] }\n" +
			"    end:   { cell: [44,0], offset: [0.5, 0] }\n",
			"walls[2]", "passes through no floor at all"},
		{"a wall drawn twice", tombSeamName, tombSeamName + theEntranceSeam,
			"walls[2]", "runs exactly where walls[0] already does"},
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
		{"a wall height below standard", tombSeamName, tombSeamName + shortcutWall + "    height: 0.5\n",
			"walls[2].height", "outside [1,3]"},
		{"a wall height above the ceiling", tombSeamName, tombSeamName + shortcutWall + "    height: 3.5\n",
			"walls[2].height", "outside [1,3]"},
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
		// The door refusals the line form makes possible (F10, F11): a door
		// is a position on a wall, so it can miss the wall, catch two, or
		// stand where there is no crossing to open.
		{"a door no wall passes through", theEntranceDoor,
			"    at: { cell: [9,3], offset: [-0.5, 0] }\n",
			"doors[0].at", "no wall passes through this point"},
		{"a door in the middle of a hex", theEntranceDoor,
			"    at: { cell: [6,4], offset: [0, 0] }\n",
			"doors[0].at", "where there is no crossing to open"},
		{"a door offset outside the seven", theEntranceDoor,
			"    at: { cell: [6,4], offset: [0.3, 0.4] }\n",
			"doors[0].at", "not one of the seven points"},
		// Authoring coherence (rpg-project#351): the room hides with its
		// door, and the incoherent halves refuse — each at the region,
		// which is the field the form-filler flips.
		{"a room only enterable through a concealed door", "locked: [{ ability: dex, dc: 12 }]",
			"locked: [{ ability: dex, dc: 12 }]\n    concealed: [{ ability: perception, dc: 15 }]",
			"regions[2].concealed", "can only be entered through a concealed door"},
		{"a concealed room anyone can walk into", "  - id: tomb\n",
			"  - id: tomb\n    concealed: true\n",
			"regions[2].concealed", "a walk-in room cannot be a secret"},
		{"a door with no id", "  - id: entrance-hall\n", "  - id: \"\"\n", "doors[0].id", "no id"},
	} {
		s.Run(tc.name, func() {
			errs := s.validate(s.tombWith(tc.old, tc.new))
			s.Require().NotEmpty(errs, "the defect must be found")
			s.Equal([]string{tc.path}, paths(errs), "exactly one defect, at the thing that is wrong")
			s.Contains(errs[0].Message, tc.says)
		})
	}

	// The concealed-check shape cases stand on an INNER shortcut door — both
	// endpoints in the hall — so each differs from a valid file by exactly the
	// malformed check, with no coherence refusal beside it (a shortcut inside
	// a room is nobody's entrance).
	for _, tc := range []struct{ name, check, path, says string }{
		{"a concealed door with no find approach", "\n    concealed: []",
			"doors[0].concealed", "at least one way to find it"},
		{"a find approach with no ability", "\n    concealed: [{ dc: 15 }]",
			"doors[0].concealed[0].ability", "ability"},
		{"a find approach nothing has to beat", "\n    concealed: [{ ability: perception, dc: 0 }]",
			"doors[0].concealed[0].dc", "nothing to beat"},
	} {
		s.Run(tc.name, func() {
			errs := s.validate(s.tombWithShortcut(tc.check))
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

	s.Run("a concealed room with a doorless open way in", func() {
		// The coherent secret room — tomb concealed, its door concealed —
		// with one wall of its border erased: a bare crossing no author can
		// conceal, named in the refusal by the file's own coordinates.
		secret := s.tombWith("locked: [{ ability: dex, dc: 12 }]",
			"locked: [{ ability: dex, dc: 12 }]\n    concealed: [{ ability: perception, dc: 15 }]")
		secret = strings.Replace(secret, "  - id: tomb\n", "  - id: tomb\n    concealed: true\n", 1)
		s.Empty(s.validate(secret), "concealed door + concealed room is the coherent whole")

		// The tomb seam, one position short at its north end: row 0's
		// crossing is past where the line now stops, and nothing else is.
		gap := strings.Replace(secret,
			"    end:   { cell: [16,0], offset: [-0.25, -0.375] }",
			"    end:   { cell: [16,0], offset: [-0.25, 0.375] }", 1)
		s.Require().NotEqual(secret, gap)
		errs := s.validate(gap)
		s.Equal([]string{"regions[2].concealed"}, paths(errs))
		s.Contains(errs[0].Message, "the open way between [15,0] and [16,0]")
		s.Contains(errs[0].Message, "a walk-in room cannot be a secret")
	})

	s.Run("the minimal secret closet compiles", func() {
		// The review probe that caught the entrance-local first draft
		// (rpg-project#351's reformulation): a visible start room whose
		// ONLY crossing is the one concealed door is slice 1's smallest
		// honest dungeon, and the frontier form admits it.
		closet := `
version: 2
key: closet
orientation: pointy
void: opaque
regions:
  - id: study
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0]]
  - id: closet
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[2,0]]
start: [0, 0]
walls:
  - start: { cell: [1,-1], offset: [0, 0] }
    end:   { cell: [1,1], offset: [0, 0] }
doors:
  - id: secret
    at: { cell: [2,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
`
		s.Empty(s.validate(closet))
	})

	s.Run("a two-room secret suite compiles", func() {
		// Everything wholly inside hidden space is nobody's business: the
		// suite's interior door between two concealed rooms obliges nobody,
		// and only the one frontier crossing must be the concealed door.
		suite := `
version: 2
key: suite
orientation: pointy
void: opaque
regions:
  - id: study
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0]]
  - id: vault
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[2,0]]
  - id: sanctum
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[3,0]]
start: [0, 0]
walls:
  - start: { cell: [1,-1], offset: [0, 0] }
    end:   { cell: [1,1], offset: [0, 0] }
  - start: { cell: [2,-1], offset: [0, 0] }
    end:   { cell: [2,1], offset: [0, 0] }
doors:
  - id: secret
    at: { cell: [2,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
  - id: inner
    at: { cell: [3,0], offset: [-0.5, 0] }
    closed: true
`
		s.Empty(s.validate(suite))
	})

	s.Run("a wider doorway is two doors, and two ways in", func() {
		// A door is ONE crossing now (F11), so the two-cell gate the pair
		// form wrote as one door with two edges is two doors on one wall —
		// and each is its own way into the vault, so the frontier names
		// both. Rows 0 and 2 are both even, so under pointy-top the seam has
		// exactly the two straight crossings the gate stands in and no
		// staggered third; the one wall runs through both of them and
		// through the void between.
		gate := `
version: 2
key: gate
orientation: pointy
void: opaque
regions:
  - id: hallway
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0]]
      - [[0,2],[1,2]]
  - id: vault
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[2,0],[2,2]]
start: [0, 0]
walls:
  - start: { cell: [1,-1], offset: [0, 0] }
    end:   { cell: [1,3], offset: [0, 0] }
    name: the gate wall
doors:
  - id: gate-north
    at: { cell: [2,0], offset: [-0.5, 0] }
    closed: true
  - id: gate-south
    at: { cell: [2,2], offset: [-0.5, 0] }
    closed: true
`
		errs := s.validate(gate)
		s.Equal([]string{"regions[1].concealed", "regions[1].concealed"}, paths(errs),
			"two doors are two holes in the secret, and the author has to close both")
		s.Contains(errs[0].Message, `its door "gate-north" (doors[0])`)
		s.Contains(errs[1].Message, `its door "gate-south" (doors[1])`)
		for _, e := range errs {
			s.Contains(e.Message, "a walk-in room cannot be a secret")
		}

		// Conceal both and it is a coherent secret, which is the half that
		// says the refusal above was about the doors and not about the wall.
		sealed := strings.ReplaceAll(gate, "    closed: true\n",
			"    closed: true\n    concealed: [{ ability: perception, dc: 15 }]\n")
		s.Empty(s.validate(sealed))
	})

	s.Run("one open door and one concealed shortcut stays legal", func() {
		// The room is no secret, the shortcut is (rpg-project#351): the
		// hall keeps its two plain entrances, and a concealed inner door
		// obliges nobody to conceal anything.
		s.Empty(s.validate(s.tombWithShortcut("\n    concealed: [{ ability: perception, dc: 15 }]")))
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
// file's own terms (the rpg-toolkit#1141/#1150 lesson: one table per
// orientation, not a swapped pair).
//
// The seven positions TURN WITH THE HEXES. A pointy-top hex's side midpoints
// sit a quarter of a width and three eighths of a height from its centre; a
// flat-top hex's sit three eighths of a width and a quarter of a height, which
// is the same six points on the same hex rotated 30°. So none of the pointy
// values exists under flat, and the tomb read as a flat-top dungeon is refused
// at every wall end and every door — by NAME, quoting the value back, never
// snapped to the nearest member of the other table.
//
// A symmetric mistake — one table used for both layouts, or the two swapped —
// would let this file compile under either word, which is exactly the class of
// bug that cost this workspace twice.
func (s *DialectSuite) TestTheSameWallIsARefusalUnderTheOtherLayout() {
	errs := s.validate(s.tombWith("orientation: pointy", "orientation: flat"))
	s.Require().NotEmpty(errs)
	for _, e := range errs {
		s.Contains(e.Message, "not one of the seven points a wall may stand at under flat hexes")
	}
	s.Equal([]string{
		"walls[0].start", "walls[0].end",
		"walls[1].start", "walls[1].end",
		"doors[0].at", "doors[1].at",
	}, paths(errs), "every end of every line, and every door on them")

	// And the mirror: a flat-top file's own positions are refused under
	// pointy, so neither table is the one this build always reaches for.
	flat := `
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
walls:
  - start: { cell: [1,0], offset: [0.375, 0.25] }
    end:   { cell: [1,1], offset: [0.375, 0.25] }
`
	s.Empty(s.validate(flat), "authored flat-top, it compiles")

	errs = s.validate(strings.Replace(flat, "orientation: flat", "orientation: pointy", 1))
	s.Require().Len(errs, 2)
	for _, e := range errs {
		s.Contains(e.Message, "under pointy hexes")
	}
}
