package selectables

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
)

type SelectionContextTestSuite struct {
	suite.Suite
	ctx    SelectionContext
	roller dice.Roller
}

// SetupTest runs before EACH test function
func (s *SelectionContextTestSuite) SetupTest() {
	s.roller = &dice.CryptoRoller{}
	s.ctx = NewBasicSelectionContext()
}

// SetupSubTest runs before EACH s.Run()
func (s *SelectionContextTestSuite) SetupSubTest() {
	// Reset context to clean state for each subtest
	s.ctx = NewBasicSelectionContext()
}

func TestSelectionContextSuite(t *testing.T) {
	suite.Run(t, new(SelectionContextTestSuite))
}

func (s *SelectionContextTestSuite) TestBasicContextCreation() {
	s.Run("creates context with default roller", func() {
		ctx := NewBasicSelectionContext()

		s.Assert().NotNil(ctx)
		s.Assert().NotNil(ctx.GetDiceRoller())
		s.Assert().Empty(ctx.Keys())
	})

	s.Run("creates context with custom roller", func() {
		testRoller := NewTestRoller([]int{42})
		ctx := NewSelectionContextWithRoller(testRoller)

		s.Assert().NotNil(ctx)
		s.Assert().Equal(testRoller, ctx.GetDiceRoller())
		s.Assert().Empty(ctx.Keys())
	})
}

func (s *SelectionContextTestSuite) TestContextValueOperations() {
	s.Run("sets and gets string values", func() {
		newCtx := s.ctx.Set("environment", "forest")

		value, exists := newCtx.Get("environment")
		s.Assert().True(exists)
		s.Assert().Equal("forest", value)
	})

	s.Run("sets and gets integer values", func() {
		newCtx := s.ctx.Set("player_level", 5)

		value, exists := newCtx.Get("player_level")
		s.Assert().True(exists)
		s.Assert().Equal(5, value)
	})

	s.Run("sets and gets boolean values", func() {
		newCtx := s.ctx.Set("is_night", true)

		value, exists := newCtx.Get("is_night")
		s.Assert().True(exists)
		s.Assert().Equal(true, value)
	})

	s.Run("sets and gets float values", func() {
		newCtx := s.ctx.Set("difficulty_multiplier", 1.5)

		value, exists := newCtx.Get("difficulty_multiplier")
		s.Assert().True(exists)
		s.Assert().Equal(1.5, value)
	})

	s.Run("returns false for non-existent keys", func() {
		value, exists := s.ctx.Get("non_existent_key")
		s.Assert().False(exists)
		s.Assert().Nil(value)
	})

	s.Run("overwrites existing values", func() {
		ctx1 := s.ctx.Set("key", "value1")
		ctx2 := ctx1.Set("key", "value2")

		value, exists := ctx2.Get("key")
		s.Assert().True(exists)
		s.Assert().Equal("value2", value)
	})
}

func (s *SelectionContextTestSuite) TestContextImmutability() {
	s.Run("setting values returns new context", func() {
		originalCtx := s.ctx
		newCtx := s.ctx.Set("key", "value")

		s.Assert().NotEqual(originalCtx, newCtx, "Should return new context instance")

		// Original context should be unchanged
		_, exists := originalCtx.Get("key")
		s.Assert().False(exists, "Original context should not have new value")

		// New context should have the value
		value, exists := newCtx.Get("key")
		s.Assert().True(exists)
		s.Assert().Equal("value", value)
	})

	s.Run("setting dice roller returns new context", func() {
		originalRoller := s.ctx.GetDiceRoller()
		newRoller := NewTestRoller([]int{42})

		newCtx := s.ctx.SetDiceRoller(newRoller)

		s.Assert().NotEqual(s.ctx, newCtx, "Should return new context instance")
		s.Assert().Equal(originalRoller, s.ctx.GetDiceRoller(), "Original roller unchanged")
		s.Assert().Equal(newRoller, newCtx.GetDiceRoller(), "New context has new roller")
	})

	s.Run("preserves existing values when setting new ones", func() {
		ctx1 := s.ctx.Set("key1", "value1")
		ctx2 := ctx1.Set("key2", "value2")

		// Both values should exist in ctx2
		value1, exists1 := ctx2.Get("key1")
		value2, exists2 := ctx2.Get("key2")

		s.Assert().True(exists1)
		s.Assert().Equal("value1", value1)
		s.Assert().True(exists2)
		s.Assert().Equal("value2", value2)

		// ctx1 should only have key1
		_, exists := ctx1.Get("key2")
		s.Assert().False(exists)
	})
}

func (s *SelectionContextTestSuite) TestContextKeys() {
	s.Run("returns empty slice for empty context", func() {
		keys := s.ctx.Keys()
		s.Assert().Empty(keys)
	})

	s.Run("returns all keys", func() {
		ctx := s.ctx.
			Set("key1", "value1").
			Set("key2", "value2").
			Set("key3", "value3")

		keys := ctx.Keys()
		s.Assert().Len(keys, 3)
		s.Assert().Contains(keys, "key1")
		s.Assert().Contains(keys, "key2")
		s.Assert().Contains(keys, "key3")
	})

	s.Run("keys list updates with context changes", func() {
		ctx1 := s.ctx.Set("key1", "value1")
		keys1 := ctx1.Keys()
		s.Assert().Len(keys1, 1)
		s.Assert().Contains(keys1, "key1")

		ctx2 := ctx1.Set("key2", "value2")
		keys2 := ctx2.Keys()
		s.Assert().Len(keys2, 2)
		s.Assert().Contains(keys2, "key1")
		s.Assert().Contains(keys2, "key2")
	})
}

func (s *SelectionContextTestSuite) TestDiceRollerOperations() {
	s.Run("preserves dice roller through value operations", func() {
		testRoller := NewTestRoller([]int{100})
		ctx := NewSelectionContextWithRoller(testRoller)

		newCtx := ctx.Set("key", "value")

		s.Assert().Equal(testRoller, newCtx.GetDiceRoller())
	})

	s.Run("preserves values through dice roller operations", func() {
		ctx1 := s.ctx.Set("key", "value")
		newRoller := NewTestRoller([]int{42})
		ctx2 := ctx1.SetDiceRoller(newRoller)

		value, exists := ctx2.Get("key")
		s.Assert().True(exists)
		s.Assert().Equal("value", value)
	})
}

func (s *SelectionContextTestSuite) TestContextBuilder() {
	s.Run("creates context with builder pattern", func() {
		ctx := NewContextBuilder().
			SetString("environment", "dungeon").
			SetInt("player_level", 10).
			SetBool("is_night", false).
			SetFloat("multiplier", 2.5).
			Build()

		env, _ := ctx.Get("environment")
		level, _ := ctx.Get("player_level")
		night, _ := ctx.Get("is_night")
		multiplier, _ := ctx.Get("multiplier")

		s.Assert().Equal("dungeon", env)
		s.Assert().Equal(10, level)
		s.Assert().Equal(false, night)
		s.Assert().Equal(2.5, multiplier)
	})

	s.Run("creates context with custom dice roller", func() {
		testRoller := NewTestRoller([]int{42})
		ctx := NewContextBuilderWithRoller(testRoller).
			SetString("key", "value").
			Build()

		s.Assert().Equal(testRoller, ctx.GetDiceRoller())

		value, exists := ctx.Get("key")
		s.Assert().True(exists)
		s.Assert().Equal("value", value)
	})

	s.Run("supports method chaining", func() {
		builder := NewContextBuilder()
		result := builder.
			SetString("str", "test").
			SetInt("int", 42).
			SetBool("bool", true).
			SetFloat("float", 3.14)

		s.Assert().Equal(builder, result, "Builder methods should return self for chaining")

		ctx := builder.Build()

		s.Assert().Len(ctx.Keys(), 4)
	})

	s.Run("with dice roller method chaining", func() {
		testRoller := NewTestRoller([]int{100})

		ctx := NewContextBuilder().
			SetString("key", "value").
			WithDiceRoller(testRoller).
			SetInt("number", 42).
			Build()

		s.Assert().Equal(testRoller, ctx.GetDiceRoller())
		s.Assert().Len(ctx.Keys(), 2)
	})
}

func (s *SelectionContextTestSuite) TestContextHelpers() {
	s.Run("GetStringValue with existing value", func() {
		ctx := s.ctx.Set("environment", "forest")

		value := GetStringValue(ctx, "environment", "default")
		s.Assert().Equal("forest", value)
	})

	s.Run("GetStringValue with default", func() {
		value := GetStringValue(s.ctx, "non_existent", "default_value")
		s.Assert().Equal("default_value", value)
	})

	s.Run("GetStringValue with wrong type", func() {
		ctx := s.ctx.Set("number", 42) // Not a string

		value := GetStringValue(ctx, "number", "default")
		s.Assert().Equal("default", value) // Should return default for wrong type
	})

	s.Run("GetIntValue with existing value", func() {
		ctx := s.ctx.Set("level", 15)

		value := GetIntValue(ctx, "level", 1)
		s.Assert().Equal(15, value)
	})

	s.Run("GetIntValue with default", func() {
		value := GetIntValue(s.ctx, "non_existent", 42)
		s.Assert().Equal(42, value)
	})

	s.Run("GetIntValue with wrong type", func() {
		ctx := s.ctx.Set("text", "not_a_number")

		value := GetIntValue(ctx, "text", 100)
		s.Assert().Equal(100, value) // Should return default for wrong type
	})

	s.Run("GetBoolValue with existing value", func() {
		ctx := s.ctx.Set("is_active", true)

		value := GetBoolValue(ctx, "is_active", false)
		s.Assert().True(value)
	})

	s.Run("GetBoolValue with default", func() {
		value := GetBoolValue(s.ctx, "non_existent", true)
		s.Assert().True(value)
	})

	s.Run("GetBoolValue with wrong type", func() {
		ctx := s.ctx.Set("number", 42)

		value := GetBoolValue(ctx, "number", false)
		s.Assert().False(value) // Should return default for wrong type
	})

	s.Run("GetFloatValue with existing value", func() {
		ctx := s.ctx.Set("multiplier", 2.5)

		value := GetFloatValue(ctx, "multiplier", 1.0)
		s.Assert().Equal(2.5, value)
	})

	s.Run("GetFloatValue with default", func() {
		value := GetFloatValue(s.ctx, "non_existent", 3.14)
		s.Assert().Equal(3.14, value)
	})

	s.Run("GetFloatValue with wrong type", func() {
		ctx := s.ctx.Set("text", "not_a_float")

		value := GetFloatValue(ctx, "text", 1.5)
		s.Assert().Equal(1.5, value) // Should return default for wrong type
	})
}

func (s *SelectionContextTestSuite) TestComplexContextOperations() {
	s.Run("handles complex nested operations", func() {
		// Build a complex context through multiple operations
		ctx := s.ctx.
			Set("player", map[string]interface{}{
				"level": 10,
				"class": "warrior",
			}).
			Set("environment", "dungeon").
			Set("party_size", 4).
			Set("difficulty", 1.5)

		// Verify all values are accessible
		player, exists := ctx.Get("player")
		s.Assert().True(exists)
		s.Assert().IsType(map[string]interface{}{}, player)

		env := GetStringValue(ctx, "environment", "")
		size := GetIntValue(ctx, "party_size", 0)
		diff := GetFloatValue(ctx, "difficulty", 0.0)

		s.Assert().Equal("dungeon", env)
		s.Assert().Equal(4, size)
		s.Assert().Equal(1.5, diff)

		s.Assert().Len(ctx.Keys(), 4)
	})

	s.Run("maintains context integrity through multiple changes", func() {
		// Start with base context
		ctx1 := s.ctx.Set("base", "value")

		// Create multiple derived contexts
		ctx2 := ctx1.Set("derived1", "value1")
		ctx3 := ctx1.Set("derived2", "value2")
		ctx4 := ctx2.Set("derived3", "value3")

		// Verify each context has correct values
		s.Assert().Len(ctx1.Keys(), 1)
		s.Assert().Len(ctx2.Keys(), 2)
		s.Assert().Len(ctx3.Keys(), 2)
		s.Assert().Len(ctx4.Keys(), 3)

		// All should have base value
		for _, ctx := range []SelectionContext{ctx1, ctx2, ctx3, ctx4} {
			value, exists := ctx.Get("base")
			s.Assert().True(exists)
			s.Assert().Equal("value", value)
		}

		// ctx3 should not have derived1 or derived3
		_, exists := ctx3.Get("derived1")
		s.Assert().False(exists)
		_, exists = ctx3.Get("derived3")
		s.Assert().False(exists)
	})
}
