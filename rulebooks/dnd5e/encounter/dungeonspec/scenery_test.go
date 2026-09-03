// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// scenery_test.go is FLOOR NOBODY STANDS ON (rpg-project#360, wall-geometry
// design §3.1 and §4.1) — the compiler's half of slice 1.
//
// A cell carries two facts now: an OWNER, which decides visibility and
// meaning, and whether anyone can STAND on it. Scenery is the first thing that
// has one without the other: floor with no owner, that walls stand on, props
// sit on, sight crosses, and feet never touch.
//
// Every scene below is a whole small dungeon rather than an edit of the tomb,
// because what these rules are about is the SHAPE of a floor — a room, a strip
// of scenery, and a room beyond it — and that shape is easier to read written
// out than diffed in.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ScenerySuite struct {
	suite.Suite
}

func TestScenerySuite(t *testing.T) {
	suite.Run(t, new(ScenerySuite))
}

// validate decodes and validates, returning every defect. Decode-level
// refusals come back as defects too, so a scene that mistypes a key reads the
// same as one that paints a cell twice.
func (s *ScenerySuite) validate(raw string) []dungeonspec.FieldError {
	spec, err := dungeonspec.Decode([]byte(raw))
	if err != nil {
		var verr *dungeonspec.ValidationError
		s.Require().ErrorAs(err, &verr)
		return verr.Errors
	}
	return dungeonspec.Validate(spec)
}

func (s *ScenerySuite) load(raw string) dungeonspec.Compiled {
	compiled, err := dungeonspec.Load([]byte(raw))
	s.Require().NoError(err, "the scene was meant to compile")
	return compiled
}

// sceneryPaths is the paths of every defect, for the same reason
// dialect_test.go reads them: the builder draws each refusal on the canvas at
// the path it names.
func sceneryPaths(errs []dungeonspec.FieldError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Path)
	}
	return out
}

// theStrip is the shape every scene here is a variant of: a start room, a
// one-cell strip of scenery, and a room on the far side of it, all on row 0
// where consecutive columns are neighbours under pointy-top.
//
//	study [0,0] [1,0] | scenery [2,0] | vault [3,0]
//
// `far` is written into the far room's own block, so a scene can concealed it,
// wall it off, or leave it plain with one substitution.
func theStrip(far string) string {
	return `
version: 2
key: strip
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
` + far + `    cells:
      - [[3,0]]
scenery:
  - [[2,0]]
start: [0, 0]
`
}

// theLongStrip is theStrip with TWO scenery cells, so something can stand in
// the middle of a way rather than only at its ends:
//
//	study [0,0] [1,0] | scenery [2,0] [3,0] | vault [4,0]
//
// `middle` is written between the two scenery cells; `far` goes in the far
// room's own block.
func theLongStrip(far, middle string) string {
	return `
version: 2
key: longstrip
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
` + far + `    cells:
      - [[4,0]]
scenery:
  - [[2,0],[3,0]]
start: [0, 0]
` + middle
}

// wallAcross is a wall standing in the one crossing between [col,0] and
// [col+1,0], and in nothing else.
//
// EVERY SCENE HERE IS ONE ROW, so the shape has to be exact. The line runs
// along the side those two hexes share — the thick line, which leaves both of
// them whole — from that side's midpoint to the middle of the staggered cell
// SOUTH of it, which is half the distance to the next position on the same
// line.
//
// The half length is the whole trick, and it is worth stating because the
// obvious longer version is wrong. A flat-side line passes through the MIDDLES
// of the staggered cells it runs between, and a wall through a cell's middle
// blocks every crossing out of it (design F14) — so a wall drawn from one
// staggered middle to the next would seal whatever stands north of the
// crossing it was meant to close. Stopping at the first middle south leaves
// the north side untouched.
//
// The cells it names are in row 0 and row 1, and neither has to be floor: a
// position names the frame a point is measured from, never a place that has to
// exist.
func wallAcross(col int) string {
	return fmt.Sprintf(`walls:
  - start: { cell: [%d,0], offset: [-0.5, 0] }
    end:   { cell: [%d,1], offset: [0, 0] }
`, col+1, col)
}

// doorAcross is a door standing in that same crossing: the midpoint of the
// side between [col,0] and [col+1,0], named from the eastern of the two.
func doorAcross(col int, body string) string {
	return fmt.Sprintf(`doors:
  - id: panel
    at: { cell: [%d,0], offset: [-0.5, 0] }
%s`, col+1, body)
}

// twoHoles is the asymmetry fixture: one scenery cell that `near` meets by TWO
// crossings and `far` by one.
//
//	near [2,0] [2,1] | scenery [3,0] | far [4,0]
//
// [2,0] and [2,1] are both neighbours of [3,0] under pointy-top and of each
// other, so `near` is one room with two holes into the strip; [4,0] touches
// neither of them, so the only route between the rooms runs through it.
// `nearConcealed` says which of the two is the secret, which is the whole
// subject: the walk runs from the LOWER-INDEXED room, so swapping them swaps
// which side's holes are the ones enumerated.
func twoHoles(first, second string, nearConcealed bool) string {
	conceal := func(id string, yes bool) string {
		if !yes {
			return ""
		}
		_ = id
		return "    concealed: true\n"
	}
	start := "[4, 0]"
	if !nearConcealed {
		start = "[2, 0]"
	}
	return `
version: 2
key: twoholes
orientation: pointy
void: opaque
regions:
  - id: ` + first + `
    archetype: crypt
    lighting: { intensity: 1 }
` + conceal(first, nearConcealed) + `    cells:
      - [[2,0],[2,1]]
  - id: ` + second + `
    archetype: crypt
    lighting: { intensity: 1 }
` + conceal(second, !nearConcealed) + `    cells:
      - [[4,0]]
scenery:
  - [[3,0]]
start: ` + start + `
`
}

// TestF1_ACellIsFloorOnce — a cell belongs to a region or to the scenery, and
// nothing may claim it twice. Ownership has to be unique for "who owns this
// cell" to be an answer rather than a guess, and scenery is a second claimant
// on the same map.
func (s *ScenerySuite) TestF1_ACellIsFloorOnce() {
	s.Run("a cell in both a region and the scenery", func() {
		both := strings.Replace(theStrip(""), "scenery:\n  - [[2,0]]", "scenery:\n  - [[2,0],[1,0]]", 1)
		errs := s.validate(both)
		s.Require().Equal([]string{"scenery[0][1]"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "[1,0]")
		s.Contains(errs[0].Message, `region "study"`)
	})

	s.Run("a cell listed twice in the scenery", func() {
		twice := strings.Replace(theStrip(""), "scenery:\n  - [[2,0]]", "scenery:\n  - [[2,0],[2,0]]", 1)
		errs := s.validate(twice)
		s.Require().Equal([]string{"scenery[0][1]"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "[2,0]")
		s.Contains(errs[0].Message, "twice")
	})

	s.Run("the strip itself compiles", func() {
		s.Empty(s.validate(theStrip("")))
	})
}

// TestF2_PropsSitOnSceneryAndNobodyStandsOnIt — a prop may stand on any floor;
// a monster and the party's start may only stand where feet can go.
func (s *ScenerySuite) TestF2_PropsSitOnSceneryAndNobodyStandsOnIt() {
	s.Run("a prop on scenery compiles and reaches the field", func() {
		withProp := theStrip("") + `place:
  - { ref: "dnd5e:props:rubble", at: [2,0], blocks_movement: false, blocks_los: false }
`
		s.Require().Empty(s.validate(withProp))

		compiled := s.load(withProp)
		s.Require().Len(compiled.Field.Props, 1)
		s.Equal(spatial.Position{X: 2, Y: 0}, compiled.Field.Props[0].At,
			"the prop stands on the scenery cell it was authored on")
	})

	s.Run("a monster on scenery is refused by name", func() {
		withMonster := theStrip("") + `place:
  - { ref: "dnd5e:monsters:skeleton", at: [2,0] }
`
		errs := s.validate(withMonster)
		s.Require().Equal([]string{"place[0].at"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "dnd5e:monsters:skeleton")
		s.Contains(errs[0].Message, "[2,0]")
		s.Contains(errs[0].Message, "nobody can stand")
	})

	s.Run("the party may not start on scenery", func() {
		onStrip := strings.Replace(theStrip(""), "start: [0, 0]", "start: [2, 0]", 1)
		errs := s.validate(onStrip)
		s.Require().Equal([]string{"start"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "[2,0]")
		s.Contains(errs[0].Message, "nobody can stand")
	})
}

// TestC2_WallsAndDoorsStandOnScenery — a wall may run across scenery and a
// door may stand in a crossing one of whose sides is scenery. This is how a
// wall stands against something other than a room (design §1.9): what it needs
// underfoot is floor, and scenery is floor.
func (s *ScenerySuite) TestC2_WallsAndDoorsStandOnScenery() {
	s.Run("a wall standing on a scenery cell", func() {
		walled := theStrip("") + wallAcross(2)
		s.Require().Empty(s.validate(walled))

		compiled := s.load(walled)
		s.Require().Len(compiled.Field.Walls, 1)
		s.Equal(spatial.Position{X: 2, Y: 0}, compiled.Field.Walls[0].From)
		s.Equal(spatial.Position{X: 3, Y: 0}, compiled.Field.Walls[0].To)

		s.Require().Len(compiled.Field.Segments, 1)
		s.ElementsMatch([]spatial.Position{{X: 2, Y: 0}, {X: 3, Y: 0}},
			compiled.Field.Segments[0].Footprint,
			"the wall stands on the scenery cell and on the room's, and takes nothing from either")
		s.Empty(compiled.Field.Sealed, "a wall along a flat side leaves both hexes whole")
	})

	s.Run("a door standing in one", func() {
		doored := theStrip("") + wallAcross(2) + doorAcross(2, "    closed: true\n")
		s.Require().Empty(s.validate(doored))
		s.Require().Len(s.load(doored).Field.Doors, 1)
	})

	s.Run("a wall against the void is the ordinary case", func() {
		// THE RULE THE LINE FORM DELETED, and deliberately (design §1.9,
		// C2). Under the pair form a wall was a crossing and both its cells
		// had to be floor, so a wall could not be drawn along the outside of
		// a room at all — the envelope was implied and unsayable. A line
		// stands where it is drawn: it needs floor to stand ON, not floor on
		// both sides, so a wall on the edge of the world compiles and blocks
		// nothing that was ever passable.
		edge := theStrip("") + wallAcross(3)
		s.Require().Empty(s.validate(edge), "[4,0] is nothing at all, and that is allowed now")

		compiled := s.load(edge)
		s.Empty(compiled.Field.Walls, "the crossing into the void was impassable already")
		s.Require().Len(compiled.Field.Segments, 1)
		s.Equal([]spatial.Position{{X: 3, Y: 0}}, compiled.Field.Segments[0].Footprint,
			"and the wall stands on the one floor cell it touches")
	})

	s.Run("a wall that touches no floor at all is still refused", func() {
		// The rule that survived: a wall has to stand somewhere.
		nowhere := theStrip("") + wallAcross(9)
		errs := s.validate(nowhere)
		s.Require().Equal([]string{"walls[0]"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "passes through no floor at all")
	})
}

// TestC4_TheConcealmentWalkCrossesScenery is the acceptance case A3 and the
// rule under it.
//
// A WAY between two rooms is a wall-free path whose interior cells are all
// scenery, and it is a concealed way only if its FIRST crossing is a concealed
// door. Before scenery existed every way was one crossing long, so a strip of
// scenery between a visible room and a secret one would have been a frontier
// the walk could not see — a hole in the secret that compiled.
func (s *ScenerySuite) TestC4_TheConcealmentWalkCrossesScenery() {
	secret := "    concealed: true\n"

	s.Run("A3: the forgotten wall", func() {
		errs := s.validate(theStrip(secret))
		s.Require().Equal([]string{"regions[1].concealed"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "the scenery at [2,0]", "the refusal names the scenery cell the path enters")
		s.Contains(errs[0].Message, "a walk-in room cannot be a secret")
	})

	s.Run("A3: add the wall and it compiles", func() {
		walled := theStrip(secret) + wallAcross(2)
		s.Empty(s.validate(walled), "a wall across the strip closes the way")
	})

	s.Run("a concealed door on the secret's own edge closes the way", func() {
		// The ordinary shape: a strip of rubble in front of a hidden door.
		doored := theStrip(secret) + wallAcross(2) +
			doorAcross(2, "    closed: true\n    concealed: [{ ability: perception, dc: 15 }]\n")
		s.Empty(s.validate(doored))
	})

	s.Run("a concealed door on the far edge closes it too", func() {
		// The other end. A non-knower meets the mask that door wears before
		// they ever reach the strip, so their map is the honest twin's and
		// there is nothing to give away.
		doored := theStrip(secret) + wallAcross(1) +
			doorAcross(1, "    closed: true\n    concealed: [{ ability: perception, dc: 15 }]\n")
		s.Empty(s.validate(doored))
	})

	s.Run("a plain door closes nothing, and the refusal names the secret's own edge", func() {
		// Neither end is concealed, so the strip is a way anybody walks. The
		// refusal points at the crossing on the secret's own side — the one
		// its author has to wall or conceal — not at the plain door at the
		// far end.
		doored := theStrip(secret) + wallAcross(1) + doorAcross(1, "    closed: true\n")
		errs := s.validate(doored)
		s.Require().Equal([]string{"regions[1].concealed"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "the open way between [3,0] and the scenery at [2,0]")
	})

	s.Run("one refusal per path, not one per direction", func() {
		// Crossings are undirected and the walk could meet the strip from
		// either room. A path is ONE way, so the forgotten wall is one defect
		// on the canvas rather than the same hole drawn twice.
		s.Len(s.validate(theStrip(secret)), 1)
	})

	s.Run("a wall inside the strip is not a way at all", func() {
		// The crossing that closes a way need not be at either end. A wall
		// between the two scenery cells stops the flood exactly as a wall on
		// a room's own edge does, and there is no way left to conceal.
		walled := theLongStrip(secret, wallAcross(2))
		s.Empty(s.validate(walled))
	})

	s.Run("a concealed door between two scenery cells conceals the way", func() {
		// The ruled rule is ANY crossing along the way, so the author may put
		// the secret's door in the middle of the rubble rather than on either
		// room's edge. A rule that classified a way by its two ends would
		// call this a walk-in secret and refuse the dungeon.
		doored := theLongStrip(secret, wallAcross(2)+
			doorAcross(2, "    closed: true\n    concealed: [{ ability: perception, dc: 15 }]\n"))
		s.Empty(s.validate(doored))

		// And the same door left plain does not conceal it — so what made the
		// scene above legal is the concealment, not the door.
		plain := theLongStrip(secret, wallAcross(2)+doorAcross(2, "    closed: true\n"))
		errs := s.validate(plain)
		s.Require().Equal([]string{"regions[1].concealed"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "the open way between [4,0] and the scenery at [3,0]")
	})

	s.Run("a scenery area touching two visible rooms and one hidden one", func() {
		// A way is not a strip with two ends: one scenery cell has six
		// neighbours, and this one touches three rooms. The study's crossing
		// is a concealed door, so the study's way in is sealed — but the
		// hall's is bare and the vault's is bare, so the hall's way is a
		// walk-in secret and is refused through it.
		//
		// The hall touches the study directly, so visible reach still holds
		// and the only defect in the scene is the one it is about.
		three := `
version: 2
key: junction
orientation: pointy
void: opaque
regions:
  - id: study
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[2,0]]
  - id: hall
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[2,-1]]
  - id: vault
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[4,0]]
scenery:
  - [[3,0]]
start: [2, 0]
walls:
  - start: { cell: [3,0], offset: [-0.5, 0] }
    end:   { cell: [2,1], offset: [0, 0] }
doors:
  - id: panel
    at: { cell: [3,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
`
		errs := s.validate(three)
		s.Require().Equal([]string{"regions[2].concealed"}, sceneryPaths(errs),
			"one refusal: the hall's way in, not the study's sealed one")
		s.Contains(errs[0].Message, "the open way between [4,0] and the scenery at [3,0]")

		// Conceal the vault's own crossing instead and every way in carries a
		// concealed door, so the junction is authorable.
		// A DOOR IS ONE CROSSING NOW (F11), so the second way in is a second
		// door, standing in a second wall of its own.
		sealed := strings.Replace(three, "doors:\n",
			"  - start: { cell: [4,0], offset: [-0.5, 0] }\n"+
				"    end:   { cell: [3,1], offset: [0, 0] }\n"+
				"doors:\n  - id: vault-panel\n"+
				"    at: { cell: [4,0], offset: [-0.5, 0] }\n"+
				"    closed: true\n"+
				"    concealed: [{ ability: perception, dc: 15 }]\n", 1)
		s.Empty(s.validate(sealed))
	})

	s.Run("two holes on the secret's own side are two defects", func() {
		// The junction, mirrored: the SECRET is the lower-indexed room and it
		// has two crossings into one scenery cell, which reaches the hall by
		// one. Each hole is a separate thing the author has to close, so each
		// is named. Keyed by the cell alone the flood marked the strip on the
		// first hole and the second vanished, and the author met its twin only
		// after walling the one the refusal named.
		errs := s.validate(twoHoles("vault", "hall", true))
		s.Require().Len(errs, 2, "two holes, two refusals")
		s.Equal([]string{"regions[0].concealed", "regions[0].concealed"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "the open way between [2,0] and the scenery at [3,0]")
		s.Contains(errs[1].Message, "the open way between [2,1] and the scenery at [3,0]")
	})

	s.Run("two holes on the visible side are one defect", func() {
		// The same shape with the rooms swapped. Now the two holes are the
		// VISIBLE room's, and the sentence the author reads is about the
		// secret's single crossing — walling either hole is not what fixes it,
		// so saying it twice would be the same defect said twice.
		errs := s.validate(twoHoles("hall", "vault", false))
		s.Require().Len(errs, 1, "two ways, one crossing named, one refusal")
		s.Equal([]string{"regions[1].concealed"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "the open way between [4,0] and the scenery at [3,0]")
	})

	s.Run("two arrivals on the secret side stay two", func() {
		// One hole out of the hall, two cells of the vault on the strip. Two
		// crossings the author has to close, named separately, exactly as
		// before this enumeration was fixed.
		twoArrivals := `
version: 2
key: arrivals
orientation: pointy
void: opaque
regions:
  - id: hall
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[2,0]]
  - id: vault
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[4,0],[3,1]]
scenery:
  - [[3,0]]
start: [2, 0]
`
		errs := s.validate(twoArrivals)
		s.Require().Len(errs, 2)
		s.Equal([]string{"regions[1].concealed", "regions[1].concealed"}, sceneryPaths(errs))
	})

	s.Run("visible reach extends through scenery", func() {
		// study is the start. gallery is only reachable ACROSS the scenery
		// strip, and its own concealed closet is what makes the walk run at
		// all. Without the scenery hop gallery is a room only a found secret
		// opens, and this scene is refused.
		reach := `
version: 2
key: reach
orientation: pointy
void: opaque
regions:
  - id: study
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0]]
  - id: gallery
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[3,0]]
  - id: closet
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[4,0]]
scenery:
  - [[2,0]]
start: [0, 0]
walls:
  - start: { cell: [3,-1], offset: [0, 0] }
    end:   { cell: [3,1], offset: [0, 0] }
doors:
  - id: panel
    at: { cell: [4,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
`
		s.Empty(s.validate(reach))
	})

	s.Run("a walled strip does not extend reach", func() {
		// The same dungeon with the strip walled off from the study: the
		// gallery is now genuinely unreachable except through the secret, and
		// that is the refusal the reach rule exists to make.
		walled := `
version: 2
key: reach
orientation: pointy
void: opaque
regions:
  - id: study
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0]]
  - id: gallery
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[3,0]]
  - id: closet
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[4,0]]
scenery:
  - [[2,0]]
start: [0, 0]
walls:
  - start: { cell: [1,-1], offset: [0, 0] }
    end:   { cell: [1,1], offset: [0, 0] }
  - start: { cell: [3,-1], offset: [0, 0] }
    end:   { cell: [3,1], offset: [0, 0] }
doors:
  - id: panel
    at: { cell: [4,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
`
		errs := s.validate(walled)
		s.Require().Equal([]string{"regions[1].concealed"}, sceneryPaths(errs))
		s.Contains(errs[0].Message, "can only be entered through a concealed door")
	})
}

// TestSceneryReachesTheField — the compiler carries the strip to the
// composition as its own list, and a dungeon that authors none carries none.
func (s *ScenerySuite) TestSceneryReachesTheField() {
	compiled := s.load(theStrip(""))
	s.Equal([]spatial.Position{{X: 2, Y: 0}}, compiled.Field.Scenery)

	tomb := s.load(tombYAML(s.T()))
	s.Empty(tomb.Field.Scenery, "the reference tomb authors no scenery")

	// And the field it hands over is a field the composition accepts.
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		Field:   compiled.Field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
}
