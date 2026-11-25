// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// SecondWindDefinitionSuite pins the description a fighter's Second Wind
// compiles to: the data an author would write, with this owner's numbers.
type SecondWindDefinitionSuite struct {
	suite.Suite
}

func TestSecondWindDefinitionSuite(t *testing.T) { suite.Run(t, new(SecondWindDefinitionSuite)) }

func (s *SecondWindDefinitionSuite) fighter(level int) *StubEntity {
	return &StubEntity{id: "hero", levels: map[classes.Class]int{classes.Fighter: level}}
}

func (s *SecondWindDefinitionSuite) TestALevelOneFightersDescription() {
	feature := newSecondWindForTest("sw", "hero")

	definition, err := feature.Definition(s.fighter(1))

	s.Require().NoError(err)
	s.Require().NotNil(definition)
	s.Equal(*refs.Features.SecondWind(), definition.Ref)
	s.Equal("Second Wind", definition.Name)

	s.Require().NotNil(definition.Cost)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	s.Equal(map[coreResources.ResourceKey]int{resources.SecondWind: 1}, definition.Cost.Pools)
	s.Empty(definition.Cost.Capacity)
	s.Empty(definition.Cost.Grants)
	s.Empty(definition.Cost.Requires)

	s.Nil(definition.Attack)
	s.Nil(definition.Cast)
	s.Nil(definition.Sequence)
	s.Require().NotNil(definition.Effect)
	s.Equal(actions.EffectSelf, definition.Effect.To)
	s.Require().NotNil(definition.Effect.Healing)
	s.Equal("1d10", definition.Effect.Healing.Dice)
	s.Require().Len(definition.Effect.Healing.Modifiers, 1)
	modifier := definition.Effect.Healing.Modifiers[0]
	s.Equal(1, modifier.Amount)
	s.Require().NotNil(modifier.Source.Ref)
	s.Equal(*refs.Classes.Fighter(), *modifier.Source.Ref)
	s.Equal("Fighter level", modifier.Source.Label)
	s.Equal("hero", modifier.Source.SourceID)

	s.Require().NoError(definition.Validate())
}

func (s *SecondWindDefinitionSuite) TestTheModifierIsTheFighterLevel() {
	definition, err := newSecondWindForTest("sw", "hero").Definition(s.fighter(3))

	s.Require().NoError(err)
	s.Require().Len(definition.Effect.Healing.Modifiers, 1)
	s.Equal(3, definition.Effect.Healing.Modifiers[0].Amount)
	s.Require().NoError(definition.Validate())
}

func (s *SecondWindDefinitionSuite) TestAnOwnerWithNoFighterLevelIsRefusedAsActivateRefuses() {
	feature := newSecondWindForTest("sw", "hero")
	owner := &StubEntity{id: "hero", levels: map[classes.Class]int{classes.Barbarian: 2}}

	definition, err := feature.Definition(owner)

	s.Require().Error(err)
	s.Nil(definition)
	_, wantErr := ownerClassLevel(owner, classes.Fighter, "second wind")
	s.Require().Error(wantErr)
	s.Equal(rpgerr.GetCode(wantErr), rpgerr.GetCode(err))
	s.Equal(wantErr.Error(), err.Error())
}

func (s *SecondWindDefinitionSuite) TestSecondWindCarriesADescription() {
	var feature Feature = newSecondWindForTest("sw", "hero")

	_, ok := feature.(Described)

	s.True(ok, "Second Wind answers the Described question")
}
