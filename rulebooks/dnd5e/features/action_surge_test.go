package features

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

type ActionSurgeTestSuite struct {
	suite.Suite
	actionSurge *ActionSurge
	owner       *StubEntity
}

func TestActionSurgeSuite(t *testing.T) {
	suite.Run(t, new(ActionSurgeTestSuite))
}

func (s *ActionSurgeTestSuite) SetupTest() {
	// Create action surge via factory. The config is accepted and ignored.
	output, err := CreateFromRef(&CreateFromRefInput{
		Ref:         refs.Features.ActionSurge().String(),
		CharacterID: "fighter-1",
		Config:      json.RawMessage(`{"uses": 3}`),
	})
	s.Require().NoError(err)
	s.Require().NotNil(output)

	var ok bool
	s.actionSurge, ok = output.Feature.(*ActionSurge)
	s.Require().True(ok, "feature should be *ActionSurge")
	s.owner = &StubEntity{
		id:        "fighter-1",
		resources: map[coreResources.ResourceKey]int{resources.ActionSurge: 1},
	}
}

func (s *ActionSurgeTestSuite) TestGetID() {
	s.Equal(refs.Features.ActionSurge().ID, s.actionSurge.GetID())
}

func (s *ActionSurgeTestSuite) TestGetType() {
	s.Equal(EntityTypeFeature, s.actionSurge.GetType())
}

func (s *ActionSurgeTestSuite) TestCanActivateAsksTheOwnersPool() {
	s.NoError(s.actionSurge.CanActivate(context.Background(), s.owner, FeatureInput{}),
		"no action economy is asked for")

	ledger := &surgeLedger{owner: s.owner, actions: 1}
	s.Require().NoError(useActionSurgeFromItsDescription(s.T(), s.actionSurge, ledger))

	err := s.actionSurge.CanActivate(context.Background(), s.owner, FeatureInput{})
	s.Require().Error(err)
	s.True(rpgerr.IsResourceExhausted(err))
	s.Contains(err.Error(), "no action surge uses remaining")
}

func (s *ActionSurgeTestSuite) TestCanActivateRefusesAnOwnerWithNoResources() {
	err := s.actionSurge.CanActivate(context.Background(), &mockEntity{id: "x", entityType: "character"}, FeatureInput{})
	s.Require().Error(err)
	s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
}

func (s *ActionSurgeTestSuite) TestPaidAtTheGateTakesTheUseAndGivesAnAction() {
	ledger := &surgeLedger{owner: s.owner, actions: 1}

	s.Require().NoError(useActionSurgeFromItsDescription(s.T(), s.actionSurge, ledger))
	s.Equal(0, s.owner.resources[resources.ActionSurge])
	s.Equal(2, ledger.actions)

	s.Require().Error(useActionSurgeFromItsDescription(s.T(), s.actionSurge, ledger), "no use left")
	s.Equal(2, ledger.actions, "a refused payment changes nothing")
}

func (s *ActionSurgeTestSuite) TestActivateRefusesAndTouchesNothing() {
	err := s.actionSurge.Activate(context.Background(), s.owner, FeatureInput{})

	s.Require().Error(err)
	s.True(errors.Is(err, ErrUsedFromItsDescription), "got %v", err)
	s.Equal(1, s.owner.resources[resources.ActionSurge], "the refusal spends nothing")
}

func (s *ActionSurgeTestSuite) TestToJSONCarriesNoCount() {
	jsonData, err := s.actionSurge.ToJSON()
	s.Require().NoError(err)

	loaded := &ActionSurge{}
	s.Require().NoError(loaded.loadJSON(jsonData))

	s.Equal(s.actionSurge.id, loaded.id)
	s.Equal(s.actionSurge.name, loaded.name)
	s.NotContains(string(jsonData), "uses")
	s.NotContains(string(jsonData), "max_uses")
}

// mockEntity is a simple entity with no resources, for testing.
type mockEntity struct {
	id         string
	entityType core.EntityType
}

func (m *mockEntity) GetID() string {
	return m.id
}

func (m *mockEntity) GetType() core.EntityType {
	return m.entityType
}

func (s *ActionSurgeTestSuite) TestStatusNeedsAnOwnerThatCarriesThePool() {
	_, err := s.actionSurge.Status(&StatusInput{})
	s.Require().Error(err, "no owner")

	_, err = s.actionSurge.Status(&StatusInput{Owner: stubResourceReader{}})
	s.Require().Error(err, "an owner without the pool")
}
