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

	enc   *encounter.Encounter
	mover *recordingMover

	casterCell spatial.Position
	moverCell  spatial.Position
	pillarCell spatial.Position
}

func TestDirectiveSuite(t *testing.T) {
	suite.Run(t, new(DirectiveTestSuite))
}

func (s *DirectiveTestSuite) SetupTest() {
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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: &scriptedDriver{}, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
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

	beats := s.movedBeats(alice)
	s.Require().NotEmpty(beats)
	last := beats[len(beats)-1]
	s.Equal(string(goblin), last["member"])
	s.Equal(thunderwaveRef.String(), last["cause"], "an observer can tell a shove from a step")

	clockOf, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(alice, clockOf.Active, "a push is nobody's turn and spends nobody's turn")
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

// TestAnyPolicyButTheLineIsRefused.
//
// The line is the only policy that exists, because a policy arrives with the
// thing that carries it out. Every other word fails closed and loudly rather
// than returning an empty path — which would read as "there was nowhere to go"
// and be indistinguishable from a creature pinned against a wall.
//
// "away" is spelled out here on purpose: it is the next policy anybody will
// reach for (Dissonant Whispers), and it must be refused today exactly as
// firmly as a typo is.
func (s *DirectiveTestSuite) TestAnyPolicyButTheLineIsRefused() {
	enc := s.lineScene(false)

	for _, policy := range []encounter.MovePolicy{"", "away", "toward", "sideways"} {
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
