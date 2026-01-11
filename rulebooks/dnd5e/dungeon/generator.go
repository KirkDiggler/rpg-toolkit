package dungeon

import (
	"context"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// GeneratorConfig provides dependency injection for the Generator.
// If fields are nil, defaults are used.
type GeneratorConfig struct {
	// EnvironmentGenerator provides spatial layout generation.
	// If nil, a default GraphBasedGenerator is created.
	EnvironmentGenerator environments.EnvironmentGenerator
}

// Generator orchestrates dungeon creation by combining spatial layout
// generation with D&D 5e encounter generation.
type Generator struct {
	envGenerator environments.EnvironmentGenerator
}

// NewGenerator creates a new dungeon generator with the provided configuration.
// If config is nil or its fields are nil, sensible defaults are used.
func NewGenerator(config *GeneratorConfig) *Generator {
	g := &Generator{}

	if config != nil && config.EnvironmentGenerator != nil {
		g.envGenerator = config.EnvironmentGenerator
	} else {
		// Create default environment generator
		g.envGenerator = environments.NewGraphBasedGenerator(environments.GraphBasedGeneratorConfig{
			ID:   "dungeon_generator",
			Type: "dungeon",
		})
	}

	return g
}

// GenerateInput configures dungeon generation.
type GenerateInput struct {
	// Required fields

	// Theme configures monster pools, wall materials, and visual style.
	Theme Theme
	// TargetCR is the total challenge rating budget for the dungeon.
	TargetCR float64
	// RoomCount is the number of rooms to generate.
	RoomCount int

	// Optional fields

	// Seed for reproducible generation. 0 means use random seed.
	Seed int64
	// Layout specifies the room arrangement pattern.
	// Default: auto-selected based on room count.
	Layout environments.LayoutType
}

// GenerateOutput contains the result of dungeon generation.
type GenerateOutput struct {
	// Dungeon is the runtime object, ready for gameplay.
	Dungeon *Dungeon
	// Seed is the actual seed used (for reproducibility).
	Seed int64
}

// Generate creates a complete dungeon from the provided configuration.
// It orchestrates spatial layout generation, CR budget allocation, and
// encounter generation for each room.
func (g *Generator) Generate(ctx context.Context, input *GenerateInput) (*GenerateOutput, error) {
	// 1. Validate input
	if err := g.validateInput(input); err != nil {
		return nil, err
	}

	// 2. Generate or use provided seed
	seed := input.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	// 3. Determine layout type if not specified
	layout := input.Layout
	if layout == 0 {
		layout = g.selectLayoutForRoomCount(input.RoomCount)
	}

	// 4. Generate spatial layout using environments package
	envConfig := environments.GenerationConfig{
		ID:        fmt.Sprintf("dungeon_%d", seed),
		Type:      environments.GenerationTypeGraph,
		Seed:      seed,
		Theme:     input.Theme.ID,
		RoomCount: input.RoomCount,
		Layout:    layout,
	}

	env, err := g.envGenerator.Generate(ctx, envConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate environment: %w", err)
	}

	// 5. Get room IDs and identify boss room (last room)
	roomIDs := g.extractRoomIDs(env)
	if len(roomIDs) == 0 {
		return nil, fmt.Errorf("environment generated with no rooms")
	}

	bossRoomID := roomIDs[len(roomIDs)-1]
	startRoomID := roomIDs[0]

	// 6. Allocate CR budget across rooms
	allocation := allocateBudget(&allocateBudgetInput{
		RoomIDs:        roomIDs,
		BossRoomID:     bossRoomID,
		TargetCR:       input.TargetCR,
		DifficultyRamp: true,
	})

	// 7. Generate encounters for each room
	rooms := make(map[string]*RoomData)
	for i, roomID := range roomIDs {
		budget := allocation.RoomBudgets[roomID]
		isBossRoom := roomID == bossRoomID

		// Select room type based on position
		roomType := g.selectRoomType(i, len(roomIDs), isBossRoom)

		// Generate encounter with per-room seed for reproducibility
		roomSeed := seed + int64(i)
		encounter := generateEncounter(&generateEncounterInput{
			Budget:      budget,
			MonsterPool: input.Theme.MonsterPool,
			IsBossRoom:  isBossRoom,
			BossPool:    input.Theme.BossPool,
			Seed:        roomSeed,
		})

		rooms[roomID] = &RoomData{
			Type:      roomType,
			Encounter: encounter,
			Features:  FeatureData{},
		}
	}

	// 8. Build DungeonData from environment and encounters
	dungeonData := g.buildDungeonData(env, rooms, startRoomID, bossRoomID, seed)

	// 9. Load into runtime Dungeon
	loadOutput, err := LoadFromData(&LoadFromDataInput{
		Data: dungeonData,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load dungeon from data: %w", err)
	}

	return &GenerateOutput{
		Dungeon: loadOutput.Dungeon,
		Seed:    seed,
	}, nil
}

// validateInput checks that all required fields are present and valid.
func (g *Generator) validateInput(input *GenerateInput) error {
	if input == nil {
		return ErrNilInput
	}

	if input.Theme.ID == "" {
		return ErrInvalidTheme
	}

	if input.TargetCR <= 0 {
		return ErrInvalidCR
	}

	if input.RoomCount < 1 {
		return ErrInvalidRoomCount
	}

	return nil
}

// selectLayoutForRoomCount chooses an appropriate layout based on room count.
func (g *Generator) selectLayoutForRoomCount(roomCount int) environments.LayoutType {
	switch {
	case roomCount <= 3:
		return environments.LayoutTypeLinear
	case roomCount <= 7:
		return environments.LayoutTypeBranching
	case roomCount <= 15:
		return environments.LayoutTypeOrganic
	default:
		return environments.LayoutTypeGrid
	}
}

// extractRoomIDs gets all room IDs from the environment in a consistent order.
func (g *Generator) extractRoomIDs(env environments.Environment) []string {
	rooms := env.GetRooms()
	roomIDs := make([]string, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.GetID())
	}

	// Sort for deterministic ordering - first room is entrance, last is boss
	// The environment generator creates rooms with IDs that sort naturally
	// (e.g., room_0, room_1, room_2 for linear layouts)
	sortRoomIDs(roomIDs)

	return roomIDs
}

// sortRoomIDs sorts room IDs to establish entrance-to-boss ordering.
// Rooms are typically named with numeric suffixes that sort correctly.
func sortRoomIDs(roomIDs []string) {
	// Simple bubble sort - room counts are small
	n := len(roomIDs)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if roomIDs[j] > roomIDs[j+1] {
				roomIDs[j], roomIDs[j+1] = roomIDs[j+1], roomIDs[j]
			}
		}
	}
}

// selectRoomType determines the room type based on position in the dungeon.
func (g *Generator) selectRoomType(index, total int, isBossRoom bool) RoomType {
	if isBossRoom {
		return RoomTypeBoss
	}
	if index == 0 {
		return RoomTypeEntrance
	}
	if index == total-2 && total > 2 {
		// Room before boss is often a chamber
		return RoomTypeChamber
	}
	// Alternate between chambers and corridors for variety
	if index%2 == 0 {
		return RoomTypeChamber
	}
	return RoomTypeCorridor
}

// buildDungeonData creates the persistence structure from generation results.
func (g *Generator) buildDungeonData(
	env environments.Environment,
	rooms map[string]*RoomData,
	startRoomID, bossRoomID string,
	seed int64,
) *DungeonData {
	// Get environment data using ToData method
	basicEnv, ok := env.(*environments.BasicEnvironment)
	var envData environments.EnvironmentData
	if ok {
		envData = basicEnv.ToData()
	} else {
		// Fallback: build minimal environment data
		envData = g.buildMinimalEnvironmentData(env)
	}

	return &DungeonData{
		Environment:   envData,
		StartRoomID:   startRoomID,
		BossRoomID:    bossRoomID,
		Seed:          seed,
		Rooms:         rooms,
		State:         StateActive,
		CurrentRoomID: startRoomID,
		RevealedRooms: map[string]bool{startRoomID: true},
		OpenDoors:     make(map[string]bool),
		RoomsCleared:  0,
		CreatedAt:     time.Now(),
	}
}

// buildMinimalEnvironmentData creates EnvironmentData when ToData is not available.
func (g *Generator) buildMinimalEnvironmentData(env environments.Environment) environments.EnvironmentData {
	data := environments.EnvironmentData{
		ID:       env.GetID(),
		Type:     environments.EnvironmentType(env.GetType()),
		Theme:    env.GetTheme(),
		Metadata: env.GetMetadata(),
		Zones:    make([]environments.ZoneData, 0),
		Passages: make([]environments.PassageData, 0),
	}

	// Convert rooms to zones
	for _, room := range env.GetRooms() {
		roomPos, _ := env.GetRoomPosition(room.GetID())
		grid := room.GetGrid()

		zone := environments.ZoneData{
			ID:        room.GetID(),
			Type:      string(room.GetType()),
			Origin:    roomPos,
			GridShape: environments.GridShapeHex,
		}

		if grid != nil {
			dims := grid.GetDimensions()
			zone.Width = int(dims.Width)
			zone.Height = int(dims.Height)
		} else {
			zone.Width = 10
			zone.Height = 10
		}

		data.Zones = append(data.Zones, zone)
	}

	// Convert connections to passages
	for _, conn := range env.GetConnections() {
		passage := environments.PassageData{
			ID:            conn.GetID(),
			FromZoneID:    conn.GetFromRoom(),
			ToZoneID:      conn.GetToRoom(),
			Bidirectional: conn.IsReversible(),
		}
		data.Passages = append(data.Passages, passage)
	}

	return data
}

// GetRoomPositions returns room positions from the environment for debugging.
func (g *Generator) GetRoomPositions(env environments.Environment) map[string]spatial.CubeCoordinate {
	positions := make(map[string]spatial.CubeCoordinate)
	for _, room := range env.GetRooms() {
		pos, found := env.GetRoomPosition(room.GetID())
		if found {
			positions[room.GetID()] = pos
		}
	}
	return positions
}
