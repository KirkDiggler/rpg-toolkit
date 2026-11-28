// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Bytes captured by marshalling SecondWind.Definition of a fighter named
// "hero" on origin/main 6c766ce1, before second_wind.go changed.
const (
	fighterLevel1Bytes = `{"description":"Spend a use to regain 1d10 hit points plus your fighter level.","ref":"dnd5e:features:second_wind","name":"Second Wind","cost":{"slots":{"bonus_action":1},"pools":{"second_wind":1}},"effect":{"to":"self","healing":{"Dice":"1d10","Modifiers":[{"Source":{"Ref":"dnd5e:classes:fighter","Name":"Fighter","Label":"Fighter level","SourceID":"hero"},"Amount":1}]}}}`
	fighterLevel3Bytes = `{"description":"Spend a use to regain 1d10 hit points plus your fighter level.","ref":"dnd5e:features:second_wind","name":"Second Wind","cost":{"slots":{"bonus_action":1},"pools":{"second_wind":1}},"effect":{"to":"self","healing":{"Dice":"1d10","Modifiers":[{"Source":{"Ref":"dnd5e:classes:fighter","Name":"Fighter","Label":"Fighter level","SourceID":"hero"},"Amount":3}]}}}`
)

// SecondWindNamedAbilitySuite pins Second Wind as a named ability any giver
// can ask for with values.
type SecondWindNamedAbilitySuite struct {
	suite.Suite
}

func TestSecondWindNamedAbilitySuite(t *testing.T) {
	suite.Run(t, new(SecondWindNamedAbilitySuite))
}

func (s *SecondWindNamedAbilitySuite) ringRef() *core.Ref {
	ref, err := core.ParseString("example:items:second_wind_ring")
	s.Require().NoError(err)
	return ref
}

func (s *SecondWindNamedAbilitySuite) ring() dnd5eEvents.RollSource {
	return dnd5eEvents.RollSource{Ref: s.ringRef(), Name: "Second Wind ring", Label: "Ring level", SourceID: "ring-17"}
}

func (s *SecondWindNamedAbilitySuite) ask(params string, pools map[coreResources.ResourceKey]int) (*actions.Definition, error) {
	return DescribeFromRef(&DescribeFromRefInput{
		Ref: *refs.Features.SecondWind(), Parameters: json.RawMessage(params), Giver: s.ring(), Pools: pools,
	})
}

// Test 1: a fighter's description is byte for byte what it was.
func (s *SecondWindNamedAbilitySuite) TestAFightersDescriptionIsUnchangedByteForByte() {
	for level, want := range map[int]string{1: fighterLevel1Bytes, 3: fighterLevel3Bytes} {
		owner := &StubEntity{id: "hero", levels: map[classes.Class]int{classes.Fighter: level}}
		definition, err := newSecondWindForTest("sw", "hero").Definition(owner)
		s.Require().NoError(err)
		got, err := json.Marshal(definition)
		s.Require().NoError(err)
		s.Equal(want, string(got), "fighter level %d", level)
	}
}

// Test 2: a giver that is not a class.
func (s *SecondWindNamedAbilitySuite) TestARingAsksForLevelOne() {
	definition, err := s.ask(`{"level": 1}`, nil)

	s.Require().NoError(err)
	s.Equal(*refs.Features.SecondWind(), definition.Ref)
	s.Equal("Second Wind", definition.Name)
	s.Equal("Regain 1d10 hit points plus 1", definition.Description)
	s.Require().NotNil(definition.Cost)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	s.Empty(definition.Cost.Pools)
	s.Require().NotNil(definition.Effect)
	s.Equal(actions.EffectSelf, definition.Effect.To)
	s.Require().NotNil(definition.Effect.Healing)
	s.Equal("1d10", definition.Effect.Healing.Dice)
	s.Require().Len(definition.Effect.Healing.Modifiers, 1)
	modifier := definition.Effect.Healing.Modifiers[0]
	s.Equal(1, modifier.Amount)
	s.Equal(s.ring(), modifier.Source)
	s.Require().NoError(definition.Validate())
}

// Test 3: the level is the giver's; the pools are copied.
func (s *SecondWindNamedAbilitySuite) TestALevelThreeRingHealsByThree() {
	definition, err := s.ask(`{"level": 3}`, nil)

	s.Require().NoError(err)
	s.Equal(3, definition.Effect.Healing.Modifiers[0].Amount)
	s.Equal("Regain 1d10 hit points plus 3", definition.Description)
	s.Require().NoError(definition.Validate())
}

func (s *SecondWindNamedAbilitySuite) TestPoolsPayAndAreCopied() {
	pools := map[coreResources.ResourceKey]int{"charges": 1}

	definition, err := s.ask(`{"level": 1}`, pools)
	pools["charges"] = 9
	pools["other"] = 1

	s.Require().NoError(err)
	s.Equal(map[coreResources.ResourceKey]int{"charges": 1}, definition.Cost.Pools)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	s.Require().NoError(definition.Validate())
}

type refusalCase struct {
	name     string
	input    *DescribeFromRefInput
	contains []string
	// parameter is true when ValidateParameters refuses it too, with the same sentence.
	parameter bool
}

func (s *SecondWindNamedAbilitySuite) refusals() []refusalCase {
	sw := *refs.Features.SecondWind()
	other := core.Ref{Module: "dnd5e", Type: "features", ID: "no_such_thing"}
	with := func(ref core.Ref, params string) *DescribeFromRefInput {
		return &DescribeFromRefInput{Ref: ref, Parameters: json.RawMessage(params), Giver: s.ring()}
	}
	return []refusalCase{
		{"unknown ref", with(other, `{"level":1}`), []string{other.String()}, true},
		{"missing parameters", with(sw, ``), []string{"second wind", "parameters"}, true},
		{"unknown member", with(sw, `{"level":1,"bonus":2}`), []string{"bonus"}, true},
		{"level zero", with(sw, `{"level":0}`), []string{"level", "at least 1"}, true},
		{"level negative", with(sw, `{"level":-2}`), []string{"level", "at least 1"}, true},
		{"level absent", with(sw, `{}`), []string{"level", "at least 1"}, true},
		{"giver without ref", &DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":1}`),
			Giver: dnd5eEvents.RollSource{Name: "n", Label: "l", SourceID: "id"}}, []string{"giver", "Ref"}, false},
		{"giver without label", &DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":1}`),
			Giver: dnd5eEvents.RollSource{Ref: s.ringRef(), Name: "n", SourceID: "id"}}, []string{"giver", "Label"}, false},
		{"giver without source id", &DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":1}`),
			Giver: dnd5eEvents.RollSource{Ref: s.ringRef(), Name: "n", Label: "l"}}, []string{"giver", "SourceID"}, false},
		{"pool with empty key", &DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":1}`), Giver: s.ring(),
			Pools: map[coreResources.ResourceKey]int{"": 1}}, []string{"pool", "key"}, false},
		{"pool below one", &DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":1}`), Giver: s.ring(),
			Pools: map[coreResources.ResourceKey]int{"charges": 0}}, []string{"charges", "at least 1"}, false},
	}
}

// Test 4: every refusal names what is wrong.
func (s *SecondWindNamedAbilitySuite) TestEachRefusalNamesTheThing() {
	_, err := DescribeFromRef(nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "input")

	for _, c := range s.refusals() {
		definition, err := DescribeFromRef(c.input)
		s.Require().Error(err, c.name)
		s.Nil(definition, c.name)
		for _, word := range c.contains {
			s.Contains(err.Error(), word, c.name)
		}
	}
}

// Test 5: one table drives both; ValidateParameters agrees on every parameter case.
func (s *SecondWindNamedAbilitySuite) TestValidateParametersAgreesWithDescribeFromRef() {
	for _, c := range s.refusals() {
		validateErr := ValidateParameters(&ValidateParametersInput{Ref: c.input.Ref, Parameters: c.input.Parameters})
		_, describeErr := DescribeFromRef(c.input)
		s.Require().Error(describeErr, c.name)
		if c.parameter {
			s.Require().Error(validateErr, c.name)
			s.Equal(describeErr.Error(), validateErr.Error(), c.name)
		} else {
			s.NoError(validateErr, "%s: giver and pools are not ValidateParameters' business", c.name)
		}
	}

	s.NoError(ValidateParameters(&ValidateParametersInput{Ref: *refs.Features.SecondWind(), Parameters: json.RawMessage(`{"level":2}`)}))
	s.Error(ValidateParameters(nil))
}

func TestNamedAbility(t *testing.T) {
	ref, ok := NamedAbility("second wind")
	if !ok || ref != *refs.Features.SecondWind() {
		t.Fatalf("second wind = %v, %v", ref, ok)
	}
	for _, name := range []string{"Second Wind", "second  wind", " second wind", "no such ability", ""} {
		if _, ok := NamedAbility(name); ok {
			t.Errorf("%q must not be found", name)
		}
	}
	names := NamedAbilities()
	if len(names) != 3 || names[0] != "action surge" || names[1] != "rage" || names[2] != "second wind" {
		t.Fatalf("NamedAbilities = %v", names)
	}
}

// Every listed name is an ability the rulebook can describe.
func TestNamedAbilitiesAreDescribable(t *testing.T) {
	valid := map[string]string{"second wind": `{"level":1}`, "action surge": `{}`, "rage": `{"damage_bonus":2}`}
	for _, name := range NamedAbilities() {
		ref, ok := NamedAbility(name)
		if !ok {
			t.Fatalf("%q is listed but not found", name)
		}
		params, known := valid[name]
		if !known {
			t.Fatalf("%q has no valid values in this test", name)
		}
		if err := ValidateParameters(&ValidateParametersInput{Ref: ref, Parameters: json.RawMessage(params)}); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
}
