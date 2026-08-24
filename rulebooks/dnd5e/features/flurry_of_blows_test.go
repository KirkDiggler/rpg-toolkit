package features_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/stretchr/testify/suite"
)

// mockResourceAccessor implements coreResources.ResourceAccessor for testing
// and is shared by the other feature tests in this package.
type mockResourceAccessor struct {
	id        string
	resources map[coreResources.ResourceKey]*combat.RecoverableResource
}

func (m *mockResourceAccessor) GetID() string {
	return m.id
}

func (m *mockResourceAccessor) GetType() core.EntityType {
	return "character"
}

func (m *mockResourceAccessor) IsResourceAvailable(key coreResources.ResourceKey) bool {
	if m.resources == nil {
		return false
	}
	resource, ok := m.resources[key]
	return ok && resource.IsAvailable()
}

func (m *mockResourceAccessor) UseResource(key coreResources.ResourceKey, amount int) error {
	if m.resources == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "resource %s not found", key)
	}
	resource, ok := m.resources[key]
	if !ok {
		return rpgerr.Newf(rpgerr.CodeNotFound, "resource %s not found", key)
	}
	return resource.Use(amount)
}

func (m *mockResourceAccessor) GetResource(key coreResources.ResourceKey) *combat.RecoverableResource {
	if resource, ok := m.resources[key]; ok {
		return resource
	}
	return combat.NewRecoverableResource(combat.RecoverableResourceConfig{ID: "", Maximum: 0})
}

func (m *mockResourceAccessor) AddResource(
	key coreResources.ResourceKey, resource *combat.RecoverableResource,
) {
	if m.resources == nil {
		m.resources = make(map[coreResources.ResourceKey]*combat.RecoverableResource)
	}
	m.resources[key] = resource
}

type mockMonkCharacter struct {
	mockResourceAccessor
	capacity map[combat.CapacityType]int
}

func (m *mockMonkCharacter) BankCapacity(key combat.CapacityType, quantity int) {
	if m.capacity == nil {
		m.capacity = make(map[combat.CapacityType]int)
	}
	m.capacity[key] += quantity
}

func (m *mockMonkCharacter) CapacityLeft(key combat.CapacityType) int {
	return m.capacity[key]
}

type FlurryOfBlowsTestSuite struct {
	suite.Suite
	ctx       context.Context
	character *mockMonkCharacter
	feature   features.Feature
}

func TestFlurryOfBlowsTestSuite(t *testing.T) {
	suite.Run(t, new(FlurryOfBlowsTestSuite))
}

func (s *FlurryOfBlowsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.character = &mockMonkCharacter{
		mockResourceAccessor: mockResourceAccessor{id: "test-monk"},
	}

	ki := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          string(resources.Ki),
		Maximum:     3,
		CharacterID: s.character.GetID(),
		ResetType:   coreResources.ResetShortRest,
	})
	s.character.AddResource(resources.Ki, ki)

	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.FlurryOfBlows().String(),
		Config:      json.RawMessage(`{}`),
		CharacterID: s.character.GetID(),
	})
	s.Require().NoError(err)
	s.feature = output.Feature
}

func (s *FlurryOfBlowsTestSuite) TestCanActivate_WithKi() {
	s.Require().NoError(s.feature.CanActivate(s.ctx, s.character, features.FeatureInput{}))
}

func (s *FlurryOfBlowsTestSuite) TestCanActivate_WithoutKi() {
	ki := s.character.GetResource(resources.Ki)
	s.Require().NoError(ki.Use(3))

	err := s.feature.CanActivate(s.ctx, s.character, features.FeatureInput{})

	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.True(errors.As(err, &rpgErr))
	s.Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *FlurryOfBlowsTestSuite) TestActivateSpendsKiAndBanksTwoFlurryStrikes() {
	before := s.character.GetResource(resources.Ki).Current()

	err := s.feature.Activate(s.ctx, s.character, features.FeatureInput{})

	s.Require().NoError(err)
	s.Equal(before-1, s.character.GetResource(resources.Ki).Current())
	s.Equal(2, s.character.CapacityLeft(combat.CapacityFlurryStrike))
}

func (s *FlurryOfBlowsTestSuite) TestActivateWithoutKiBanksNothing() {
	ki := s.character.GetResource(resources.Ki)
	s.Require().NoError(ki.Use(3))
	before := s.character.CapacityLeft(combat.CapacityFlurryStrike)

	err := s.feature.Activate(s.ctx, s.character, features.FeatureInput{})

	s.Require().Error(err)
	s.Equal(before, s.character.CapacityLeft(combat.CapacityFlurryStrike))
	var rpgErr *rpgerr.Error
	s.True(errors.As(err, &rpgErr))
	s.Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *FlurryOfBlowsTestSuite) TestToJSON() {
	jsonData, err := s.feature.ToJSON()
	s.Require().NoError(err)
	s.NotEmpty(jsonData)

	var data map[string]interface{}
	s.Require().NoError(json.Unmarshal(jsonData, &data))
	s.Contains(data, "ref")
	s.Contains(data, "character_id")
	s.Equal(s.character.GetID(), data["character_id"])
}

func (s *FlurryOfBlowsTestSuite) TestLoadJSON() {
	originalJSON, err := s.feature.ToJSON()
	s.Require().NoError(err)

	loaded, err := features.LoadJSON(originalJSON)

	s.Require().NoError(err)
	s.NotNil(loaded)
	s.Equal(s.feature.GetID(), loaded.GetID())
}

func (s *FlurryOfBlowsTestSuite) TestRoundTrip() {
	jsonData, err := s.feature.ToJSON()
	s.Require().NoError(err)

	loaded, err := features.LoadJSON(jsonData)

	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	s.Equal(s.feature.GetID(), loaded.GetID())
}

func (s *FlurryOfBlowsTestSuite) TestCreateFromRef() {
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.FlurryOfBlows().String(),
		Config:      json.RawMessage(`{}`),
		CharacterID: "test-char",
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Require().NotNil(output.Feature)
	s.Equal(refs.Features.FlurryOfBlows().ID, output.Feature.GetID())
}
