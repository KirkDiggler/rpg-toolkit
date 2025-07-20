package environments

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// GetDefaultSpatialIntentProfile returns a profile for the given spatial feeling
// Purpose: Provides sensible defaults based on the feeling-based design approach
func GetDefaultSpatialIntentProfile(feeling SpatialFeeling) SpatialIntentProfile {
	switch feeling {
	case SpatialFeelingTight:
		return SpatialIntentProfile{
			Feeling:              SpatialFeelingTight,
			EntityDensityTarget:  0.8, // High density for intimate spaces
			MovementFreedomIndex: 0.3, // Limited movement space
			VisualScopeIndex:     0.4, // Close-quarters visibility
			TacticalComplexity:   0.7, // High complexity for tactical positioning
		}
	case SpatialFeelingVast:
		return SpatialIntentProfile{
			Feeling:              SpatialFeelingVast,
			EntityDensityTarget:  0.2, // Low density for epic spaces
			MovementFreedomIndex: 0.8, // Lots of open movement
			VisualScopeIndex:     0.9, // Long sight lines
			TacticalComplexity:   0.4, // Lower complexity, more open combat
		}
	default: // SpatialFeelingNormal
		return SpatialIntentProfile{
			Feeling:              SpatialFeelingNormal,
			EntityDensityTarget:  0.5, // Balanced density
			MovementFreedomIndex: 0.6, // Good movement options
			VisualScopeIndex:     0.6, // Balanced visibility
			TacticalComplexity:   0.6, // Balanced tactical options
		}
	}
}

// CalculateOptimalRoomSize determines the best room dimensions for the given parameters
// Purpose: Core feeling-based sizing function that translates intent into concrete dimensions
func CalculateOptimalRoomSize(intentProfile SpatialIntentProfile, entityCount int) spatial.Dimensions {
	if entityCount <= 0 {
		// Return minimum viable room for no entities
		return spatial.Dimensions{Width: 5, Height: 5}
	}

	// Calculate base area needed for entities
	baseEntityArea := float64(entityCount) / intentProfile.EntityDensityTarget

	// Factor in movement freedom requirements
	movementSpaceMultiplier := 1.0 + (intentProfile.MovementFreedomIndex * 2.0) // 1.0-3.0 range
	totalArea := baseEntityArea * movementSpaceMultiplier

	// Factor in visual scope requirements (longer sight lines need more space)
	visualSpaceMultiplier := 1.0 + (intentProfile.VisualScopeIndex * 0.5) // 1.0-1.5 range
	totalArea *= visualSpaceMultiplier

	// Factor in tactical complexity (more cover needs more space)
	tacticalSpaceMultiplier := 1.0 + (intentProfile.TacticalComplexity * 0.3) // 1.0-1.3 range
	totalArea *= tacticalSpaceMultiplier

	// Apply feeling-specific modifications
	switch intentProfile.Feeling {
	case SpatialFeelingTight:
		// Tight spaces use minimal area but ensure entities can still fit
		totalArea *= 0.8
		// Ensure minimum per entity
		minArea := float64(entityCount) * 2.0 // 2 units per entity minimum
		if totalArea < minArea {
			totalArea = minArea
		}
	case SpatialFeelingVast:
		// Vast spaces get significant area boost
		totalArea *= 2.0
	case SpatialFeelingNormal:
		// Normal uses calculated area as-is
		break
	}

	// Convert area to dimensions (prefer slightly rectangular rooms)
	aspectRatio := 1.2 // Slightly wider than tall
	width := math.Sqrt(totalArea * aspectRatio)
	height := totalArea / width

	// Round to reasonable grid values and ensure minimums
	dimensions := spatial.Dimensions{
		Width:  math.Ceil(width),
		Height: math.Ceil(height),
	}

	// Enforce reasonable minimums and maximums
	if dimensions.Width < 5.0 {
		dimensions.Width = 5.0
	}
	if dimensions.Height < 5.0 {
		dimensions.Height = 5.0
	}
	if dimensions.Width > 100.0 {
		dimensions.Width = 100.0
	}
	if dimensions.Height > 100.0 {
		dimensions.Height = 100.0
	}

	return dimensions
}

// EstimateRoomCapacity analyzes a room size and provides detailed capacity information
// Purpose: Comprehensive capacity analysis for decision making and room optimization
func EstimateRoomCapacity(size spatial.Dimensions, constraints CapacityConstraints) CapacityEstimate {
	totalArea := size.Area()

	// Calculate usable area based on constraints
	usableArea := totalArea * constraints.MinMovementSpace

	// Apply wall density modifier
	usableArea *= (1.0 - constraints.WallDensityModifier*0.3) // Walls reduce usable space

	// Apply pathway multiplier
	usableArea /= constraints.RequiredPathwayMultiplier

	// Calculate entity capacity based on target spatial feeling
	targetProfile := GetDefaultSpatialIntentProfile(constraints.TargetSpatialFeeling)

	// Base entity count from density target - but account for minimum spacing
	baseCapacityFromDensity := int(usableArea * targetProfile.EntityDensityTarget)
	baseCapacityFromSpacing := int(usableArea / (constraints.MinEntitySpacing * constraints.MinEntitySpacing))

	// Use the more restrictive of the two
	recommendedCount := baseCapacityFromDensity
	if baseCapacityFromSpacing < recommendedCount {
		recommendedCount = baseCapacityFromSpacing
	}

	maxCount := int(float64(recommendedCount) * 1.5) // 50% over recommended

	// Apply constraints
	if constraints.MaxEntitiesPerRoom > 0 && recommendedCount > constraints.MaxEntitiesPerRoom {
		recommendedCount = constraints.MaxEntitiesPerRoom
	}
	if constraints.MaxEntitiesPerRoom > 0 && maxCount > constraints.MaxEntitiesPerRoom {
		maxCount = constraints.MaxEntitiesPerRoom
	}

	// Calculate actual spatial feeling based on final capacity
	actualDensity := float64(recommendedCount) / usableArea
	actualFeeling := classifySpatialFeeling(actualDensity, targetProfile)

	// Calculate movement freedom
	spacePerEntity := usableArea / float64(recommendedCount)
	movementFreedom := math.Min(1.0, spacePerEntity/4.0) // 4+ units per entity = full freedom

	// Calculate quality score (how well this matches intent)
	qualityScore := calculateQualityScore(targetProfile, actualFeeling, movementFreedom)

	estimate := CapacityEstimate{
		RecommendedEntityCount: recommendedCount,
		MaxEntityCount:         maxCount,
		SpatialFeelingActual:   actualFeeling,
		MovementFreedomActual:  movementFreedom,
		QualityScore:           qualityScore,
		UsableArea:             usableArea,
	}

	return estimate
}

// GetSplitOptions provides room splitting options without making decisions
// Purpose: Advisory function that gives games splitting options to consider
func GetSplitOptions(size spatial.Dimensions, entityCount int, constraints CapacityConstraints) []RoomSplit {
	return generateSplitRecommendations(size, constraints, entityCount)
}

// AnalyzeRoomCapacityForEntityCount analyzes how well a room handles a specific entity count
// Purpose: Provides games with capacity analysis to help them make layout decisions
func AnalyzeRoomCapacityForEntityCount(
	size spatial.Dimensions, entityCount int, constraints CapacityConstraints,
) CapacityAnalysis {
	estimate := EstimateRoomCapacity(size, constraints)

	// Calculate how the requested entity count compares to room capacity
	capacityUtilization := float64(entityCount) / float64(estimate.RecommendedEntityCount)

	// Determine spatial feeling that would result from this entity count
	actualDensity := float64(entityCount) / estimate.UsableArea
	resultingFeeling := classifyDensityAsFeeling(actualDensity)

	return CapacityAnalysis{
		RoomCapacity:            estimate,
		RequestedEntityCount:    entityCount,
		CapacityUtilization:     capacityUtilization,
		ResultingSpatialFeeling: resultingFeeling,
		SplitOptions:            GetSplitOptions(size, entityCount, constraints),
	}
}

// GetDefaultConstraintsForFeeling provides sensible default constraints for a spatial feeling
// Purpose: Helper function for games that want default constraints but want to make their own decisions
func GetDefaultConstraintsForFeeling(feeling SpatialFeeling) CapacityConstraints {
	return CapacityConstraints{
		MaxEntitiesPerRoom:        0,   // No arbitrary limit - let games decide
		MinMovementSpace:          0.6, // 60% must be navigable
		TargetSpatialFeeling:      feeling,
		MinEntitySpacing:          2.0, // Require 2x2 area per entity minimum
		WallDensityModifier:       0.5, // Moderate wall density
		RequiredPathwayMultiplier: 1.2, // 20% extra for pathways
	}
}

// Helper functions

func classifyDensityAsFeeling(density float64) SpatialFeeling {
	// Compare actual density to feeling ranges
	if density >= 0.7 {
		return SpatialFeelingTight
	} else if density <= 0.3 {
		return SpatialFeelingVast
	}
	return SpatialFeelingNormal
}

func classifySpatialFeeling(density float64, _ SpatialIntentProfile) SpatialFeeling {
	// Compare actual density to feeling ranges
	switch {
	case density >= 0.7:
		return SpatialFeelingTight
	case density <= 0.3:
		return SpatialFeelingVast
	default:
		return SpatialFeelingNormal
	}
}

func calculateQualityScore(target SpatialIntentProfile, actualFeeling SpatialFeeling, actualMovement float64) float64 {
	score := 1.0

	// Penalize feeling mismatch
	if target.Feeling != actualFeeling {
		score *= 0.7
	}

	// Penalize movement freedom mismatch
	movementDiff := math.Abs(target.MovementFreedomIndex - actualMovement)
	score *= (1.0 - movementDiff*0.5) // Max 50% penalty for movement mismatch

	return math.Max(0.0, score)
}

func generateSplitRecommendations(size spatial.Dimensions, _ CapacityConstraints, entityCount int) []RoomSplit {
	recommendations := make([]RoomSplit, 0, 3)

	// Option 1: Two equal rooms side-by-side
	halfWidth := size.Width / 2
	if halfWidth >= 3 { // Minimum viable room size for splits
		split1 := RoomSplit{
			SuggestedSize: spatial.Dimensions{Width: halfWidth, Height: size.Height},
			ConnectionPoints: []spatial.Position{
				{X: halfWidth, Y: size.Height / 2},
			},
			RecommendedEntityDistribution: map[string]int{
				"room_1": entityCount / 2,
				"room_2": (entityCount + 1) / 2, // Handle odd numbers
			},
			RecommendedConnectionType:    "door",
			SplitReason:                  "Horizontal split for manageable entity density",
			EstimatedCapacityImprovement: 0.8,
		}
		recommendations = append(recommendations, split1)
	}

	// Option 2: Two equal rooms top-and-bottom
	halfHeight := size.Height / 2
	if halfHeight >= 3 {
		split2 := RoomSplit{
			SuggestedSize: spatial.Dimensions{Width: size.Width, Height: halfHeight},
			ConnectionPoints: []spatial.Position{
				{X: size.Width / 2, Y: halfHeight},
			},
			RecommendedEntityDistribution: map[string]int{
				"room_1": entityCount / 2,
				"room_2": (entityCount + 1) / 2,
			},
			RecommendedConnectionType:    "door",
			SplitReason:                  "Vertical split for manageable entity density",
			EstimatedCapacityImprovement: 0.8,
		}
		recommendations = append(recommendations, split2)
	}

	// Option 3: Central hub with smaller side rooms (if space allows)
	if size.Width >= 15 && size.Height >= 15 {
		centralWidth := size.Width - 8
		centralHeight := size.Height - 8
		split3 := RoomSplit{
			SuggestedSize: spatial.Dimensions{Width: centralWidth, Height: centralHeight},
			ConnectionPoints: []spatial.Position{
				{X: centralWidth / 2, Y: 0},             // North connection
				{X: centralWidth, Y: centralHeight / 2}, // East connection
				{X: centralWidth / 2, Y: centralHeight}, // South connection
				{X: 0, Y: centralHeight / 2},            // West connection
			},
			RecommendedEntityDistribution: map[string]int{
				"central_room": entityCount * 2 / 3,
				"side_room_1":  entityCount / 6,
				"side_room_2":  entityCount / 6,
			},
			RecommendedConnectionType:    "passage",
			SplitReason:                  "Hub and spoke design for complex encounters",
			EstimatedCapacityImprovement: 0.9,
		}
		recommendations = append(recommendations, split3)
	}

	return recommendations
}
