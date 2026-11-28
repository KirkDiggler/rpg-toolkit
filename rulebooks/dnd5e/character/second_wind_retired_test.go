// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// SecondWindRetiredSuite pins that Second Wind's own activate code is gone:
// the feature refuses to activate itself, the refusal leaves the turn and the
// pool alone, the description still works at the gate, and a feature with no
// description goes through the old path untouched (rpg-project#1149).
type SecondWindRetiredSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func TestSecondWindRetiredSuite(t *testing.T) { suite.Run(t, new(SecondWindRetiredSuite)) }

func (s *SecondWindRetiredSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	char, err := newFighterDraft(s.T()).ToCharacter(s.ctx, "retired-fighter", s.bus)
	s.Require().NoError(err)
	s.char = char
	s.char.hitPoints = 8
	s.char.maxHitPoints = 20
	markSaved(s.char)
	_, err = s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	s.Require().True(s.char.InCombat())
}

func (s *SecondWindRetiredSuite) TestAskingTheFeatureToActivateItselfIsRefusedAndTouchesNothing() {
	var healed int
	_, err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, _ dnd5eEvents.HealingReceivedEvent) error {
			healed++
			return nil
		})
	s.Require().NoError(err)
	bonusBefore := s.char.actionEconomy.BonusActionsRemaining
	s.Require().Equal(1, bonusBefore)

	out, err := s.char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.SecondWind(), Roller: &countingBridgeRoller{face: 6},
	})

	s.Require().NoError(err)
	s.False(out.Success)
	s.Equal(features.ErrUsedFromItsDescription.Error(), out.Error)
	s.Equal("this ability is used from its description: build the action with Definition and pay its cost at the gate", out.Error)
	s.Equal(bonusBefore, s.char.actionEconomy.BonusActionsRemaining, "the bonus action is still available")
	current, maximum, ok := s.char.ResourceStatus(resources.SecondWind)
	s.Require().True(ok)
	s.Equal(1, current, "the pool still reads 1 of 1")
	s.Equal(1, maximum)
	s.Zero(healed, "no healing event was published")
}

func (s *SecondWindRetiredSuite) TestTheFeatureRefusesWhenAskedDirectly() {
	secondWind := &features.SecondWind{}

	err := secondWind.Activate(s.ctx, s.char, features.FeatureInput{Bus: s.bus, Roller: &countingBridgeRoller{face: 6}})

	s.Require().Error(err)
	s.True(errors.Is(err, features.ErrUsedFromItsDescription), "got %v", err)
	current, _, _ := s.char.ResourceStatus(resources.SecondWind)
	s.Equal(1, current, "the refusal spends nothing")
}

func (s *SecondWindRetiredSuite) TestTheDescriptionStillWorksWithoutTheFeaturesCode() {
	definition, ok, err := s.char.AbilityDefinition(*refs.Features.SecondWind())
	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(definition)

	s.Require().NoError(combat.Pay(s.char, definition.Cost))
	s.Equal(0, s.char.actionEconomy.BonusActionsRemaining, "paying takes the bonus action")
	current, maximum, _ := s.char.ResourceStatus(resources.SecondWind)
	s.Equal(0, current, "paying takes one point of the pool")
	s.Equal(1, maximum)

	resolved, err := healing.Resolve(s.ctx, *definition.Effect.Healing,
		dnd5eEvents.RollSource{Ref: refs.Features.SecondWind(), Name: "Second Wind", SourceID: s.char.id},
		&countingBridgeRoller{face: 6})
	s.Require().NoError(err)
	s.Equal(6+1, resolved.Total, "the d10 plus the fighter's level")
}

func (s *SecondWindRetiredSuite) TestAFeatureWithNoDescriptionIsUntouched() {
	monk, err := LoadFromData(s.ctx, &Data{
		Levels:           syntheticLevels(classes.Monk, 3),
		ID:               "retired-monk",
		PlayerID:         "retired-player",
		Name:             "Li",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 16, abilities.CHA: 8,
		},
		HitPoints:    18,
		MaxHitPoints: 24,
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			resources.Ki: {Current: 3, Maximum: 3, ResetType: coreResources.ResetShortRest},
		},
		Features: []json.RawMessage{json.RawMessage(
			`{"ref":{"module":"dnd5e","type":"features","id":"flurry_of_blows"},"id":"flurry_of_blows","name":"Flurry of Blows"}`)},
	}, s.bus)
	s.Require().NoError(err)
	_, err = monk.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	kiBefore, _, _ := monk.ResourceStatus(resources.Ki)
	bonusBefore := monk.actionEconomy.BonusActionsRemaining
	s.Require().Equal(1, bonusBefore)

	out, err := monk.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.FlurryOfBlows()})

	s.Require().NoError(err)
	s.True(out.Success, "flurry of blows activates: %s", out.Error)
	kiAfter, _, _ := monk.ResourceStatus(resources.Ki)
	s.Equal(kiBefore-1, kiAfter, "a charge is spent")
	s.Equal(bonusBefore-1, monk.actionEconomy.BonusActionsRemaining, "the bonus action is taken")
}
