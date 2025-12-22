package features

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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
	s.Equal(2, rage.resource.Current())
	s.Equal(3, rage.resource.Maximum())

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
	originalRage := newRageForTest("rage-roundtrip", 7, "test-barbarian")

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
	s.Equal(originalRage.resource.Current(), loadedRage.resource.Current())
	s.Equal(originalRage.resource.Maximum(), loadedRage.resource.Maximum())
}

func (s *LoaderTestSuite) TestActionTypes() {
	// Test that each feature returns the correct action type per D&D 5e rules
	testCases := []struct {
		name       string
		ref        string
		wantAction combat.ActionType
	}{
		{
			name:       "Rage is bonus action",
			ref:        refs.Features.Rage().String(),
			wantAction: combat.ActionBonus,
		},
		{
			name:       "Second Wind is bonus action",
			ref:        refs.Features.SecondWind().String(),
			wantAction: combat.ActionBonus,
		},
		{
			name:       "Action Surge is free (grants extra action)",
			ref:        refs.Features.ActionSurge().String(),
			wantAction: combat.ActionFree,
		},
		{
			name:       "Flurry of Blows is bonus action",
			ref:        refs.Features.FlurryOfBlows().String(),
			wantAction: combat.ActionBonus,
		},
		{
			name:       "Patient Defense is bonus action",
			ref:        refs.Features.PatientDefense().String(),
			wantAction: combat.ActionBonus,
		},
		{
			name:       "Step of the Wind is bonus action",
			ref:        refs.Features.StepOfTheWind().String(),
			wantAction: combat.ActionBonus,
		},
		{
			name:       "Deflect Missiles is reaction",
			ref:        refs.Features.DeflectMissiles().String(),
			wantAction: combat.ActionReaction,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			output, err := CreateFromRef(&CreateFromRefInput{
				Ref:         tc.ref,
				CharacterID: "test-char",
			})
			s.Require().NoError(err)
			s.Equal(tc.wantAction, output.Feature.ActionType())
		})
	}
}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}
