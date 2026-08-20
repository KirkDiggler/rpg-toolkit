// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// compile_test.go is WHAT THE COMPILER BUILT (rpg-toolkit#1127's geometry
// half).
//
// tomb_test.go asserts that the SCENE happens and deliberately hardcodes almost
// no cell, so that it survives a change of grid family or doorway rule. The
// price of that is that it cannot tell a compiler that puts the chambers in the
// right places from one that puts them all in the wrong places consistently —
// a walk through a dungeon whose every anchor is off by one is still a walk.
// This file is the other half: the arithmetic, pinned.
//
// Every case here builds the tomb or a small variant of it, because a fixture
// invented for a test is a fixture nobody authors.

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
}

func TestCompileSuite(t *testing.T) {
	suite.Run(t, new(CompileSuite))
}

func (s *CompileSuite) load(raw string) dungeonspec.Compiled {
	compiled, err := dungeonspec.Load([]byte(raw))
	s.Require().NoError(err)

	return compiled
}

// TestTheChambersSitInARow — the whole layout language, and the reason
// declaration order is geometry rather than presentation.
func (s *CompileSuite) TestTheChambersSitInARow() {
	c := s.load(tombYAML)

	s.Require().Len(c.Field.Rooms, 3)
	for i, want := range []struct {
		id    string
		col   float64
		width int
	}{{"entrance", 0, 6}, {"hall", 6, 10}, {"tomb", 16, 12}} {
		room := c.Field.Rooms[i]
		s.Equal(want.id, room.ID)
		s.Equal(want.width, room.Width)
		s.Equal(8, room.Height, "every chamber is the dungeon's height")
		s.Equal(spatial.Position{X: want.col, Y: 0}, room.Origin,
			"chamber %d starts where the ones before it end", i)
		s.Equal(spatial.GridShapeHex, room.Grid)
	}
}

// TestTheFieldCarriesWhatTheAuthorDeclared — the two the composition refuses to
// pick for itself (rpg-toolkit#1116 and #1033), carried through rather than
// re-decided.
func (s *CompileSuite) TestTheFieldCarriesWhatTheAuthorDeclared() {
	s.Equal(encounter.VoidOpaque, s.load(tombYAML).Field.Canvas.Void.Kind())
	s.Equal(encounter.OrientationPointyTop, s.load(tombYAML).Field.Canvas.Orientation.Kind())

	flat := s.load(tombWith("orientation: pointy", "orientation: flat"))
	s.Equal(encounter.OrientationFlatTop, flat.Field.Canvas.Orientation.Kind(),
		"both layouts are settable — Kirk's ruling, and the mask is what made it reachable")

	clear := s.load(tombWith("void: opaque", "void: transparent"))
	s.Equal(encounter.VoidTransparent, clear.Field.Canvas.Void.Kind())
}

// TestASeamIsWalledExceptWhereItOpens is the wall the compiler draws, which is
// the thing nothing in the authored file says and everything in the scene
// depends on.
//
// Asserted against the crossings the GRID says exist rather than a count, so it
// cannot pass by drawing the right number of wrong edges. Both orientations,
// because which pairs are adjacent is exactly what changes between them.
func (s *CompileSuite) TestASeamIsWalledExceptWhereItOpens() {
	for _, layout := range []struct {
		name string
		o    encounter.Orientation
		yaml string
	}{
		{"pointy-top", encounter.HexesArePointyTop(), tombYAML},
		{"flat-top", encounter.HexesAreFlatTop(), tombWith("orientation: pointy", "orientation: flat")},
	} {
		s.Run(layout.name, func() {
			c := s.load(layout.yaml)
			grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})

			// The entrance's east wall: it is 6 wide, so its own last column
			// is 5 and the hall's first is 6.
			walled := map[[2][2]int]bool{}
			for _, b := range c.Field.Rooms[0].Boundaries {
				s.True(b.BlocksMovement, "a seam wall stops a step")
				s.True(b.BlocksLineOfSight, "and a sightline")
				walled[[2][2]int{
					{int(b.From.X), int(b.From.Y)}, {int(b.To.X), int(b.To.Y)},
				}] = true
			}

			opening := 0
			for row := 0; row < 8; row++ {
				for to := 0; to < 8; to++ {
					near := encounter.HexCellAt(layout.o, 5, row)
					far := encounter.HexCellAt(layout.o, 6, to)
					crossing := [2][2]int{{5, row}, {6, to}}

					if grid.Distance(near, far) != 1 {
						s.False(walled[crossing],
							"[5,%d]-[6,%d] is not a crossing on this grid and must not be walled", row, to)
						continue
					}
					if walled[crossing] {
						continue
					}
					opening++
					s.Equal(4, row, "the only crossing left open is the doorway, on row height/2")
					s.Equal(4, to)
				}
			}
			s.Equal(1, opening, "one opening, and it is one edge wide")
		})
	}
}

// TestASeamWithNoConnectorIsSolid — the wall is drawn because two chambers
// touch, and the opening because a connector says so. Two facts, not one.
func (s *CompileSuite) TestASeamWithNoConnectorIsSolid() {
	c := s.load(withoutLine("  - { from: entrance, to: hall }"))

	for _, b := range c.Field.Rooms[0].Boundaries {
		s.True(b.BlocksMovement)
	}
	s.Len(c.Field.Rooms[0].Boundaries, len(s.load(tombYAML).Field.Rooms[0].Boundaries)+1,
		"exactly one more edge than the tomb's — the one its connector opens")
	s.Len(c.Field.Connections, 1, "and no connection through it")
}

// TestAnOpeningIsAConnectionAndALockIsADoor.
//
// An open connector compiles to a name and nothing else — crossing it is an
// ordinary step, and the connection exists so a step can be NAMED. A locked one
// compiles to the same name plus a door standing in that one crossing. The door
// is what refuses; the connection still decides nothing.
func (s *CompileSuite) TestAnOpeningIsAConnectionAndALockIsADoor() {
	c := s.load(tombYAML)

	s.Require().Len(c.Field.Connections, 2)
	s.Require().Len(c.Field.Doors, 1, "one of the two connectors is locked")

	arch := c.Field.Connections[0]
	s.Equal("entrance", arch.From)
	s.Equal("hall", arch.To)
	s.Equal(spatial.Position{X: 5, Y: 4}, arch.FromPosition, "the entrance's own last column, on the doorway row")
	s.Equal(spatial.Position{X: 0, Y: 4}, arch.ToPosition, "and the hall's first")

	door := c.Field.Doors[0]
	s.Equal(c.Field.Connections[1].ID, door.ID, "the door is named for the connector it stands in")
	s.Require().Len(door.Edges, 1)
	s.Equal(encounter.HexCellAt(encounter.HexesArePointyTop(), 15, 4), door.Edges[0].From,
		"absolute, because a door spans two chambers and belongs to neither")
	s.Equal(encounter.HexCellAt(encounter.HexesArePointyTop(), 16, 4), door.Edges[0].To)

	lock, locked := door.State.Lock()
	s.Require().True(locked)
	s.Equal(12, lock.DC, "the DC the author wrote")
	s.Equal("dex", lock.Ability, "and the ability, carried opaquely")
}

// TestAConnectorMayBeWrittenEitherWayRound — `from` and `to` are the author's
// words for two chambers, not a direction of travel, and which of them is west
// is the room list's business.
func (s *CompileSuite) TestAConnectorMayBeWrittenEitherWayRound() {
	c := s.load(tombWith("  - { from: entrance, to: hall }", "  - { from: hall, to: entrance }"))

	arch := c.Field.Connections[0]
	s.Equal("hall", arch.From)
	s.Equal(spatial.Position{X: 0, Y: 4}, arch.FromPosition, "the hall is east, so it contributes its first column")
	s.Equal("entrance", arch.To)
	s.Equal(spatial.Position{X: 5, Y: 4}, arch.ToPosition, "and the entrance its last")
}

// TestTheSameSeamMintsTheSameIDEitherWayRound — a door's identity is the SEAM,
// not the sentence that declared it.
//
// [Validate] already treats a connector as undirected: it canonicalises the
// pair by room index before de-duplicating, so the same seam declared twice is
// refused whichever way round either line was written. The minted ID has to
// agree with that, because a door's STATE persists under its ID
// (rpg-toolkit#1123) — if the ID followed the author's words, swapping two of
// them would orphan every door a party had already opened, silently and at
// load time. Found by Copilot on rpg-toolkit#1133.
func (s *CompileSuite) TestTheSameSeamMintsTheSameIDEitherWayRound() {
	canonical := s.load(tombYAML)
	swapped := s.load(tombWith(
		"  - { from: hall, to: tomb, locked: { dc: 12, ability: dex } }",
		"  - { from: tomb, to: hall, locked: { dc: 12, ability: dex } }"))

	s.Require().Len(swapped.Field.Doors, 1)
	s.Equal(canonical.Field.Doors[0].ID, swapped.Field.Doors[0].ID,
		"the locked seam is ONE door, however the line was written")

	s.Require().Len(swapped.Field.Connections, 2)
	s.Equal(canonical.Field.Connections[1].ID, swapped.Field.Connections[1].ID,
		"and one connection")

	s.Equal("tomb", swapped.Field.Connections[1].From,
		"while the author's own words survive on the endpoints, which is where they belong")
}

// TestAPropCarriesBothAnswersAndOwnsThem.
//
// The copy is the point of the second half. [encounter.PropInput] holds the two
// flags as POINTERS, so a compiled field pointing at the spec's own bools would
// change behaviour under a caller who edited the spec afterwards — the aliasing
// defect rpg-toolkit#1128 found one indirection down, available again here.
func (s *CompileSuite) TestAPropCarriesBothAnswersAndOwnsThem() {
	c := s.load(tombYAML)

	byRef := map[string]encounter.PropInput{}
	for _, room := range c.Field.Rooms {
		for _, p := range room.Props {
			byRef[p.Ref] = p
		}
	}

	s.Require().Contains(byRef, "dnd5e:props:coffin")
	s.True(*byRef["dnd5e:props:coffin"].BlocksMovement, "you do not walk through a coffin")
	s.False(*byRef["dnd5e:props:coffin"].BlocksLineOfSight, "and you see over one")
	s.True(*byRef["dnd5e:props:pillar"].BlocksLineOfSight, "a pillar is both")
	s.False(*byRef["dnd5e:props:candles"].BlocksMovement, "and candles are neither")

	s.Equal("dnd5e:props:brazier", c.Field.Rooms[0].Props[0].Ref, "a prop keeps the ref content gave it")
	s.Equal(spatial.Position{X: 1, Y: 1}, c.Field.Rooms[0].Props[0].At, "room-local, in the authored frame")

	// The aliasing itself, asked of the SAME spec the field was compiled from
	// — which is what [dungeonspec.Compile] being separately available makes
	// possible, and is the only way to ask: through Load the spec is internal
	// and no caller could hold the bools to flip them.
	spec, err := dungeonspec.Decode([]byte(tombYAML))
	s.Require().NoError(err)
	s.Require().NoError(dungeonspec.Validate(spec))
	compiled, err := dungeonspec.Compile(spec)
	s.Require().NoError(err)

	for _, room := range spec.Rooms {
		for i := range room.Place {
			if room.Place[i].BlocksLoS != nil {
				*room.Place[i].BlocksLoS = !*room.Place[i].BlocksLoS
			}
			if room.Place[i].BlocksMovement != nil {
				*room.Place[i].BlocksMovement = !*room.Place[i].BlocksMovement
			}
		}
	}

	var coffins int
	for _, room := range compiled.Field.Rooms {
		for _, p := range room.Props {
			if p.Ref != "dnd5e:props:coffin" {
				continue
			}
			coffins++
			s.False(*p.BlocksLineOfSight, "editing the spec must not reach into a compiled field")
			s.True(*p.BlocksMovement)
		}
	}
	s.Equal(1, coffins, "and the tomb has exactly one to check")
}

// TestMonstersComeOutInOrderAndUninterpreted — the roster half, which this
// package may carry and may not read (design law C1).
func (s *CompileSuite) TestMonstersComeOutInOrderAndUninterpreted() {
	c := s.load(tombYAML)

	s.Require().Len(c.Monsters, 3)
	s.Equal([]string{"hall", "hall", "tomb"},
		[]string{c.Monsters[0].Region, c.Monsters[1].Region, c.Monsters[2].Region},
		"chamber order, then the order the author wrote them")

	s.Equal("dnd5e:monsters:skeleton", c.Monsters[0].Ref)
	s.Equal(spatial.Position{X: 5, Y: 3}, c.Monsters[0].At, "room-local, in the authored frame")
	s.Equal("lowest-health", c.Monsters[0].Targeting, "the author's word, uninterpreted")
	s.False(c.Monsters[0].Boss)

	s.Equal("dnd5e:monsters:skeleton-captain", c.Monsters[2].Ref)
	s.True(c.Monsters[2].Boss, "and the flag says which one ends things")

	none := s.load(tombWith(", targeting: lowest-health", ""))
	s.Empty(none.Monsters[0].Targeting, "a monster that says nothing carries nothing")
}

// TestThePartyHasSomewhereToStand.
//
// The census (rpg-project#227) recorded "no party-start seat at the seam" as a
// gap, and Kirk ruled the start becomes explicit. What the author declares is
// ONE cell; what a party needs is several, so the ordering is the whole value
// of the answer.
func (s *CompileSuite) TestThePartyHasSomewhereToStand() {
	c := s.load(tombYAML)

	s.Require().NotEmpty(c.PartyStart)
	s.Equal("entrance", c.PartyStart[0].Region, "resolved from the cell, not declared beside it")
	s.Equal(spatial.Position{X: 1, Y: 3}, c.PartyStart[0].At, "seat 0 is the cell they wrote")

	// The entrance is 6x8 with two braziers in it.
	s.Len(c.PartyStart, 6*8-2, "every free cell of the chamber is a seat")

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

	s.Equal(c.PartyStart, s.load(tombYAML).PartyStart,
		"and the same every time — a roster that reshuffled between loads would move people for no reason")
}

// TestABadSpecNeverReachesTheCompiler — Load is Decode then Validate then
// build, and the refusals belong to the first two.
func (s *CompileSuite) TestABadSpecNeverReachesTheCompiler() {
	for _, tc := range []struct{ name, yaml, wants string }{
		{"unreadable", "version: [1]", "cannot unmarshal"},
		{"not a dungeon", tombWith("void: opaque", "void: soup"), "void"},
	} {
		s.Run(tc.name, func() {
			_, err := dungeonspec.Load([]byte(tc.yaml))
			s.Require().ErrorIs(err, dungeonspec.ErrBadSpec)
			s.Contains(strings.ToLower(err.Error()), tc.wants)
		})
	}
}
