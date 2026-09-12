// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// directive_test.go is the push: an effect names a creature, a direction and a
// budget, and the creature moves whether or not it is their turn.
//
// THE SCENE IS ONE STRAIGHT LINE, and it is straight in the frame that
// matters. Under pointy-top, authored row 0 converts to axial (col, 0) — the
// offset shift is on the ROW — so the authored cells [0,0]..[4,0] are the
// axial cells (0,0)..(4,0) and the line from the caster through the mover
// continues along +Q with no rounding to argue about. A scene drawn on any
// other row would be testing the hex line algorithm; this one tests the
// policy.
//
//	authored:  0    1    2    3    4
//	row 0:     C    M    .    .    .
//
// The pillar scenes put a movement-blocking prop at [3,0]: two cells past the
// mover, which is the last cell a 2-cell push would reach.
const (
	thunderwaveModule = "dnd5e"
	pillarRef         = "dnd5e:props:pillar"
)

// thunderwaveRef is the cause every directed move in this file names. A push
// with no cause is refused, so there is no scene here without one.
var thunderwaveRef = core.Ref{Module: thunderwaveModule, Type: "spells", ID: "thunderwave"}

type DirectiveTestSuite struct {
	suite.Suite

	enc    *encounter.Encounter
	mover  *recordingMover
	driver *scriptedDriver

	casterCell spatial.Position
	moverCell  spatial.Position
	pillarCell spatial.Position
}

func TestDirectiveSuite(t *testing.T) {
	suite.Run(t, new(DirectiveTestSuite))
}

func (s *DirectiveTestSuite) SetupTest() {
	s.driver = nil
	s.casterCell = cellAt(0, 0)
	s.moverCell = cellAt(1, 0)
	s.pillarCell = cellAt(3, 0)
}

// lineScene paints the corridor above, optionally with the pillar standing on
// [3,0], and puts alice (the caster) and goblin (the mover) on it.
func (s *DirectiveTestSuite) lineScene(withPillar bool) *encounter.Encounter {
	corridor := encounter.RegionInput{
		ID:   room1,
		Name: room1,
		Cells: []spatial.Position{
			{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}, {X: 4, Y: 0},
		},
		Archetype: testArchetype,
		Lighting:  fullLight(),
	}

	field := encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{corridor},
	}
	if withPillar {
		// One cell wide, so it obstructs no sightline on its own: the
		// question stays about walking, exactly as the route's own pillar
		// scene keeps it (TestSeenMemberPathWalksAroundAPillar).
		field.Props = []encounter.PropInput{{
			Ref:               pillarRef,
			At:                spatial.Position{X: 3, Y: 0},
			BlocksMovement:    propTrue(),
			BlocksLineOfSight: propFalse(),
		}}
	}

	s.mover = &recordingMover{}
	if s.driver == nil {
		s.driver = &scriptedDriver{}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: s.driver, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Mover: s.mover, Announcer: quietAnnouncer{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
			{
				ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 0},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{{Ref: testMeleeAction, Name: "Claw", RangeFeet: 5, Kind: "melee"}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
	return enc
}

// cellOfMember reads where a member stands, through the same roster read every
// other member question goes through.
func (s *DirectiveTestSuite) cellOfMember(id encounter.MemberID) spatial.Position {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("no such member", "%s is not on the roster", id)
	return spatial.Position{}
}

// movedBeats decodes every "moved" beat one member can hear.
func (s *DirectiveTestSuite) movedBeats(audience encounter.MemberID) []map[string]any {
	story, err := s.enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	beats := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == "moved" {
			beats = append(beats, beat)
		}
	}
	return beats
}

// TestLineWithOpenFloorGoesTheWholeBudget.
//
// The continuation of the line from the caster THROUGH the mover, for as many
// cells as the budget pays for. Every cell is a step from the last, and every
// one is farther from the caster than the mover already was — which is what
// "blown away from you" has to mean before any of the rest is worth checking.
func (s *DirectiveTestSuite) TestLineWithOpenFloorGoesTheWholeBudget() {
	enc := s.lineScene(false)

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveLine, Anchor: s.casterCell, Budget: 2,
	})
	s.Require().NoError(err)
	s.Len(out.Path, 2)
	s.Empty(out.StoppedBy, "open floor the whole way stops it nowhere")

	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	grid := canvas.GetGrid()
	s.True(grid.IsAdjacent(s.moverCell, out.Path[0]), "the first pushed cell is a step from where it stands")
	for i := 1; i < len(out.Path); i++ {
		s.True(grid.IsAdjacent(out.Path[i-1], out.Path[i]), "a push is a walk: every cell is a step from the last")
	}

	was := enc.Distance(s.casterCell, s.moverCell)
	for _, p := range out.Path {
		s.Greater(enc.Distance(s.casterCell, p), was, "every pushed cell is farther from the anchor")
	}
}

// TestLineStopsBeforeAPillarAndSaysSo.
//
// The fold refuses the pillar's cell, so the route ends in front of it — and
// the route is the thing that knows WHY it is shorter than the budget, so it
// is the thing that says the word "pillar".
func (s *DirectiveTestSuite) TestLineStopsBeforeAPillarAndSaysSo() {
	enc := s.lineScene(true)

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveLine, Anchor: s.casterCell, Budget: 2,
	})
	s.Require().NoError(err)
	s.Len(out.Path, 1, "one open cell, then the pillar")
	s.NotEqual(s.pillarCell, out.Path[0], "the route may not end on the pillar's own cell")
	s.Contains(out.StoppedBy, pillarRef, "the refusal names the thing, not the category")
}

// TestDirectMovesOffTurnAndTheBeatSaysWhy.
//
// The mover does not hold the active turn — alice does, and goblin's own Step
// is refused for exactly that reason. A directive is not their action, so the
// door it comes through is not guarded by whose turn it is, and the story it
// writes says what moved them.
func (s *DirectiveTestSuite) TestDirectMovesOffTurnAndTheBeatSaysWhy() {
	enc := s.lineScene(true)

	_, err := enc.Step(&encounter.StepInput{Member: goblin, To: s.cellOfMember(alice)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNotActive, "the mover's own step is refused: it is not their turn")

	route, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveLine, Anchor: s.casterCell, Budget: 2,
	})
	s.Require().NoError(err)

	out, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: goblin, Cause: thunderwaveRef, Route: route.Path,
	})
	s.Require().NoError(err)
	s.Equal(1, out.Moved)
	s.Empty(out.StoppedBy, "the walk took every cell the route gave it")
	s.Equal(route.Path[0], s.cellOfMember(goblin), "the mover stands on the cell the route named")

	s.Require().Len(s.mover.calls, 1, "the push announced its step through the same seam a walk does")
	s.Equal(goblin, s.mover.calls[0].Mover)
	s.Equal(s.moverCell, s.mover.calls[0].From)
	s.Equal(route.Path[0], s.mover.calls[0].To)
	s.Equal(s.moverCell, s.mover.calls[0].StoodAt,
		"announced BEFORE the step, exactly as a chosen walk announces it")
	s.True(s.mover.calls[0].Forced, "a push that does not provoke says so to the Mover")
	s.Equal(thunderwaveRef, s.mover.calls[0].Cause, "and names what is doing the pushing")

	beats := s.movedBeats(alice)
	s.Require().NotEmpty(beats)
	last := beats[len(beats)-1]
	s.Equal(string(goblin), last["member"])
	s.Equal(thunderwaveRef.String(), last["cause"], "an observer can tell a shove from a step")

	clockOf, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(alice, clockOf.Active, "a push is nobody's turn and spends nobody's turn")
}

// TestADirectiveThatProvokesSaysSoToTheMover.
//
// The other half of the flag, and the reason it is not simply "a directed move
// never provokes". Dissonant Whispers sends a creature fleeing and IS struck
// for it; the fold that drops opportunity attacks reads a suppression source,
// so a directive that wants them must not send one.
func (s *DirectiveTestSuite) TestADirectiveThatProvokesSaysSoToTheMover() {
	enc := s.lineScene(false)

	_, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: goblin, Cause: thunderwaveRef, Route: []spatial.Position{cellAt(2, 0)},
		Provokes: true,
	})
	s.Require().NoError(err)

	s.Require().Len(s.mover.calls, 1)
	s.False(s.mover.calls[0].Forced, "a directive that provokes is announced like any other step")
	s.Equal(thunderwaveRef, s.mover.calls[0].Cause, "it still says what moved them")
}

// TestAChosenWalkIsNeitherForcedNorCaused.
//
// THE ZERO VALUE HAS TO BE THE WALK. Every existing Mover call in this module
// is a step somebody chose, and the flag is named Forced rather than Provokes
// precisely so that forgetting it cannot switch an opportunity attack off —
// false means "ordinary walk", which is the least permissive reading.
func (s *DirectiveTestSuite) TestAChosenWalkIsNeitherForcedNorCaused() {
	// The goblin's own driver walks it one cell, which is walkPath's path
	// rather than Direct's — the same body, reached the other way.
	s.driver = &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Move{Path: []spatial.Position{cellAt(2, 0)}},
	}}
	enc := s.lineScene(false)

	_, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: goblin, Cause: thunderwaveRef, Route: []spatial.Position{cellAt(2, 0)},
	})
	s.Require().NoError(err)
	s.Require().Len(s.mover.calls, 1)
	s.Require().True(s.mover.calls[0].Forced, "the push is the forced one")

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Greater(len(s.mover.calls), 1, "the driven turn walked")
	for _, call := range s.mover.calls[1:] {
		s.False(call.Forced, "a step a creature chose is not forced")
		s.Equal(core.Ref{}, call.Cause, "and nothing caused it but the creature")
	}
}

// TestAChosenStepCarriesNoCause.
//
// The other half of one walker: a step a creature chose has no cause, and the
// shared body must not invent one for it.
func (s *DirectiveTestSuite) TestAChosenStepCarriesNoCause() {
	enc := s.lineScene(false)

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().Error(err, "goblin is standing there")

	_, err = enc.Direct(context.Background(), encounter.DirectInput{
		Mover: goblin, Cause: thunderwaveRef, Route: []spatial.Position{cellAt(2, 0)},
	})
	s.Require().NoError(err)

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().NoError(err)

	beats := s.movedBeats(alice)
	s.Require().Len(beats, 2)
	s.Equal(thunderwaveRef.String(), beats[0]["cause"], "the push says why")
	s.NotContains(beats[1], "cause", "a step alice chose has no cause to name")
}

// TestAnyPolicyButTheThreeIsRefused.
//
// A policy exists only once something carries it out. Every other word fails
// closed and loudly rather than returning an empty path — which would read as
// "there was nowhere to go" and be indistinguishable from a creature pinned
// against a wall.
//
// "away" and "toward" both used to be spelled out in this loop as the next
// policies anybody would reach for. They arrived — away with Dissonant
// Whispers, toward with Command — so they left the loop and grew tests of
// their own, and the empty word and the typo are still refused exactly as
// firmly as they always were.
func (s *DirectiveTestSuite) TestAnyPolicyButTheThreeIsRefused() {
	enc := s.lineScene(false)

	for _, policy := range []encounter.MovePolicy{"", "backward", "sideways"} {
		_, err := enc.Route(encounter.RouteInput{
			Mover: goblin, Policy: policy, Anchor: s.casterCell, Budget: 2,
		})
		s.ErrorIs(err, encounter.ErrUnsupportedPolicy, "policy %q", policy)
	}
}

// TestRouteRefusesAnAnchorOnTheMover.
//
// The line from the anchor through the mover does not exist when they are the
// same cell, and a direction guessed from nothing is the one answer a push
// must never produce.
func (s *DirectiveTestSuite) TestRouteRefusesAnAnchorOnTheMover() {
	enc := s.lineScene(false)

	_, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveLine, Anchor: s.moverCell, Budget: 2,
	})
	s.ErrorIs(err, encounter.ErrBadReach)
}

// TestDirectRefusesAMoveThatNamesNoCause.
//
// The cause is what makes a directed move legible to an observer, so it is
// required rather than defaulted. An unnamed push would be indistinguishable
// from a step the creature chose, which is the whole thing the beat's cause
// exists to prevent.
func (s *DirectiveTestSuite) TestDirectRefusesAMoveThatNamesNoCause() {
	enc := s.lineScene(false)

	_, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: goblin, Route: []spatial.Position{cellAt(2, 0)},
	})
	s.ErrorIs(err, encounter.ErrNoCause)
	s.Equal(s.moverCell, s.cellOfMember(goblin), "a refused directive moves nobody")
}

// awayScene is the second family of scenes in this file, and it is a room
// rather than a corridor because "away" is a SEARCH and a corridor would let
// it pass by walking in the only direction there is.
//
//	authored:  0    1    2    3    4    5    6
//	rows 0-4:  open floor, seven wide and five deep
//	row 2:               C    M
//
// The caster stands at [3,2] and the mover beside them at [4,2]. Neither
// away scene in this file carries a wall or a door, which is what lets
// reachedStandableWithin below flood the same cells the route does while
// reading nothing but the fold.
func (s *DirectiveTestSuite) awayScene() *encounter.Encounter {
	return s.sceneOfCells(rectCells(0, 0, 7, 5), spatial.Position{X: 3, Y: 2}, spatial.Position{X: 4, Y: 2})
}

// allyCorridorScene is "may cross is not may stop" drawn as a floor: one
// row-2 corridor with a FRIENDLY monster standing in it.
//
//	authored:  3    4    5    6    7
//	row 2:     C    M    .    A    .
//
// bob is a monster like the goblin, so the two share a faction and the fold
// calls bob's cell PassThrough rather than Blocked — crossable, and not a
// place to stop. At budget 2 bob's cell is the FARTHEST the flood reaches; at
// budget 3 the cell beyond it is, and the only way there is through him.
func (s *DirectiveTestSuite) allyCorridorScene() *encounter.Encounter {
	cells := []spatial.Position{{X: 3, Y: 2}, {X: 4, Y: 2}, {X: 5, Y: 2}, {X: 6, Y: 2}, {X: 7, Y: 2}}
	return s.sceneOfCells(cells, spatial.Position{X: 3, Y: 2}, spatial.Position{X: 4, Y: 2},
		encounter.MemberInput{
			ID: bob, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 2},
			SpeedFeet: 30, Targeting: "closest",
		})
}

// ringScene is the floor where running changes nothing: the caster's cell and
// the six cells around it, so every cell the mover can reach is EXACTLY as far
// from the caster as the cell it is standing on.
//
// It is the scene the two-cell pinnedScene is not. There the mover had nowhere
// to walk at all, so "nowhere farther" and "nowhere" were the same sentence
// and the strictly-farther rule never had to decide anything. Here there is
// plenty of floor and the answer is still nowhere.
func (s *DirectiveTestSuite) ringScene() *encounter.Encounter {
	cells := []spatial.Position{
		{X: 3, Y: 2},
		{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 2}, {X: 3, Y: 3}, {X: 2, Y: 3}, {X: 2, Y: 2},
	}
	return s.sceneOfCells(cells, spatial.Position{X: 3, Y: 2}, spatial.Position{X: 4, Y: 2})
}

// pocketScene is the dead end that bends back.
//
// Two ways out of the mover's cell. One is two cells of open floor heading
// straight away. The other is a five-cell pocket that climbs, turns, and ends
// at [2,1] — which is FIVE steps of walking and ONE cell from the caster by
// the ruler, so a route that measured the walk would run down it and finish
// next to the thing it was fleeing.
//
//	authored:  0    1    2    3    4    5    6
//	row 0:               .    .    .
//	row 1:               .              M'   .      ([4,1] is the pocket mouth)
//	row 2:                    C    M    .    .
//
// (The drawing is the authored frame; under pointy-top [2,1] is a neighbour of
// the caster's own cell, which is the bend the test is named for.)
func (s *DirectiveTestSuite) pocketScene() *encounter.Encounter {
	cells := []spatial.Position{
		{X: 3, Y: 2}, {X: 4, Y: 2}, {X: 5, Y: 2}, {X: 6, Y: 2},
		{X: 4, Y: 1}, {X: 4, Y: 0}, {X: 3, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 1},
	}
	return s.sceneOfCells(cells, spatial.Position{X: 3, Y: 2}, spatial.Position{X: 4, Y: 2})
}

// pinnedScene is the whole floor: two cells, the caster standing in the one
// that is the mouth. There is nowhere farther to go and no wall to blame.
func (s *DirectiveTestSuite) pinnedScene() *encounter.Encounter {
	cells := []spatial.Position{{X: 3, Y: 2}, {X: 4, Y: 2}}
	return s.sceneOfCells(cells, spatial.Position{X: 3, Y: 2}, spatial.Position{X: 4, Y: 2})
}

// sceneOfCells paints one region out of the authored cells given and seats
// alice (the caster) and goblin (the mover) on it, with the suite's anchor and
// mover cells set to match. It is lineScene's wiring with the floor as a
// parameter.
func (s *DirectiveTestSuite) sceneOfCells(
	cells []spatial.Position, caster, mover spatial.Position, extra ...encounter.MemberInput,
) *encounter.Encounter {
	s.casterCell = cellAt(int(caster.X), int(caster.Y))
	s.moverCell = cellAt(int(mover.X), int(mover.Y))

	s.mover = &recordingMover{}
	if s.driver == nil {
		s.driver = &scriptedDriver{}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: s.driver, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Mover: s.mover, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{{
				ID: room1, Name: room1, Cells: cells,
				Archetype: testArchetype, Lighting: fullLight(),
			}},
		},
		Members: append([]encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: caster},
			{
				ID: goblin, Kind: encounter.KindMonster, Position: mover,
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{{Ref: testMeleeAction, Name: "Claw", RangeFeet: 5, Kind: "melee"}},
			},
		}, extra...),
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
	return enc
}

// reachedStandableWithin is the ORACLE the away tests measure the route
// against: every cell a mover could reach in `budget` steps and legally stop
// on, keyed by how many steps it took, computed from the exported grid and
// [encounter.Encounter.CellAt] alone.
//
// It is deliberately NOT the route's own helper. A test that asked the
// implementation what it reached could only ever agree with it; this floods
// the same floor through the public surface and then asks whether the cell the
// route picked is the farthest one in it.
//
// It reads no wall, so it is only honest on a scene that has none — which both
// away scenes state in their own docs.
func reachedStandableWithin(
	enc *encounter.Encounter, mover encounter.MemberID, from spatial.Position, budget int,
) map[spatial.Position]int {
	canvas, err := enc.Canvas()
	if err != nil {
		panic(err)
	}
	field, err := spatial.Field(canvas.GetGrid(), spatial.FieldInput{
		Sources: []spatial.Position{from},
		Passable: func(_, to spatial.Position) bool {
			return enc.CellAt(encounter.CellAtInput{Cell: to, Mover: mover}).Passage != encounter.PassageBlocked
		},
		Limit: budget,
	})
	if err != nil {
		panic(err)
	}

	out := make(map[spatial.Position]int, len(field.Dist))
	for cell, dist := range field.Dist {
		if cell == from {
			continue
		}
		if enc.CellAt(encounter.CellAtInput{Cell: cell, Mover: mover}).Passage != encounter.PassageStandable {
			continue
		}
		out[cell] = dist
	}
	return out
}

// reachedWithin is the same oracle WITHOUT the standable filter: every cell
// the flood entered, stoppable or not. It is what lets a test say "the flood
// reached the ally's cell and the fold is what refused it" rather than leaving
// the two indistinguishable.
func reachedWithin(
	enc *encounter.Encounter, mover encounter.MemberID, from spatial.Position, budget int,
) map[spatial.Position]int {
	canvas, err := enc.Canvas()
	if err != nil {
		panic(err)
	}
	field, err := spatial.Field(canvas.GetGrid(), spatial.FieldInput{
		Sources: []spatial.Position{from},
		Passable: func(_, to spatial.Position) bool {
			return enc.CellAt(encounter.CellAtInput{Cell: to, Mover: mover}).Passage != encounter.PassageBlocked
		},
		Limit: budget,
	})
	if err != nil {
		panic(err)
	}
	out := make(map[spatial.Position]int, len(field.Dist))
	for cell, dist := range field.Dist {
		if cell != from {
			out[cell] = dist
		}
	}
	return out
}

// TestAwayEndsAtTheReachedCellFarthestByTheRuler.
//
// The whole policy in one assertion: of every cell the mover could reach and
// stop on inside its speed, the one it ends on is the one FARTHEST FROM THE
// ANCHOR BY THE RULER. Not the longest walk, not the last cell of a direction
// — the greatest distance, which is what "away from you" means at a table.
func (s *DirectiveTestSuite) TestAwayEndsAtTheReachedCellFarthestByTheRuler() {
	enc := s.awayScene()
	const budget = 4

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: budget,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Path, "there is open floor to flee across")

	end := out.Path[len(out.Path)-1]
	start := s.cellOfMember(goblin)
	s.Greater(enc.Distance(s.casterCell, end), enc.Distance(s.casterCell, start),
		"a flee that ends no farther away is not a flee")

	for cell := range reachedStandableWithin(enc, goblin, start, budget) {
		s.LessOrEqual(enc.Distance(s.casterCell, cell), enc.Distance(s.casterCell, end),
			"no reached standable cell is farther from the anchor than the chosen end")
	}

	s.LessOrEqual(len(out.Path), budget, "the walk is bounded by the speed that paid for it")
	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	grid := canvas.GetGrid()
	s.True(grid.IsAdjacent(start, out.Path[0]), "the first cell is a step from where it stands")
	for i := 1; i < len(out.Path); i++ {
		s.True(grid.IsAdjacent(out.Path[i-1], out.Path[i]), "every cell is a step from the last")
	}
}

// TestAwayDoesNotTakeTheDeadEndThatBendsBack.
//
// The ruler, not the walk (the directed-movement design's rejected
// alternative). The pocket is the longer run by four cells and ends beside the
// caster; the open floor is two cells and ends three away. A creature that
// flees down the pocket has obeyed its speed and disobeyed the spell.
func (s *DirectiveTestSuite) TestAwayDoesNotTakeTheDeadEndThatBendsBack() {
	enc := s.pocketScene()
	const budget = 6

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: budget,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Path)

	end := out.Path[len(out.Path)-1]
	s.Equal(cellAt(6, 2), end, "the open floor's far cell, three from the caster by the ruler")

	pocketFar := cellAt(2, 1)
	for _, cell := range out.Path {
		s.NotEqual(pocketFar, cell, "the route does not enter the pocket")
	}

	// The teeth: the pocket WAS affordable, and away declined it.
	reached := reachedStandableWithin(enc, goblin, s.cellOfMember(goblin), budget)
	pocketWalk, ok := reached[pocketFar]
	s.Require().True(ok, "the pocket's far cell is inside the budget")
	s.Greater(pocketWalk, len(out.Path), "a longer walk was on offer and away did not take it")
	s.Less(enc.Distance(s.casterCell, pocketFar), enc.Distance(s.casterCell, end),
		"and it is nearer the caster, which is why")
}

// TestAwayPinnedIsAnEmptyRouteThatSaysSo.
//
// The pinned case is an EMPTY PATH WITH A SENTENCE, which is what makes it
// distinguishable from a policy nobody implemented — the distinction
// ErrUnsupportedPolicy's own doc insists on.
func (s *DirectiveTestSuite) TestAwayPinnedIsAnEmptyRouteThatSaysSo() {
	enc := s.pinnedScene()

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: 6,
	})
	s.Require().NoError(err, "nowhere to run is not an error")
	s.Empty(out.Path)
	s.Contains(out.StoppedBy, "nowhere farther", "the route says why it is empty")
}

// TestAwayTiesAreStable.
//
// Two ties, in order: among the cells equally far by the ruler the SHORTER
// WALK wins, and among those the scan order decides — so ranging over the
// flood's map cannot leak iteration order into the answer (C8). Asked twice
// against an unchanged floor, it is the same cell both times, not merely an
// equally good one.
func (s *DirectiveTestSuite) TestAwayTiesAreStable() {
	enc := s.awayScene()
	// SIX, NOT FOUR, and the number is the test. At four the room's farthest
	// corners are all the same walk away and only the scan order ever
	// decides; at six the two corners BEHIND the caster join the tie at the
	// same ruler distance and a longer walk, so a route that preferred the
	// longer walk would pick one of them and be caught.
	const budget = 6

	in := encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: budget,
	}
	first, err := enc.Route(in)
	s.Require().NoError(err)
	second, err := enc.Route(in)
	s.Require().NoError(err)
	s.Equal(first.Path, second.Path, "the same question, the same answer")
	s.Require().NotEmpty(first.Path)

	// The oracle applies the two tie rules itself and lands on one cell.
	start := s.cellOfMember(goblin)
	reached := reachedStandableWithin(enc, goblin, start, budget)
	here := enc.Distance(s.casterCell, start)
	var want spatial.Position
	wantFar, wantWalk, found := 0.0, 0, false
	for cell, walk := range reached {
		far := enc.Distance(s.casterCell, cell)
		if far <= here {
			continue
		}
		better := !found || far > wantFar ||
			(far == wantFar && (walk < wantWalk ||
				(walk == wantWalk && (cell.X < want.X || (cell.X == want.X && cell.Y < want.Y)))))
		if better {
			want, wantFar, wantWalk, found = cell, far, walk, true
		}
	}
	s.Require().True(found)
	s.Equal(want, first.Path[len(first.Path)-1], "farthest, then shortest walk, then scan order")
	s.Equal(wantWalk, len(first.Path), "and the walk to it is the flood's own shortest")
}

// TestAwayCrossesAnAllyAndDoesNotStopOnOne is 2014's rule for moving around
// other creatures, which the flee obeys because it reads the same fold every
// other route reads — not because it checks for allies.
//
// The two halves are asked of one floor at two budgets, so neither can pass by
// accident of the other: at 2 the ally's cell is the farthest the flood
// reaches and the route declines it; at 3 the cell past the ally wins and the
// only way there is straight through him.
func (s *DirectiveTestSuite) TestAwayCrossesAnAllyAndDoesNotStopOnOne() {
	enc := s.allyCorridorScene()
	allyCell := cellAt(6, 2)
	s.Require().Equal(allyCell, s.cellOfMember(bob), "the ally stands where the scene says")

	near, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: 2,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(near.Path)
	end := near.Path[len(near.Path)-1]
	s.Equal(cellAt(5, 2), end, "it stops beside the ally, not inside him")
	s.Greater(enc.Distance(s.casterCell, allyCell), enc.Distance(s.casterCell, end),
		"and the cell it declined was the farther one, which is the whole point")
	s.Require().Contains(reachedWithin(enc, goblin, s.cellOfMember(goblin), 2), allyCell,
		"the flood did reach it; the fold is what refused it")

	far, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: 3,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(far.Path)
	s.Equal(cellAt(7, 2), far.Path[len(far.Path)-1], "one more cell of budget reaches past him")
	s.Contains(far.Path, allyCell, "and the way past him is through him")
}

// TestAwayRefusesToShuffleSidewaysWhenNothingIsFarther.
//
// Strictly farther, or nowhere. Every cell on the ring is the same distance
// from the caster as the one the mover is standing on, so running anywhere on
// it is running nowhere — and a flee that spent its whole speed to end up
// equally close would be obeying the budget instead of the spell.
func (s *DirectiveTestSuite) TestAwayRefusesToShuffleSidewaysWhenNothingIsFarther() {
	enc := s.ringScene()
	const budget = 6

	start := s.cellOfMember(goblin)
	here := enc.Distance(s.casterCell, start)
	reached := reachedStandableWithin(enc, goblin, start, budget)
	s.Require().NotEmpty(reached, "there is floor to walk on, which is what makes this a real refusal")
	for cell := range reached {
		s.Equal(here, enc.Distance(s.casterCell, cell), "every cell on the ring is equally close")
	}

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: budget,
	})
	s.Require().NoError(err)
	s.Empty(out.Path, "equally far is not farther")
	s.Contains(out.StoppedBy, "nowhere farther")
	s.Equal(start, s.cellOfMember(goblin))
}

// TestAwayWithNoBudgetRoutesNowhere. Zero is unbounded to the field's own
// Limit, so a route that asked it with zero would flood the whole floor and
// hand a creature with no movement the far corner of the dungeon.
func (s *DirectiveTestSuite) TestAwayWithNoBudgetRoutesNowhere() {
	enc := s.awayScene()

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveAway, Anchor: s.casterCell, Budget: 0,
	})
	s.Require().NoError(err, "a pointless directive is legal, as RouteInput.Budget says")
	s.Empty(out.Path)
	s.Empty(out.StoppedBy, "nothing stopped it; it was never paid for")
}

// The toward family. "Away" is a search for the farthest cell; "toward" is a
// WALK to the ring around the anchor, and the scenes below are shaped by that
// difference: a corridor is a fine floor for testing a walk, and the one place
// the ruler is asked at all is when the walk cannot get there.

// towardCorridorScene is the hall: authored row 2, columns 1 through 7, with
// the anchor (alice) standing in the westmost cell and the mover (goblin) in
// the eastmost.
//
//	authored:  1    2    3    4    5    6    7
//	row 2:     A    .    .    .    .    .    M
//
// The mover is six cells from the anchor by the ruler and by the walk, which
// is exactly the speed a 30-foot monster pays for — so a route that stops
// beside the anchor has DECLINED a cell it could afford, and one that spent
// everything would be visibly wrong.
func (s *DirectiveTestSuite) towardCorridorScene(extra ...encounter.MemberInput) *encounter.Encounter {
	cells := []spatial.Position{
		{X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 4, Y: 2},
		{X: 5, Y: 2}, {X: 6, Y: 2}, {X: 7, Y: 2},
	}
	return s.sceneOfCells(cells, spatial.Position{X: 1, Y: 2}, spatial.Position{X: 7, Y: 2}, extra...)
}

// TestTowardStopsBesideTheAnchorAndNotOnIt.
//
// The whole policy in one corridor: the mover walks the shortest way in and
// stops on the ring, and the cell it declines is the one the anchor is
// standing on. The budget that could have carried it there is the test —
// "within 5 feet" is where the approach ends, not where the movement does.
func (s *DirectiveTestSuite) TestTowardStopsBesideTheAnchorAndNotOnIt() {
	enc := s.towardCorridorScene()
	anchor := cellAt(1, 2)
	const budget = 6

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: budget,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Path, "there is a hall to walk down")

	end := out.Path[len(out.Path)-1]
	s.Equal(cellAt(2, 2), end, "it ends on the ring around the anchor")
	s.Equal(float64(1), enc.Distance(anchor, end))
	s.NotContains(out.Path, anchor, "the anchor's own cell is never the destination")
	s.Len(out.Path, 5, "the fewest steps to the ring, and one less than the budget")
	s.Empty(out.StoppedBy, "nothing stopped it; it arrived")

	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	grid := canvas.GetGrid()
	s.True(grid.IsAdjacent(s.cellOfMember(goblin), out.Path[0]), "the first cell is a step from where it stands")
	for i := 1; i < len(out.Path); i++ {
		s.True(grid.IsAdjacent(out.Path[i-1], out.Path[i]), "every cell is a step from the last")
	}
}

// towardTwoWaysScene is the fork: two ways from the mover to the anchor's own
// ring, one three steps long and one five.
//
//	authored:  1    2    3    4    5
//	row 1:     .    .    .    .    .      the long way round
//	row 2:     A    .    .    .    M      the short hall
//
// Under pointy-top, [1,1] is a neighbour of the anchor's cell and so is
// [2,2] — two different cells of the same ring, reached by two different
// walks. A route that took the first goal cell it found rather than the
// nearest would pick whichever the flood's map happened to hand it first.
func (s *DirectiveTestSuite) towardTwoWaysScene() *encounter.Encounter {
	cells := []spatial.Position{
		{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}, {X: 5, Y: 1},
		{X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 4, Y: 2}, {X: 5, Y: 2},
	}
	return s.sceneOfCells(cells, spatial.Position{X: 1, Y: 2}, spatial.Position{X: 5, Y: 2})
}

// TestTowardTakesTheFewestStepsToTheRing.
//
// Two cells of the anchor's ring are reachable and they are not the same walk
// away. The route takes the near one, and the test proves the far one was
// genuinely on offer rather than merely absent.
func (s *DirectiveTestSuite) TestTowardTakesTheFewestStepsToTheRing() {
	enc := s.towardTwoWaysScene()
	anchor := cellAt(1, 2)
	const budget = 6

	longWay := cellAt(1, 1)
	s.Require().Equal(float64(1), enc.Distance(anchor, longWay), "the long way ends on the ring too")
	reached := reachedStandableWithin(enc, goblin, s.cellOfMember(goblin), budget)
	longWalk, ok := reached[longWay]
	s.Require().True(ok, "and it is inside the budget, so the route had a real choice")

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: budget,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Path)

	s.Equal(cellAt(2, 2), out.Path[len(out.Path)-1], "the near cell of the ring")
	s.Len(out.Path, 3)
	s.Less(len(out.Path), longWalk, "and the walk it declined was the longer one")
}

// TestTowardWalledOffFallsBackToTheNearestCellByTheRuler.
//
// The anchor is behind a wall. A creature compelled to approach does not stand
// still because the door is shut: it gets as close as the floor allows, which
// is the ruler's question and the one place this policy asks it.
func (s *DirectiveTestSuite) TestTowardWalledOffFallsBackToTheNearestCellByTheRuler() {
	enc := s.towardCorridorWalledScene()
	anchor := cellAt(1, 2)
	const budget = 6

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: budget,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Path, "a shut door is not a creature that stays put")

	end := out.Path[len(out.Path)-1]
	s.Equal(cellAt(4, 2), end, "the last cell on this side of the wall")
	s.Equal(float64(3), enc.Distance(anchor, end))
	s.Less(enc.Distance(anchor, end), enc.Distance(anchor, s.cellOfMember(goblin)),
		"strictly nearer than where it started, which is what makes it an approach")
	s.Len(out.Path, 3)
}

// towardCorridorWalledScene is towardCorridorScene with the crossing between
// [3,2] and [4,2] sealed, so the anchor's half of the hall cannot be walked
// into at all.
func (s *DirectiveTestSuite) towardCorridorWalledScene() *encounter.Encounter {
	s.casterCell = cellAt(1, 2)
	s.moverCell = cellAt(7, 2)
	s.mover = &recordingMover{}
	if s.driver == nil {
		s.driver = &scriptedDriver{}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: s.driver, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Mover: s.mover, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{{
				ID: room1, Name: room1,
				Cells: []spatial.Position{
					{X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 4, Y: 2},
					{X: 5, Y: 2}, {X: 6, Y: 2}, {X: 7, Y: 2},
				},
				Archetype: testArchetype, Lighting: fullLight(),
			}},
			Walls: []encounter.WallInput{wall(3, 2, 4, 2)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 2},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{{Ref: testMeleeAction, Name: "Claw", RangeFeet: 5, Kind: "melee"}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
	return enc
}

// towardRingScene is the floor where approaching changes nothing: the twelve
// cells exactly two from the anchor, and the anchor's own cell and its whole
// ring left off the map.
//
// Every cell the mover can walk to is the same two cells from the anchor, so
// there is plenty of floor and nowhere nearer. It is the scene that tells
// "strictly nearer" from "no nearer", which a route comparing with <= would
// fail by shuffling around the ring to spend its budget.
func (s *DirectiveTestSuite) towardRingScene() *encounter.Encounter {
	cells := []spatial.Position{
		{X: 2, Y: 0}, {X: 3, Y: 0}, {X: 4, Y: 0},
		{X: 1, Y: 1}, {X: 4, Y: 1},
		{X: 1, Y: 2}, {X: 5, Y: 2},
		{X: 1, Y: 3}, {X: 4, Y: 3},
		{X: 2, Y: 4}, {X: 3, Y: 4}, {X: 4, Y: 4},
	}
	return s.sceneOfCells(cells, spatial.Position{X: 2, Y: 0}, spatial.Position{X: 3, Y: 4})
}

// TestTowardRefusesToShuffleWhenNothingIsNearer.
//
// Strictly nearer, or nowhere. Everything the mover can reach is exactly as
// far from the anchor as the cell it is standing on, so walking anywhere is
// walking nowhere — and a creature that spent its whole speed to end up
// equally far would be obeying the budget instead of the spell.
func (s *DirectiveTestSuite) TestTowardRefusesToShuffleWhenNothingIsNearer() {
	enc := s.towardRingScene()
	anchor := cellAt(3, 2)
	const budget = 6

	start := s.cellOfMember(goblin)
	here := enc.Distance(anchor, start)
	s.Require().Equal(float64(2), here, "the mover stands on the ring, two from the hole in it")
	reached := reachedStandableWithin(enc, goblin, start, budget)
	s.Require().NotEmpty(reached, "there is floor to walk on, which is what makes this a real refusal")
	for cell := range reached {
		s.Equal(here, enc.Distance(anchor, cell), "every cell of the ring is equally far")
	}

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: budget,
	})
	s.Require().NoError(err, "nowhere nearer is not an error")
	s.Empty(out.Path, "equally near is not nearer")
	s.Contains(out.StoppedBy, "nowhere nearer", "the route says why it is empty")
	s.Equal(start, s.cellOfMember(goblin))
}

// TestTowardCrossesAnAllyAndDoesNotStopOnOne is 2014's rule for moving around
// other creatures, on the approach rather than the rout — obeyed because this
// route reads the same fold every other route reads, not because it looks for
// allies.
//
// The two halves are two floors, because an ally cannot be asked to move. With
// him standing ON the anchor's ring the route stops short of him and the
// FALLBACK is what answers; with him standing in the middle of the hall the
// route walks straight through him to the ring beyond.
func (s *DirectiveTestSuite) TestTowardCrossesAnAllyAndDoesNotStopOnOne() {
	anchor := cellAt(1, 2)

	onTheRing := s.towardCorridorScene(encounter.MemberInput{
		ID: bob, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 2},
		SpeedFeet: 30, Targeting: "closest",
	})
	s.Require().Equal(cellAt(2, 2), s.cellOfMember(bob))
	s.Require().Contains(reachedWithin(onTheRing, goblin, cellAt(7, 2), 6), cellAt(2, 2),
		"the flood did reach the ring cell; the fold is what refuses to stop on it")

	out, err := onTheRing.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: 6,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Path)
	s.Equal(cellAt(3, 2), out.Path[len(out.Path)-1], "it stops beside the ally, not inside him")
	s.NotContains(out.Path, cellAt(2, 2), "and never enters the only ring cell there is")

	midHall := s.towardCorridorScene(encounter.MemberInput{
		ID: bob, Kind: encounter.KindMonster, Position: spatial.Position{X: 4, Y: 2},
		SpeedFeet: 30, Targeting: "closest",
	})
	through, err := midHall.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: 6,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(through.Path)
	s.Equal(cellAt(2, 2), through.Path[len(through.Path)-1], "the ring is reached")
	s.Contains(through.Path, cellAt(4, 2), "and the way there is through him")
}

// TestTowardIsBoundedByTheBudget.
//
// A budget that does not reach the ring is not a refusal: the creature closes
// as far as it can pay for, which is the fallback answering with the nearest
// cell the flood was allowed to reach.
func (s *DirectiveTestSuite) TestTowardIsBoundedByTheBudget() {
	enc := s.towardCorridorScene()
	anchor := cellAt(1, 2)
	const budget = 2

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: budget,
	})
	s.Require().NoError(err)
	s.Len(out.Path, budget, "every cell the budget paid for, and not one more")
	s.Equal(cellAt(5, 2), out.Path[len(out.Path)-1])
	s.Less(enc.Distance(anchor, out.Path[len(out.Path)-1]), enc.Distance(anchor, s.cellOfMember(goblin)))
}

// TestTowardAnAnchorAlreadyBesideTheMoverRoutesNowhere.
//
// Two cases, one answer, and it is the empty route rather than the refusal
// [MoveLine] gives: an anchor standing on the mover, and an anchor one cell
// away. "Get next to that" is already true, so nothing stopped the move and
// StoppedBy has nothing to say.
func (s *DirectiveTestSuite) TestTowardAnAnchorAlreadyBesideTheMoverRoutesNowhere() {
	enc := s.towardCorridorScene()
	moverCell := s.cellOfMember(goblin)

	for _, tc := range []struct {
		name   string
		anchor spatial.Position
	}{
		{name: "on the mover", anchor: moverCell},
		{name: "one cell away", anchor: cellAt(6, 2)},
	} {
		s.Run(tc.name, func() {
			out, err := enc.Route(encounter.RouteInput{
				Mover: goblin, Policy: encounter.MoveToward, Anchor: tc.anchor, Budget: 6,
			})
			s.Require().NoError(err, "already there is not a refusal")
			s.Empty(out.Path)
			s.Empty(out.StoppedBy, "nothing stopped it; it was already over")
		})
	}
}

// TestTowardWithNoBudgetRoutesNowhere. Zero is unbounded to the field's own
// Limit, so a route that asked it with zero would flood the whole floor and
// hand a creature with no movement a walk across the dungeon.
func (s *DirectiveTestSuite) TestTowardWithNoBudgetRoutesNowhere() {
	enc := s.towardCorridorScene()

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: cellAt(1, 2), Budget: 0,
	})
	s.Require().NoError(err, "a pointless directive is legal, as RouteInput.Budget says")
	s.Empty(out.Path)
	s.Empty(out.StoppedBy, "nothing stopped it; it was never paid for")
}

// TestTowardStopsBesideAnEMPTYAnchorCell is the goal scan's own test, and the
// only scene in this file where the scan and the ruler fallback disagree.
//
// Every other Toward scene aims at a cell somebody is standing on, which the
// fold already calls unstandable — so the fallback, which only knows "nearest
// by the ruler", happens to give the same answer and the fewest-steps scan
// could be deleted without a test noticing.
//
// [Encounter.Route]'s Anchor is a POSITION, not a member: a rule may aim a
// directive at a cell nobody occupies, and [MoveToward]'s headline guarantee
// says it still stops beside rather than on it — "an empty anchor cell is
// still not where 'within 5 feet' ends". Without the scan the route walks the
// extra cell and ends ON the anchor at distance zero.
func (s *DirectiveTestSuite) TestTowardStopsBesideAnEMPTYAnchorCell() {
	enc := s.towardCorridorScene()
	anchor := cellAt(2, 2)
	const budget = 6

	s.Require().NotEqual(anchor, s.cellOfMember(alice), "the anchor cell is nobody's")
	s.Require().Equal(encounter.PassageStandable,
		enc.CellAt(encounter.CellAtInput{Cell: anchor, Mover: goblin}).Passage,
		"and the mover could legally stop on it, which is what makes this a real refusal")
	s.Require().Contains(reachedStandableWithin(enc, goblin, s.cellOfMember(goblin), budget), anchor,
		"and it is inside the budget")

	out, err := enc.Route(encounter.RouteInput{
		Mover: goblin, Policy: encounter.MoveToward, Anchor: anchor, Budget: budget,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Path)

	end := out.Path[len(out.Path)-1]
	s.Equal(cellAt(3, 2), end, "it stops on the ring")
	s.Equal(float64(1), enc.Distance(anchor, end), "beside the anchor, not on it")
	s.NotContains(out.Path, anchor, "and never enters the cell it was aimed at")
	s.Len(out.Path, 4)
}
