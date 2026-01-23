package environments

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type EnvironmentDataTestSuite struct {
	suite.Suite
}

func TestEnvironmentDataSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentDataTestSuite))
}

func (s *EnvironmentDataTestSuite) TestTypedStringSerialization() {
	s.Run("EnvironmentType serializes as string", func() {
		type testStruct struct {
			Type EnvironmentType `json:"type"`
		}
		ts := testStruct{Type: "dungeon"}

		data, err := json.Marshal(ts)
		s.Require().NoError(err)
		s.Assert().Contains(string(data), `"type":"dungeon"`)

		var restored testStruct
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)
		s.Assert().Equal(EnvironmentType("dungeon"), restored.Type)
	})

	s.Run("GridShapeValue constants serialize correctly", func() {
		shapes := []GridShapeValue{GridShapeHex, GridShapeSquare, GridShapeGridless}
		expected := []string{"hex", "square", "gridless"}

		for i, shape := range shapes {
			s.Assert().Equal(expected[i], string(shape))
		}
	})

	s.Run("HexOrientationValue constants serialize correctly", func() {
		orientations := []HexOrientationValue{HexOrientationPointy, HexOrientationFlat}
		expected := []string{"pointy", "flat"}

		for i, orientation := range orientations {
			s.Assert().Equal(expected[i], string(orientation))
		}
	})

	s.Run("PropertyKey used as map key", func() {
		props := map[PropertyKey]any{
			PropertyKey("open"):   false,
			PropertyKey("locked"): true,
			PropertyKey("dc"):     15,
		}

		data, err := json.Marshal(props)
		s.Require().NoError(err)

		var restored map[PropertyKey]any
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Assert().Equal(false, restored["open"])
		s.Assert().Equal(true, restored["locked"])
		s.Assert().Equal(float64(15), restored["dc"]) // JSON numbers become float64
	})
}

func (s *EnvironmentDataTestSuite) TestEnvironmentDataRoundTrip() {
	s.Run("complete environment serializes and deserializes correctly", func() {
		original := EnvironmentData{
			ID:    "env-001",
			Type:  "dungeon",
			Theme: "dark-crypt",
			Metadata: EnvironmentMetadata{
				Name:        "The Forgotten Crypt",
				Description: "An ancient burial ground",
				Theme:       "dark-crypt",
				Tags:        []string{"undead", "dungeon", "level-3"},
				Properties:  map[string]string{"difficulty": "hard"},
				GeneratedAt: time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
				GeneratedBy: "graph-generator",
				Version:     "1.0.0",
			},
			Zones: []ZoneData{
				{
					ID:          "zone-entrance",
					Type:        "entrance",
					Origin:      spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
					Width:       10,
					Height:      10,
					GridShape:   GridShapeHex,
					Orientation: HexOrientationPointy,
					EntityIDs:   []string{"entity-1", "entity-2"},
				},
				{
					ID:          "zone-main",
					Type:        "combat",
					Origin:      spatial.CubeCoordinate{X: 15, Y: -8, Z: -7},
					Width:       15,
					Height:      15,
					GridShape:   GridShapeHex,
					Orientation: HexOrientationPointy,
					EntityIDs:   []string{"entity-3"},
				},
			},
			Passages: []PassageData{
				{
					ID:                  "passage-1",
					FromZoneID:          "zone-entrance",
					ToZoneID:            "zone-main",
					ControllingEntityID: "entity-2",
					Bidirectional:       true,
				},
			},
			Entities: []PlacedEntityData{
				{
					ID:             "entity-1",
					Type:           "monster",
					Position:       spatial.CubeCoordinate{X: 3, Y: -1, Z: -2},
					Size:           1,
					BlocksMovement: true,
					BlocksLoS:      false,
					ZoneID:         "zone-entrance",
					Subtype:        "skeleton",
					Properties: map[PropertyKey]any{
						PropertyKey("hp"): 13,
					},
				},
				{
					ID:             "entity-2",
					Type:           "door",
					Position:       spatial.CubeCoordinate{X: 9, Y: -4, Z: -5},
					Size:           1,
					BlocksMovement: true,
					BlocksLoS:      true,
					ZoneID:         "zone-entrance",
					Subtype:        "wooden",
					Properties: map[PropertyKey]any{
						PropertyKey("open"):   false,
						PropertyKey("locked"): true,
					},
				},
				{
					ID:             "entity-3",
					Type:           "monster",
					Position:       spatial.CubeCoordinate{X: 18, Y: -10, Z: -8},
					Size:           2,
					BlocksMovement: true,
					BlocksLoS:      false,
					ZoneID:         "zone-main",
					Subtype:        "ogre",
				},
			},
			Walls: []WallSegmentData{
				{
					Start:          spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
					End:            spatial.CubeCoordinate{X: 9, Y: -4, Z: -5},
					BlocksMovement: true,
					BlocksLoS:      true,
				},
			},
		}

		// Serialize
		data, err := json.Marshal(original)
		s.Require().NoError(err)
		s.Require().NotEmpty(data)

		// Deserialize
		var restored EnvironmentData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		// Verify top-level fields
		s.Assert().Equal(original.ID, restored.ID)
		s.Assert().Equal(original.Type, restored.Type)
		s.Assert().Equal(original.Theme, restored.Theme)

		// Verify metadata
		s.Assert().Equal(original.Metadata.Name, restored.Metadata.Name)
		s.Assert().Equal(original.Metadata.Description, restored.Metadata.Description)
		s.Assert().Equal(original.Metadata.Tags, restored.Metadata.Tags)
		s.Assert().Equal(original.Metadata.GeneratedBy, restored.Metadata.GeneratedBy)

		// Verify zones
		s.Require().Len(restored.Zones, 2)
		s.Assert().Equal(original.Zones[0].ID, restored.Zones[0].ID)
		s.Assert().Equal(original.Zones[0].Origin, restored.Zones[0].Origin)
		s.Assert().Equal(original.Zones[0].GridShape, restored.Zones[0].GridShape)
		s.Assert().Equal(original.Zones[0].Orientation, restored.Zones[0].Orientation)
		s.Assert().Equal(original.Zones[1].Origin, restored.Zones[1].Origin)

		// Verify passages
		s.Require().Len(restored.Passages, 1)
		s.Assert().Equal(original.Passages[0].ControllingEntityID, restored.Passages[0].ControllingEntityID)
		s.Assert().Equal(original.Passages[0].Bidirectional, restored.Passages[0].Bidirectional)

		// Verify entities
		s.Require().Len(restored.Entities, 3)
		s.Assert().Equal(original.Entities[0].Position, restored.Entities[0].Position)
		s.Assert().Equal(original.Entities[0].Type, restored.Entities[0].Type)
		s.Assert().Equal(original.Entities[0].Subtype, restored.Entities[0].Subtype)

		// Verify walls
		s.Require().Len(restored.Walls, 1)
		s.Assert().Equal(original.Walls[0].Start, restored.Walls[0].Start)
	})
}

func (s *EnvironmentDataTestSuite) TestCubeCoordinatesValid() {
	s.Run("positions use valid cube coordinates (x+y+z=0)", func() {
		testCoords := []spatial.CubeCoordinate{
			{X: 0, Y: 0, Z: 0},
			{X: 3, Y: -1, Z: -2},
			{X: 5, Y: -2, Z: -3},
			{X: 9, Y: -4, Z: -5},
			{X: 15, Y: -8, Z: -7},
		}

		for _, coord := range testCoords {
			s.Assert().True(coord.IsValid(), "coordinate %v should be valid", coord)
		}
	})
}

func (s *EnvironmentDataTestSuite) TestEntityTypesAreStrings() {
	s.Run("entity types are game-defined strings", func() {
		entities := []PlacedEntityData{
			{ID: "e1", Type: "monster", Position: spatial.CubeCoordinate{X: 1, Y: -1, Z: 0}},
			{ID: "e2", Type: "obstacle", Position: spatial.CubeCoordinate{X: 2, Y: -1, Z: -1}},
			{ID: "e3", Type: "door", Position: spatial.CubeCoordinate{X: 3, Y: -2, Z: -1}},
			{ID: "e4", Type: "character", Position: spatial.CubeCoordinate{X: 0, Y: 0, Z: 0}},
			{ID: "e5", Type: "custom_game_type", Position: spatial.CubeCoordinate{X: 4, Y: -2, Z: -2}},
		}

		// Serialize
		data, err := json.Marshal(entities)
		s.Require().NoError(err)

		// Deserialize
		var restored []PlacedEntityData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		// Verify types preserved as strings
		s.Require().Len(restored, 5)
		s.Assert().Equal("monster", restored[0].Type)
		s.Assert().Equal("obstacle", restored[1].Type)
		s.Assert().Equal("door", restored[2].Type)
		s.Assert().Equal("character", restored[3].Type)
		s.Assert().Equal("custom_game_type", restored[4].Type)
	})
}

func (s *EnvironmentDataTestSuite) TestPropertiesFlexible() {
	s.Run("properties map handles arbitrary game data with typed keys", func() {
		entity := PlacedEntityData{
			ID:       "door-1",
			Type:     "door",
			Position: spatial.CubeCoordinate{X: 5, Y: -3, Z: -2},
			Properties: map[PropertyKey]any{
				PropertyKey("open"):        false,
				PropertyKey("locked"):      true,
				PropertyKey("dc"):          15,
				PropertyKey("damage"):      "2d6",
				PropertyKey("key_id"):      "skeleton-key",
				PropertyKey("custom_flag"): true,
			},
		}

		// Serialize
		data, err := json.Marshal(entity)
		s.Require().NoError(err)

		// Deserialize
		var restored PlacedEntityData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		// Verify properties round-trip (JSON numbers become float64)
		s.Assert().Equal(false, restored.Properties["open"])
		s.Assert().Equal(true, restored.Properties["locked"])
		s.Assert().Equal(float64(15), restored.Properties["dc"])
		s.Assert().Equal("2d6", restored.Properties["damage"])
		s.Assert().Equal("skeleton-key", restored.Properties["key_id"])
		s.Assert().Equal(true, restored.Properties["custom_flag"])
	})
}

func (s *EnvironmentDataTestSuite) TestEmptyEnvironment() {
	s.Run("empty environment serializes correctly", func() {
		empty := EnvironmentData{
			ID:       "empty-env",
			Type:     "test",
			Zones:    []ZoneData{},
			Passages: []PassageData{},
			Entities: []PlacedEntityData{},
			Walls:    []WallSegmentData{},
		}

		data, err := json.Marshal(empty)
		s.Require().NoError(err)

		var restored EnvironmentData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Assert().Equal(empty.ID, restored.ID)
		s.Assert().Equal(empty.Type, restored.Type)
		s.Assert().Empty(restored.Zones)
		s.Assert().Empty(restored.Entities)
	})
}

func (s *EnvironmentDataTestSuite) TestZoneGridShapes() {
	s.Run("all grid shapes serialize correctly", func() {
		zones := []ZoneData{
			{
				ID:          "hex-zone",
				Type:        "room",
				Origin:      spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
				Width:       10,
				Height:      10,
				GridShape:   GridShapeHex,
				Orientation: HexOrientationPointy,
			},
			{
				ID:        "square-zone",
				Type:      "room",
				Origin:    spatial.CubeCoordinate{X: 20, Y: -10, Z: -10},
				Width:     10,
				Height:    10,
				GridShape: GridShapeSquare,
			},
			{
				ID:        "gridless-zone",
				Type:      "outdoor",
				Origin:    spatial.CubeCoordinate{X: 40, Y: -20, Z: -20},
				Width:     100,
				Height:    100,
				GridShape: GridShapeGridless,
			},
		}

		data, err := json.Marshal(zones)
		s.Require().NoError(err)

		var restored []ZoneData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Require().Len(restored, 3)
		s.Assert().Equal(GridShapeHex, restored[0].GridShape)
		s.Assert().Equal(HexOrientationPointy, restored[0].Orientation)
		s.Assert().Equal(GridShapeSquare, restored[1].GridShape)
		s.Assert().Equal(GridShapeGridless, restored[2].GridShape)
	})
}

func (s *EnvironmentDataTestSuite) TestPassageBidirectional() {
	s.Run("bidirectional passage", func() {
		passage := PassageData{
			ID:            "p-1",
			FromZoneID:    "zone-a",
			ToZoneID:      "zone-b",
			Bidirectional: true,
		}

		data, err := json.Marshal(passage)
		s.Require().NoError(err)

		var restored PassageData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Assert().True(restored.Bidirectional)
	})

	s.Run("one-way passage", func() {
		passage := PassageData{
			ID:            "p-stairs",
			FromZoneID:    "zone-upper",
			ToZoneID:      "zone-lower",
			Bidirectional: false,
		}

		data, err := json.Marshal(passage)
		s.Require().NoError(err)

		var restored PassageData
		err = json.Unmarshal(data, &restored)
		s.Require().NoError(err)

		s.Assert().False(restored.Bidirectional)
	})
}
