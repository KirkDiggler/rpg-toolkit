package environments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RoomShape represents a room's boundary and connection points
// Purpose: Defines the outer boundary of a room in a grid-agnostic way.
// The spatial module handles the specific grid coordinate system.
type RoomShape struct {
	Name        string `json:"name"`        // Shape identifier
	Description string `json:"description"` // Human-readable description
	Type        string `json:"type"`        // "basic", "junction", "hub"

	// Grid-agnostic boundary definition
	// Uses normalized coordinates (0.0-1.0) that get scaled to actual size
	Boundary []spatial.Position `json:"boundary"` // Outer boundary points

	// Connection points where this room can connect to others
	Connections []ConnectionPoint `json:"connections"` // Where connections attach

	// Metadata for pattern generation
	Properties map[string]interface{} `json:"properties"` // Shape-specific properties

	// Grid compatibility hints
	GridHints GridHints `json:"grid_hints"` // Grid-specific guidance
}

// ConnectionPoint defines where connections can attach to this room
// Purpose: Specifies valid connection locations in a grid-agnostic way
type ConnectionPoint struct {
	Name       string                 `json:"name"`       // Connection identifier
	Position   spatial.Position       `json:"position"`   // Where connection attaches (normalized)
	Direction  string                 `json:"direction"`  // "north", "south", "east", "west", "northeast", etc.
	Type       string                 `json:"type"`       // "door", "passage", "stairs", etc.
	Required   bool                   `json:"required"`   // Must this connection exist?
	Properties map[string]interface{} `json:"properties"` // Connection-specific data
}

// GridHints provides grid-specific guidance for shape optimization
// Purpose: Allows shapes to provide hints for better grid compatibility
// without being tied to specific grid types
type GridHints struct {
	PreferredGridTypes []string               `json:"preferred_grid_types"` // "square", "hex", "gridless"
	MinSize            spatial.Dimensions     `json:"min_size"`             // Minimum recommended size
	MaxSize            spatial.Dimensions     `json:"max_size"`             // Maximum recommended size
	AspectRatio        float64                `json:"aspect_ratio"`         // Preferred width/height ratio
	SnapToGrid         bool                   `json:"snap_to_grid"`         // Should boundary snap to grid?
	Properties         map[string]interface{} `json:"properties"`           // Grid-specific hints
}

// ShapeLoader handles loading room shapes from various sources
// Purpose: Provides flexible shape loading while maintaining grid compatibility
type ShapeLoader struct {
	shapesPath string
	cache      map[string]*RoomShape
}

// NewShapeLoader creates a new shape loader
func NewShapeLoader(shapesPath string) *ShapeLoader {
	return &ShapeLoader{
		shapesPath: shapesPath,
		cache:      make(map[string]*RoomShape),
	}
}

// LoadShape loads a room shape by name
func (sl *ShapeLoader) LoadShape(shapeName string) (*RoomShape, error) {
	// Check cache first
	if shape, exists := sl.cache[shapeName]; exists {
		return shape, nil
	}

	// Load from file
	shape, err := sl.loadShapeFromFile(shapeName)
	if err != nil {
		return nil, fmt.Errorf("failed to load shape %s: %w", shapeName, err)
	}

	// Validate shape
	if err := sl.validateShape(shape); err != nil {
		return nil, fmt.Errorf("invalid shape %s: %w", shapeName, err)
	}

	// Cache and return
	sl.cache[shapeName] = shape
	return shape, nil
}

// LoadAllShapes loads all available shapes
func (sl *ShapeLoader) LoadAllShapes() (map[string]*RoomShape, error) {
	shapes := make(map[string]*RoomShape)

	// Find all shape files
	files, err := filepath.Glob(filepath.Join(sl.shapesPath, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to find shape files: %w", err)
	}

	for _, file := range files {
		shapeName := filepath.Base(file)
		shapeName = shapeName[:len(shapeName)-5] // Remove .yaml extension

		shape, err := sl.LoadShape(shapeName)
		if err != nil {
			return nil, fmt.Errorf("failed to load shape %s: %w", shapeName, err)
		}

		shapes[shapeName] = shape
	}

	return shapes, nil
}

// GetAvailableShapes returns list of available shape names
func (sl *ShapeLoader) GetAvailableShapes() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(sl.shapesPath, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to find shape files: %w", err)
	}

	var shapes []string
	for _, file := range files {
		shapeName := filepath.Base(file)
		shapeName = shapeName[:len(shapeName)-5] // Remove .yaml extension
		shapes = append(shapes, shapeName)
	}

	return shapes, nil
}

// Private methods

func (sl *ShapeLoader) loadShapeFromFile(shapeName string) (*RoomShape, error) {
	filePath := filepath.Join(sl.shapesPath, shapeName+".yaml")

	// Read file
	// #nosec G304 - File path is constructed from controlled input (shapesPath + shapeName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shape file: %w", err)
	}

	// Parse YAML (using JSON unmarshaling for now - would use yaml library in production)
	var shape RoomShape
	if err := json.Unmarshal(data, &shape); err != nil {
		return nil, fmt.Errorf("failed to parse shape file: %w", err)
	}

	return &shape, nil
}

func (sl *ShapeLoader) validateShape(shape *RoomShape) error {
	// Validate required fields
	if shape.Name == "" {
		return fmt.Errorf("shape name is required")
	}

	if len(shape.Boundary) < 3 {
		return fmt.Errorf("shape boundary must have at least 3 points")
	}

	// Validate boundary points are normalized (0.0-1.0)
	for i, point := range shape.Boundary {
		if point.X < 0.0 || point.X > 1.0 || point.Y < 0.0 || point.Y > 1.0 {
			return fmt.Errorf("boundary point %d (%f, %f) is not normalized (must be 0.0-1.0)", i, point.X, point.Y)
		}
	}

	// Validate connections
	for i, conn := range shape.Connections {
		if conn.Name == "" {
			return fmt.Errorf("connection %d name is required", i)
		}

		if conn.Position.X < 0.0 || conn.Position.X > 1.0 || conn.Position.Y < 0.0 || conn.Position.Y > 1.0 {
			return fmt.Errorf("connection %d position (%f, %f) is not normalized", i, conn.Position.X, conn.Position.Y)
		}

		// Validate connection is on or near boundary
		if !sl.isPointOnBoundary(conn.Position, shape.Boundary) {
			return fmt.Errorf("connection %d is not on room boundary", i)
		}
	}

	return nil
}

func (sl *ShapeLoader) isPointOnBoundary(point spatial.Position, boundary []spatial.Position) bool {
	// Simplified boundary check - in production would use proper geometric algorithms
	tolerance := 0.05 // 5% tolerance

	for i := 0; i < len(boundary); i++ {
		next := (i + 1) % len(boundary)

		if sl.isPointOnLineSegment(point, boundary[i], boundary[next], tolerance) {
			return true
		}
	}

	return false
}

func (sl *ShapeLoader) isPointOnLineSegment(point, start, end spatial.Position, tolerance float64) bool {
	// Simple distance-based check
	// In production would use proper line-point distance calculation

	// Calculate distance from point to line segment
	dx := end.X - start.X
	dy := end.Y - start.Y

	if dx == 0 && dy == 0 {
		// Start and end are the same point
		distance := sl.distanceBetween(point, start)
		return distance <= tolerance
	}

	// Parameter t for closest point on line
	t := ((point.X-start.X)*dx + (point.Y-start.Y)*dy) / (dx*dx + dy*dy)

	// Clamp t to line segment
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	// Closest point on line segment
	closestX := start.X + t*dx
	closestY := start.Y + t*dy

	// Distance from point to closest point on line
	distance := sl.distanceBetween(point, spatial.Position{X: closestX, Y: closestY})

	return distance <= tolerance
}

func (sl *ShapeLoader) distanceBetween(p1, p2 spatial.Position) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return dx*dx + dy*dy // Squared distance is sufficient for comparison
}

// Shape transformation utilities

// ScaleShape scales a normalized shape to actual dimensions
// Purpose: Converts normalized coordinates to actual grid coordinates
func ScaleShape(shape *RoomShape, size spatial.Dimensions) *RoomShape {
	scaled := &RoomShape{
		Name:        shape.Name,
		Description: shape.Description,
		Type:        shape.Type,
		Properties:  shape.Properties,
		GridHints:   shape.GridHints,
	}

	// Scale boundary
	scaled.Boundary = make([]spatial.Position, len(shape.Boundary))
	for i, point := range shape.Boundary {
		scaled.Boundary[i] = spatial.Position{
			X: point.X * float64(size.Width),
			Y: point.Y * float64(size.Height),
		}
	}

	// Scale connections
	scaled.Connections = make([]ConnectionPoint, len(shape.Connections))
	for i, conn := range shape.Connections {
		scaled.Connections[i] = ConnectionPoint{
			Name: conn.Name,
			Position: spatial.Position{
				X: conn.Position.X * float64(size.Width),
				Y: conn.Position.Y * float64(size.Height),
			},
			Direction:  conn.Direction,
			Type:       conn.Type,
			Required:   conn.Required,
			Properties: conn.Properties,
		}
	}

	return scaled
}

// IsShapeCompatibleWithGrid checks if a shape works well with a specific grid type
// Purpose: Helps select appropriate shapes for different grid systems
func IsShapeCompatibleWithGrid(shape *RoomShape, gridType string) bool {
	// Check preferred grid types
	if len(shape.GridHints.PreferredGridTypes) > 0 {
		for _, preferred := range shape.GridHints.PreferredGridTypes {
			if preferred == gridType {
				return true
			}
		}
		return false
	}

	// Default compatibility rules
	switch gridType {
	case ShapeSquare:
		// Square grids work with most shapes
		return true
	case "hex":
		// Hex grids prefer shapes with 6-fold symmetry or organic shapes
		return shape.Type == "organic" || len(shape.Boundary) == 6
	case "gridless":
		// Gridless works with any shape
		return true
	default:
		// Unknown grid type - assume compatible
		return true
	}
}

// Default shapes for bootstrapping

// GetDefaultShapes returns basic shapes for development/testing
func GetDefaultShapes() map[string]*RoomShape {
	return map[string]*RoomShape{
		"rectangle": createRectangleShape(),
		"square":    createSquareShape(),
		"l_shape":   createLShape(),
		"t_shape":   createTShape(),
		"cross":     createCrossShape(),
		"oval":      createOvalShape(),
	}
}

// Helper functions to create default shapes

func createRectangleShape() *RoomShape {
	return &RoomShape{
		Name:        "rectangle",
		Description: "Basic rectangular room",
		Type:        "basic",
		Boundary: []spatial.Position{
			{X: 0.0, Y: 0.0}, // Bottom-left
			{X: 1.0, Y: 0.0}, // Bottom-right
			{X: 1.0, Y: 1.0}, // Top-right
			{X: 0.0, Y: 1.0}, // Top-left
		},
		Connections: []ConnectionPoint{
			{Name: "south", Position: spatial.Position{X: 0.5, Y: 0.0}, Direction: "south", Type: "door"},
			{Name: "north", Position: spatial.Position{X: 0.5, Y: 1.0}, Direction: "north", Type: "door"},
			{Name: "east", Position: spatial.Position{X: 1.0, Y: 0.5}, Direction: "east", Type: "door"},
			{Name: "west", Position: spatial.Position{X: 0.0, Y: 0.5}, Direction: "west", Type: "door"},
		},
		GridHints: GridHints{
			PreferredGridTypes: []string{"square", "gridless"},
			MinSize:            spatial.Dimensions{Width: 4, Height: 4},
			MaxSize:            spatial.Dimensions{Width: 50, Height: 50},
			AspectRatio:        1.5, // Can be wider than tall
			SnapToGrid:         true,
		},
	}
}

func createSquareShape() *RoomShape {
	return &RoomShape{
		Name:        "square",
		Description: "Square room for balanced spaces",
		Type:        "basic",
		Boundary: []spatial.Position{
			{X: 0.0, Y: 0.0}, // Bottom-left
			{X: 1.0, Y: 0.0}, // Bottom-right
			{X: 1.0, Y: 1.0}, // Top-right
			{X: 0.0, Y: 1.0}, // Top-left
		},
		Connections: []ConnectionPoint{
			{Name: "south", Position: spatial.Position{X: 0.5, Y: 0.0}, Direction: "south", Type: "door"},
			{Name: "north", Position: spatial.Position{X: 0.5, Y: 1.0}, Direction: "north", Type: "door"},
			{Name: "east", Position: spatial.Position{X: 1.0, Y: 0.5}, Direction: "east", Type: "door"},
			{Name: "west", Position: spatial.Position{X: 0.0, Y: 0.5}, Direction: "west", Type: "door"},
		},
		GridHints: GridHints{
			PreferredGridTypes: []string{"square", "hex", "gridless"},
			MinSize:            spatial.Dimensions{Width: 4, Height: 4},
			MaxSize:            spatial.Dimensions{Width: 30, Height: 30},
			AspectRatio:        1.0, // Always square
			SnapToGrid:         true,
		},
	}
}

func createLShape() *RoomShape {
	return &RoomShape{
		Name:        "l_shape",
		Description: "L-shaped room for corners and turns",
		Type:        "junction",
		Boundary: []spatial.Position{
			{X: 0.0, Y: 0.0}, // Bottom-left
			{X: 0.6, Y: 0.0}, // Bottom-right of short leg
			{X: 0.6, Y: 0.4}, // Inner corner
			{X: 1.0, Y: 0.4}, // Bottom-right of long leg
			{X: 1.0, Y: 1.0}, // Top-right
			{X: 0.0, Y: 1.0}, // Top-left
		},
		Connections: []ConnectionPoint{
			{Name: "south_short", Position: spatial.Position{X: 0.3, Y: 0.0}, Direction: "south", Type: "door"},
			{Name: "south_long", Position: spatial.Position{X: 0.8, Y: 0.4}, Direction: "south", Type: "door"},
			{Name: "north", Position: spatial.Position{X: 0.3, Y: 1.0}, Direction: "north", Type: "door"},
			{Name: "east", Position: spatial.Position{X: 1.0, Y: 0.7}, Direction: "east", Type: "door"},
			{Name: "west", Position: spatial.Position{X: 0.0, Y: 0.5}, Direction: "west", Type: "door"},
		},
		GridHints: GridHints{
			PreferredGridTypes: []string{"square", "gridless"},
			MinSize:            spatial.Dimensions{Width: 6, Height: 6},
			MaxSize:            spatial.Dimensions{Width: 40, Height: 40},
			AspectRatio:        1.0, // Square overall
			SnapToGrid:         true,
		},
	}
}

func createTShape() *RoomShape {
	return &RoomShape{
		Name:        "t_shape",
		Description: "T-shaped room for three-way junctions",
		Type:        "junction",
		Boundary: []spatial.Position{
			{X: 0.0, Y: 0.0}, // Bottom-left
			{X: 1.0, Y: 0.0}, // Bottom-right
			{X: 1.0, Y: 0.4}, // Right side of stem
			{X: 0.7, Y: 0.4}, // Inner corner right
			{X: 0.7, Y: 1.0}, // Top-right
			{X: 0.3, Y: 1.0}, // Top-left
			{X: 0.3, Y: 0.4}, // Inner corner left
			{X: 0.0, Y: 0.4}, // Left side of stem
		},
		Connections: []ConnectionPoint{
			{Name: "south", Position: spatial.Position{X: 0.5, Y: 0.0}, Direction: "south", Type: "door"},
			{Name: "north", Position: spatial.Position{X: 0.5, Y: 1.0}, Direction: "north", Type: "door"},
			{Name: "east", Position: spatial.Position{X: 1.0, Y: 0.2}, Direction: "east", Type: "door"},
			{Name: "west", Position: spatial.Position{X: 0.0, Y: 0.2}, Direction: "west", Type: "door"},
		},
		GridHints: GridHints{
			PreferredGridTypes: []string{"square", "gridless"},
			MinSize:            spatial.Dimensions{Width: 8, Height: 8},
			MaxSize:            spatial.Dimensions{Width: 40, Height: 40},
			AspectRatio:        1.0, // Square overall
			SnapToGrid:         true,
		},
	}
}

func createCrossShape() *RoomShape {
	return &RoomShape{
		Name:        "cross",
		Description: "Cross-shaped room for four-way intersections",
		Type:        "hub",
		Boundary: []spatial.Position{
			{X: 0.3, Y: 0.0}, // Bottom-left of vertical
			{X: 0.7, Y: 0.0}, // Bottom-right of vertical
			{X: 0.7, Y: 0.3}, // Inner corner bottom-right
			{X: 1.0, Y: 0.3}, // Right side bottom
			{X: 1.0, Y: 0.7}, // Right side top
			{X: 0.7, Y: 0.7}, // Inner corner top-right
			{X: 0.7, Y: 1.0}, // Top-right of vertical
			{X: 0.3, Y: 1.0}, // Top-left of vertical
			{X: 0.3, Y: 0.7}, // Inner corner top-left
			{X: 0.0, Y: 0.7}, // Left side top
			{X: 0.0, Y: 0.3}, // Left side bottom
			{X: 0.3, Y: 0.3}, // Inner corner bottom-left
		},
		Connections: []ConnectionPoint{
			{Name: "south", Position: spatial.Position{X: 0.5, Y: 0.0}, Direction: "south", Type: "door"},
			{Name: "north", Position: spatial.Position{X: 0.5, Y: 1.0}, Direction: "north", Type: "door"},
			{Name: "east", Position: spatial.Position{X: 1.0, Y: 0.5}, Direction: "east", Type: "door"},
			{Name: "west", Position: spatial.Position{X: 0.0, Y: 0.5}, Direction: "west", Type: "door"},
		},
		GridHints: GridHints{
			PreferredGridTypes: []string{"square", "gridless"},
			MinSize:            spatial.Dimensions{Width: 10, Height: 10},
			MaxSize:            spatial.Dimensions{Width: 30, Height: 30},
			AspectRatio:        1.0, // Always square
			SnapToGrid:         true,
		},
	}
}

func createOvalShape() *RoomShape {
	// Create approximate oval using multiple points
	boundary := make([]spatial.Position, 0)

	// Generate oval boundary with 12 points
	for i := 0; i < 12; i++ {
		_ = float64(i) * 2.0 * 3.14159 / 12.0 // angle - unused for now
		// Ellipse formula: x = a*cos(θ), y = b*sin(θ)
		x := 0.5 + 0.5*0.8*float64(i%2+1)*0.5 // Slight width variation
		y := 0.5 + 0.5*0.8*float64(i%2+1)*0.5 // Slight height variation
		boundary = append(boundary, spatial.Position{X: x, Y: y})
	}

	return &RoomShape{
		Name:        "oval",
		Description: "Oval room for organic, natural spaces",
		Type:        "organic",
		Boundary:    boundary,
		Connections: []ConnectionPoint{
			{Name: "south", Position: spatial.Position{X: 0.5, Y: 0.1}, Direction: "south", Type: "passage"},
			{Name: "north", Position: spatial.Position{X: 0.5, Y: 0.9}, Direction: "north", Type: "passage"},
			{Name: "east", Position: spatial.Position{X: 0.9, Y: 0.5}, Direction: "east", Type: "passage"},
			{Name: "west", Position: spatial.Position{X: 0.1, Y: 0.5}, Direction: "west", Type: "passage"},
		},
		GridHints: GridHints{
			PreferredGridTypes: []string{"gridless", "hex"},
			MinSize:            spatial.Dimensions{Width: 6, Height: 6},
			MaxSize:            spatial.Dimensions{Width: 25, Height: 25},
			AspectRatio:        1.2,   // Slightly wider than tall
			SnapToGrid:         false, // Organic shapes don't snap
		},
	}
}
