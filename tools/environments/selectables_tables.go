package environments

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
)

// GetDefaultDensityTable returns a table for wall density range selection
// Purpose: Provides variety in wall density while maintaining constraint profiles
func GetDefaultDensityTable() selectables.SelectionTable[Range] {
	table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
		ID: "default_wall_density",
	})

	// Sparse walls - minimal obstacles
	table.Add(Range{Min: 0.1, Max: 0.3}, 20)

	// Light walls - some tactical cover
	table.Add(Range{Min: 0.3, Max: 0.5}, 35)

	// Medium walls - balanced tactical complexity
	table.Add(Range{Min: 0.4, Max: 0.7}, 30)

	// Heavy walls - complex navigation
	table.Add(Range{Min: 0.6, Max: 0.9}, 15)

	return table
}

// GetDefaultDestructibleTable returns a table for destructible wall ratio selection
// Purpose: Provides variety in wall interaction possibilities
func GetDefaultDestructibleTable() selectables.SelectionTable[Range] {
	table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
		ID: "default_destructible_ratio",
	})

	// Mostly permanent walls
	table.Add(Range{Min: 0.1, Max: 0.3}, 25)

	// Balanced destructible/permanent
	table.Add(Range{Min: 0.4, Max: 0.7}, 40)

	// Mostly destructible walls
	table.Add(Range{Min: 0.7, Max: 0.9}, 35)

	return table
}

// GetDefaultPatternTable returns a table for wall pattern algorithm selection
// Purpose: Provides variety in wall arrangement styles
func GetDefaultPatternTable() selectables.SelectionTable[string] {
	table := selectables.NewBasicTable[string](selectables.BasicTableConfig{
		ID: "default_wall_patterns",
	})

	// Empty rooms for movement-focused gameplay
	table.Add("empty", 25)

	// Random scattered walls for tactical variety
	table.Add("random", 50)

	// Future patterns can be added here:
	// table.Add("clustered", 15)
	// table.Add("maze", 10)

	return table
}

// GetDefaultShapeTable returns a table for room shape selection
// Purpose: Provides variety in room geometry while respecting common usage patterns
func GetDefaultShapeTable() selectables.SelectionTable[string] {
	table := selectables.NewBasicTable[string](selectables.BasicTableConfig{
		ID: "default_room_shapes",
	})

	// Rectangle - most versatile and common
	table.Add("rectangle", 40)

	// Square - balanced and symmetric
	table.Add("square", 35)

	// T-shape - interesting for multi-connection rooms
	table.Add("t_shape", 15)

	// Hexagon - organic feeling
	table.Add("hexagon", 10)

	return table
}

// GetDefaultRotationModeTable returns a table for rotation mode selection
// Purpose: Provides variety in room orientation
func GetDefaultRotationModeTable() selectables.SelectionTable[string] {
	table := selectables.NewBasicTable[string](selectables.BasicTableConfig{
		ID: "default_rotation_modes",
	})

	// Random rotation for maximum variety
	table.Add("random", 70)

	// No rotation for predictable layouts
	table.Add("fixed", 20)

	// Cardinal directions only (0, 90, 180, 270)
	table.Add("cardinal_only", 10)

	return table
}

// GetDefaultSafetyProfileTable returns a table for safety profile selection
// Purpose: Provides variety in pathfinding and safety requirements
func GetDefaultSafetyProfileTable() selectables.SelectionTable[SafetyProfile] {
	table := selectables.NewBasicTable[SafetyProfile](selectables.BasicTableConfig{
		ID: "default_safety_profiles",
	})

	// Standard safety for most rooms
	standardSafety := SafetyProfile{
		Name:              "standard",
		Description:       "Balanced safety requirements for typical gameplay",
		MinPathWidth:      2.0,
		MinOpenSpace:      0.6,
		EntitySize:        1.0,
		EmergencyFallback: true,
	}
	table.Add(standardSafety, 50)

	// High mobility for fast-paced combat
	highMobility := SafetyProfile{
		Name:              "high_mobility",
		Description:       "High mobility requirements for fast-paced combat",
		MinPathWidth:      2.5,
		MinOpenSpace:      0.7,
		EntitySize:        1.0,
		EmergencyFallback: true,
	}
	table.Add(highMobility, 30)

	// Tight spaces for tactical challenge
	tightSpaces := SafetyProfile{
		Name:              "tight_spaces",
		Description:       "Minimal space requirements for challenging navigation",
		MinPathWidth:      1.5,
		MinOpenSpace:      0.5,
		EntitySize:        1.0,
		EmergencyFallback: true,
	}
	table.Add(tightSpaces, 20)

	return table
}

// GetDenseCoverTables returns tables for high wall density rooms
// Purpose: Provides dense wall coverage (0.6-0.9 range) for complex navigation and tactical positioning
func GetDenseCoverTables() RoomTables {
	return RoomTables{
		DensityTable: func() selectables.SelectionTable[Range] {
			table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
				ID: "dense_cover_density",
			})
			// High density ranges for complex navigation
			table.Add(Range{Min: 0.6, Max: 0.8}, 50)
			table.Add(Range{Min: 0.7, Max: 0.9}, 50)
			return table
		}(),
		DestructibleTable: func() selectables.SelectionTable[Range] {
			table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
				ID: "dense_cover_destructible",
			})
			// Moderate to high destructible ratios for interaction opportunities
			table.Add(Range{Min: 0.5, Max: 0.7}, 40)
			table.Add(Range{Min: 0.6, Max: 0.9}, 60)
			return table
		}(),
		PatternTable:      GetDefaultPatternTable(),
		ShapeTable:        GetDefaultShapeTable(),
		RotationModeTable: GetDefaultRotationModeTable(),
		SafetyTable:       GetDefaultSafetyProfileTable(),
	}
}

// GetSparseCoverTables returns tables for low wall density rooms
// Purpose: Provides sparse wall coverage (0.1-0.4 range) for movement-focused gameplay
func GetSparseCoverTables() RoomTables {
	return RoomTables{
		DensityTable: func() selectables.SelectionTable[Range] {
			table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
				ID: "sparse_cover_density",
			})
			// Low density ranges for easy movement
			table.Add(Range{Min: 0.1, Max: 0.3}, 60)
			table.Add(Range{Min: 0.2, Max: 0.4}, 40)
			return table
		}(),
		DestructibleTable: func() selectables.SelectionTable[Range] {
			table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
				ID: "sparse_cover_destructible",
			})
			// Moderate destructible ratios for interaction opportunities
			table.Add(Range{Min: 0.3, Max: 0.6}, 50)
			table.Add(Range{Min: 0.4, Max: 0.7}, 50)
			return table
		}(),
		PatternTable:      GetDefaultPatternTable(),
		ShapeTable:        GetDefaultShapeTable(),
		RotationModeTable: GetDefaultRotationModeTable(),
		SafetyTable:       GetDefaultSafetyProfileTable(),
	}
}

// RoomTables aggregates all selectables tables needed for room generation
// Purpose: Provides a complete set of tables for modular room generation
type RoomTables struct {
	DensityTable      selectables.SelectionTable[Range]
	DestructibleTable selectables.SelectionTable[Range]
	PatternTable      selectables.SelectionTable[string]
	ShapeTable        selectables.SelectionTable[string]
	RotationModeTable selectables.SelectionTable[string]
	SafetyTable       selectables.SelectionTable[SafetyProfile]
}

// GetBalancedCoverTables returns tables for medium wall density rooms
// Purpose: Provides balanced wall coverage (0.4-0.7 range) for general use
func GetBalancedCoverTables() RoomTables {
	return RoomTables{
		DensityTable: func() selectables.SelectionTable[Range] {
			table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
				ID: "balanced_cover_density",
			})
			// Medium density ranges for balanced gameplay
			table.Add(Range{Min: 0.4, Max: 0.6}, 50)
			table.Add(Range{Min: 0.5, Max: 0.7}, 50)
			return table
		}(),
		DestructibleTable: func() selectables.SelectionTable[Range] {
			table := selectables.NewBasicTable[Range](selectables.BasicTableConfig{
				ID: "balanced_cover_destructible",
			})
			// Balanced destructible ratios for varied interactions
			table.Add(Range{Min: 0.4, Max: 0.7}, 50)
			table.Add(Range{Min: 0.6, Max: 0.8}, 50)
			return table
		}(),
		PatternTable:      GetDefaultPatternTable(),
		ShapeTable:        GetDefaultShapeTable(),
		RotationModeTable: GetDefaultRotationModeTable(),
		SafetyTable:       GetDefaultSafetyProfileTable(),
	}
}

// GetDefaultRoomTables returns the standard set of tables for room generation
// Purpose: Provides default tables with balanced variety for general use
func GetDefaultRoomTables() RoomTables {
	return RoomTables{
		DensityTable:      GetDefaultDensityTable(),
		DestructibleTable: GetDefaultDestructibleTable(),
		PatternTable:      GetDefaultPatternTable(),
		ShapeTable:        GetDefaultShapeTable(),
		RotationModeTable: GetDefaultRotationModeTable(),
		SafetyTable:       GetDefaultSafetyProfileTable(),
	}
}
