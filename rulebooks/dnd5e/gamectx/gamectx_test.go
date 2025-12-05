// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
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
