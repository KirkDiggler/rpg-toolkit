// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// AbilityDefinitionSuite pins that a character can be asked for the
// description of an ability it has, and that Second Wind's description and
// its code say the same thing (rpg-project#1131).
type AbilityDefinitionSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func TestAbilityDefinitionSuite(t *testing.T) { suite.Run(t, new(AbilityDefinitionSuite)) }

func (s *AbilityDefinitionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	char, err := newFighterDraft(s.T()).ToCharacter(s.ctx, "described-fighter", s.bus)
	s.Require().NoError(err)
	s.char = char
}

func (s *AbilityDefinitionSuite) TestAFighterIsHandedSecondWindsDescription() {
	definition, ok, err := s.char.AbilityDefinition(*refs.Features.SecondWind())

	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(definition)
	s.Equal(*refs.Features.SecondWind(), definition.Ref)
	s.Require().NotNil(definition.Effect)
	s.Require().Len(definition.Effect.Healing.Modifiers, 1)
	s.Equal(1, definition.Effect.Healing.Modifiers[0].Amount)
	s.Equal(s.char.id, definition.Effect.Healing.Modifiers[0].Source.SourceID)
	s.Require().NoError(definition.Validate())
}

func (s *AbilityDefinitionSuite) TestARefTheCharacterLacksIsNotAnError() {
	definition, ok, err := s.char.AbilityDefinition(*refs.Features.Rage())

	s.Require().NoError(err)
	s.False(ok)
	s.Nil(definition)
}

func (s *AbilityDefinitionSuite) TestAFeatureWithNoDescriptionIsNotAnError() {
	raw, err := json.Marshal(features.RageData{Ref: refs.Features.Rage(), ID: "rage-1", Name: "Rage"})
	s.Require().NoError(err)
	rage, err := features.LoadJSON(raw)
	s.Require().NoError(err)
	barbarian := &Character{
		id: "b1", classID: classes.Barbarian, levels: syntheticLevels(classes.Barbarian, 1),
		features: []features.Feature{rage},
	}

	definition, ok, err := barbarian.AbilityDefinition(*refs.Features.Rage())

	s.Require().NoError(err)
	s.False(ok, "Rage carries no description yet")
	s.Nil(definition)
}
