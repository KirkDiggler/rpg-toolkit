// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// mockCharacterRegistry is a test implementation of CharacterRegistry
type mockCharacterRegistry struct {
	characters map[string]*gamectx.CharacterWeapons
}

func newMockCharacterRegistry() *mockCharacterRegistry {
	return &mockCharacterRegistry{
		characters: make(map[string]*gamectx.CharacterWeapons),
	}
}

func (m *mockCharacterRegistry) GetCharacterWeapons(id string) *gamectx.CharacterWeapons {
	return m.characters[id]
}

func (m *mockCharacterRegistry) addCharacter(id string, weapons *gamectx.CharacterWeapons) {
	m.characters[id] = weapons
}

// GameContextTestSuite tests GameContext creation and CharacterRegistry access
type GameContextTestSuite struct {
	suite.Suite
}

func (s *GameContextTestSuite) TestEmptyGameContext() {
	// Test that creating an empty GameContext has a valid CharacterRegistry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{})

	s.Require().NotNil(gameCtx)
	s.Require().NotNil(gameCtx.Characters())

	// Empty registry should return nil for any character lookup
	character := gameCtx.Characters().GetCharacterWeapons("any-id")
	s.Nil(character)
}

func (s *GameContextTestSuite) TestGameContextWithRegistry() {
	// Create a mock registry with a test character
	mockRegistry := newMockCharacterRegistry()
	longsword := &gamectx.EquippedWeapon{
		ID:   "longsword-1",
		Name: "Longsword",
		Slot: gamectx.SlotMainHand,
	}
	testWeapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{longsword})
	mockRegistry.addCharacter("hero-1", testWeapons)

	// Create GameContext with the registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry,
	})

	s.Require().NotNil(gameCtx)
	s.Require().NotNil(gameCtx.Characters())

	// Verify we can retrieve the character
	retrievedWeapons := gameCtx.Characters().GetCharacterWeapons("hero-1")
	s.Require().NotNil(retrievedWeapons)
	s.Equal("longsword-1", retrievedWeapons.MainHand().ID)

	// Verify non-existent character returns nil
	notFound := gameCtx.Characters().GetCharacterWeapons("not-found")
	s.Nil(notFound)
}

func TestGameContextSuite(t *testing.T) {
	suite.Run(t, new(GameContextTestSuite))
}

// ContextWrappingTestSuite tests context wrapping and retrieval functions
type ContextWrappingTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *ContextWrappingTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ContextWrappingTestSuite) TestWithGameContext() {
	// Create a GameContext with a mock registry
	mockRegistry := newMockCharacterRegistry()
	axe := &gamectx.EquippedWeapon{
		ID:   "greataxe-1",
		Name: "Greataxe",
		Slot: gamectx.SlotMainHand,
	}
	testWeapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{axe})
	mockRegistry.addCharacter("warrior-1", testWeapons)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry,
	})

	// Wrap the context
	wrappedCtx := gamectx.WithGameContext(s.ctx, gameCtx)
	s.Require().NotNil(wrappedCtx)

	// Verify the wrapped context is different from the original
	s.NotEqual(s.ctx, wrappedCtx)
}

func (s *ContextWrappingTestSuite) TestCharactersRetrievalSuccess() {
	// Create and wrap GameContext
	mockRegistry := newMockCharacterRegistry()
	staff := &gamectx.EquippedWeapon{
		ID:   "staff-1",
		Name: "Quarterstaff",
		Slot: gamectx.SlotMainHand,
	}
	testWeapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{staff})
	mockRegistry.addCharacter("mage-1", testWeapons)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry,
	})
	wrappedCtx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Retrieve the CharacterRegistry
	registry, ok := gamectx.Characters(wrappedCtx)
	s.Require().True(ok, "Expected to find CharacterRegistry in context")
	s.Require().NotNil(registry)

	// Verify we can use the registry
	retrievedWeapons := registry.GetCharacterWeapons("mage-1")
	s.Require().NotNil(retrievedWeapons)
	s.Equal("staff-1", retrievedWeapons.MainHand().ID)
}

func (s *ContextWrappingTestSuite) TestCharactersRetrievalNotFound() {
	// Use a context without GameContext
	registry, ok := gamectx.Characters(s.ctx)
	s.False(ok, "Expected not to find CharacterRegistry in plain context")
	s.Nil(registry)
}

func (s *ContextWrappingTestSuite) TestRequireCharactersSuccess() {
	// Create and wrap GameContext
	mockRegistry := newMockCharacterRegistry()
	dagger := &gamectx.EquippedWeapon{
		ID:   "dagger-1",
		Name: "Dagger",
		Slot: gamectx.SlotMainHand,
	}
	testWeapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{dagger})
	mockRegistry.addCharacter("rogue-1", testWeapons)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry,
	})
	wrappedCtx := gamectx.WithGameContext(s.ctx, gameCtx)

	// RequireCharacters should succeed
	registry := gamectx.RequireCharacters(wrappedCtx)
	s.Require().NotNil(registry)

	// Verify we can use the registry
	retrievedWeapons := registry.GetCharacterWeapons("rogue-1")
	s.Require().NotNil(retrievedWeapons)
	s.Equal("dagger-1", retrievedWeapons.MainHand().ID)
}

func (s *ContextWrappingTestSuite) TestRequireCharactersPanics() {
	// RequireCharacters should panic when no GameContext is present
	s.Require().Panics(func() {
		gamectx.RequireCharacters(s.ctx)
	}, "RequireCharacters should panic when no GameContext is in context")
}

func (s *ContextWrappingTestSuite) TestMultipleContextLayers() {
	// Test that we can wrap and re-wrap contexts
	mockRegistry1 := newMockCharacterRegistry()
	sword := &gamectx.EquippedWeapon{
		ID:   "sword-1",
		Name: "Shortsword",
		Slot: gamectx.SlotMainHand,
	}
	weapons1 := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{sword})
	mockRegistry1.addCharacter("char-1", weapons1)

	gameCtx1 := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry1,
	})
	wrappedCtx1 := gamectx.WithGameContext(s.ctx, gameCtx1)

	// Verify first wrapping works
	registry1, ok := gamectx.Characters(wrappedCtx1)
	s.Require().True(ok)
	s.Require().NotNil(registry1.GetCharacterWeapons("char-1"))
	s.Equal("sword-1", registry1.GetCharacterWeapons("char-1").MainHand().ID)

	// Re-wrap with a different GameContext
	mockRegistry2 := newMockCharacterRegistry()
	axe := &gamectx.EquippedWeapon{
		ID:   "axe-2",
		Name: "Handaxe",
		Slot: gamectx.SlotMainHand,
	}
	weapons2 := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{axe})
	mockRegistry2.addCharacter("char-2", weapons2)

	gameCtx2 := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry2,
	})
	wrappedCtx2 := gamectx.WithGameContext(wrappedCtx1, gameCtx2)

	// Verify the newer context takes precedence
	registry2, ok := gamectx.Characters(wrappedCtx2)
	s.Require().True(ok)
	s.Require().NotNil(registry2.GetCharacterWeapons("char-2"))
	s.Equal("axe-2", registry2.GetCharacterWeapons("char-2").MainHand().ID)
	s.Nil(registry2.GetCharacterWeapons("char-1"), "Should not find char-1 in new context")
}

func TestContextWrappingSuite(t *testing.T) {
	suite.Run(t, new(ContextWrappingTestSuite))
}

// RoomContextTestSuite tests Room context wrapping and retrieval functions
type RoomContextTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *RoomContextTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RoomContextTestSuite) TestWithRoom() {
	// Create a mock room (using spatial.BasicRoom)
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "combat",
		Grid: grid,
	})

	// Wrap the context with room
	wrappedCtx := gamectx.WithRoom(s.ctx, room)
	s.Require().NotNil(wrappedCtx)

	// Verify the wrapped context is different from the original
	s.NotEqual(s.ctx, wrappedCtx)
}

func (s *RoomContextTestSuite) TestRoomRetrievalSuccess() {
	// Create and place entities in a room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "combat",
		Grid: grid,
	})

	// Wrap context with room
	wrappedCtx := gamectx.WithRoom(s.ctx, room)

	// Retrieve the room
	retrievedRoom, ok := gamectx.Room(wrappedCtx)
	s.Require().True(ok, "Expected to find Room in context")
	s.Require().NotNil(retrievedRoom)
	s.Equal("test-room", retrievedRoom.GetID())
}

func (s *RoomContextTestSuite) TestRoomRetrievalNotFound() {
	// Use a context without Room
	room, ok := gamectx.Room(s.ctx)
	s.False(ok, "Expected not to find Room in plain context")
	s.Nil(room)
}

func (s *RoomContextTestSuite) TestRequireRoomSuccess() {
	// Create room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "required-room",
		Type: "combat",
		Grid: grid,
	})

	wrappedCtx := gamectx.WithRoom(s.ctx, room)

	// RequireRoom should succeed
	retrievedRoom := gamectx.RequireRoom(wrappedCtx)
	s.Require().NotNil(retrievedRoom)
	s.Equal("required-room", retrievedRoom.GetID())
}

func (s *RoomContextTestSuite) TestRequireRoomPanics() {
	// RequireRoom should panic when no Room is present
	s.Require().Panics(func() {
		gamectx.RequireRoom(s.ctx)
	}, "RequireRoom should panic when no Room is in context")
}

func TestRoomContextSuite(t *testing.T) {
	suite.Run(t, new(RoomContextTestSuite))
}

// AbilityScoresTestSuite tests AbilityScores and modifier calculations
type AbilityScoresTestSuite struct {
	suite.Suite
}

func (s *AbilityScoresTestSuite) TestModifierCalculation() {
	testCases := []struct {
		name     string
		score    int
		expected int
	}{
		{"score 1", 1, -5},
		{"score 8", 8, -1},
		{"score 10", 10, 0},
		{"score 11", 11, 0},
		{"score 12", 12, 1},
		{"score 14", 14, 2},
		{"score 16", 16, 3},
		{"score 18", 18, 4},
		{"score 20", 20, 5},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			scores := &gamectx.AbilityScores{
				Strength:     tc.score,
				Dexterity:    tc.score,
				Constitution: tc.score,
				Intelligence: tc.score,
				Wisdom:       tc.score,
				Charisma:     tc.score,
			}

			s.Equal(tc.expected, scores.StrengthMod())
			s.Equal(tc.expected, scores.DexterityMod())
			s.Equal(tc.expected, scores.ConstitutionMod())
			s.Equal(tc.expected, scores.IntelligenceMod())
			s.Equal(tc.expected, scores.WisdomMod())
			s.Equal(tc.expected, scores.CharismaMod())
		})
	}
}

func (s *AbilityScoresTestSuite) TestMixedScores() {
	scores := &gamectx.AbilityScores{
		Strength:     16, // +3
		Dexterity:    14, // +2
		Constitution: 12, // +1
		Intelligence: 10, // +0
		Wisdom:       8,  // -1
		Charisma:     6,  // -2
	}

	s.Equal(3, scores.StrengthMod())
	s.Equal(2, scores.DexterityMod())
	s.Equal(1, scores.ConstitutionMod())
	s.Equal(0, scores.IntelligenceMod())
	s.Equal(-1, scores.WisdomMod())
	s.Equal(-2, scores.CharismaMod())
}

func TestAbilityScoresSuite(t *testing.T) {
	suite.Run(t, new(AbilityScoresTestSuite))
}

// CharacterRegistryAbilityScoresTestSuite tests ability scores in CharacterRegistry
type CharacterRegistryAbilityScoresTestSuite struct {
	suite.Suite
}

func (s *CharacterRegistryAbilityScoresTestSuite) TestGetCharacterAbilityScores() {
	registry := gamectx.NewBasicCharacterRegistry()

	// Add a character with ability scores
	scores := &gamectx.AbilityScores{
		Strength:     16,
		Dexterity:    14,
		Constitution: 12,
		Intelligence: 10,
		Wisdom:       13,
		Charisma:     8,
	}
	registry.AddAbilityScores("hero-1", scores)

	// Retrieve and verify
	retrieved := registry.GetCharacterAbilityScores("hero-1")
	s.Require().NotNil(retrieved)
	s.Equal(16, retrieved.Strength)
	s.Equal(3, retrieved.StrengthMod())
	s.Equal(14, retrieved.Dexterity)
	s.Equal(2, retrieved.DexterityMod())
}

func (s *CharacterRegistryAbilityScoresTestSuite) TestGetCharacterAbilityScoresNotFound() {
	registry := gamectx.NewBasicCharacterRegistry()

	// Should return nil for non-existent character
	scores := registry.GetCharacterAbilityScores("not-found")
	s.Nil(scores)
}

func TestCharacterRegistryAbilityScoresSuite(t *testing.T) {
	suite.Run(t, new(CharacterRegistryAbilityScoresTestSuite))
}

// IntegrationTestSuite tests that features can query all game context data
type IntegrationTestSuite struct {
	suite.Suite
	ctx      context.Context
	room     spatial.Room
	registry *gamectx.BasicCharacterRegistry
}

func (s *IntegrationTestSuite) SetupTest() {
	s.ctx = context.Background()

	// Create a room with a grid
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "combat-room",
		Type: "combat",
		Grid: grid,
	})

	// Create registry with character data
	s.registry = gamectx.NewBasicCharacterRegistry()
}

// mockEntity implements core.Entity for testing
type mockEntity struct {
	id         string
	entityType core.EntityType
}

func (m *mockEntity) GetID() string            { return m.id }
func (m *mockEntity) GetType() core.EntityType { return m.entityType }

func (s *IntegrationTestSuite) TestFeatureCanQueryAllyPositions() {
	// Scenario: Sneak Attack needs to check if an ally is within 5ft of target

	// Place rogue at position (2, 2)
	rogue := &mockEntity{id: "rogue-1", entityType: core.EntityType("character")}
	err := s.room.PlaceEntity(rogue, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Place target goblin at position (5, 5)
	goblin := &mockEntity{id: "goblin-1", entityType: core.EntityType("monster")}
	err = s.room.PlaceEntity(goblin, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)

	// Place ally fighter at position (5, 6) - within 5ft of goblin
	fighter := &mockEntity{id: "fighter-1", entityType: core.EntityType("character")}
	err = s.room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 6})
	s.Require().NoError(err)

	// Set up context with room
	ctx := gamectx.WithRoom(s.ctx, s.room)

	// Feature code: Get room from context and check ally positions
	room, ok := gamectx.Room(ctx)
	s.Require().True(ok, "Room should be accessible from context")

	// Get target position
	targetPos, found := room.GetEntityPosition("goblin-1")
	s.Require().True(found, "Target should be in room")

	// Query entities within 5ft (1 square in D&D 5e grid) of target
	// Using radius 1.5 to catch adjacent squares
	entitiesNearTarget := room.GetEntitiesInRange(targetPos, 1.5)

	// Should find fighter (adjacent) but not rogue (too far)
	var allyFound bool
	for _, entity := range entitiesNearTarget {
		if entity.GetID() == "fighter-1" {
			allyFound = true
			break
		}
	}
	s.True(allyFound, "Fighter should be within range of goblin")

	// Verify rogue is NOT in range
	var rogueInRange bool
	for _, entity := range entitiesNearTarget {
		if entity.GetID() == "rogue-1" {
			rogueInRange = true
			break
		}
	}
	s.False(rogueInRange, "Rogue should NOT be within range of goblin")
}

func (s *IntegrationTestSuite) TestFeatureCanQueryAbilityModifiers() {
	// Scenario: Two-Weapon Fighting needs to get character's ability modifier

	// Set up ability scores for fighter (DEX 16 = +3)
	scores := &gamectx.AbilityScores{
		Strength:     14,
		Dexterity:    16,
		Constitution: 12,
		Intelligence: 10,
		Wisdom:       10,
		Charisma:     8,
	}
	s.registry.AddAbilityScores("fighter-1", scores)

	// Set up game context
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: s.registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Feature code: Get ability modifier from context
	registry, ok := gamectx.Characters(ctx)
	s.Require().True(ok, "Registry should be accessible")

	// BasicCharacterRegistry implements GetCharacterAbilityScores
	basicRegistry := registry.(*gamectx.BasicCharacterRegistry)
	abilityScores := basicRegistry.GetCharacterAbilityScores("fighter-1")
	s.Require().NotNil(abilityScores, "Ability scores should be found")

	// Verify DEX modifier is +3
	s.Equal(3, abilityScores.DexterityMod(), "DEX 16 should give +3 modifier")
}

func (s *IntegrationTestSuite) TestCombinedContextAccess() {
	// Test that Room and GameContext can both be accessed from the same context

	// Set up room with entity
	hero := &mockEntity{id: "hero-1", entityType: core.EntityType("character")}
	err := s.room.PlaceEntity(hero, spatial.Position{X: 3, Y: 3})
	s.Require().NoError(err)

	// Set up ability scores
	scores := &gamectx.AbilityScores{Dexterity: 14}
	s.registry.AddAbilityScores("hero-1", scores)

	// Create combined context with both Room and GameContext
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: s.registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, s.room)

	// Access room
	room, ok := gamectx.Room(ctx)
	s.Require().True(ok)
	pos, found := room.GetEntityPosition("hero-1")
	s.True(found)
	s.Equal(float64(3), pos.X)

	// Access ability scores
	registry, ok := gamectx.Characters(ctx)
	s.Require().True(ok)
	basicRegistry := registry.(*gamectx.BasicCharacterRegistry)
	abilityScores := basicRegistry.GetCharacterAbilityScores("hero-1")
	s.Equal(2, abilityScores.DexterityMod())
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
