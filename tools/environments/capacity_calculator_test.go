package environments

import (
	"context"
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
	// Test tight feeling
	tightProfile := GetDefaultSpatialIntentProfile(SpatialFeelingTight)
	s.Assert().Equal(SpatialFeelingTight, tightProfile.Feeling)
	s.Assert().Equal(0.8, tightProfile.EntityDensityTarget)  // High density
	s.Assert().Equal(0.3, tightProfile.MovementFreedomIndex) // Limited movement

	// Test vast feeling
	vastProfile := GetDefaultSpatialIntentProfile(SpatialFeelingVast)
	s.Assert().Equal(SpatialFeelingVast, vastProfile.Feeling)
	s.Assert().Equal(0.2, vastProfile.EntityDensityTarget)  // Low density
	s.Assert().Equal(0.8, vastProfile.MovementFreedomIndex) // Lots of movement

	// Test normal feeling
	normalProfile := GetDefaultSpatialIntentProfile(SpatialFeelingNormal)
	s.Assert().Equal(SpatialFeelingNormal, normalProfile.Feeling)
	s.Assert().Equal(0.5, normalProfile.EntityDensityTarget)  // Balanced density
	s.Assert().Equal(0.6, normalProfile.MovementFreedomIndex) // Balanced movement
}

func (s *CapacityCalculatorTestSuite) TestCalculateOptimalRoomSize() {
	// Test with zero entities
	dimensions := CalculateOptimalRoomSize(GetDefaultSpatialIntentProfile(SpatialFeelingNormal), 0)
	s.Assert().Equal(5.0, dimensions.Width)  // Minimum size
	s.Assert().Equal(5.0, dimensions.Height) // Minimum size

	// Test normal feeling with moderate entity count
	normalProfile := GetDefaultSpatialIntentProfile(SpatialFeelingNormal)
	dimensions = CalculateOptimalRoomSize(normalProfile, 10)
	s.Assert().GreaterOrEqual(dimensions.Width, 5.0)
	s.Assert().GreaterOrEqual(dimensions.Height, 5.0)
	s.Assert().LessOrEqual(dimensions.Width, 100.0)  // Should respect maximum
	s.Assert().LessOrEqual(dimensions.Height, 100.0) // Should respect maximum

	// Test tight feeling should produce smaller rooms than vast
	tightProfile := GetDefaultSpatialIntentProfile(SpatialFeelingTight)
	vastProfile := GetDefaultSpatialIntentProfile(SpatialFeelingVast)

	tightDimensions := CalculateOptimalRoomSize(tightProfile, 10)
	vastDimensions := CalculateOptimalRoomSize(vastProfile, 10)

	tightArea := tightDimensions.Area()
	vastArea := vastDimensions.Area()

	s.Assert().Less(tightArea, vastArea, "Tight rooms should be smaller than vast rooms for same entity count")
}

func (s *CapacityCalculatorTestSuite) TestEstimateRoomCapacity() {
	roomSize := spatial.Dimensions{Width: 20.0, Height: 20.0}
	constraints := CapacityConstraints{
		MaxEntitiesPerRoom:        15,
		MinMovementSpace:          0.6,
		TargetSpatialFeeling:      SpatialFeelingNormal,
		MinEntitySpacing:          1.0,
		WallDensityModifier:       0.5,
		RequiredPathwayMultiplier: 1.2,
	}

	estimate := EstimateRoomCapacity(roomSize, constraints)

	// Basic validation
	s.Assert().GreaterOrEqual(estimate.RecommendedEntityCount, 0)
	s.Assert().GreaterOrEqual(estimate.MaxEntityCount, estimate.RecommendedEntityCount)
	s.Assert().GreaterOrEqual(estimate.QualityScore, 0.0)
	s.Assert().LessOrEqual(estimate.QualityScore, 1.0)
	s.Assert().Greater(estimate.UsableArea, 0.0)

	// Should respect max entities constraint
	s.Assert().LessOrEqual(estimate.RecommendedEntityCount, constraints.MaxEntitiesPerRoom)
	s.Assert().LessOrEqual(estimate.MaxEntityCount, constraints.MaxEntitiesPerRoom)

	// Usable area should be less than total area
	totalArea := roomSize.Area()
	s.Assert().Less(estimate.UsableArea, totalArea)
}

func (s *CapacityCalculatorTestSuite) TestGetSplitOptions() {
	constraints := GetDefaultConstraintsForFeeling(SpatialFeelingNormal)

	// Small room with many entities should provide split options
	splits := GetSplitOptions(spatial.Dimensions{Width: 8.0, Height: 8.0}, 30, constraints)
	s.Assert().NotEmpty(splits, "Should provide split options for small room with many entities")

	// Large room should still provide split options if requested (games decide when to use them)
	splits = GetSplitOptions(spatial.Dimensions{Width: 50.0, Height: 50.0}, 3, constraints)
	s.Assert().NotEmpty(splits, "Should provide split options even for large rooms - games decide when to use them")

	// Verify split options have valid structure
	for _, split := range splits {
		s.Assert().Greater(split.SuggestedSize.Width, 0.0)
		s.Assert().Greater(split.SuggestedSize.Height, 0.0)
		s.Assert().NotEmpty(split.SplitReason)
		s.Assert().NotEmpty(split.ConnectionPoints)
		s.Assert().NotEmpty(split.RecommendedEntityDistribution)
	}
}

func (s *CapacityCalculatorTestSuite) TestAnalyzeRoomCapacityForEntityCount() {
	roomSize := spatial.Dimensions{Width: 15.0, Height: 15.0}
	constraints := GetDefaultConstraintsForFeeling(SpatialFeelingNormal)

	// Test analysis with reasonable entity count
	analysis := AnalyzeRoomCapacityForEntityCount(roomSize, 8, constraints)

	s.Assert().Equal(8, analysis.RequestedEntityCount)
	s.Assert().Greater(analysis.CapacityUtilization, 0.0)
	s.Assert().NotEmpty(analysis.SplitOptions) // Should always provide options for games to consider

	// Test analysis with excessive entity count
	analysis = AnalyzeRoomCapacityForEntityCount(roomSize, 30, constraints)

	s.Assert().Equal(30, analysis.RequestedEntityCount)
	s.Assert().Greater(analysis.CapacityUtilization, 1.0) // Over capacity
	s.Assert().NotEmpty(analysis.SplitOptions)

	// Verify the resulting spatial feeling is classified correctly
	s.Assert().Contains([]SpatialFeeling{SpatialFeelingTight, SpatialFeelingNormal, SpatialFeelingVast},
		analysis.ResultingSpatialFeeling)
}

// Integration test with query handler
func (s *CapacityCalculatorTestSuite) TestQueryHandlerIntegration() {
	// Create a basic query handler (without dependencies for this test)
	handler := &BasicQueryHandler{}

	// Test capacity query
	capacityQuery := CapacityQuery{
		RoomSize: spatial.Dimensions{Width: 15.0, Height: 15.0},
		Constraints: CapacityConstraints{
			MaxEntitiesPerRoom:        10,
			MinMovementSpace:          0.6,
			TargetSpatialFeeling:      SpatialFeelingNormal,
			MinEntitySpacing:          1.0,
			WallDensityModifier:       0.5,
			RequiredPathwayMultiplier: 1.2,
		},
		ExistingEntityCount: 8,
		IncludeSplitOptions: true,
	}

	response, err := handler.HandleCapacityQuery(context.Background(), capacityQuery)
	s.Assert().NoError(err)
	s.Assert().GreaterOrEqual(response.Estimate.RecommendedEntityCount, 0)

	// Test sizing query
	sizingQuery := SizingQuery{
		IntentProfile:   GetDefaultSpatialIntentProfile(SpatialFeelingNormal),
		EntityCount:     12,
		AdditionalSpace: 1.2,
		MinDimensions:   spatial.Dimensions{Width: 8.0, Height: 8.0},
		MaxDimensions:   spatial.Dimensions{Width: 50.0, Height: 50.0},
	}

	dimensions, err := handler.HandleSizingQuery(context.Background(), sizingQuery)
	s.Assert().NoError(err)
	s.Assert().GreaterOrEqual(dimensions.Width, sizingQuery.MinDimensions.Width)
	s.Assert().GreaterOrEqual(dimensions.Height, sizingQuery.MinDimensions.Height)
	s.Assert().LessOrEqual(dimensions.Width, sizingQuery.MaxDimensions.Width)
	s.Assert().LessOrEqual(dimensions.Height, sizingQuery.MaxDimensions.Height)
}
