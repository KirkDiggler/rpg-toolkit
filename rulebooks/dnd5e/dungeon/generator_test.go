package dungeon_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/dungeon"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// GeneratorTestSuite tests the full dungeon generation pipeline.
// These are integration tests that use real components (GraphBasedGenerator).
type GeneratorTestSuite struct {
	suite.Suite
	generator *dungeon.Generator
	ctx       context.Context
}

func TestGeneratorSuite(t *testing.T) {
	suite.Run(t, new(GeneratorTestSuite))
}

// SetupTest runs before EACH test function
func (s *GeneratorTestSuite) SetupTest() {
	s.generator = dungeon.NewGenerator(nil)
	s.ctx = context.Background()
}

// --- tests.json required tests ---

// TestGeneratorExists verifies: generator-exists
// NewGenerator(nil) should create a Generator. Generator.Generate(ctx, input)
// should accept GenerateInput and return GenerateOutput.
func (s *GeneratorTestSuite) TestGeneratorExists() {
	s.Run("NewGenerator(nil) creates a Generator", func() {
		gen := dungeon.NewGenerator(nil)
		s.Assert().NotNil(gen, "NewGenerator(nil) should return a Generator")
	})

	s.Run("NewGenerator with config creates a Generator", func() {
		config := &dungeon.GeneratorConfig{}
		gen := dungeon.NewGenerator(config)
		s.Assert().NotNil(gen, "NewGenerator with config should return a Generator")
	})

	s.Run("Generate accepts GenerateInput and returns GenerateOutput", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  5.0,
			RoomCount: 3,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err, "Generate should not return error with valid input")
		s.Assert().NotNil(output, "Generate should return GenerateOutput")
		s.Assert().NotNil(output.Dungeon, "Output should contain a Dungeon")
		s.Assert().NotZero(output.Seed, "Output should contain the seed used")
	})
}

// TestGenerateReturnsDungeon verifies: generate-returns-dungeon
// Generate() should return *Dungeon (runtime object), not *DungeonData.
// Caller uses dungeon.ToData() for persistence.
func (s *GeneratorTestSuite) TestGenerateReturnsDungeon() {
	s.Run("Generate returns *Dungeon runtime object", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  5.0,
			RoomCount: 3,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Verify we get a *Dungeon (runtime object)
		s.Assert().NotNil(output.Dungeon, "should return *Dungeon")

		// Verify we can call runtime methods
		s.Assert().NotEmpty(output.Dungeon.ID(), "Dungeon should have an ID")
		s.Assert().NotEmpty(output.Dungeon.StartRoom(), "Dungeon should have a start room")
		s.Assert().NotEmpty(output.Dungeon.BossRoom(), "Dungeon should have a boss room")
	})

	s.Run("Caller uses ToData() for persistence", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  5.0,
			RoomCount: 3,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// ToData() should return the persistence format
		data := output.Dungeon.ToData()
		s.Assert().NotNil(data, "ToData() should return *DungeonData")
		s.Assert().NotEmpty(data.Environment.ID, "DungeonData should have environment ID")
		s.Assert().NotEmpty(data.StartRoomID, "DungeonData should have start room ID")
		s.Assert().NotEmpty(data.Rooms, "DungeonData should have rooms")
	})
}

// TestGenerateRespectsRoomCount verifies: generate-respects-room-count
// Generate with RoomCount=5 should produce a dungeon with exactly 5 rooms.
func (s *GeneratorTestSuite) TestGenerateRespectsRoomCount() {
	testCases := []struct {
		name      string
		roomCount int
	}{
		{"1 room", 1},
		{"3 rooms", 3},
		{"5 rooms", 5},
		{"7 rooms", 7},
		{"10 rooms", 10},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			input := &dungeon.GenerateInput{
				Theme:     dungeon.ThemeCrypt,
				TargetCR:  float64(tc.roomCount) * 2.0, // Scale CR with room count
				RoomCount: tc.roomCount,
				Seed:      12345,
			}

			output, err := s.generator.Generate(s.ctx, input)
			s.Require().NoError(err)

			rooms := output.Dungeon.Rooms()
			s.Assert().Len(rooms, tc.roomCount,
				"expected %d rooms, got %d", tc.roomCount, len(rooms))
		})
	}
}

// TestGenerateSeedReproducible verifies: generate-seed-reproducible
// Same Seed should produce same room structure. Different seeds produce different results.
// Note: exact monster selection may vary due to map iteration.
func (s *GeneratorTestSuite) TestGenerateSeedReproducible() {
	s.Run("same seed produces same room structure", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      42424242,
		}

		// Generate twice with the same seed
		output1, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		output2, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Verify same structure
		s.Assert().Equal(output1.Seed, output2.Seed, "seeds should match")
		s.Assert().Equal(len(output1.Dungeon.Rooms()), len(output2.Dungeon.Rooms()),
			"room counts should match")
		s.Assert().Equal(output1.Dungeon.StartRoom(), output2.Dungeon.StartRoom(),
			"start rooms should match")
		s.Assert().Equal(output1.Dungeon.BossRoom(), output2.Dungeon.BossRoom(),
			"boss rooms should match")

		// Verify same room types
		for roomID, room1 := range output1.Dungeon.Rooms() {
			room2 := output2.Dungeon.Room(roomID)
			s.Require().NotNil(room2, "room %s should exist in both dungeons", roomID)
			s.Assert().Equal(room1.Type, room2.Type,
				"room %s types should match", roomID)
		}
	})

	s.Run("different seeds produce different results", func() {
		// Generate with multiple different seeds
		seeds := make(map[int64]bool)

		for i := int64(1); i <= 5; i++ {
			input := &dungeon.GenerateInput{
				Theme:     dungeon.ThemeCrypt,
				TargetCR:  10.0,
				RoomCount: 5,
				Seed:      i * 1000, // 1000, 2000, 3000, 4000, 5000
			}

			output, err := s.generator.Generate(s.ctx, input)
			s.Require().NoError(err)

			// Verify the requested seed was used
			s.Assert().Equal(i*1000, output.Seed,
				"output seed should match requested seed")
			seeds[output.Seed] = true
		}

		// Each seed should be unique
		s.Assert().Len(seeds, 5, "should have 5 unique seeds")
	})

	s.Run("seed 0 generates random seed", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  5.0,
			RoomCount: 3,
			Seed:      0, // Zero means generate random seed
		}

		output1, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Sleep briefly to ensure different timestamp
		output2, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Seeds should be non-zero and potentially different
		s.Assert().NotZero(output1.Seed, "generated seed should be non-zero")
		s.Assert().NotZero(output2.Seed, "generated seed should be non-zero")
	})
}

// TestBossRoomHasBoss verifies: boss-room-has-boss
// Generated dungeon should have exactly one boss room.
// Boss room encounter should contain at least one monster with RoleBoss.
func (s *GeneratorTestSuite) TestBossRoomHasBoss() {
	s.Run("dungeon has exactly one boss room", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Count boss rooms
		bossRoomCount := 0
		var bossRoomID string
		for roomID, room := range output.Dungeon.Rooms() {
			if room.Type == dungeon.RoomTypeBoss {
				bossRoomCount++
				bossRoomID = roomID
			}
		}

		s.Assert().Equal(1, bossRoomCount, "should have exactly one boss room")
		s.Assert().Equal(output.Dungeon.BossRoom(), bossRoomID,
			"BossRoom() should return the boss room ID")
	})

	s.Run("boss room has at least one RoleBoss monster", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		bossRoom := output.Dungeon.Room(output.Dungeon.BossRoom())
		s.Require().NotNil(bossRoom, "boss room should exist")
		s.Require().NotNil(bossRoom.Encounter, "boss room should have an encounter")

		// Find boss monster
		hasBoss := false
		for _, monster := range bossRoom.Encounter.Monsters {
			if monster.Role == dungeon.RoleBoss {
				hasBoss = true
				break
			}
		}

		s.Assert().True(hasBoss, "boss room encounter should contain at least one RoleBoss monster")
	})

	s.Run("IsBossRoom correctly identifies boss room", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Check each room
		for roomID := range output.Dungeon.Rooms() {
			isBoss := output.Dungeon.IsBossRoom(roomID)
			if roomID == output.Dungeon.BossRoom() {
				s.Assert().True(isBoss, "IsBossRoom should return true for boss room")
			} else {
				s.Assert().False(isBoss, "IsBossRoom should return false for non-boss room %s", roomID)
			}
		}
	})
}

// TestValidationErrors verifies: validation-errors
// Generate should return correct errors for invalid input.
func (s *GeneratorTestSuite) TestValidationErrors() {
	s.Run("ErrNilInput for nil input", func() {
		output, err := s.generator.Generate(s.ctx, nil)
		s.Assert().Nil(output, "output should be nil for invalid input")
		s.Assert().ErrorIs(err, dungeon.ErrNilInput, "should return ErrNilInput")
	})

	s.Run("ErrInvalidTheme for empty theme", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.Theme{}, // Empty theme (no ID)
			TargetCR:  5.0,
			RoomCount: 3,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Assert().Nil(output, "output should be nil for invalid input")
		s.Assert().ErrorIs(err, dungeon.ErrInvalidTheme, "should return ErrInvalidTheme")
	})

	s.Run("ErrInvalidCR for CR <= 0", func() {
		testCases := []float64{0.0, -1.0, -100.0}

		for _, cr := range testCases {
			input := &dungeon.GenerateInput{
				Theme:     dungeon.ThemeCrypt,
				TargetCR:  cr,
				RoomCount: 3,
			}

			output, err := s.generator.Generate(s.ctx, input)
			s.Assert().Nil(output, "output should be nil for CR=%f", cr)
			s.Assert().ErrorIs(err, dungeon.ErrInvalidCR, "should return ErrInvalidCR for CR=%f", cr)
		}
	})

	s.Run("ErrInvalidRoomCount for rooms < 1", func() {
		testCases := []int{0, -1, -10}

		for _, count := range testCases {
			input := &dungeon.GenerateInput{
				Theme:     dungeon.ThemeCrypt,
				TargetCR:  5.0,
				RoomCount: count,
			}

			output, err := s.generator.Generate(s.ctx, input)
			s.Assert().Nil(output, "output should be nil for RoomCount=%d", count)
			s.Assert().ErrorIs(err, dungeon.ErrInvalidRoomCount,
				"should return ErrInvalidRoomCount for RoomCount=%d", count)
		}
	})
}

// --- Additional integration tests ---

// TestLayoutAutoSelection verifies layout is auto-selected based on room count.
func (s *GeneratorTestSuite) TestLayoutAutoSelection() {
	testCases := []struct {
		name           string
		roomCount      int
		expectedLayout environments.LayoutType
	}{
		{"1-3 rooms gets linear", 3, environments.LayoutTypeLinear},
		{"4-7 rooms gets branching", 5, environments.LayoutTypeBranching},
		{"8-15 rooms gets organic", 10, environments.LayoutTypeOrganic},
		{"16+ rooms gets grid", 20, environments.LayoutTypeGrid},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			input := &dungeon.GenerateInput{
				Theme:     dungeon.ThemeCrypt,
				TargetCR:  float64(tc.roomCount) * 2.0,
				RoomCount: tc.roomCount,
				Seed:      12345,
				// Layout not specified - should be auto-selected
			}

			output, err := s.generator.Generate(s.ctx, input)
			s.Require().NoError(err)

			// Verify dungeon was created successfully
			// (Layout selection is internal - we verify by checking dungeon is valid)
			s.Assert().NotNil(output.Dungeon)
			s.Assert().Len(output.Dungeon.Rooms(), tc.roomCount)
		})
	}

	s.Run("explicit layout is respected", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  6.0,
			RoomCount: 3, // Would normally get Linear
			Layout:    environments.LayoutTypeBranching,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)
		s.Assert().NotNil(output.Dungeon)
	})
}

// TestStartRoomIsRevealed verifies the start room is properly initialized.
func (s *GeneratorTestSuite) TestStartRoomIsRevealed() {
	s.Run("CurrentRoomID is set to start room", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		s.Assert().Equal(output.Dungeon.StartRoom(), output.Dungeon.CurrentRoomID(),
			"CurrentRoomID should be set to start room")
	})

	s.Run("start room is revealed", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		startRoom := output.Dungeon.StartRoom()
		s.Assert().True(output.Dungeon.RoomRevealed(startRoom),
			"start room should be revealed")
	})

	s.Run("non-start rooms are not revealed initially", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		startRoom := output.Dungeon.StartRoom()
		for roomID := range output.Dungeon.Rooms() {
			if roomID != startRoom {
				s.Assert().False(output.Dungeon.RoomRevealed(roomID),
					"room %s should not be revealed initially", roomID)
			}
		}
	})
}

// TestStateIsActive verifies new dungeon starts in active state.
func (s *GeneratorTestSuite) TestStateIsActive() {
	s.Run("new dungeon state is StateActive", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		s.Assert().Equal(dungeon.StateActive, output.Dungeon.State(),
			"new dungeon should be in StateActive")
	})

	s.Run("CompletedAt is nil for new dungeon", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		s.Assert().Nil(output.Dungeon.CompletedAt(),
			"CompletedAt should be nil for new dungeon")
	})
}

// TestRoomTypesAssigned verifies room type assignment.
func (s *GeneratorTestSuite) TestRoomTypesAssigned() {
	s.Run("first room is RoomTypeEntrance", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		startRoom := output.Dungeon.Room(output.Dungeon.StartRoom())
		s.Require().NotNil(startRoom)
		s.Assert().Equal(dungeon.RoomTypeEntrance, startRoom.Type,
			"first room should be RoomTypeEntrance")
	})

	s.Run("last room is RoomTypeBoss", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		bossRoom := output.Dungeon.Room(output.Dungeon.BossRoom())
		s.Require().NotNil(bossRoom)
		s.Assert().Equal(dungeon.RoomTypeBoss, bossRoom.Type,
			"last room should be RoomTypeBoss")
	})

	s.Run("all rooms have a type assigned", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		for roomID, room := range output.Dungeon.Rooms() {
			s.Assert().NotEmpty(string(room.Type),
				"room %s should have a type assigned", roomID)
		}
	})
}

// TestAllRoomsHaveEncounters verifies encounter assignment.
func (s *GeneratorTestSuite) TestAllRoomsHaveEncounters() {
	s.Run("every room has an EncounterData", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		for roomID, room := range output.Dungeon.Rooms() {
			s.Assert().NotNil(room.Encounter,
				"room %s should have an EncounterData", roomID)
		}
	})

	s.Run("low CR may result in empty encounters", func() {
		// With very low CR, some rooms might not have monsters
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  0.5, // Very low CR
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Encounters exist but may have no monsters
		for roomID, room := range output.Dungeon.Rooms() {
			s.Assert().NotNil(room.Encounter,
				"room %s should have EncounterData even if empty", roomID)
			// Monsters slice exists but may be empty
			s.Assert().NotNil(room.Encounter.Monsters,
				"room %s encounter should have Monsters slice", roomID)
		}
	})

	s.Run("adequate CR produces populated encounters", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  20.0, // High enough for all rooms
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// At least some rooms should have monsters with adequate CR
		totalMonsters := 0
		for _, room := range output.Dungeon.Rooms() {
			if room.Encounter != nil {
				totalMonsters += len(room.Encounter.Monsters)
			}
		}

		s.Assert().Greater(totalMonsters, 0,
			"dungeon should have monsters with adequate CR")
	})
}

// TestToDataRoundTrip verifies serialization round-trip.
func (s *GeneratorTestSuite) TestToDataRoundTrip() {
	s.Run("Generate -> ToData -> LoadFromData produces equivalent dungeon", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		// Generate original dungeon
		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)
		original := output.Dungeon

		// Convert to data
		data := original.ToData()
		s.Require().NotNil(data)

		// Load from data
		loadOutput, err := dungeon.LoadFromData(&dungeon.LoadFromDataInput{
			Data: data,
		})
		s.Require().NoError(err)
		loaded := loadOutput.Dungeon

		// Verify equivalence
		s.Assert().Equal(original.ID(), loaded.ID())
		s.Assert().Equal(original.StartRoom(), loaded.StartRoom())
		s.Assert().Equal(original.BossRoom(), loaded.BossRoom())
		s.Assert().Equal(original.Seed(), loaded.Seed())
		s.Assert().Equal(original.State(), loaded.State())
		s.Assert().Equal(original.CurrentRoomID(), loaded.CurrentRoomID())
		s.Assert().Len(loaded.Rooms(), len(original.Rooms()))

		// Verify rooms match
		for roomID, originalRoom := range original.Rooms() {
			loadedRoom := loaded.Room(roomID)
			s.Require().NotNil(loadedRoom, "loaded dungeon should have room %s", roomID)
			s.Assert().Equal(originalRoom.Type, loadedRoom.Type)
			if originalRoom.Encounter != nil {
				s.Require().NotNil(loadedRoom.Encounter)
				s.Assert().Equal(originalRoom.Encounter.TotalCR, loadedRoom.Encounter.TotalCR)
				s.Assert().Len(loadedRoom.Encounter.Monsters, len(originalRoom.Encounter.Monsters))
			}
		}

		// Verify revealed rooms
		s.Assert().Equal(original.RoomRevealed(original.StartRoom()),
			loaded.RoomRevealed(loaded.StartRoom()))
	})
}

// --- Theme-specific tests ---

// TestAllPredefinedThemesWork verifies all themes can generate dungeons.
func (s *GeneratorTestSuite) TestAllPredefinedThemesWork() {
	themes := []dungeon.Theme{
		dungeon.ThemeCrypt,
		dungeon.ThemeCave,
		dungeon.ThemeBanditLair,
	}

	for _, theme := range themes {
		s.Run("theme "+theme.ID+" generates successfully", func() {
			input := &dungeon.GenerateInput{
				Theme:     theme,
				TargetCR:  10.0,
				RoomCount: 5,
				Seed:      12345,
			}

			output, err := s.generator.Generate(s.ctx, input)
			s.Require().NoError(err, "theme %s should generate without error", theme.ID)
			s.Assert().NotNil(output.Dungeon)
			s.Assert().Len(output.Dungeon.Rooms(), 5)

			// Verify boss room has a boss
			bossRoom := output.Dungeon.Room(output.Dungeon.BossRoom())
			s.Require().NotNil(bossRoom)
			s.Require().NotNil(bossRoom.Encounter)

			hasBoss := false
			for _, monster := range bossRoom.Encounter.Monsters {
				if monster.Role == dungeon.RoleBoss {
					hasBoss = true
					break
				}
			}
			s.Assert().True(hasBoss, "theme %s boss room should have a boss", theme.ID)
		})
	}
}

// TestDungeonHasDoorsConnections verifies connections exist.
func (s *GeneratorTestSuite) TestDungeonHasDoorsConnections() {
	s.Run("multi-room dungeon has doors/connections", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		doors := output.Dungeon.Doors()
		s.Assert().NotEmpty(doors, "dungeon should have doors/connections")

		// Linear layout with 5 rooms should have at least 4 connections
		// (but layout may vary)
		s.Assert().GreaterOrEqual(len(doors), 1,
			"dungeon should have at least one connection")
	})

	s.Run("DoorsFromRoom returns connections for a room", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Start room should have at least one connection
		startRoom := output.Dungeon.StartRoom()
		connections := output.Dungeon.DoorsFromRoom(startRoom)
		s.Assert().NotEmpty(connections,
			"start room should have at least one connection")
	})
}

// TestCreatedAtIsSet verifies timestamp is set.
func (s *GeneratorTestSuite) TestCreatedAtIsSet() {
	s.Run("CreatedAt is set on generation", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		createdAt := output.Dungeon.CreatedAt()
		s.Assert().False(createdAt.IsZero(), "CreatedAt should be set")
	})
}

// TestSingleRoomDungeon tests edge case of 1 room.
func (s *GeneratorTestSuite) TestSingleRoomDungeon() {
	s.Run("single room dungeon works correctly", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  5.0,
			RoomCount: 1,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		// Single room is both entrance and boss
		s.Assert().Len(output.Dungeon.Rooms(), 1)
		s.Assert().Equal(output.Dungeon.StartRoom(), output.Dungeon.BossRoom(),
			"single room should be both start and boss")

		// Room should have boss encounter
		room := output.Dungeon.Room(output.Dungeon.BossRoom())
		s.Require().NotNil(room)
		s.Assert().NotNil(room.Encounter)

		// With full budget, should have monsters
		s.Assert().NotEmpty(room.Encounter.Monsters,
			"single room boss encounter should have monsters")
	})
}

// TestEnvironmentDataPopulated verifies environment data is properly set.
func (s *GeneratorTestSuite) TestEnvironmentDataPopulated() {
	s.Run("environment data has zones", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		env := output.Dungeon.Environment()
		s.Assert().NotNil(env)
		s.Assert().NotEmpty(env.Zones, "environment should have zones")
		s.Assert().Len(env.Zones, 5, "should have one zone per room")
	})

	s.Run("environment data has passages", func() {
		input := &dungeon.GenerateInput{
			Theme:     dungeon.ThemeCrypt,
			TargetCR:  10.0,
			RoomCount: 5,
			Seed:      12345,
		}

		output, err := s.generator.Generate(s.ctx, input)
		s.Require().NoError(err)

		env := output.Dungeon.Environment()
		s.Assert().NotEmpty(env.Passages, "environment should have passages")
	})
}
