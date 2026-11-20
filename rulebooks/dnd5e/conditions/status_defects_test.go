// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/checks"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// statusFixture holds the shapes the first data-authored status exposed:
// a recipient-coordinated status, a user-coordinated one without
// concentration, and one a user concentrates on.
const statusFixture = `{
 "parts":{
  "str-checks":{"kind":"advantage","params":{"on":"ability_check"},"when":{"fact":{"name":"action.ability","op":"eq","value":"str"}}},
  "rest":{"kind":"end_on","params":{"beat":"rest","reason":"rest"}},
  "hold":{"kind":"concentration"}
 },
 "benefit_sets":{"strong":["str-checks"]},
 "definitions":{
  "mighty":{"name":"Mighty","coordinator":"recipient","maintenance":[{"key":"rest","part":"rest"}],"benefits":[{"key":"strong","set":"strong"}]},
  "charm":{"name":"Mighty charm","coordinator":"user","maintenance":[{"key":"rest","part":"rest"}],"benefits":[{"key":"strong","set":"strong"}]},
  "blessing":{"name":"Mighty blessing","coordinator":"user","maintenance":[{"key":"hold","part":"hold"}],"benefits":[{"key":"strong","set":"strong"}]}
 }
}`

type statusDefectsSuite struct{ suite.Suite }

func TestStatusDefectsSuite(t *testing.T) { suite.Run(t, new(statusDefectsSuite)) }

func (s *statusDefectsSuite) setup() (*definitionWorld, *conditions.CompiledDefinitions) {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(statusFixture)})
	s.Require().NoError(err)
	return w, defs.Definitions
}

func (s *statusDefectsSuite) give(w *definitionWorld, defs *conditions.CompiledDefinitions, id, user, target, application string) {
	prepared, _, err := w.prepare(defs, id, user, target, application)
	s.Require().NoError(err)
	s.Require().NoError(w.apply(prepared))
}

// rows is how many status-view rows a holder shows under a name.
func (s *statusDefectsSuite) rows(w *definitionWorld, holder, name string) int {
	view, err := w.holder(holder).StatusView(&character.StatusViewInput{})
	s.Require().NoError(err)
	n := 0
	for _, condition := range view.View.Conditions {
		if condition.Name == name {
			n++
		}
	}
	return n
}

// check rolls a real ability check through the checks package on the world's
// bus, and reports whether its keep record names the status.
func (s *statusDefectsSuite) check(w *definitionWorld, skill skills.Skill, ability abilities.Ability, status string) bool {
	route := dndevents.RollSource{Ref: &core.Ref{Module: "example", Type: "checks", ID: "route"}, Name: "Route"}
	result, err := checks.MakeAbilityCheck(context.Background(), &checks.AbilityCheckInput{
		Roller: dice.NewRoller(), EventBus: w.bus, CheckerID: "B", Skill: skill, Ability: ability, DC: 10,
		D20Source: route, ModifierSource: route,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result.Calculation)
	s.Require().NotEmpty(result.Calculation.Components)
	trace := result.Calculation.Components[0].Dice
	s.Require().NotNil(trace)
	if trace.Keep == nil {
		return false
	}
	for _, granted := range trace.Keep.Granted {
		if granted.Name == status {
			return true
		}
	}
	return false
}

// A BARE-ABILITY CHECK SEES THE ABILITY: a lock's Strength check answers an
// ability-guarded advantage exactly as an Athletics check does, and a check
// of another ability, skill or bare, carries none.
func (s *statusDefectsSuite) TestAnAbilityGuardAnswersSkillAndBareAbilityChecks() {
	w, defs := s.setup()
	s.give(w, defs, "mighty", "B", "B", "mighty")

	s.True(s.check(w, "", abilities.STR, "Mighty"), "bare Strength check")
	s.True(s.check(w, skills.Athletics, "", "Mighty"), "Athletics check")
	s.True(s.check(w, skills.Athletics, abilities.STR, "Mighty"), "Athletics naming its own ability")
	s.False(s.check(w, "", abilities.DEX, "Mighty"), "bare Dexterity check")
	s.False(s.check(w, skills.Acrobatics, "", "Mighty"), "Acrobatics check")
}

// A check whose stated ability is not its skill's is refused: the two would
// answer an ability guard differently.
func (s *statusDefectsSuite) TestASkillAndADisagreeingAbilityAreRefused() {
	w, _ := s.setup()
	route := dndevents.RollSource{Ref: &core.Ref{Module: "example", Type: "checks", ID: "route"}, Name: "Route"}
	out, err := checks.MakeAbilityCheck(context.Background(), &checks.AbilityCheckInput{
		Roller: dice.NewRoller(), EventBus: w.bus, CheckerID: "B", Skill: skills.Athletics, Ability: abilities.DEX, DC: 10,
		D20Source: route, ModifierSource: route,
	})
	s.Error(err)
	s.Nil(out)
}

// ONE STATUS, ONE ROW: a member holding both halves of one application shows
// it once, by the definition's name — whether the definition coordinates on
// the recipient or the user gives it to themselves.
func (s *statusDefectsSuite) TestBothHalvesOnOneSheetShowOneRow() {
	w, defs := s.setup()
	s.give(w, defs, "mighty", "B", "B", "mighty")
	s.give(w, defs, "charm", "A", "A", "charm")

	s.Equal(1, s.rows(w, "B", "Mighty"))
	s.Equal(1, s.rows(w, "A", "Mighty charm"))
	s.Require().NoError(w.reload())
	s.Equal(1, s.rows(w, "B", "Mighty"), "and still once after a reload")
}

// SPLIT HALVES KEEP THEIR ROWS: a status one member gives another without
// concentration shows on each sheet, and a concentrating caster shows
// "Concentrating on" beside the benefit — on another's sheet or their own.
func (s *statusDefectsSuite) TestSplitOrConcentratingHalvesKeepTheirRows() {
	w, defs := s.setup()
	s.give(w, defs, "charm", "A", "B", "charm")
	s.Equal(1, s.rows(w, "A", "Mighty charm"), "the giver's coordinator")
	s.Equal(1, s.rows(w, "B", "Mighty charm"), "the receiver's benefit")

	s.give(w, defs, "blessing", "C", "B", "blessing")
	s.Equal(1, s.rows(w, "C", "Concentrating on Mighty blessing"))
	s.Equal(1, s.rows(w, "B", "Mighty blessing"))

	self, selfDefs := s.setup()
	s.give(self, selfDefs, "blessing", "A", "A", "self-blessing")
	s.Equal(1, s.rows(self, "A", "Concentrating on Mighty blessing"))
	s.Equal(1, s.rows(self, "A", "Mighty blessing"))
}
