// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// towardIntent is a `toward` row's walk at its own pace, the shape the older
// driver tests ask for by name.
func (d TableDriver) towardIntent(view MonsterView, entry Answer) TurnIntent {
	return d.walkIntent(view, entry, *entry.Toward, walkPace(view))
}

// dash_internal_test.go pins the driver's half of the dash (rpg-project#671)
// on hand-built views: the run's cut and flag, where a row lands for
// `threat`, and which landings this view cannot know.

type DashDriverSuite struct {
	suite.Suite
}

func TestDashDriverSuite(t *testing.T) { suite.Run(t, new(DashDriverSuite)) }

// row is a straight run of n cells east from x=1.
func row(n int) []spatial.Position {
	out := make([]spatial.Position, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, spatial.Position{X: float64(1 + i), Y: 0})
	}

	return out
}

// farView is a view whose one foe is a twenty-cell walk away, in sight.
func farView() MonsterView {
	return MonsterView{
		Position: spatial.Position{X: 1, Y: 0},
		Budget:   TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Seen: []SeenMember{{
			ID: "alice", Opposed: true, Standing: true, DistanceCells: 21, Path: row(20),
		}},
	}
}

// TestADashRowIsCutToTwiceTheBudgetAndFlagged: six cells of movement, a run
// of twelve, marked so the encounter charges it; a `toward` on the same view
// is the walk's six and is not.
func (s *DashDriverSuite) TestADashRowIsCutToTwiceTheBudgetAndFlagged() {
	d := TableDriver{}
	view := farView()
	enemy := &Selector{Word: SelectorEnemy}

	dash, err := d.intentFor(view, Answer{Dash: enemy})
	s.Require().NoError(err)
	s.Equal(Move{Path: row(12), Target: "alice", Dash: true}, dash)

	walk, err := d.intentFor(view, Answer{Toward: enemy})
	s.Require().NoError(err)
	s.Equal(Move{Path: row(6), Target: "alice"}, walk)
}

// TestADashToACellIsRoutedUnderItsOwnCause: the encounter routes a run to a
// cell as it routes a walk to one, under the dash's cause.
func (s *DashDriverSuite) TestADashToACellIsRoutedUnderItsOwnCause() {
	cell := spatialOrigin
	got, err := TableDriver{}.intentFor(farView(), Answer{Dash: &Selector{At: &cell}})
	s.Require().NoError(err)
	routed, ok := got.(Routed)
	s.Require().True(ok)
	s.True(routed.Dash)
	s.Equal("encounter:table:dash", routed.Cause.String())
}

// TestAThreatIsJudgedAtTheRowsLandingAndNotItsTarget: a run lands twelve
// cells on, a walk six, a hold where the creature stands. A zone that covers
// only the run's landing reads `walk` for the run and `none` for the others;
// the same zone belonging to the row's own target counts for nobody.
func (s *DashDriverSuite) TestAThreatIsJudgedAtTheRowsLandingAndNotItsTarget() {
	enemy := &Selector{Word: SelectorEnemy}
	runLands := row(12)[11]

	view := farView()
	view.Threats = []ThreatZone{{Member: "bob", Walk: []spatial.Position{runLands}}}
	threat := threatJudge(view)
	s.Equal(behavior.ThreatWalk, threat(Answer{Dash: enemy}), "the run lands in bob's walk")
	s.Equal(behavior.ThreatNone, threat(Answer{Toward: enemy}), "the walk lands short of it")
	s.Equal(behavior.ThreatNone, threat(Answer{Hold: true}), "and standing is nowhere near")

	view.Threats = []ThreatZone{{Member: "bob", Dash: []spatial.Position{runLands}}}
	s.Equal(behavior.ThreatDash, threatJudge(view)(Answer{Dash: enemy}), "only a dash brings bob there")

	view.Threats = []ThreatZone{{Member: "alice", Walk: []spatial.Position{runLands}}}
	s.Equal(behavior.ThreatNone, threatJudge(view)(Answer{Dash: enemy}),
		"alice is the run's own target, and hitting back is the point")
}

// TestARowWithNoMovementLeftStillExcludesItsTarget: after the run the
// creature stands beside alice with nothing left to walk; its dash row
// lands where it stands, and alice is still the row's own target.
func (s *DashDriverSuite) TestARowWithNoMovementLeftStillExcludesItsTarget() {
	view := farView()
	view.Budget.MovementFeet = 0
	view.Threats = []ThreatZone{{Member: "alice", Walk: []spatial.Position{view.Position}}}
	s.Equal(behavior.ThreatNone, threatJudge(view)(Answer{Dash: &Selector{Word: SelectorEnemy}}))
}

// TestALandingTheViewCannotKnowIsNoAnswer: a walk the encounter routes itself
// has no path on the view, so its threat is the empty answer and a `threat`
// row of that word is never a candidate, `none` included.
func (s *DashDriverSuite) TestALandingTheViewCannotKnowIsNoAnswer() {
	cell := spatialOrigin
	threat := threatJudge(farView())
	for name, answer := range map[string]Answer{
		"a run to a cell":     {Dash: &Selector{At: &cell}},
		"a walk to a cell":    {Toward: &Selector{At: &cell}},
		"a run to a sound":    {Dash: &Selector{Word: SelectorSound}},
		"a run from somebody": {Away: &Selector{Word: SelectorEnemy}},
		"a search":            {Search: &Search{Distance: 3, Heading: HeadingWent}},
	} {
		s.Equal(behavior.ThreatWord(""), threat(answer), name)
	}
}

// TestOnlyATableThatAsksPaysForTheZones: `threat` anywhere — in a group, in
// a `not` — is what makes the view carry zones; a table that never asks
// carries none.
func (s *DashDriverSuite) TestOnlyATableThatAsksPaysForTheZones() {
	asks := When{Threat: behavior.ThreatNone}
	s.False(tableNamesThreat([]Answer{{Hold: true, When: []When{{Enemy: EnemySeen}}}}))
	s.True(tableNamesThreat([]Answer{{Hold: true, When: []When{asks}}}))
	s.True(tableNamesThreat([]Answer{{Hold: true, When: []When{{Not: &asks}}}}))
	s.True(tableNamesThreat([]Answer{{Rows: []Answer{{Hold: true, When: []When{asks}}}}}))
}

// TestARememberedFoeIsMeasuredToReachNotOntoItsCell: the route walks onto
// the memory, and reach stops one short of it — so a seven-cell route is a
// six-cell walk, `walk` on six cells of movement.
func (s *DashDriverSuite) TestARememberedFoeIsMeasuredToReachNotOntoItsCell() {
	view := MonsterView{
		Position: spatial.Position{X: 1, Y: 0},
		Budget:   TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Remembered: []RememberedMember{{
			ID: "alice", Opposed: true, Position: row(7)[6], DistanceCells: 7, Path: row(7), ReachTail: 1,
		}},
	}
	facts := factsFromView(view)
	s.True(facts.EnemyWalk)
	s.False(facts.EnemyDash)

	view.Remembered[0].ReachTail = 0
	facts = factsFromView(view)
	s.True(facts.EnemyDash, "a hand-built view with no tail reads the whole route")
}
