package environments

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ShapeTestSuite struct {
	suite.Suite
	testShape *RoomShape
}

func (s *ShapeTestSuite) SetupTest() {
	// Create a test rectangle shape with connections
	s.testShape = &RoomShape{
		Name:        "test_rectangle",
		Description: "Test rectangular room",
		Type:        "basic",
		Boundary: []spatial.Position{
			{X: 0.0, Y: 0.0}, // Bottom-left
			{X: 1.0, Y: 0.0}, // Bottom-right
			{X: 1.0, Y: 1.0}, // Top-right
			{X: 0.0, Y: 1.0}, // Top-left
		},
		Connections: []ConnectionPoint{
			{
				Name:      "south",
				Position:  spatial.Position{X: 0.5, Y: 0.0},
				Direction: "south",
				Type:      "door",
				Required:  false,
			},
			{
				Name:      "east",
				Position:  spatial.Position{X: 1.0, Y: 0.5},
				Direction: "east",
				Type:      "door",
				Required:  false,
			},
			{
				Name:      "north",
				Position:  spatial.Position{X: 0.5, Y: 1.0},
				Direction: "north",
				Type:      "door",
				Required:  false,
			},
			{
				Name:      "west",
				Position:  spatial.Position{X: 0.0, Y: 0.5},
				Direction: "west",
				Type:      "door",
				Required:  false,
			},
		},
		Properties: map[string]interface{}{
			"test": "value",
		},
		GridHints: GridHints{
			PreferredGridTypes: []string{"square"},
		},
	}
}

func (s *ShapeTestSuite) TestRotateShape_NoRotation() {
	rotated := RotateShape(s.testShape, 0)

	// Should be identical to original (same instance for 0 degrees)
	s.Equal(s.testShape, rotated)
}

func (s *ShapeTestSuite) TestRotateShape_90Degrees() {
	rotated := RotateShape(s.testShape, 90)

	// Test metadata preservation
	s.Equal(s.testShape.Name, rotated.Name)
	s.Equal(s.testShape.Description, rotated.Description)
	s.Equal(s.testShape.Type, rotated.Type)
	s.Equal(s.testShape.Properties, rotated.Properties)
	s.Equal(s.testShape.GridHints, rotated.GridHints)

	// Test boundary rotation (90° clockwise)
	// Original: (0,0), (1,0), (1,1), (0,1)
	// Expected: (1,0), (1,1), (0,1), (0,0)
	expected := []spatial.Position{
		{X: 1.0, Y: 0.0}, // (0,0) -> (1,0)
		{X: 1.0, Y: 1.0}, // (1,0) -> (1,1)
		{X: 0.0, Y: 1.0}, // (1,1) -> (0,1)
		{X: 0.0, Y: 0.0}, // (0,1) -> (0,0)
	}

	s.Require().Len(rotated.Boundary, len(expected))
	for i, expectedPoint := range expected {
		s.InDelta(expectedPoint.X, rotated.Boundary[i].X, 0.0001,
			"Boundary point %d X coordinate mismatch", i)
		s.InDelta(expectedPoint.Y, rotated.Boundary[i].Y, 0.0001,
			"Boundary point %d Y coordinate mismatch", i)
	}
}

func (s *ShapeTestSuite) TestRotateShape_180Degrees() {
	rotated := RotateShape(s.testShape, 180)

	// Test boundary rotation (180°)
	// Original: (0,0), (1,0), (1,1), (0,1)
	// Expected: (1,1), (0,1), (0,0), (1,0)
	expected := []spatial.Position{
		{X: 1.0, Y: 1.0}, // (0,0) -> (1,1)
		{X: 0.0, Y: 1.0}, // (1,0) -> (0,1)
		{X: 0.0, Y: 0.0}, // (1,1) -> (0,0)
		{X: 1.0, Y: 0.0}, // (0,1) -> (1,0)
	}

	s.Require().Len(rotated.Boundary, len(expected))
	for i, expectedPoint := range expected {
		s.InDelta(expectedPoint.X, rotated.Boundary[i].X, 0.0001,
			"Boundary point %d X coordinate mismatch", i)
		s.InDelta(expectedPoint.Y, rotated.Boundary[i].Y, 0.0001,
			"Boundary point %d Y coordinate mismatch", i)
	}
}

func (s *ShapeTestSuite) TestRotateShape_270Degrees() {
	rotated := RotateShape(s.testShape, 270)

	// Test boundary rotation (270° clockwise = 90° counter-clockwise)
	// Original: (0,0), (1,0), (1,1), (0,1)
	// Expected: (0,1), (0,0), (1,0), (1,1)
	expected := []spatial.Position{
		{X: 0.0, Y: 1.0}, // (0,0) -> (0,1)
		{X: 0.0, Y: 0.0}, // (1,0) -> (0,0)
		{X: 1.0, Y: 0.0}, // (1,1) -> (1,0)
		{X: 1.0, Y: 1.0}, // (0,1) -> (1,1)
	}

	s.Require().Len(rotated.Boundary, len(expected))
	for i, expectedPoint := range expected {
		s.InDelta(expectedPoint.X, rotated.Boundary[i].X, 0.0001,
			"Boundary point %d X coordinate mismatch", i)
		s.InDelta(expectedPoint.Y, rotated.Boundary[i].Y, 0.0001,
			"Boundary point %d Y coordinate mismatch", i)
	}
}

func (s *ShapeTestSuite) TestRotateShape_ConnectionDirections() {
	rotated := RotateShape(s.testShape, 90)

	// Test connection direction rotation (90° clockwise)
	expectedDirections := map[string]string{
		"south": "east",  // south -> east
		"east":  "north", // east -> north
		"north": "west",  // north -> west
		"west":  "south", // west -> south
	}

	s.Require().Len(rotated.Connections, len(s.testShape.Connections))
	for _, conn := range rotated.Connections {
		expectedDir := expectedDirections[s.getOriginalDirection(conn.Name)]
		s.Equal(expectedDir, conn.Direction,
			"Connection %s direction should be %s, got %s", conn.Name, expectedDir, conn.Direction)
	}
}

func (s *ShapeTestSuite) TestRotateShape_ConnectionPositions() {
	rotated := RotateShape(s.testShape, 90)

	// Test connection position rotation (90° clockwise)
	// Original south (0.5, 0.0) -> Expected east (1.0, 0.5)
	// Original east (1.0, 0.5) -> Expected north (0.5, 1.0)
	// Original north (0.5, 1.0) -> Expected west (0.0, 0.5)
	// Original west (0.0, 0.5) -> Expected south (0.5, 0.0)

	expectedPositions := map[string]spatial.Position{
		"south": {X: 1.0, Y: 0.5}, // (0.5, 0.0) -> (1.0, 0.5)
		"east":  {X: 0.5, Y: 1.0}, // (1.0, 0.5) -> (0.5, 1.0)
		"north": {X: 0.0, Y: 0.5}, // (0.5, 1.0) -> (0.0, 0.5)
		"west":  {X: 0.5, Y: 0.0}, // (0.0, 0.5) -> (0.5, 0.0)
	}

	for _, conn := range rotated.Connections {
		expected := expectedPositions[conn.Name]
		s.InDelta(expected.X, conn.Position.X, 0.0001,
			"Connection %s position X should be %f, got %f", conn.Name, expected.X, conn.Position.X)
		s.InDelta(expected.Y, conn.Position.Y, 0.0001,
			"Connection %s position Y should be %f, got %f", conn.Name, expected.Y, conn.Position.Y)
	}
}

func (s *ShapeTestSuite) TestRotateShape_NegativeAngle() {
	rotated := RotateShape(s.testShape, -90)

	// -90° should be equivalent to 270°
	expected := RotateShape(s.testShape, 270)

	s.Equal(len(expected.Boundary), len(rotated.Boundary))
	for i := range expected.Boundary {
		s.InDelta(expected.Boundary[i].X, rotated.Boundary[i].X, 0.0001)
		s.InDelta(expected.Boundary[i].Y, rotated.Boundary[i].Y, 0.0001)
	}
}

func (s *ShapeTestSuite) TestRotateShape_LargeAngle() {
	rotated := RotateShape(s.testShape, 450) // 450° = 90°

	expected := RotateShape(s.testShape, 90)

	s.Equal(len(expected.Boundary), len(rotated.Boundary))
	for i := range expected.Boundary {
		s.InDelta(expected.Boundary[i].X, rotated.Boundary[i].X, 0.0001)
		s.InDelta(expected.Boundary[i].Y, rotated.Boundary[i].Y, 0.0001)
	}
}

func (s *ShapeTestSuite) TestRotateDirection() {
	testCases := []struct {
		original string
		degrees  int
		expected string
	}{
		// 90° clockwise rotation (matches position-based test expectations)
		{"north", 90, "west"},
		{"east", 90, "north"},
		{"south", 90, "east"},
		{"west", 90, "south"},
		{"northeast", 90, "northwest"},
		{"southeast", 90, "northeast"},
		{"southwest", 90, "southeast"},
		{"northwest", 90, "southwest"},
		// 180° rotation
		{"north", 180, "south"},
		{"east", 180, "west"},
		// 270° rotation
		{"north", 270, "east"},
		{"unknown", 90, "unknown"}, // Unknown direction should be preserved
	}

	for _, tc := range testCases {
		result := rotateDirection(tc.original, tc.degrees)
		s.Equal(tc.expected, result,
			"rotateDirection(%s, %d) should be %s, got %s",
			tc.original, tc.degrees, tc.expected, result)
	}
}

func (s *ShapeTestSuite) TestRotatePointAroundCenter() {
	testCases := []struct {
		name     string
		point    spatial.Position
		degrees  int
		expected spatial.Position
	}{
		{
			name:     "90 degrees - bottom center to right center",
			point:    spatial.Position{X: 0.5, Y: 0.0},
			degrees:  90,
			expected: spatial.Position{X: 1.0, Y: 0.5},
		},
		{
			name:     "180 degrees - center point unchanged",
			point:    spatial.Position{X: 0.5, Y: 0.5},
			degrees:  180,
			expected: spatial.Position{X: 0.5, Y: 0.5},
		},
		{
			name:     "270 degrees - bottom left to top left",
			point:    spatial.Position{X: 0.0, Y: 0.0},
			degrees:  270,
			expected: spatial.Position{X: 0.0, Y: 1.0},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			radians := float64(tc.degrees) * math.Pi / 180.0
			cosTheta := math.Cos(radians)
			sinTheta := math.Sin(radians)

			result := rotatePointAroundCenter(tc.point, cosTheta, sinTheta)

			s.InDelta(tc.expected.X, result.X, 0.0001,
				"Point X coordinate mismatch")
			s.InDelta(tc.expected.Y, result.Y, 0.0001,
				"Point Y coordinate mismatch")
		})
	}
}

func (s *ShapeTestSuite) TestRotateShape_PreservesConnectionMetadata() {
	rotated := RotateShape(s.testShape, 90)

	// All connection metadata except position and direction should be preserved
	s.Require().Len(rotated.Connections, len(s.testShape.Connections))

	for i, originalConn := range s.testShape.Connections {
		rotatedConn := rotated.Connections[i]

		s.Equal(originalConn.Name, rotatedConn.Name)
		s.Equal(originalConn.Type, rotatedConn.Type)
		s.Equal(originalConn.Required, rotatedConn.Required)
		s.Equal(originalConn.Properties, rotatedConn.Properties)
		// Position and Direction are tested separately
	}
}

// Helper function to get original direction by connection name
func (s *ShapeTestSuite) getOriginalDirection(connName string) string {
	for _, conn := range s.testShape.Connections {
		if conn.Name == connName {
			return conn.Direction
		}
	}
	return ""
}

func TestShapeTestSuite(t *testing.T) {
	suite.Run(t, new(ShapeTestSuite))
}
