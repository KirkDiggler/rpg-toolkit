// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package definitions_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/definitions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type completeRagingDefinitionSuite struct{ suite.Suite }

func TestCompleteRagingDefinitionSuite(t *testing.T) {
	suite.Run(t, new(completeRagingDefinitionSuite))
}
func (s *completeRagingDefinitionSuite) TestWholeGroupsAndFieldsAreTheSameValues() {
	bonus := 2
	rage, err := definitions.Raging(&definitions.RagingInput{DamageBonus: definitions.RagingDamageBonus{Fixed: &bonus}})
	s.Require().NoError(err)
	whole, err := effects.Compile(&effects.CompileInput{Definition: rage})
	s.Require().NoError(err)
	var paths [][]string
	for _, leaf := range whole.Definition.Leaves() {
		paths = append(paths, leaf.Path)
	}
	s.Equal([][]string{
		{"benefits", "damage-bonus"}, {"benefits", "physical-resistance"}, {"benefits", "strength-saving-throws"}, {"benefits", "strength-checks"},
		{"maintenance", "combat-activity"}, {"maintenance", "duration"}, {"maintenance", "endings", "zero-hp"}, {"maintenance", "endings", "rest"}, {"maintenance", "endings", "combat-end"},
	}, paths)
	group, err := effects.Compile(&effects.CompileInput{Definition: rage.Benefits})
	s.Require().NoError(err)
	s.Require().Len(group.Definition.Leaves(), 4)
	single, err := effects.Compile(&effects.CompileInput{Definition: rage.Benefits.PhysicalResistance})
	s.Require().NoError(err)
	s.Require().Len(single.Definition.Leaves(), 1)
	s.Equal(effects.KindResistance, single.Definition.Leaves()[0].Effect.Kind)
	s.Equal(group.Definition.Leaves()[1].Effect, single.Definition.Leaves()[0].Effect)
	s.True(rage.Maintenance.CombatActivity.SkipFirst)
	s.Equal(10, rage.Maintenance.Duration.Rounds)
	s.Equal("duration_expired", rage.Maintenance.Duration.ExpiredReason)
	s.Equal("clock_reset", rage.Maintenance.Duration.ResetReason)
}
func (s *completeRagingDefinitionSuite) TestFieldsDriveProjectionAndCompilationOwnsCopies() {
	bonus := 2
	a, err := definitions.Raging(&definitions.RagingInput{DamageBonus: definitions.RagingDamageBonus{Fixed: &bonus}})
	s.Require().NoError(err)
	b, err := definitions.Raging(&definitions.RagingInput{DamageBonus: definitions.RagingDamageBonus{Fixed: &bonus}})
	s.Require().NoError(err)
	frozen, err := effects.Compile(&effects.CompileInput{Definition: a.Benefits})
	s.Require().NoError(err)
	bonus = 99
	a.Benefits.PhysicalResistance.Types[0] = damage.Fire
	*a.Benefits.DamageBonus.Amount.Fixed = 5
	a.Benefits.StrengthSavingThrows.When.Fact.Value = "dex"
	s.Equal("str", a.Benefits.StrengthChecks.When.Fact.Value, "one field does not alias its sibling")
	changed, err := effects.Compile(&effects.CompileInput{Definition: a.Benefits})
	s.Require().NoError(err)
	s.Equal(5, *changed.Definition.Leaves()[0].Effect.Amount.Fixed)
	s.Contains(string(changed.Definition.Leaves()[1].Effect.Params), "fire")
	s.Equal(2, *frozen.Definition.Leaves()[0].Effect.Amount.Fixed)
	s.NotContains(string(frozen.Definition.Leaves()[1].Effect.Params), "fire")
	s.Equal(damage.Bludgeoning, b.Benefits.PhysicalResistance.Types[0])
	s.Equal(2, *b.Benefits.DamageBonus.Amount.Fixed)
}
func (s *completeRagingDefinitionSuite) TestNativeCatalogueProjectsTheCompleteTypedDefinition() {
	catalogue, err := definitions.Catalogue(&definitions.CatalogueInput{})
	s.Require().NoError(err)
	resolved, err := effects.Resolve(&effects.ResolveInput{Catalogue: catalogue.Catalogue, Definition: effects.AuthoredDefinition{Use: &effects.DefinitionUse{Reference: effects.DefinitionReference{Definition: *refs.Definitions.Raging()}}}})
	s.Require().NoError(err)
	native, err := definitions.Raging(&definitions.RagingInput{DamageBonus: definitions.RagingDamageBonus{BarbarianLevel: true}})
	s.Require().NoError(err)
	compiled, err := effects.Compile(&effects.CompileInput{Definition: native})
	s.Require().NoError(err)
	s.Equal(compiled.Definition.Data(), resolved.Definition)
	levels := compiled.Definition.Leaves()[0].Effect.Amount.ByClassLevel
	s.Require().NotNil(levels)
	s.Equal(2, levels.Steps[1])
	s.Equal(3, levels.Steps[9])
	s.Equal(4, levels.Steps[16])
}
func (s *completeRagingDefinitionSuite) TestInvalidOrAmbiguousDamageChoiceRefuses() {
	out, err := definitions.Raging(nil)
	s.Error(err)
	s.Nil(out)
	zero, negative, positive := 0, -1, 2
	for _, choice := range []definitions.RagingDamageBonus{{}, {Fixed: &zero}, {Fixed: &negative}, {Fixed: &positive, BarbarianLevel: true}} {
		out, err = definitions.Raging(&definitions.RagingInput{DamageBonus: choice})
		s.Error(err)
		s.Nil(out)
	}
	catalogue, err := definitions.Catalogue(nil)
	s.Error(err)
	s.Nil(catalogue)
}
