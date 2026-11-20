// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

// cannotFixture is the Keep's hold in author's terms: the guard's hold takes
// moving away until the holder rests, a snare takes it away until the run
// ends, and a feast takes nothing away.
const cannotFixture = `{
 "parts":{
  "no-move":{"kind":"cannot","params":{"what":["move"]}},
  "quick":{"kind":"initiative_bonus","amount":{"fixed":1}},
  "rest":{"kind":"end_on","params":{"beat":"rest","reason":"rested"}},
  "run-end":{"kind":"end_on","params":{"beat":"run_end","reason":"run_ended"}}
 },
 "benefit_sets":{"hold":["no-move"],"quick":["quick"]},
 "definitions":{
  "held":{"name":"Held","coordinator":"recipient","maintenance":[{"key":"rest","part":"rest"}],"benefits":[{"key":"hold","set":"hold"}]},
  "snared":{"name":"Snared","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"hold","set":"hold"}]},
  "well-fed":{"name":"Well fed","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"quick","set":"quick"}]}
 }
}`

type cannotSuite struct{ suite.Suite }

func TestCannotSuite(t *testing.T) { suite.Run(t, new(cannotSuite)) }

func (s *cannotSuite) setup() (*definitionWorld, *conditions.CompiledDefinitions) {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(cannotFixture)})
	s.Require().NoError(err)
	return w, defs.Definitions
}

func (s *cannotSuite) give(w *definitionWorld, defs *conditions.CompiledDefinitions, id, target string) {
	prepared, _, err := w.prepare(defs, id, target, target, id+"-"+target)
	s.Require().NoError(err)
	s.Require().NoError(w.apply(prepared))
}

func (s *cannotSuite) cannotMove(w *definitionWorld, who string) *conditions.CannotMoveOutput {
	out, err := w.holder(who).CannotMove()
	s.Require().NoError(err)
	return out
}

// Catches a reader that answers for a non-holder, one that keeps answering
// after the hold ends, or one that ignores the kind.
func (s *cannotSuite) TestAHoldTakesMovingFromItsHolderUntilItEnds() {
	w, defs := s.setup()
	s.False(s.cannotMove(w, "B").CannotMove, "a member holding nothing may move")

	s.give(w, defs, "held", "B")
	held := s.cannotMove(w, "B")
	s.True(held.CannotMove)
	s.Equal([]string{"Held"}, held.Sources)
	s.False(s.cannotMove(w, "A").CannotMove, "another member may move")
	s.Empty(s.cannotMove(w, "A").Sources)

	s.Require().NoError(w.rest("B"))
	free := s.cannotMove(w, "B")
	s.False(free.CannotMove, "ending the hold gives moving back")
	s.Empty(free.Sources)
}

// Catches a reader that keeps only the last hold, or one that drops both when
// one ends.
func (s *cannotSuite) TestTwoHoldsAreBothNamedAndEndingOneLeavesTheOther() {
	w, defs := s.setup()
	s.give(w, defs, "held", "B")
	s.give(w, defs, "snared", "B")
	s.give(w, defs, "well-fed", "B")
	both := s.cannotMove(w, "B")
	s.True(both.CannotMove)
	s.ElementsMatch([]string{"Held", "Snared"}, both.Sources, "a status that takes nothing away is not named")

	s.Require().NoError(w.rest("B"))
	left := s.cannotMove(w, "B")
	s.True(left.CannotMove)
	s.Equal([]string{"Snared"}, left.Sources)
}

// Catches a hold that is lost, or doubled, by a save and load.
func (s *cannotSuite) TestAHoldSurvivesASaveAndReload() {
	w, defs := s.setup()
	s.give(w, defs, "snared", "B")
	s.Require().NoError(w.reload())
	held := s.cannotMove(w, "B")
	s.True(held.CannotMove)
	s.Equal([]string{"Snared"}, held.Sources)
	s.False(s.cannotMove(w, "A").CannotMove)
}

// Catches the per-condition reader reading the wrong kind.
func (s *cannotSuite) TestTakenAwayReadsOneCondition() {
	w, defs := s.setup()
	s.give(w, defs, "held", "B")
	s.give(w, defs, "well-fed", "C")
	found := false
	for _, held := range w.holder("B").GetConditions() {
		out, err := conditions.TakenAway(&conditions.TakenAwayInput{Condition: held})
		s.Require().NoError(err)
		if len(out.Taken) > 0 {
			s.Equal([]dndfx.Taken{dndfx.TakenMove}, out.Taken)
			found = true
		}
	}
	s.True(found, "the hold takes moving away")
	for _, held := range w.holder("C").GetConditions() {
		out, err := conditions.TakenAway(&conditions.TakenAwayInput{Condition: held})
		s.Require().NoError(err)
		s.Empty(out.Taken, "initiative takes nothing away")
	}
	_, err := conditions.TakenAway(nil)
	s.Error(err)
	_, err = conditions.TakenAway(&conditions.TakenAwayInput{})
	s.Error(err)
	_, err = conditions.CannotMove(nil)
	s.Error(err)
}

// Catches a row that claims to apply to an attack, or one with no plain words.
func (s *cannotSuite) TestTheHoldDescribesItselfInItsRow() {
	w, defs := s.setup()
	s.give(w, defs, "held", "B")
	frame, err := w.frame("B", "C")
	s.Require().NoError(err)
	info, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: w.holder("B").GetConditions(), Frame: frame})
	s.Require().NoError(err)
	found := false
	for _, row := range info.Effects {
		if row.Description == "Cannot move." {
			found = true
			s.Equal(contributions.StateDoesNotApply, row.State, "it does not bear on an attack")
			s.Empty(row.Benefit)
		}
	}
	s.True(found, "a row reads \"Cannot move.\"")
}

// Catches a kind that accepts an amount, a guard, an empty, unknown or
// repeated list, or no params at all.
func (s *cannotSuite) TestRefusals() {
	for name, tc := range map[string]struct{ part, want string }{
		"amount":    {`{"kind":"cannot","params":{"what":["move"]},"amount":{"fixed":1}}`, "does not take an amount"},
		"guard":     {`{"kind":"cannot","params":{"what":["move"]},"when":{"fact":{"name":"action.melee","op":"eq","value":true}}}`, "does not take a guard"},
		"empty":     {`{"kind":"cannot","params":{"what":[]}}`, "at least one thing taken away"},
		"unknown":   {`{"kind":"cannot","params":{"what":["speak"]}}`, `does not know "speak"; it can take away: move`},
		"repeated":  {`{"kind":"cannot","params":{"what":["move","move"]}}`, `lists "move" twice`},
		"no params": {`{"kind":"cannot"}`, "requires params naming what is taken"},
	} {
		s.Run(name, func() {
			raw := strings.Replace(cannotFixture, `{"kind":"cannot","params":{"what":["move"]}}`, tc.part, 1)
			s.Require().NotEqual(cannotFixture, raw)
			_, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
			s.Require().Error(err)
			s.Contains(err.Error(), tc.want)
		})
	}
}
