// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// compile_test.go pins what the compiler BUILDS from a version-2 file: the
// regions verbatim, the walls and props at their absolute cells, the doors in
// their crossings with the state the file gives them, and the two halves it
// hands back. The forcing case — that the re-authored tomb is the tomb — is
// golden_test.go's; this file is about the shape of each piece.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type CompileSuite struct {
	suite.Suite
	tomb string
}

func TestCompileSuite(t *testing.T) {
	suite.Run(t, new(CompileSuite))
}

func (s *CompileSuite) SetupTest() { s.tomb = tombYAML(s.T()) }

func (s *CompileSuite) load(raw string) dungeonspec.Compiled {
	compiled, err := dungeonspec.Load([]byte(raw))
	s.Require().NoError(err)
	return compiled
}

// tombWith is the shipping tomb with one line (or block) swapped, so every
// case differs from a VALID spec by exactly the thing it is about.
func (s *CompileSuite) tombWith(old, new string) string {
	s.Require().Contains(s.tomb, old, "tombWith: anchor not present in the tomb")
	return strings.Replace(s.tomb, old, new, 1)
}

// TestTheRegionsAreCarriedVerbatim — a region comes out as it went in, its
// rows flattened, with the archetype and lighting it was authored with.
func (s *CompileSuite) TestTheRegionsAreCarriedVerbatim() {
	c := s.load(s.tomb)

	s.Require().Len(c.Field.Regions, 3)
	for i, want := range []struct {
		id        string
		cells     int
		intensity float64
	}{{"entrance", 48, 0.6}, {"hall", 80, 0.4}, {"tomb", 96, 0.15}} {
		r := c.Field.Regions[i]
		s.Equal(want.id, r.ID)
		s.Len(r.Cells, want.cells, "the rows are flattened")
		s.Equal("crypt", r.Archetype)
		s.Require().NotNil(r.Lighting)
		s.Equal(want.intensity, r.Lighting.Intensity)
	}
	s.Equal(spatial.Position{X: 6, Y: 0}, c.Field.Regions[1].Cells[0], "absolute, in the authored frame")
	s.Equal("Hall", c.Field.Regions[1].Name)
}

// TestTheFieldCarriesWhatTheAuthorDeclared — the two the composition refuses to
// pick for itself (rpg-toolkit#1116 and #1033), carried through rather than
// re-decided.
func (s *CompileSuite) TestTheFieldCarriesWhatTheAuthorDeclared() {
	s.Equal(encounter.VoidOpaque, s.load(s.tomb).Field.Canvas.Void.Kind())
	s.Equal(encounter.OrientationPointyTop, s.load(s.tomb).Field.Canvas.Orientation.Kind())

	// Both layouts are settable (Kirk's ruling) — but not by swapping the
	// word on a file whose walls were drawn for the other one, since half its
	// staggered crossings stop touching (dialect_test.go pins which). A
	// flat-top dungeon is authored flat-top.
	flat := s.load(`
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
  - [[1,0],[2,1]]
`)
	s.Equal(encounter.OrientationFlatTop, flat.Field.Canvas.Orientation.Kind())

	clear := s.load(s.tombWith("void: opaque", "void: transparent"))
	s.Equal(encounter.VoidTransparent, clear.Field.Canvas.Void.Kind())
}

// TestWallsAreEdgesThatBlockBothWays — every wall is carried as written, in
// the authored frame, blocking movement and sight.
func (s *CompileSuite) TestWallsAreEdgesThatBlockBothWays() {
	c := s.load(s.tomb)
	s.Require().Len(c.Field.Walls, 28)
	s.Equal(encounter.WallInput{Boundary: spatial.Boundary{
		From: spatial.Position{X: 5, Y: 0}, To: spatial.Position{X: 6, Y: 0},
		BlocksMovement: true, BlocksLineOfSight: true,
	}}, c.Field.Walls[0], "a bare entry compiles at height 0: not authored, the standard height")
}

// TestAWallMayAuthorItsHeight — the object form carries its multiplier
// through the compile verbatim; every bare sibling stays at 0, the carrier's
// word for "not authored" (rpg-project#273).
func (s *CompileSuite) TestAWallMayAuthorItsHeight() {
	c := s.load(s.tombWith("  - [[5,1],[6,0]]", "  - { between: [[5,1],[6,0]], height: 2.5 }"))
	s.Require().Len(c.Field.Walls, 28, "the object form is one wall, exactly as the bare form is")
	raised := c.Field.Walls[1]
	s.Equal(spatial.Position{X: 5, Y: 1}, raised.From)
	s.Equal(spatial.Position{X: 6, Y: 0}, raised.To)
	s.True(raised.BlocksMovement, "height changes nothing about what a wall blocks")
	s.True(raised.BlocksLineOfSight, "a wall cannot be seen past at ANY height (Kirk's ruling)")
	s.Equal(2.5, raised.Height)
	for i, w := range c.Field.Walls {
		if i != 1 {
			s.Zero(w.Height, "walls[%d] authored no height", i)
		}
	}
}

// TestADoorIsMintedUnderTheDungeonsKeyInItsAuthoredState — `<key>/<id>`, so two
// dungeons in one process cannot collide, with open / closed / locked read
// off the file.
func (s *CompileSuite) TestADoorIsMintedUnderTheDungeonsKeyInItsAuthoredState() {
	c := s.load(s.tomb)
	s.Require().Len(c.Field.Doors, 2)

	open := c.Field.Doors[0]
	s.Equal(encounter.DoorID("reference-tomb/entrance-hall"), open.ID)
	s.Equal(encounter.DoorOpen, open.State.Kind(), "no lock and not closed: an open doorway")
	s.Equal([]encounter.DoorEdge{{
		From: encounter.HexCellAt(encounter.HexesArePointyTop(), 5, 4),
		To:   encounter.HexCellAt(encounter.HexesArePointyTop(), 6, 4),
	}}, open.Edges, "edges are the absolute axial cells DoorInput takes — the one conversion this package makes")

	locked := c.Field.Doors[1]
	s.Equal(encounter.DoorID("reference-tomb/hall-tomb"), locked.ID)
	s.Equal(encounter.DoorLocked, locked.State.Kind())
	lock, ok := locked.State.Lock()
	s.Require().True(ok)
	s.Equal(encounter.Lock{DC: 12, Ability: "dex"}, lock)

	shut := s.load(s.tombWith("    locked: { dc: 12, ability: dex }", "    closed: true"))
	s.Equal(encounter.DoorClosed, shut.Field.Doors[1].State.Kind(), "closed: shut but not locked")
}

// TestCompile_DoorInsideARegionIsLegal — a door need not sit on a seam.
func (s *CompileSuite) TestCompile_DoorInsideARegionIsLegal() {
	c := s.load(s.tombWith("doors:\n", "doors:\n  - id: inner\n    edges: [[[8,3],[9,3]]]\n    closed: true\n"))
	s.Require().Len(c.Field.Doors, 3)
	s.Equal(encounter.DoorID("reference-tomb/inner"), c.Field.Doors[0].ID)

	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		Field:   c.Field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err, "and the composition accepts it: a door is an edge between two floor cells, wherever they are")
}

// TestAPropCarriesBothAnswersAndOwnsThem.
//
// The copy is the point of the second half. [encounter.PropInput] holds the two
// flags as POINTERS, so a compiled field pointing at the spec's own bools would
// change behaviour under a caller who edited the spec afterwards.
func (s *CompileSuite) TestAPropCarriesBothAnswersAndOwnsThem() {
	c := s.load(s.tomb)

	byRef := map[string]encounter.PropInput{}
	for _, p := range c.Field.Props {
		byRef[p.Ref] = p
	}
	s.Require().Len(c.Field.Props, 15)
	s.Require().Contains(byRef, "dnd5e:props:coffin")
	s.True(*byRef["dnd5e:props:coffin"].BlocksMovement, "you do not walk through a coffin")
	s.False(*byRef["dnd5e:props:coffin"].BlocksLineOfSight, "and you see over one")
	s.True(*byRef["dnd5e:props:pillar"].BlocksLineOfSight, "a pillar is both")
	s.False(*byRef["dnd5e:props:candles"].BlocksMovement, "and candles are neither")
	s.Equal(spatial.Position{X: 22, Y: 3}, byRef["dnd5e:props:coffin"].At, "absolute, in the authored frame")

	spec, err := dungeonspec.Decode([]byte(s.tomb))
	s.Require().NoError(err)
	s.Require().Empty(dungeonspec.Validate(spec))
	compiled, err := dungeonspec.Compile(spec)
	s.Require().NoError(err)

	for i := range spec.Place {
		if spec.Place[i].BlocksLoS != nil {
			*spec.Place[i].BlocksLoS = !*spec.Place[i].BlocksLoS
		}
		if spec.Place[i].BlocksMovement != nil {
			*spec.Place[i].BlocksMovement = !*spec.Place[i].BlocksMovement
		}
	}
	*spec.Regions[0].Lighting.Intensity = 0

	var coffins int
	for _, p := range compiled.Field.Props {
		if p.Ref != "dnd5e:props:coffin" {
			continue
		}
		coffins++
		s.False(*p.BlocksLineOfSight, "editing the spec must not reach into a compiled field")
		s.True(*p.BlocksMovement)
	}
	s.Equal(1, coffins, "and the tomb has exactly one to check")
	s.Equal(0.6, compiled.Field.Regions[0].Lighting.Intensity, "nor into a region's lighting")
}

// TestMonstersComeOutInOrderAndUninterpreted — the roster half, which this
// package may carry and may not read (design law C1).
func (s *CompileSuite) TestMonstersComeOutInOrderAndUninterpreted() {
	c := s.load(s.tomb)

	s.Require().Len(c.Monsters, 3)
	s.Equal([]string{"hall", "hall", "tomb"},
		[]string{c.Monsters[0].Region, c.Monsters[1].Region, c.Monsters[2].Region},
		"the order the author wrote them, each naming the region its cell is in")

	s.Equal("dnd5e:monsters:skeleton", c.Monsters[0].Ref)
	s.Equal(spatial.Position{X: 11, Y: 3}, c.Monsters[0].At, "absolute, in the authored frame")
	s.Equal("lowest-health", c.Monsters[0].Targeting, "the author's word, uninterpreted")
	s.False(c.Monsters[0].Boss)

	s.Equal("dnd5e:monsters:skeleton-captain", c.Monsters[2].Ref)
	s.True(c.Monsters[2].Boss, "and the flag says which one ends things")

	none := s.load(s.tombWith("at: [11,3], targeting: lowest-health", "at: [11,3]"))
	s.Empty(none.Monsters[0].Targeting, "a monster that says nothing carries nothing")
}

// TestThePartyHasSomewhereToStand — the author declares ONE cell; what a party
// needs is several, so the ordering is the whole value of the answer.
func (s *CompileSuite) TestThePartyHasSomewhereToStand() {
	c := s.load(s.tomb)

	s.Require().NotEmpty(c.PartyStart)
	s.Equal("entrance", c.PartyStart[0].Region, "resolved from the floor, not declared beside it")
	s.Equal(spatial.Position{X: 1, Y: 3}, c.PartyStart[0].At, "seat 0 is the cell they wrote")

	// The entrance is 48 cells with two braziers in it.
	s.Len(c.PartyStart, 48-2, "every free cell of the region is a seat")

	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})
	from := encounter.HexCellAt(encounter.HexesArePointyTop(), 1, 3)
	last := 0.0
	for _, seat := range c.PartyStart {
		s.Equal("entrance", seat.Region, "the party arrives together")
		s.NotEqual(spatial.Position{X: 1, Y: 1}, seat.At, "nobody is seated on a brazier")
		s.NotEqual(spatial.Position{X: 1, Y: 6}, seat.At)

		d := grid.Distance(from, encounter.HexCellAt(
			encounter.HexesArePointyTop(), int(seat.At.X), int(seat.At.Y)))
		s.GreaterOrEqual(d, last, "nearest first, so the first four are four people standing together")
		last = d
	}

	s.Equal(c.PartyStart, s.load(s.tomb).PartyStart,
		"and the same every time — a roster that reshuffled between loads would move people for no reason")
}

// TestABadSpecNeverReachesTheCompiler — Load is Decode then Validate then
// build, and the refusals belong to the first two, as one ErrBadSpec that
// carries every defect.
func (s *CompileSuite) TestABadSpecNeverReachesTheCompiler() {
	_, err := dungeonspec.Load([]byte(s.tombWith("void: opaque", "void: fog")))
	s.Require().ErrorIs(err, dungeonspec.ErrBadSpec)

	var verr *dungeonspec.ValidationError
	s.Require().ErrorAs(err, &verr)
	s.Require().Len(verr.Errors, 1)
	s.Equal("void", verr.Errors[0].Path)
	s.Contains(err.Error(), "fog")
}
