package features_test

import (
	"context"
	"encoding/json"
	"testing"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type RecklessAttackTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	character *mockResourceAccessor
	feature   features.Feature
}

func TestRecklessAttackTestSuite(t *testing.T) {
	suite.Run(t, new(RecklessAttackTestSuite))
}

func (s *RecklessAttackTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	s.character = &mockResourceAccessor{
		id: "test-barbarian",
	}

	// Create the Reckless Attack feature via factory
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.RecklessAttack().String(),
		Config:      json.RawMessage(`{}`),
		CharacterID: s.character.GetID(),
	})
	s.Require().NoError(err)
	s.feature = output.Feature
}

func (s *RecklessAttackTestSuite) TestCanActivate_Always() {
	// Reckless Attack has no cost — always activatable
	err := s.feature.CanActivate(s.ctx, s.character, features.FeatureInput{})
	s.Require().NoError(err)
}

func (s *RecklessAttackTestSuite) TestActivate_AppliesCondition() {
	// Activate Reckless Attack
	err := s.feature.Activate(s.ctx, s.character, features.FeatureInput{
		Bus: s.bus,
	})
	s.Require().NoError(err)

	// Verify the condition is active by checking attack chain
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: s.character.GetID(),
		TargetID:   "goblin-1",
		IsMelee:    true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.AdvantageSources, 1, "Should have advantage after activation")
	s.Equal("Reckless Attack", finalEvent.AdvantageSources[0].Reason)
}

func (s *RecklessAttackTestSuite) TestActivate_FailsWithoutBus() {
	err := s.feature.Activate(s.ctx, s.character, features.FeatureInput{})
	s.Require().Error(err, "Should fail without event bus")
}

func (s *RecklessAttackTestSuite) TestToJSON() {
	jsonData, err := s.feature.ToJSON()
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)

	var data map[string]interface{}
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Assert().Contains(data, "ref")
	s.Assert().Contains(data, "character_id")
	s.Assert().Equal(s.character.GetID(), data["character_id"])
}

func (s *RecklessAttackTestSuite) TestLoadJSON() {
	originalJSON, err := s.feature.ToJSON()
	s.Require().NoError(err)

	loaded, err := features.LoadJSON(originalJSON)
	s.Require().NoError(err)
	s.Assert().NotNil(loaded)
	s.Assert().Equal(s.feature.GetID(), loaded.GetID())
}

func (s *RecklessAttackTestSuite) TestRoundTrip() {
	jsonData, err := s.feature.ToJSON()
	s.Require().NoError(err)

	loaded, err := features.LoadJSON(jsonData)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	s.Assert().Equal(s.feature.GetID(), loaded.GetID())
}

func (s *RecklessAttackTestSuite) TestCreateFromRef() {
	config := json.RawMessage(`{}`)

	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.RecklessAttack().String(),
		Config:      config,
		CharacterID: "test-char",
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Require().NotNil(output.Feature)
	s.Assert().Equal(refs.Features.RecklessAttack().ID, output.Feature.GetID())
}

func (s *RecklessAttackTestSuite) TestActionType_IsFree() {
	s.Assert().Equal(coreCombat.ActionFree, s.feature.ActionType())
}
