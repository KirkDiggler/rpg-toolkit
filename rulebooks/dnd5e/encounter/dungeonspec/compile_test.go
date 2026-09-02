// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// compile_test.go pins what the compiler BUILDS from a version-2 file: the
// regions verbatim, the walls and props at their absolute cells, the doors in
// their crossings with the state the file gives them, and the two halves it
// hands back. The forcing case — that the re-authored tomb is the tomb — is
// golden_test.go's; this file is about the shape of each piece.

import (
	"fmt"
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

// TestGroupingHasNoMechanicalConsequence is the ruling of rpg-project#355 as a
// pin: a group is an AUTHORING fact and nothing else, so the same edges in the
// same order compile to the identical wall set however they are bracketed.
// Everything downstream — the engine, the renderer's own derived runs — is
// entitled to never learn a group existed.
func (s *CompileSuite) TestGroupingHasNoMechanicalConsequence() {
	flat := s.load(s.tomb).Field.Walls
	s.Require().Len(flat, 28)

	for _, runs := range []int{1, 2, 5, 28} {
		s.Run(fmt.Sprintf("%d runs", runs), func() {
			raw := regroupedTomb(s.T(), s.tomb, runs, false)
			spec, err := dungeonspec.Decode([]byte(raw))
			s.Require().NoError(err)
			s.Require().Empty(dungeonspec.Validate(spec))
			s.Require().Len(spec.Walls, runs, "the FILE really is grouped differently")
			s.Equal(flat, s.load(raw).Field.Walls, "and the atlas cannot tell")
		})
	}
}

// TestADoorStandsInAWall — rpg-project#355 reverses "a door cannot stand in a
// wall". The author writes the run unbroken and lists the door separately; the
// compiler subtracts the door's edges, so the wall set is what it always was.
//
// The contrast is the point: the spec carries 30 wall edges and the atlas gets
// 28, so a compiler that forgot to subtract would fail here rather than pass
// on a fixture with nothing to subtract.
func (s *CompileSuite) TestADoorStandsInAWall() {
	raw := regroupedTomb(s.T(), s.tomb, 2, true)
	spec, err := dungeonspec.Decode([]byte(raw))
	s.Require().NoError(err)
	s.Empty(dungeonspec.Validate(spec), "a door in a wall is the ordinary case now")

	authored := 0
	for _, w := range spec.Walls {
		authored += len(w.Edges)
	}
	s.Require().Equal(30, authored, "the file claims both door crossings as wall")

	c := s.load(raw)
	s.Len(c.Field.Walls, 28, "and the compiler hands each one back to its door")
	s.Equal(s.load(s.tomb).Field.Walls, c.Field.Walls,
		"an unbroken run with its doors named compiles to the punched-out list")

	for _, w := range c.Field.Walls {
		for _, d := range [][2]spatial.Position{
			{{X: 5, Y: 4}, {X: 6, Y: 4}}, {{X: 15, Y: 4}, {X: 16, Y: 4}},
		} {
			s.False(w.From == d[0] && w.To == d[1], "no wall stands where a door does")
		}
	}
}

// TestAConcealedDoorInAWallIsStillTheWayIn is the interaction that makes "a
// door stands in a wall" safe rather than merely convenient.
//
// The coherence check asks of every crossing between two regions whether it is
// a way in, and SKIPS the ones a wall claims — "a wall is not a way in". Once a
// wall run may name the very edge a door stands in, which of the two claims
// that crossing decides whether the secret's only entrance is seen at all. The
// door's claim wins (validate runs walls before doors, deliberately), so the
// frontier still reads a door here.
//
// The second half is the part that can fail: uncanceal the door and the hole
// MUST be reported. A build where the wall's claim won would skip this
// crossing, find no way into the tomb at all, and pass silently.
func (s *CompileSuite) TestAConcealedDoorInAWallIsStillTheWayIn() {
	const find = "\n    concealed: [{ ability: perception, dc: 15 }]"
	secret := regroupedTomb(s.T(), s.secretTomb(find), 2, true)
	s.Require().Contains(secret, "      - [[15,4],[16,4]]",
		"the hall-tomb run reclaims the edge its concealed door stands in")

	spec, err := dungeonspec.Decode([]byte(secret))
	s.Require().NoError(err)
	s.Empty(dungeonspec.Validate(spec),
		"the secret is coherent: its one way in is the concealed door in the wall")

	open := strings.Replace(secret, find, "", 1)
	s.Require().NotEqual(secret, open)
	spec, err = dungeonspec.Decode([]byte(open))
	s.Require().NoError(err)
	errs := dungeonspec.Validate(spec)
	s.Require().NotEmpty(errs, "a plain door into a concealed room is a hole in the secret")
	s.Equal("regions[2].concealed", errs[0].Path,
		"and the frontier found it THROUGH the wall run that names the same edge")
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
	s.Equal(encounter.Lock{Approaches: []encounter.CheckApproach{{Ability: "dex", DC: 12}}}, lock,
		"the authored approach list, carried whole")

	s.Nil(open.Concealed, "the tomb conceals neither door — nil is what \"said nothing\" always means")
	s.Nil(locked.Concealed)

	shut := s.load(s.tombWith("    locked: [{ ability: dex, dc: 12 }]", "    closed: true"))
	s.Equal(encounter.DoorClosed, shut.Field.Doors[1].State.Kind(), "closed: shut but not locked")
}

// TestALockMayListSeveralApproaches — the multi-approach ruling
// (rpg-project#350): success by any listed route, each priced separately,
// carried whole and in authored order. The compiler never learns which route
// an attempt would take, or what any of the words mean.
func (s *CompileSuite) TestALockMayListSeveralApproaches() {
	c := s.load(s.tombWith("locked: [{ ability: dex, dc: 12 }]",
		`locked: [{ ability: str, dc: 15 }, { ability: dex, tool: "dnd5e:item:thieves-tools", dc: 12 }]`))
	lock, ok := c.Field.Doors[1].State.Lock()
	s.Require().True(ok)
	s.Equal(encounter.Lock{Approaches: []encounter.CheckApproach{
		{Ability: "str", DC: 15},
		{Ability: "dex", Tool: "dnd5e:item:thieves-tools", DC: 12},
	}}, lock, "forcing the door and picking its lock need not cost the same")
}

// secretTomb is the shipping tomb with its crypt made a coherent secret:
// the hall-tomb door concealed behind the find check, and the tomb region
// declared concealed with it — the two facts the coherence check refuses to
// see apart (rpg-project#351).
func (s *CompileSuite) secretTomb(find string) string {
	secret := s.tombWith("    locked: [{ ability: dex, dc: 12 }]",
		"    locked: [{ ability: dex, dc: 12 }]"+find)
	s.Require().Contains(secret, "  - id: tomb\n")
	return strings.Replace(secret, "  - id: tomb\n", "  - id: tomb\n    concealed: true\n", 1)
}

// TestAConcealedDoorCarriesItsFindCheckThrough — living-world slice 1, wave 1a
// (rpg-toolkit#1369): concealment is one more property on the door
// declaration, its find check an approach list in the same shape as a lock's,
// carried opaquely to [encounter.DoorInput.Concealed]. It COMPOSES with
// plain, closed, or locked underneath — the state switch never reads it and
// it never reads the state.
func (s *CompileSuite) TestAConcealedDoorCarriesItsFindCheckThrough() {
	const find = "\n    concealed: [{ ability: perception, dc: 15 }, { ability: investigation, dc: 12 }]"
	wantFind := []encounter.CheckApproach{
		{Ability: "perception", DC: 15},
		{Ability: "investigation", DC: 12},
	}

	hidden := s.load(s.secretTomb(find))
	s.Equal(encounter.DoorLocked, hidden.Field.Doors[1].State.Kind(),
		"concealment does not displace the lock underneath")
	s.Equal(wantFind, hidden.Field.Doors[1].Concealed,
		"the find approaches, in authored order, priced per route")
	s.Nil(hidden.Field.Doors[0].Concealed, "and the doorway beside it is untouched")

	plain := s.load(s.tombWith("doors:\n",
		"doors:\n  - id: shortcut\n    edges: [[[8,3],[9,3]]]"+find+"\n"))
	s.Equal(encounter.DoorOpen, plain.Field.Doors[0].State.Kind(),
		"a concealed door can stand open underneath — a hidden passage nobody shut")
	s.Equal(wantFind, plain.Field.Doors[0].Concealed)

	shut := s.load(strings.Replace(s.secretTomb(find),
		"    locked: [{ ability: dex, dc: 12 }]", "    closed: true", 1))
	s.Equal(encounter.DoorClosed, shut.Field.Doors[1].State.Kind())
	s.Equal(wantFind, shut.Field.Doors[1].Concealed)
}

// TestARegionHidesWithItsDoor — the region half of the same slice
// (rpg-project#351): the concealed marker is declared on the region, carried
// opaquely to [encounter.RegionInput.Concealed], and never cascaded — the
// door beside it stays exactly the door the author wrote.
func (s *CompileSuite) TestARegionHidesWithItsDoor() {
	c := s.load(s.secretTomb("\n    concealed: [{ ability: perception, dc: 15 }]"))
	s.Equal([]bool{false, false, true},
		[]bool{c.Field.Regions[0].Concealed, c.Field.Regions[1].Concealed, c.Field.Regions[2].Concealed},
		"the tomb is the secret; the rooms in front of it say nothing")

	base := s.load(s.tomb)
	for i, r := range base.Field.Regions {
		s.False(r.Concealed, "regions[%d] of the shipping tomb authors no concealment", i)
	}
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
