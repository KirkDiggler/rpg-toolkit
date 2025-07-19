package environments

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SelectablesTypesTestSuite struct {
	suite.Suite
}

func (s *SelectablesTypesTestSuite) SetupTest() {
	// Fresh setup for each test
}

func (s *SelectablesTypesTestSuite) TestRange() {
	s.Run("Random returns value within range", func() {
		testRange := Range{Min: 0.3, Max: 0.7}

		// Test multiple random values to ensure they're all within range
		for i := 0; i < 100; i++ {
			value := testRange.Random()
			s.Assert().GreaterOrEqual(value, 0.3, "Random value should be >= Min")
			s.Assert().LessOrEqual(value, 0.7, "Random value should be <= Max")
		}
	})

	s.Run("Random handles equal min/max", func() {
		testRange := Range{Min: 0.5, Max: 0.5}
		value := testRange.Random()
		s.Assert().Equal(0.5, value, "Should return Min when Min == Max")
	})

	s.Run("Random handles inverted range", func() {
		testRange := Range{Min: 0.7, Max: 0.3} // Invalid range
		value := testRange.Random()
		s.Assert().Equal(0.7, value, "Should return Min when Min > Max")
	})

	s.Run("Contains works correctly", func() {
		testRange := Range{Min: 0.3, Max: 0.7}

		s.Assert().True(testRange.Contains(0.3), "Should contain Min value")
		s.Assert().True(testRange.Contains(0.7), "Should contain Max value")
		s.Assert().True(testRange.Contains(0.5), "Should contain middle value")
		s.Assert().False(testRange.Contains(0.2), "Should not contain value below Min")
		s.Assert().False(testRange.Contains(0.8), "Should not contain value above Max")
	})

	s.Run("Contains handles edge cases", func() {
		testRange := Range{Min: 0.5, Max: 0.5}
		s.Assert().True(testRange.Contains(0.5), "Should contain exact value when Min == Max")
		s.Assert().False(testRange.Contains(0.4), "Should not contain other values when Min == Max")
	})

	s.Run("String returns formatted representation", func() {
		testRange := Range{Min: 0.3, Max: 0.7}
		expected := "Range{0.30-0.70}"
		s.Assert().Equal(expected, testRange.String())
	})
}

func (s *SelectablesTypesTestSuite) TestRoomProfile() {
	s.Run("creates profile with all fields", func() {
		safetyProfile := SafetyProfile{
			Name:              "standard_safety",
			Description:       "Standard safety requirements for tactical rooms",
			MinPathWidth:      2.0,
			MinOpenSpace:      0.6,
			EntitySize:        1.0,
			EmergencyFallback: true,
		}

		profile := RoomProfile{
			DensityRange:      Range{Min: 0.4, Max: 0.7},
			DestructibleRange: Range{Min: 0.6, Max: 0.9},
			PatternAlgorithm:  "random",
			Shape:             "rectangle",
			RotationMode:      "random",
			SafetyProfile:     safetyProfile,
		}

		s.Assert().Equal(Range{Min: 0.4, Max: 0.7}, profile.DensityRange)
		s.Assert().Equal(Range{Min: 0.6, Max: 0.9}, profile.DestructibleRange)
		s.Assert().Equal("random", profile.PatternAlgorithm)
		s.Assert().Equal("rectangle", profile.Shape)
		s.Assert().Equal("random", profile.RotationMode)
		s.Assert().Equal("standard_safety", profile.SafetyProfile.Name)
	})
}

func (s *SelectablesTypesTestSuite) TestRoomGenerationRequest() {
	s.Run("creates request with all fields", func() {
		customContext := map[string]interface{}{
			"theme":        "dungeon",
			"player_level": 5,
		}

		request := RoomGenerationRequest{
			Purpose:             "combat",
			Difficulty:          3,
			EntityCount:         8,
			SpatialFeeling:      SpatialFeelingNormal,
			RequiredConnections: 2,
			Context:             customContext,
		}

		s.Assert().Equal("combat", request.Purpose)
		s.Assert().Equal(3, request.Difficulty)
		s.Assert().Equal(8, request.EntityCount)
		s.Assert().Equal(SpatialFeelingNormal, request.SpatialFeeling)
		s.Assert().Equal(2, request.RequiredConnections)
		s.Assert().Equal("dungeon", request.Context["theme"])
		s.Assert().Equal(5, request.Context["player_level"])
	})

	s.Run("handles empty request", func() {
		request := RoomGenerationRequest{}

		s.Assert().Equal("", request.Purpose)
		s.Assert().Equal(0, request.Difficulty)
		s.Assert().Equal(0, request.EntityCount)
		s.Assert().Equal(SpatialFeeling(""), request.SpatialFeeling)
		s.Assert().Equal(0, request.RequiredConnections)
		s.Assert().Nil(request.Context)
	})
}

func (s *SelectablesTypesTestSuite) TestSpatialFeeling() {
	s.Run("constants are defined correctly", func() {
		s.Assert().Equal(SpatialFeeling("tight"), SpatialFeelingTight)
		s.Assert().Equal(SpatialFeeling("normal"), SpatialFeelingNormal)
		s.Assert().Equal(SpatialFeeling("vast"), SpatialFeelingVast)
	})

	s.Run("can be used as map keys", func() {
		feelings := map[SpatialFeeling]string{
			SpatialFeelingTight:  "Intimate spaces",
			SpatialFeelingNormal: "Balanced spaces",
			SpatialFeelingVast:   "Epic spaces",
		}

		s.Assert().Equal("Intimate spaces", feelings[SpatialFeelingTight])
		s.Assert().Equal("Balanced spaces", feelings[SpatialFeelingNormal])
		s.Assert().Equal("Epic spaces", feelings[SpatialFeelingVast])
	})
}

func (s *SelectablesTypesTestSuite) TestSafetyProfile() {
	s.Run("creates comparable safety profile", func() {
		safety := SafetyProfile{
			Name:              "high_mobility",
			Description:       "High mobility requirements for fast-paced combat",
			MinPathWidth:      2.5,
			MinOpenSpace:      0.7,
			EntitySize:        1.0,
			EmergencyFallback: false,
		}

		// Has all required fields
		s.Assert().Equal(2.5, safety.MinPathWidth)
		s.Assert().Equal(0.7, safety.MinOpenSpace)
		s.Assert().Equal(1.0, safety.EntitySize)
		s.Assert().False(safety.EmergencyFallback)
		s.Assert().Equal("high_mobility", safety.Name)
		s.Assert().Equal("High mobility requirements for fast-paced combat", safety.Description)
	})

	s.Run("converts to PathSafetyParams correctly", func() {
		safety := SafetyProfile{
			Name:              "test_profile",
			Description:       "Test safety profile",
			MinPathWidth:      3.0,
			MinOpenSpace:      0.8,
			EntitySize:        1.5,
			EmergencyFallback: true,
		}

		pathSafety := safety.ToPathSafetyParams()

		s.Assert().Equal(3.0, pathSafety.MinPathWidth)
		s.Assert().Equal(0.8, pathSafety.MinOpenSpace)
		s.Assert().Equal(1.5, pathSafety.EntitySize)
		s.Assert().True(pathSafety.EmergencyFallback)
		// RequiredPaths should be empty as it's set by room builder
		s.Assert().Empty(pathSafety.RequiredPaths)
	})
}

func (s *SelectablesTypesTestSuite) TestRangeStatisticalDistribution() {
	s.Run("Random produces statistically reasonable distribution", func() {
		testRange := Range{Min: 0.0, Max: 1.0}
		samples := 1000
		sum := 0.0

		for i := 0; i < samples; i++ {
			sum += testRange.Random()
		}

		mean := sum / float64(samples)
		expectedMean := 0.5 // Expected mean for uniform distribution [0,1]
		tolerance := 0.1    // Allow for statistical variance

		s.Assert().True(
			math.Abs(mean-expectedMean) < tolerance,
			"Mean should be approximately 0.5, got %f", mean,
		)
	})
}

func TestSelectablesTypesTestSuite(t *testing.T) {
	suite.Run(t, new(SelectablesTypesTestSuite))
}
