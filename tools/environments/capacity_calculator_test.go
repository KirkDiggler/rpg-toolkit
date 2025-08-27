package environments

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type CapacityCalculatorTestSuite struct {
	suite.Suite
}

func TestCapacityCalculatorSuite(t *testing.T) {
	suite.Run(t, new(CapacityCalculatorTestSuite))
}

func (s *CapacityCalculatorTestSuite) TestGetDefaultSpatialIntentProfile() {
	profile := GetDefaultSpatialIntentProfile(SpatialFeelingNormal)

	s.Assert().Equal(SpatialFeelingNormal, profile.Feeling)
	s.Assert().Greater(profile.EntityDensityTarget, 0.0)
	s.Assert().Greater(profile.MovementFreedomIndex, 0.0)
	s.Assert().GreaterOrEqual(profile.VisualScopeIndex, 0.0)
}

func (s *CapacityCalculatorTestSuite) TestEstimateRoomCapacity() {
	roomSize := spatial.Dimensions{Width: 10.0, Height: 10.0}
	constraints := CapacityConstraints{
		MaxEntitiesPerRoom:        20,
		MinMovementSpace:          0.6,
		TargetSpatialFeeling:      SpatialFeelingNormal,
		MinEntitySpacing:          1.0,
		WallDensityModifier:       0.5,
		RequiredPathwayMultiplier: 1.2,
	}

	estimate := EstimateRoomCapacity(roomSize, constraints)

	// Verify basic sanity checks
	s.Assert().GreaterOrEqual(estimate.RecommendedEntityCount, 0)
	s.Assert().GreaterOrEqual(estimate.MaxEntityCount, estimate.RecommendedEntityCount)
	s.Assert().LessOrEqual(estimate.MaxEntityCount, constraints.MaxEntitiesPerRoom)
}

func (s *CapacityCalculatorTestSuite) TestCalculateOptimalRoomSize() {
	entityCount := 8
	profile := GetDefaultSpatialIntentProfile(SpatialFeelingNormal)

	dimensions := CalculateOptimalRoomSize(profile, entityCount)

	// Verify dimensions are reasonable
	s.Assert().Greater(dimensions.Width, 0.0)
	s.Assert().Greater(dimensions.Height, 0.0)

	// Should be large enough for the requested entities
	area := dimensions.Width * dimensions.Height
	s.Assert().Greater(area, float64(entityCount))
}

func (s *CapacityCalculatorTestSuite) TestGetSplitOptions() {
	roomSize := spatial.Dimensions{Width: 20.0, Height: 20.0}
	entityCount := 30
	constraints := CapacityConstraints{
		MaxEntitiesPerRoom:        15,
		MinMovementSpace:          0.6,
		TargetSpatialFeeling:      SpatialFeelingNormal,
		MinEntitySpacing:          1.0,
		WallDensityModifier:       0.5,
		RequiredPathwayMultiplier: 1.2,
	}

	options := GetSplitOptions(roomSize, entityCount, constraints)
	s.Assert().NotEmpty(options)

	// Verify that split options are reasonable
	for _, option := range options {
		s.Assert().Greater(option.SuggestedSize.Width, 0.0)
		s.Assert().Greater(option.SuggestedSize.Height, 0.0)
		s.Assert().NotEmpty(option.SplitReason)
	}
}

func (s *CapacityCalculatorTestSuite) TestAnalyzeRoomCapacityForEntityCount() {
	roomSize := spatial.Dimensions{Width: 12.0, Height: 12.0}
	constraints := CapacityConstraints{
		MaxEntitiesPerRoom:        15,
		MinMovementSpace:          0.6,
		TargetSpatialFeeling:      SpatialFeelingNormal,
		MinEntitySpacing:          1.0,
		WallDensityModifier:       0.5,
		RequiredPathwayMultiplier: 1.2,
	}

	analysis := AnalyzeRoomCapacityForEntityCount(roomSize, 30, constraints)

	s.Assert().Equal(30, analysis.RequestedEntityCount)
	s.Assert().Greater(analysis.CapacityUtilization, 1.0) // Over capacity
	s.Assert().NotEmpty(analysis.SplitOptions)

	// Verify the resulting spatial feeling is classified correctly
	s.Assert().Contains([]SpatialFeeling{SpatialFeelingTight, SpatialFeelingNormal, SpatialFeelingVast},
		analysis.ResultingSpatialFeeling)
}

// Test capacity calculations directly
func (s *CapacityCalculatorTestSuite) TestCapacityCalculationsOnly() {
	// Test capacity calculations directly without query handler dependencies
	roomSize := spatial.Dimensions{Width: 15.0, Height: 15.0}
	constraints := CapacityConstraints{
		MaxEntitiesPerRoom:        10,
		MinMovementSpace:          0.6,
		TargetSpatialFeeling:      SpatialFeelingNormal,
		MinEntitySpacing:          1.0,
		WallDensityModifier:       0.5,
		RequiredPathwayMultiplier: 1.2,
	}

	// Test room capacity estimation
	estimate := EstimateRoomCapacity(roomSize, constraints)
	s.Assert().GreaterOrEqual(estimate.RecommendedEntityCount, 0)
	s.Assert().GreaterOrEqual(estimate.MaxEntityCount, 0)

	// Test optimal room size calculation
	optimalSize := CalculateOptimalRoomSize(GetDefaultSpatialIntentProfile(SpatialFeelingNormal), 12)
	s.Assert().Greater(optimalSize.Width, 0.0)
	s.Assert().Greater(optimalSize.Height, 0.0)

	// Test analysis functionality
	analysis := AnalyzeRoomCapacityForEntityCount(roomSize, 8, constraints)
	s.Assert().Equal(8, analysis.RequestedEntityCount)
	s.Assert().Greater(analysis.CapacityUtilization, 0.0)
}
