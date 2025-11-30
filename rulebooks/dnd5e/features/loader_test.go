package features

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

type LoaderTestSuite struct {
	suite.Suite
	bus events.EventBus
	ctx context.Context
}

func (s *LoaderTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
}

func (s *LoaderTestSuite) TestLoadRageFeature() {
	jsonData := json.RawMessage(`{
		"ref": {"module": "dnd5e", "type": "features", "id": "rage"},
		"id": "rage",
		"name": "Rage",
		"level": 5,
		"uses": 2,
		"max_uses": 3
	}`)

	feature, err := LoadJSON(jsonData)
	s.NoError(err)
	s.NotNil(feature)

	// Check it's actually a rage
	rage, ok := feature.(*Rage)
	s.True(ok, "Should be a Rage instance")
	s.Equal("rage", rage.id)
	s.Equal(5, rage.level)
	s.Equal(2, rage.resource.Current)
	s.Equal(3, rage.resource.Maximum)

	// Test that it can be activated
	owner := &StubEntity{id: "test-barbarian"}
	err = feature.CanActivate(s.ctx, owner, FeatureInput{})
	s.NoError(err)
}

func (s *LoaderTestSuite) TestLoadUnknownFeature() {
	// Skip for now - all features are treated as rage until we have more types
	s.T().Skip("Skipping until we have multiple feature types")
}

func (s *LoaderTestSuite) TestRoundTripThroughJSON() {
	// Create a rage feature
	originalRage := newRageForTest("rage-roundtrip", 7)

	// Use one charge
	owner := &StubEntity{id: "test-barbarian"}
	err := originalRage.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Convert to JSON
	jsonData, err := originalRage.ToJSON()
	s.NoError(err)

	// Load it back
	feature, err := LoadJSON(jsonData)
	s.NoError(err)

	loadedRage, ok := feature.(*Rage)
	s.True(ok)

	// Verify state was preserved
	s.Equal(originalRage.id, loadedRage.id)
	s.Equal(originalRage.level, loadedRage.level)
	s.Equal(originalRage.resource.Current, loadedRage.resource.Current)
	s.Equal(originalRage.resource.Maximum, loadedRage.resource.Maximum)
}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}
