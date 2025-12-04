// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/gamectx"
)

// mockCharacterRegistry is a test implementation of CharacterRegistry
type mockCharacterRegistry struct {
	characters map[string]interface{}
}

func newMockCharacterRegistry() *mockCharacterRegistry {
	return &mockCharacterRegistry{
		characters: make(map[string]interface{}),
	}
}

func (m *mockCharacterRegistry) GetCharacter(id string) interface{} {
	return m.characters[id]
}

func (m *mockCharacterRegistry) addCharacter(id string, character interface{}) {
	m.characters[id] = character
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
	character := gameCtx.Characters().GetCharacter("any-id")
	s.Nil(character)
}

func (s *GameContextTestSuite) TestGameContextWithRegistry() {
	// Create a mock registry with a test character
	mockRegistry := newMockCharacterRegistry()
	testCharacter := map[string]string{"name": "Hero"}
	mockRegistry.addCharacter("hero-1", testCharacter)

	// Create GameContext with the registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry,
	})

	s.Require().NotNil(gameCtx)
	s.Require().NotNil(gameCtx.Characters())

	// Verify we can retrieve the character
	retrievedChar := gameCtx.Characters().GetCharacter("hero-1")
	s.Equal(testCharacter, retrievedChar)

	// Verify non-existent character returns nil
	notFound := gameCtx.Characters().GetCharacter("not-found")
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
	testCharacter := map[string]string{"name": "Warrior"}
	mockRegistry.addCharacter("warrior-1", testCharacter)

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
	testCharacter := map[string]string{"name": "Mage"}
	mockRegistry.addCharacter("mage-1", testCharacter)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry,
	})
	wrappedCtx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Retrieve the CharacterRegistry
	registry, ok := gamectx.Characters(wrappedCtx)
	s.Require().True(ok, "Expected to find CharacterRegistry in context")
	s.Require().NotNil(registry)

	// Verify we can use the registry
	retrievedChar := registry.GetCharacter("mage-1")
	s.Equal(testCharacter, retrievedChar)
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
	testCharacter := map[string]string{"name": "Rogue"}
	mockRegistry.addCharacter("rogue-1", testCharacter)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry,
	})
	wrappedCtx := gamectx.WithGameContext(s.ctx, gameCtx)

	// RequireCharacters should succeed
	registry := gamectx.RequireCharacters(wrappedCtx)
	s.Require().NotNil(registry)

	// Verify we can use the registry
	retrievedChar := registry.GetCharacter("rogue-1")
	s.Equal(testCharacter, retrievedChar)
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
	mockRegistry1.addCharacter("char-1", "first")

	gameCtx1 := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry1,
	})
	wrappedCtx1 := gamectx.WithGameContext(s.ctx, gameCtx1)

	// Verify first wrapping works
	registry1, ok := gamectx.Characters(wrappedCtx1)
	s.Require().True(ok)
	s.Equal("first", registry1.GetCharacter("char-1"))

	// Re-wrap with a different GameContext
	mockRegistry2 := newMockCharacterRegistry()
	mockRegistry2.addCharacter("char-2", "second")

	gameCtx2 := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: mockRegistry2,
	})
	wrappedCtx2 := gamectx.WithGameContext(wrappedCtx1, gameCtx2)

	// Verify the newer context takes precedence
	registry2, ok := gamectx.Characters(wrappedCtx2)
	s.Require().True(ok)
	s.Equal("second", registry2.GetCharacter("char-2"))
	s.Nil(registry2.GetCharacter("char-1"), "Should not find char-1 in new context")
}

func TestContextWrappingSuite(t *testing.T) {
	suite.Run(t, new(ContextWrappingTestSuite))
}
