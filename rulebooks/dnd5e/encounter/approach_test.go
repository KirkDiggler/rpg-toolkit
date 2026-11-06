// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// approach_test.go pins [encounter.Encounter.Approaches] (rpg-project#872,
// D4): where a mover's own walk ends to have a target in reach and in sight,
// within a budget. THE LAW: the cell named is one a walk of that path may end
// on — Standable, past ValidateWalkEnd, inside the budget — and from it the
// target is within reach and seen; the cheapest such cell, ties in scan
// order.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ApproachesSuite struct{ suite.Suite }

func TestApproachesSuite(t *testing.T) { suite.Run(t, new(ApproachesSuite)) }

// approachCaps is the read-only capability set every scene here installs.
func approachCaps() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Actors: encounter.Actors{
			Striker:   passStriker{},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// room is one ten-by-ten lit room with the members given.
func (s *ApproachesSuite) room(members ...encounter.MemberInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 10, 10)}},
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: approachCaps(),
	})
	s.Require().NoError(err)
	return enc
}

// corridor is one row of ten cells: nowhere to go round anybody.
func (s *ApproachesSuite) corridor(members ...encounter.MemberInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("corridor", 0, 0, 10, 1)}},
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: approachCaps(),
	})
	s.Require().NoError(err)
	return enc
}

// twoRooms is room-a (columns 0–9) and room-b (10–19) with a wall between
// them, open only at row 5, where a door in the given state stands.
func (s *ApproachesSuite) twoRooms(door encounter.DoorState, members ...encounter.MemberInput) *encounter.Encounter {
	doorway := openDoorway("door1", 9, 5, 10, 5)
	doorway.State = door
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-a", 0, 0, 10, 10), rectRegion("room-b", 10, 0, 10, 10)},
			Walls:   hexOffsetSeamWall(encounter.HexesArePointyTop(), 9, 0, 9, 5),
			Doors:   []encounter.DoorInput{doorway},
		},
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: approachCaps(),
	})
	s.Require().NoError(err)
	return enc
}

// approachPlayer and approachMonster seat a member at an authored [col,row]
// pair; the encounter reports it back as [cellAt].
func approachPlayer(id encounter.MemberID, col, row int) encounter.MemberInput {
	return encounter.MemberInput{ID: id, Kind: encounter.KindPlayer, Position: spatial.Position{X: float64(col), Y: float64(row)}}
}

func approachMonster(id encounter.MemberID, col, row int) encounter.MemberInput {
	return encounter.MemberInput{ID: id, Kind: encounter.KindMonster, Position: spatial.Position{X: float64(col), Y: float64(row)}}
}

// one asks for a single target and returns its row.
func (s *ApproachesSuite) one(enc *encounter.Encounter, in encounter.ApproachesInput) encounter.Approach {
	out, err := enc.Approaches(in)
	s.Require().NoError(err)
	s.Require().Len(out.Approaches, len(in.Targets))
	return out.Approaches[0]
}

// holdsTheLaw checks a reached row against the law, from the mover's cell.
func (s *ApproachesSuite) holdsTheLaw(enc *encounter.Encounter, mover encounter.MemberID, got encounter.Approach, reachCells float64) {
	s.Require().True(got.Reached)
	from := whereIs(s.T(), enc, mover)
	holdsTheLaw(s.T(), enc, mover, from, got, reachCells)
}

func holdsTheLaw(t *testing.T, enc *encounter.Encounter, mover encounter.MemberID, from spatial.Position, got encounter.Approach, reachCells float64) {
	t.Helper()
	require.NotEmpty(t, got.Path)
	require.Equal(t, got.Cell, got.Path[len(got.Path)-1], "the path ends on the cell")
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})
	here := from
	for i, cell := range got.Path {
		require.True(t, grid.IsAdjacent(here, cell), "step %d from %v to %v is adjacent", i+1, here, cell)
		fact, err := enc.CellAt(encounter.CellAtInput{Cell: cell, Mover: mover})
		require.NoError(t, err)
		require.NotEqual(t, encounter.PassageBlocked, fact.Passage, "step %d onto %v may be entered", i+1, cell)
		here = cell
	}
	fact, err := enc.CellAt(encounter.CellAtInput{Cell: got.Cell, Mover: mover})
	require.NoError(t, err)
	require.Equal(t, encounter.PassageStandable, fact.Passage, "the walk may end on %v", got.Cell)
	require.NoError(t, enc.ValidateWalkEnd(encounter.CellAtInput{Mover: mover, Cell: got.Cell}))
	at := whereIs(t, enc, got.Target)
	require.LessOrEqual(t, enc.Distance(got.Cell, at), reachCells, "the target is in reach of %v", got.Cell)
	seen, err := enc.ExportedSeesFrom(mover, got.Cell, got.Target)
	require.NoError(t, err)
	require.True(t, seen, "the target is seen from %v", got.Cell)
}

func (s *ApproachesSuite) TestATargetInReachNeedsNoWalk() {
	enc := s.room(approachPlayer(alice, 3, 3), approachMonster(goblin, 4, 3))
	got := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true})
	s.True(got.Reached)
	s.Equal(whereIs(s.T(), enc, alice), got.Cell)
	s.Empty(got.Path)
}

func (s *ApproachesSuite) TestOpenFloorEndsBesideTheTargetByTheShortestWalk() {
	enc := s.corridor(approachPlayer(alice, 0, 0), approachMonster(goblin, 6, 0))
	got := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true})
	s.holdsTheLaw(enc, alice, got, 1)
	s.Equal(cellAt(5, 0), got.Cell)
	s.Len(got.Path, 5)
}

func (s *ApproachesSuite) TestAReachWeaponStopsFarther() {
	enc := s.corridor(approachPlayer(alice, 0, 0), approachMonster(goblin, 6, 0))
	got := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 10, Unbounded: true})
	s.holdsTheLaw(enc, alice, got, 2)
	s.Equal(cellAt(4, 0), got.Cell, "ten feet is two cells short of him")
	s.Len(got.Path, 4)
}

func (s *ApproachesSuite) TestTheBudgetBoundsTheWalk() {
	enc := s.corridor(approachPlayer(alice, 0, 0), approachMonster(goblin, 6, 0))
	targets := []encounter.MemberID{goblin}
	short := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: targets, RangeFeet: 5, Budget: 4})
	s.False(short.Reached, "four steps do not reach a walk of five")
	s.Empty(short.Path)
	exact := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: targets, RangeFeet: 5, Budget: 5})
	s.holdsTheLaw(enc, alice, exact, 1)
	s.Equal(cellAt(5, 0), exact.Cell)
	none := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: targets, RangeFeet: 5})
	s.False(none.Reached, "the zero value is a walk that goes nowhere")
}

func (s *ApproachesSuite) TestAZeroBudgetStillAnswersInReach() {
	enc := s.corridor(approachPlayer(alice, 0, 0), approachMonster(goblin, 1, 0), approachMonster("orc", 5, 0))
	out, err := enc.Approaches(encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin, "orc"}, RangeFeet: 5})
	s.Require().NoError(err)
	s.Require().Len(out.Approaches, 2)
	s.Equal(goblin, out.Approaches[0].Target)
	s.True(out.Approaches[0].Reached)
	s.Empty(out.Approaches[0].Path)
	s.Equal(encounter.MemberID("orc"), out.Approaches[1].Target)
	s.False(out.Approaches[1].Reached)
}

func (s *ApproachesSuite) TestAnAllyIsCrossedNeverStoodOn() {
	enc := s.corridor(approachPlayer(alice, 0, 0), approachPlayer("bob", 3, 0), approachMonster(goblin, 5, 0))
	got := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true})
	s.holdsTheLaw(enc, alice, got, 1)
	s.Equal(cellAt(4, 0), got.Cell)
	s.Contains(got.Path, cellAt(3, 0), "the walk crosses bob")

	// Bob beside the goblin: the only cell in reach is his, and nobody ends
	// on an ally.
	blocked := s.corridor(approachPlayer(alice, 0, 0), approachPlayer("bob", 4, 0), approachMonster(goblin, 5, 0))
	none := s.one(blocked, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true})
	s.False(none.Reached)
}

func (s *ApproachesSuite) TestAHostileInTheWayClosesIt() {
	enc := s.corridor(approachPlayer(alice, 0, 0), approachMonster("orc", 3, 0), approachMonster(goblin, 6, 0))
	got := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true})
	s.False(got.Reached, "a hostile's cell is not crossed")
}

func (s *ApproachesSuite) TestAWallIsNotReachTheWalkGoesRound() {
	// The goblin stands just past the wall from alice: the cell beside him
	// on her side is within five feet and across a wall, so the walk goes
	// through the doorway.
	enc := s.twoRooms(encounter.DoorIsOpen(), approachPlayer(alice, 8, 2), approachMonster(goblin, 10, 2))
	got := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true})
	s.holdsTheLaw(enc, alice, got, 1)
	s.Contains(got.Path, cellAt(10, 5), "the walk goes through the doorway")
	s.NotContains([]spatial.Position{cellAt(9, 1), cellAt(9, 2), cellAt(9, 3)}, got.Cell,
		"never a cell across the wall from him")
	// A fight's budget floods instead of searching, and answers the same.
	budgeted := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Budget: 20})
	s.Equal(got, budgeted)
}

func (s *ApproachesSuite) TestAShutDoorClosesThePlayersWalk() {
	// A player's step refuses a shut door (it is opened, not walked
	// through), so a walk that needs it reaches nothing.
	enc := s.twoRooms(encounter.DoorIsClosed(), approachPlayer(alice, 8, 5), approachMonster(goblin, 14, 5))
	got := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true})
	s.False(got.Reached)
	budgeted := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Budget: 20})
	s.False(budgeted.Reached, "nor does a fight's budget open it")

	open := s.twoRooms(encounter.DoorIsOpen(), approachPlayer(alice, 8, 5), approachMonster(goblin, 14, 5))
	s.holdsTheLaw(open, alice, s.one(open, encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Budget: 20}), 1)
}

// TestUnreachableTargetsCostOneFloor is free roam's bound: the first search
// that finds nothing has settled the floor, and the targets after it read
// that field, so nothing floods. The answers are the bounded flood's.
func (s *ApproachesSuite) TestUnreachableTargetsCostOneFloor() {
	// The orc closes the corridor: the goblin and the wolf beyond him are
	// seen and no walk reaches them; the rat behind alice is two steps off.
	enc := s.corridor(approachPlayer(alice, 3, 0), approachMonster("rat", 0, 0), approachMonster("orc", 5, 0),
		approachMonster(goblin, 7, 0), approachMonster("wolf", 9, 0))
	targets := []encounter.MemberID{goblin, "rat", "wolf"}
	before := enc.ExportedFloodsComputed()
	free, err := enc.Approaches(encounter.ApproachesInput{Mover: alice, Targets: targets, RangeFeet: 5, Unbounded: true})
	s.Require().NoError(err)
	s.Equal(before, enc.ExportedFloodsComputed(), "the failed search's field is the flood; nothing floods again")
	s.False(free.Approaches[0].Reached)
	s.True(free.Approaches[1].Reached, "the rat after the failed search is answered from its field")
	s.Equal(cellAt(1, 0), free.Approaches[1].Cell)
	s.Equal([]spatial.Position{cellAt(2, 0), cellAt(1, 0)}, free.Approaches[1].Path)
	s.False(free.Approaches[2].Reached)

	flooded, err := enc.Approaches(encounter.ApproachesInput{Mover: alice, Targets: targets, RangeFeet: 5, Budget: 100})
	s.Require().NoError(err)
	s.Equal(flooded, free, "the same answers as a flood")
}

// TestManyTargetsTakeOneFlood: past the search count, one unbounded flood
// answers them all, with the searches' answers.
func (s *ApproachesSuite) TestManyTargetsTakeOneFlood() {
	members := []encounter.MemberInput{approachPlayer(alice, 0, 0)}
	var targets []encounter.MemberID
	for i, at := range [][2]int{{9, 0}, {9, 9}, {0, 9}, {5, 5}, {7, 2}} {
		id := encounter.MemberID(fmt.Sprintf("m%d", i))
		members = append(members, approachMonster(id, at[0], at[1]))
		targets = append(targets, id)
	}
	enc := s.room(members...)
	before := enc.ExportedFloodsComputed()
	many, err := enc.Approaches(encounter.ApproachesInput{Mover: alice, Targets: targets, RangeFeet: 5, Unbounded: true})
	s.Require().NoError(err)
	s.Equal(before+1, enc.ExportedFloodsComputed(), "five targets take one flood")
	for i, got := range many.Approaches {
		one := s.one(enc, encounter.ApproachesInput{Mover: alice, Targets: targets[i : i+1], RangeFeet: 5, Unbounded: true})
		s.Equal(one, got, "%s: the flood answers as its own search does", got.Target)
		s.holdsTheLaw(enc, alice, got, 1)
	}
	s.Equal(before+1, enc.ExportedFloodsComputed(), "a search for one target floods nothing")
}

func (s *ApproachesSuite) TestTiesBreakInScanOrder() {
	enc := s.room(approachPlayer(alice, 1, 1), approachMonster(goblin, 6, 6))
	in := encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Unbounded: true}
	got := s.one(enc, in)
	s.holdsTheLaw(enc, alice, got, 1)
	s.Equal(got, s.one(enc, in), "asked twice, the same answer")
	s.Equal(oracleApproach(s.T(), enc, alice, goblin, 1, 0), got.Cell)
}

func (s *ApproachesSuite) TestRefusals() {
	enc := s.corridor(approachPlayer(alice, 0, 0), approachMonster(goblin, 6, 0))
	_, err := enc.Approaches(encounter.ApproachesInput{})
	s.ErrorIs(err, encounter.ErrNoMember)
	_, err = enc.Approaches(encounter.ApproachesInput{Mover: "nobody"})
	s.ErrorIs(err, encounter.ErrNotMember)
	_, err = enc.Approaches(encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{"nobody"}})
	s.ErrorIs(err, encounter.ErrNotMember)
	_, err = enc.Approaches(encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: -5})
	s.ErrorIs(err, encounter.ErrBadReach)
	_, err = enc.Approaches(encounter.ApproachesInput{Mover: alice, Targets: []encounter.MemberID{goblin}, RangeFeet: 5, Budget: -1})
	s.ErrorIs(err, encounter.ErrBadReach)
}

// oracleApproach is [oracleOrNone] for a scene where a cell exists.
func oracleApproach(
	t *testing.T, enc *encounter.Encounter, mover, target encounter.MemberID, reachCells float64, budget int,
) spatial.Position {
	t.Helper()
	dist, err := enc.ExportedWalkFlood(mover, whereIs(t, enc, mover))
	require.NoError(t, err)
	cell, found := oracleOrNone(t, enc, mover, target, reachCells, budget, dist)
	require.True(t, found, "the oracle finds a cell")
	return cell
}

// TestApproaches_CastleHoldsTheOracle is the property on the mad king's
// castle with its garrison: every player asked of every monster (all at
// once, and each alone), at a
// sword's five feet and a glaive's ten, unbounded and at a fight's thirty
// feet, gets the oracle's cell or — exactly when the oracle finds none — no
// cell, and every cell it names holds the law.
func TestApproaches_CastleHoldsTheOracle(t *testing.T) {
	enc, compiled := openSite(t, 0, true)
	roster, err := enc.Members()
	require.NoError(t, err)
	var targets []encounter.MemberID
	for _, m := range roster {
		if m.Kind == encounter.KindMonster {
			targets = append(targets, m.ID)
		}
	}
	require.NotEmpty(t, targets)
	reached := 0
	for _, p := range keepPlayers[:len(seatedParty(compiled))] {
		from := whereIs(t, enc, p)
		dist, err := enc.ExportedWalkFlood(p, from)
		require.NoError(t, err)
		for _, feet := range []int{5, 10} {
			reach := float64(encounter.CellsFromFeet(feet))
			for _, budget := range []int{0, 6} {
				in := encounter.ApproachesInput{Mover: p, Targets: targets, RangeFeet: feet, Budget: budget, Unbounded: budget == 0}
				out, err := enc.Approaches(in)
				require.NoError(t, err)
				require.Len(t, out.Approaches, len(targets))
				for i, got := range out.Approaches {
					require.Equal(t, targets[i], got.Target)
					// Asked alone the target is searched for, not flooded:
					// the same answer either way.
					alone := in
					alone.Targets = targets[i : i+1]
					single, err := enc.Approaches(alone)
					require.NoError(t, err)
					require.Equal(t, got, single.Approaches[0], "%s to %s asked alone", p, got.Target)
					want, ok := oracleOrNone(t, enc, p, got.Target, reach, budget, dist)
					require.Equal(t, ok, got.Reached, "%s to %s at %d ft, budget %d", p, got.Target, feet, budget)
					if !ok {
						continue
					}
					if len(got.Path) == 0 {
						require.Equal(t, from, got.Cell, "in reach where it stands")
						continue
					}
					require.Equal(t, want, got.Cell, "%s to %s at %d ft, budget %d", p, got.Target, feet, budget)
					holdsTheLaw(t, enc, p, from, got, reach)
					if budget > 0 {
						require.LessOrEqual(t, len(got.Path), budget)
					}
					reached++
				}
			}
		}
	}
	require.Positive(t, reached, "precondition: some walk reaches a monster")
}

// oracleOrNone is the brute-force answer: every cell the player's walk
// flood reaches within budget (zero unbounded), held to the law, the
// cheapest first and then X then Y — or none. A target already in reach and
// seen is answered as the mover's own cell.
func oracleOrNone(
	t *testing.T, enc *encounter.Encounter, mover, target encounter.MemberID, reachCells float64, budget int,
	dist map[spatial.Position]int,
) (spatial.Position, bool) {
	t.Helper()
	from := whereIs(t, enc, mover)
	at := whereIs(t, enc, target)
	if enc.Distance(from, at) <= reachCells {
		seen, err := enc.ExportedSeesFrom(mover, from, target)
		require.NoError(t, err)
		if seen {
			return from, true
		}
	}
	var best spatial.Position
	bestSteps, found := 0, false
	for cell, steps := range dist {
		if cell == from || (budget > 0 && steps > budget) || enc.Distance(cell, at) > reachCells {
			continue
		}
		fact, err := enc.CellAt(encounter.CellAtInput{Cell: cell, Mover: mover})
		require.NoError(t, err)
		if fact.Passage != encounter.PassageStandable ||
			enc.ValidateWalkEnd(encounter.CellAtInput{Mover: mover, Cell: cell}) != nil {
			continue
		}
		seen, err := enc.ExportedSeesFrom(mover, cell, target)
		require.NoError(t, err)
		if !seen {
			continue
		}
		if !found || steps < bestSteps || (steps == bestSteps && (cell.X < best.X || (cell.X == best.X && cell.Y < best.Y))) {
			best, bestSteps, found = cell, steps, true
		}
	}
	return best, found
}
