package environments

import (
	"fmt"
	"math/rand"

	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
)

// Range represents a numeric range for parameter variance multiplication
// Purpose: Enables selectables to choose constraint profiles while maintaining
// infinite procedural variety within those constraints
type Range struct {
	Min, Max float64
}

// Random returns a random value within the range using math/rand
// Note: Uses math/rand (not crypto/rand) for deterministic, reproducible
// room generation when combined with seeded random sources
func (r Range) Random() float64 {
	if r.Min >= r.Max {
		return r.Min
	}
	// #nosec G404 - Using math/rand for deterministic, reproducible room generation
	// Same seed must produce identical room parameters for gameplay consistency
	return r.Min + rand.Float64()*(r.Max-r.Min)
}

// Contains checks if a value falls within the range (inclusive)
func (r Range) Contains(value float64) bool {
	return value >= r.Min && value <= r.Max
}

// String returns a string representation of the range
func (r Range) String() string {
	return fmt.Sprintf("Range{%.2f-%.2f}", r.Min, r.Max)
}

// RoomProfile represents a complete room generation profile composed from
// multiple selectables table selections
// Purpose: Aggregates all parameters needed for room generation while
// maintaining procedural variance through range-based selection
type RoomProfile struct {
	DensityRange      Range         // Wall density constraints (0.0-1.0)
	DestructibleRange Range         // Destructible wall ratio constraints (0.0-1.0)
	PatternAlgorithm  string        // "random", "clustered", "sparse", "empty"
	Shape             string        // "rectangle", "square", "hexagon", "t_shape"
	RotationMode      string        // "random", "fixed", "cardinal_only"
	SafetyProfile     SafetyProfile // Path safety requirements
}

// RoomGenerationRequest contains all criteria for room generation
// Purpose: Provides context for selectables-driven room generation with
// optional customization capabilities
type RoomGenerationRequest struct {
	Purpose             string                                     // "combat", "exploration", "boss"
	Difficulty          int                                        // 1-5 scale influences constraints
	EntityCount         int                                        // Expected entity count for sizing
	SpatialFeeling      SpatialFeeling                             // "tight", "normal", "vast"
	RequiredConnections int                                        // Number of connection points needed
	CustomTables        map[string]selectables.SelectionTable[any] // Optional table overrides
	Context             map[string]interface{}                     // Additional context for selection
}

// SpatialFeeling represents the intended spatial experience for room design
type SpatialFeeling string

const (
	// SpatialFeelingTight creates intimate, claustrophobic spaces
	SpatialFeelingTight SpatialFeeling = "tight"
	// SpatialFeelingNormal creates balanced, tactical spaces
	SpatialFeelingNormal SpatialFeeling = "normal"
	// SpatialFeelingVast creates expansive, epic spaces
	SpatialFeelingVast SpatialFeeling = "vast"
)

// SafetyProfile represents path safety requirements for room generation
// Purpose: Provides a comparable type for selectables that creates PathSafetyParams
type SafetyProfile struct {
	Name              string  // Profile identifier for selectables
	Description       string  // Human-readable description
	MinPathWidth      float64 // Minimum corridor width
	MinOpenSpace      float64 // % of room that must remain open (0.0-1.0)
	EntitySize        float64 // Size of entities that need to move
	EmergencyFallback bool    // Fall back to empty room if validation fails
}

// ToPathSafetyParams converts SafetyProfile to PathSafetyParams for room generation
// Purpose: Bridges selectables-compatible type to existing room builder requirements
func (s SafetyProfile) ToPathSafetyParams() PathSafetyParams {
	return PathSafetyParams{
		MinPathWidth:      s.MinPathWidth,
		MinOpenSpace:      s.MinOpenSpace,
		EntitySize:        s.EntitySize,
		EmergencyFallback: s.EmergencyFallback,
		// RequiredPaths will be set by room builder based on shape connections
	}
}
