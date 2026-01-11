// Package example contains narrated tests that demonstrate dungeon generation.
// These tests are designed to be educational - each one explains a key concept
// through working code. Run with -v to see the narrated output.
package example

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/dungeon"
)

// ExampleTestSuite provides narrated examples of dungeon generation.
// Each test demonstrates a key concept with detailed output.
type ExampleTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestExampleSuite(t *testing.T) {
	suite.Run(t, new(ExampleTestSuite))
}

func (s *ExampleTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// TestBasicGeneration demonstrates the simplest way to create a dungeon.
// This is the pattern you'll use most often: create a generator, provide
// a theme and parameters, and get back a fully-formed dungeon.
func (s *ExampleTestSuite) TestBasicGeneration() {
	// Step 1: Create a generator. Pass nil for default configuration.
	// The generator orchestrates layout creation and encounter generation.
	gen := dungeon.NewGenerator(nil)

	// Step 2: Configure what kind of dungeon you want.
	// - Theme: Determines monsters, wall materials, and atmosphere
	// - TargetCR: Total challenge rating budget distributed across rooms
	// - RoomCount: How many rooms to generate
	input := &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCrypt,
		TargetCR:  5.0,
		RoomCount: 5,
	}

	// Step 3: Generate the dungeon. This creates the spatial layout and
	// populates each room with an appropriate encounter.
	output, err := gen.Generate(s.ctx, input)
	s.Require().NoError(err, "Generation should succeed with valid input")

	// Step 4: Access the dungeon. The output contains:
	// - Dungeon: The runtime object with methods for gameplay
	// - Seed: The random seed used (for reproducibility)
	d := output.Dungeon

	// The dungeon is immediately ready for use
	s.Assert().NotEmpty(d.ID(), "Dungeon has a unique identifier")
	s.Assert().NotEmpty(d.StartRoom(), "Dungeon has an entrance room")
	s.Assert().NotEmpty(d.BossRoom(), "Dungeon has a boss room at the end")
	s.Assert().Equal(dungeon.StateActive, d.State(), "Dungeon starts in active state")

	// Verify we got the requested number of rooms
	s.Assert().Len(d.Rooms(), 5, "Dungeon has requested number of rooms")

	s.T().Logf("Generated dungeon %q with %d rooms", d.ID(), len(d.Rooms()))
	s.T().Logf("Start room: %s, Boss room: %s", d.StartRoom(), d.BossRoom())
}

// TestSeedReproducibility demonstrates that the same seed produces the same
// dungeon structure. This is essential for:
// - Saving and restoring dungeon state
// - Debugging specific dungeon configurations
// - Sharing dungeon "seeds" with other players
func (s *ExampleTestSuite) TestSeedReproducibility() {
	gen := dungeon.NewGenerator(nil)

	// Use a specific seed for reproducibility
	seed := int64(42)

	// Generate with the same seed twice
	input1 := &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCave,
		TargetCR:  3.0,
		RoomCount: 4,
		Seed:      seed,
	}

	input2 := &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCave,
		TargetCR:  3.0,
		RoomCount: 4,
		Seed:      seed,
	}

	output1, err := gen.Generate(s.ctx, input1)
	s.Require().NoError(err)

	output2, err := gen.Generate(s.ctx, input2)
	s.Require().NoError(err)

	// Same seed produces same dungeon structure
	s.Assert().Equal(output1.Dungeon.StartRoom(), output2.Dungeon.StartRoom(),
		"Same seed produces same start room")
	s.Assert().Equal(output1.Dungeon.BossRoom(), output2.Dungeon.BossRoom(),
		"Same seed produces same boss room")
	s.Assert().Equal(len(output1.Dungeon.Rooms()), len(output2.Dungeon.Rooms()),
		"Same seed produces same number of rooms")

	// The returned seed should match what we provided
	s.Assert().Equal(seed, output1.Seed, "Output seed matches input seed")

	s.T().Logf("Seed %d consistently produces dungeon with start=%s, boss=%s",
		seed, output1.Dungeon.StartRoom(), output1.Dungeon.BossRoom())
}

// TestThemesProduceDifferentEncounters shows how themes affect what
// monsters appear in the dungeon. Each theme has its own monster pools
// configured for that environment's feel.
func (s *ExampleTestSuite) TestThemesProduceDifferentEncounters() {
	gen := dungeon.NewGenerator(nil)

	// Use same parameters but different themes
	baseSeed := int64(12345)
	targetCR := 4.0
	roomCount := 3

	// Generate a crypt (undead theme)
	cryptOut, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCrypt,
		TargetCR:  targetCR,
		RoomCount: roomCount,
		Seed:      baseSeed,
	})
	s.Require().NoError(err)

	// Generate a cave (beast theme)
	caveOut, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCave,
		TargetCR:  targetCR,
		RoomCount: roomCount,
		Seed:      baseSeed,
	})
	s.Require().NoError(err)

	// Generate a bandit lair (humanoid theme)
	banditOut, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeBanditLair,
		TargetCR:  targetCR,
		RoomCount: roomCount,
		Seed:      baseSeed,
	})
	s.Require().NoError(err)

	// Each theme uses different monster pools, so even with the same seed,
	// the actual monsters will be theme-appropriate
	s.T().Log("Themes configure different monster pools:")
	s.T().Logf("  - Crypt uses undead: skeletons, zombies, ghouls")
	s.T().Logf("  - Cave uses beasts: giant rats, spiders, wolves")
	s.T().Logf("  - Bandit Lair uses humanoids: bandits, thugs, goblins")

	// All dungeons should have encounters, but from different pools
	for _, out := range []*dungeon.GenerateOutput{cryptOut, caveOut, banditOut} {
		for roomID, room := range out.Dungeon.Rooms() {
			if room.Encounter != nil && len(room.Encounter.Monsters) > 0 {
				s.T().Logf("Room %s has %d monsters", roomID, len(room.Encounter.Monsters))
			}
		}
	}

	// Verify themes affect wall materials
	s.Assert().Equal(dungeon.WallMaterialStone, dungeon.ThemeCrypt.WallMaterial,
		"Crypt has stone walls")
	s.Assert().Equal(dungeon.WallMaterialRock, dungeon.ThemeCave.WallMaterial,
		"Cave has rough rock walls")
	s.Assert().Equal(dungeon.WallMaterialWood, dungeon.ThemeBanditLair.WallMaterial,
		"Bandit lair has wooden walls")
}

// TestAccessingDungeonData demonstrates how to read dungeon data after
// generation. The Dungeon object provides methods to access rooms,
// encounters, connections, and state.
func (s *ExampleTestSuite) TestAccessingDungeonData() {
	gen := dungeon.NewGenerator(nil)

	output, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCrypt,
		TargetCR:  6.0,
		RoomCount: 4,
		Seed:      99999,
	})
	s.Require().NoError(err)

	d := output.Dungeon

	// --- Room Access ---
	// Get all room IDs
	roomIDs := d.RoomIDs()
	s.Assert().Len(roomIDs, 4, "Should have 4 rooms")

	// Get a specific room by ID
	startRoom := d.Room(d.StartRoom())
	s.Assert().NotNil(startRoom, "Can access start room by ID")
	s.Assert().Equal(dungeon.RoomTypeEntrance, startRoom.Type, "Start room is entrance type")

	// Get the current room (initially the start room)
	current := d.CurrentRoom()
	s.Assert().Equal(startRoom, current, "Current room starts as entrance")

	// --- Boss Room ---
	// The boss room is always the last room in generation order
	bossRoom := d.Room(d.BossRoom())
	s.Assert().NotNil(bossRoom, "Boss room exists")
	s.Assert().Equal(dungeon.RoomTypeBoss, bossRoom.Type, "Boss room has boss type")
	s.Assert().True(d.IsBossRoom(d.BossRoom()), "IsBossRoom returns true for boss room")

	// --- Encounters ---
	// Each room may have an encounter with monsters
	roomWithMonsters := 0
	totalMonsters := 0
	for _, room := range d.Rooms() {
		if room.Encounter != nil && len(room.Encounter.Monsters) > 0 {
			roomWithMonsters++
			totalMonsters += len(room.Encounter.Monsters)
		}
	}
	s.Assert().Greater(roomWithMonsters, 0, "At least one room has monsters")
	s.T().Logf("Found %d total monsters across %d rooms", totalMonsters, roomWithMonsters)

	// --- Doors/Connections ---
	// Doors connect rooms and can be queried
	doors := d.Doors()
	s.Assert().NotEmpty(doors, "Dungeon has doors connecting rooms")
	s.T().Logf("Dungeon has %d door connections", len(doors))

	// Get doors from a specific room
	doorsFromStart := d.DoorsFromRoom(d.StartRoom())
	s.Assert().NotEmpty(doorsFromStart, "Start room has at least one exit")

	// --- Visibility State ---
	// Rooms start unrevealed except for the start room
	s.Assert().True(d.RoomRevealed(d.StartRoom()), "Start room is revealed")

	// Doors start closed
	if len(doors) > 0 {
		s.Assert().False(d.DoorOpen(doors[0].ID), "Doors start closed")
	}

	// --- Persistence ---
	// Convert to data format for saving
	data := d.ToData()
	s.Assert().NotNil(data, "ToData returns persistence format")
	s.Assert().Equal(d.ID(), data.Environment.ID, "Data preserves dungeon ID")
}

// TestBossRoomEncounter verifies that the boss room contains a boss monster.
// This is a key D&D 5e design pattern: dungeons culminate in a boss fight.
func (s *ExampleTestSuite) TestBossRoomEncounter() {
	gen := dungeon.NewGenerator(nil)

	output, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCrypt,
		TargetCR:  8.0, // Higher CR gives more budget for boss
		RoomCount: 5,
		Seed:      777,
	})
	s.Require().NoError(err)

	d := output.Dungeon
	bossRoom := d.Room(d.BossRoom())
	s.Require().NotNil(bossRoom, "Boss room exists")

	// The boss room should have an encounter
	s.Assert().NotNil(bossRoom.Encounter, "Boss room has an encounter")

	// Check for a boss-role monster
	hasBoss := false
	if bossRoom.Encounter != nil {
		for _, monster := range bossRoom.Encounter.Monsters {
			if monster.Role == dungeon.RoleBoss {
				hasBoss = true
				s.T().Logf("Boss monster: %s (CR %.2f)", monster.MonsterID, monster.CR)
				break
			}
		}
	}
	s.Assert().True(hasBoss, "Boss room encounter includes a boss monster")
}

// TestDifferentRoomCounts shows how room count affects dungeon layout.
// The generator automatically selects an appropriate layout pattern
// based on the number of rooms requested.
func (s *ExampleTestSuite) TestDifferentRoomCounts() {
	gen := dungeon.NewGenerator(nil)

	// Small dungeons (1-3 rooms) get linear layouts
	smallOut, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCrypt,
		TargetCR:  2.0,
		RoomCount: 3,
		Seed:      1,
	})
	s.Require().NoError(err)
	s.Assert().Len(smallOut.Dungeon.Rooms(), 3, "Small dungeon has 3 rooms")

	// Medium dungeons (4-7 rooms) get branching layouts
	mediumOut, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeCave,
		TargetCR:  5.0,
		RoomCount: 6,
		Seed:      2,
	})
	s.Require().NoError(err)
	s.Assert().Len(mediumOut.Dungeon.Rooms(), 6, "Medium dungeon has 6 rooms")

	// Larger dungeons (8-15 rooms) get organic layouts
	largeOut, err := gen.Generate(s.ctx, &dungeon.GenerateInput{
		Theme:     dungeon.ThemeBanditLair,
		TargetCR:  10.0,
		RoomCount: 10,
		Seed:      3,
	})
	s.Require().NoError(err)
	s.Assert().Len(largeOut.Dungeon.Rooms(), 10, "Large dungeon has 10 rooms")

	s.T().Log("Layout selection based on room count:")
	s.T().Log("  - 1-3 rooms: Linear (simple path)")
	s.T().Log("  - 4-7 rooms: Branching (decision points)")
	s.T().Log("  - 8-15 rooms: Organic (natural cave-like)")
	s.T().Log("  - 16+ rooms: Grid (structured dungeon)")
}
