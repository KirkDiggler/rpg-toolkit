package selectables

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

type BasicTableTestSuite struct {
	suite.Suite
	eventBus events.EventBus
	ctx      SelectionContext
	table    SelectionTable[string]
	config   BasicTableConfig
}

// SetupTest runs before EACH test function
func (s *BasicTableTestSuite) SetupTest() {
	// Create event bus for testing
	s.eventBus = events.NewBus()

	// Create test context with deterministic dice roller for predictable tests
	testRoller := NewTestRoller([]int{50}) // Predictable rolls
	s.ctx = NewSelectionContextWithRoller(testRoller)

	// Create table configuration
	s.config = BasicTableConfig{
		ID:       "test_table",
		EventBus: s.eventBus,
		Configuration: TableConfiguration{
			EnableEvents:    true,
			EnableDebugging: true,
			CacheWeights:    true,
			MinWeight:       1,
			MaxWeight:       1000,
		},
	}

	// Create fresh table for each test
	s.table = NewBasicTable[string](s.config)
}

// SetupSubTest runs before EACH s.Run()
func (s *BasicTableTestSuite) SetupSubTest() {
	// Reset table to clean state for each subtest
	s.table = NewBasicTable[string](s.config)

	// Reset context to clean state
	testRoller := NewTestRoller([]int{50})
	s.ctx = NewSelectionContextWithRoller(testRoller)
}

func TestBasicTableSuite(t *testing.T) {
	suite.Run(t, new(BasicTableTestSuite))
}

func (s *BasicTableTestSuite) TestTableCreation() {
	s.Run("creates table with default configuration", func() {
		table := NewBasicTable[string](BasicTableConfig{})

		s.Assert().NotNil(table)
		s.Assert().True(table.IsEmpty())
		s.Assert().Equal(0, table.Size())
	})

	s.Run("creates table with custom configuration", func() {
		config := BasicTableConfig{
			ID:       "custom_table",
			EventBus: s.eventBus,
			Configuration: TableConfiguration{
				EnableEvents: true,
				MinWeight:    5,
				MaxWeight:    100,
			},
		}

		table := NewBasicTable[string](config)

		s.Assert().NotNil(table)
		s.Assert().True(table.IsEmpty())
	})
}

func (s *BasicTableTestSuite) TestAddItems() {
	s.Run("adds single item successfully", func() {
		result := s.table.Add("sword", 10)

		s.Assert().Equal(s.table, result) // Should return self for chaining
		s.Assert().False(s.table.IsEmpty())
		s.Assert().Equal(1, s.table.Size())

		items := s.table.GetItems()
		s.Assert().Equal(10, items["sword"])
	})

	s.Run("adds multiple items successfully", func() {
		s.table.
			Add("sword", 10).
			Add("shield", 15).
			Add("potion", 25)

		s.Assert().Equal(3, s.table.Size())

		items := s.table.GetItems()
		s.Assert().Equal(10, items["sword"])
		s.Assert().Equal(15, items["shield"])
		s.Assert().Equal(25, items["potion"])
	})

	s.Run("updates existing item weight", func() {
		s.table.Add("sword", 10)
		s.table.Add("sword", 20) // Update weight

		s.Assert().Equal(1, s.table.Size())

		items := s.table.GetItems()
		s.Assert().Equal(20, items["sword"])
	})

	s.Run("enforces minimum weight", func() {
		// Configure table with min weight of 5
		config := s.config
		config.Configuration.MinWeight = 5
		table := NewBasicTable[string](config)

		table.Add("weak_item", 1) // Below minimum

		items := table.GetItems()
		s.Assert().Equal(5, items["weak_item"]) // Should be adjusted to minimum
	})

	s.Run("enforces maximum weight", func() {
		// Configure table with max weight of 50
		config := s.config
		config.Configuration.MaxWeight = 50
		table := NewBasicTable[string](config)

		table.Add("strong_item", 100) // Above maximum

		items := table.GetItems()
		s.Assert().Equal(50, items["strong_item"]) // Should be adjusted to maximum
	})
}

func (s *BasicTableTestSuite) TestSingleSelection() {
	s.Run("selects item from single-item table", func() {
		s.table.Add("only_item", 10)

		selected, err := s.table.Select(s.ctx)

		s.Require().NoError(err)
		s.Assert().Equal("only_item", selected)
	})

	s.Run("selects item from multi-item table", func() {
		s.table.
			Add("common", 70).
			Add("uncommon", 25).
			Add("rare", 5)

		// Use predictable dice roller
		testRoller := NewTestRoller([]int{50}) // Should select "common"
		ctx := s.ctx.SetDiceRoller(testRoller)

		selected, err := s.table.Select(ctx)

		s.Require().NoError(err)
		s.Assert().Contains([]string{"common", "uncommon", "rare"}, selected)
	})

	s.Run("fails on empty table", func() {
		selected, err := s.table.Select(s.ctx)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrEmptyTable)
		s.Assert().Equal("", selected) // Zero value for string
	})

	s.Run("fails without context", func() {
		s.table.Add("item", 10)

		selected, err := s.table.Select(nil)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrContextRequired)
		s.Assert().Equal("", selected)
	})

	s.Run("fails without dice roller", func() {
		s.table.Add("item", 10)

		// Create context without dice roller
		ctx := &BasicSelectionContext{
			values:     make(map[string]interface{}),
			diceRoller: nil,
		}

		selected, err := s.table.Select(ctx)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrDiceRollerRequired)
		s.Assert().Equal("", selected)
	})
}

func (s *BasicTableTestSuite) TestMultipleSelection() {
	s.Run("selects multiple items with replacement", func() {
		s.table.
			Add("sword", 50).
			Add("shield", 30).
			Add("potion", 20)

		selected, err := s.table.SelectMany(s.ctx, 5)

		s.Require().NoError(err)
		s.Assert().Len(selected, 5)

		// Verify all selected items are valid
		validItems := map[string]bool{"sword": true, "shield": true, "potion": true}
		for _, item := range selected {
			s.Assert().True(validItems[item], "Selected item %s should be valid", item)
		}
	})

	s.Run("allows duplicate selections", func() {
		s.table.Add("common_item", 100) // Only item with high weight

		selected, err := s.table.SelectMany(s.ctx, 3)

		s.Require().NoError(err)
		s.Assert().Len(selected, 3)

		// All selections should be the same item
		for _, item := range selected {
			s.Assert().Equal("common_item", item)
		}
	})

	s.Run("fails with invalid count", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectMany(s.ctx, 0)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrInvalidCount)
		s.Assert().Nil(selected)
	})

	s.Run("fails with negative count", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectMany(s.ctx, -1)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrInvalidCount)
		s.Assert().Nil(selected)
	})

	s.Run("fails on empty table", func() {
		selected, err := s.table.SelectMany(s.ctx, 3)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrEmptyTable)
		s.Assert().Nil(selected)
	})
}

func (s *BasicTableTestSuite) TestUniqueSelection() {
	s.Run("selects unique items without replacement", func() {
		s.table.
			Add("sword", 25).
			Add("shield", 25).
			Add("potion", 25).
			Add("scroll", 25)

		selected, err := s.table.SelectUnique(s.ctx, 3)

		s.Require().NoError(err)
		s.Assert().Len(selected, 3)

		// Verify all items are unique
		uniqueItems := make(map[string]bool)
		for _, item := range selected {
			s.Assert().False(uniqueItems[item], "Item %s should not be selected twice", item)
			uniqueItems[item] = true
		}
	})

	s.Run("selects all items when count equals table size", func() {
		s.table.
			Add("item1", 10).
			Add("item2", 20).
			Add("item3", 30)

		selected, err := s.table.SelectUnique(s.ctx, 3)

		s.Require().NoError(err)
		s.Assert().Len(selected, 3)

		// Should contain all items
		selectedSet := make(map[string]bool)
		for _, item := range selected {
			selectedSet[item] = true
		}

		s.Assert().True(selectedSet["item1"])
		s.Assert().True(selectedSet["item2"])
		s.Assert().True(selectedSet["item3"])
	})

	s.Run("fails when requesting more items than available", func() {
		s.table.
			Add("item1", 10).
			Add("item2", 20)

		selected, err := s.table.SelectUnique(s.ctx, 5) // More than 2 available

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrInsufficientItems)
		s.Assert().Nil(selected)
	})

	s.Run("fails with invalid count", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectUnique(s.ctx, 0)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrInvalidCount)
		s.Assert().Nil(selected)
	})

	s.Run("fails on empty table", func() {
		selected, err := s.table.SelectUnique(s.ctx, 1)

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrEmptyTable)
		s.Assert().Nil(selected)
	})
}

func (s *BasicTableTestSuite) TestVariableSelection() {
	s.Run("selects items with dice expression quantity", func() {
		s.table.
			Add("sword", 50).
			Add("potion", 50)

		// Mock dice to return predictable values
		testRoller := NewTestRoller([]int{3}) // 1d4+1 could return 3
		ctx := s.ctx.SetDiceRoller(testRoller)

		selected, err := s.table.SelectVariable(ctx, "1d4")

		s.Require().NoError(err)
		s.Assert().NotEmpty(selected)
		s.Assert().LessOrEqual(len(selected), 4) // Maximum from 1d4
	})

	s.Run("handles minimum quantity of 1", func() {
		s.table.Add("item", 100)

		// Mock dice to return 0 (should be adjusted to 1)
		testRoller := NewTestRoller([]int{0})
		ctx := s.ctx.SetDiceRoller(testRoller)

		selected, err := s.table.SelectVariable(ctx, "1d1-1") // Could result in 0

		s.Require().NoError(err)
		s.Assert().GreaterOrEqual(len(selected), 1) // Should be at least 1
	})

	s.Run("fails with invalid dice expression", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectVariable(s.ctx, "invalid_expression")

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrInvalidDiceExpression)
		s.Assert().Nil(selected)
	})

	s.Run("fails without context", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectVariable(nil, "1d6")

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrContextRequired)
		s.Assert().Nil(selected)
	})

	s.Run("fails without dice roller", func() {
		s.table.Add("item", 10)

		// Create context without dice roller
		ctx := &BasicSelectionContext{
			values:     make(map[string]interface{}),
			diceRoller: nil,
		}

		selected, err := s.table.SelectVariable(ctx, "1d6")

		s.Assert().Error(err)
		s.Assert().ErrorIs(err, ErrDiceRollerRequired)
		s.Assert().Nil(selected)
	})
}

func (s *BasicTableTestSuite) TestAddNestedTable() {
	s.Run("adds nested table items to main table", func() {
		// Create nested table
		nestedTable := NewBasicTable[string](BasicTableConfig{ID: "nested"})
		nestedTable.
			Add("nested_item1", 30).
			Add("nested_item2", 70)

		// Add nested table to main table
		s.table.AddTable("nested_category", nestedTable, 50)

		s.Assert().Equal(2, s.table.Size()) // Should have 2 items from nested table

		items := s.table.GetItems()
		s.Assert().Contains(items, "nested_item1")
		s.Assert().Contains(items, "nested_item2")

		// Weights should be proportionally adjusted
		s.Assert().Greater(items["nested_item2"], items["nested_item1"]) // 70 > 30 proportion maintained
	})

	s.Run("combines nested table with existing items", func() {
		// Add some items to main table
		s.table.Add("main_item", 100)

		// Create and add nested table
		nestedTable := NewBasicTable[string](BasicTableConfig{ID: "nested"})
		nestedTable.Add("nested_item", 50)

		s.table.AddTable("nested", nestedTable, 25)

		s.Assert().Equal(2, s.table.Size())

		items := s.table.GetItems()
		s.Assert().Contains(items, "main_item")
		s.Assert().Contains(items, "nested_item")
	})

	s.Run("handles empty nested table", func() {
		emptyTable := NewBasicTable[string](BasicTableConfig{ID: "empty"})

		s.table.AddTable("empty_category", emptyTable, 50)

		s.Assert().Equal(0, s.table.Size()) // No items should be added
	})

	s.Run("enforces weight limits on nested items", func() {
		// Configure table with weight limits
		config := s.config
		config.Configuration.MinWeight = 10
		config.Configuration.MaxWeight = 20
		table := NewBasicTable[string](config)

		// Create nested table with extreme weights
		nestedTable := NewBasicTable[string](BasicTableConfig{ID: "nested"})
		nestedTable.
			Add("low_item", 1).    // Very low weight
			Add("high_item", 1000) // Very high weight

		table.AddTable("nested", nestedTable, 50)

		items := table.GetItems()

		// All weights should be within limits
		for _, weight := range items {
			s.Assert().GreaterOrEqual(weight, 10, "Weight should be at least minimum")
			s.Assert().LessOrEqual(weight, 20, "Weight should be at most maximum")
		}
	})
}

func (s *BasicTableTestSuite) TestTableIntrospection() {
	s.Run("GetItems returns correct item map", func() {
		s.table.
			Add("sword", 10).
			Add("shield", 15).
			Add("potion", 5)

		items := s.table.GetItems()

		s.Assert().Len(items, 3)
		s.Assert().Equal(10, items["sword"])
		s.Assert().Equal(15, items["shield"])
		s.Assert().Equal(5, items["potion"])
	})

	s.Run("GetItems returns copy not reference", func() {
		s.table.Add("item", 10)

		items1 := s.table.GetItems()
		items2 := s.table.GetItems()

		// Modify one copy
		items1["item"] = 999

		// Other copy should be unchanged
		s.Assert().Equal(10, items2["item"])

		// Original table should be unchanged
		originalItems := s.table.GetItems()
		s.Assert().Equal(10, originalItems["item"])
	})

	s.Run("IsEmpty works correctly", func() {
		s.Assert().True(s.table.IsEmpty())

		s.table.Add("item", 10)
		s.Assert().False(s.table.IsEmpty())
	})

	s.Run("Size works correctly", func() {
		s.Assert().Equal(0, s.table.Size())

		s.table.Add("item1", 10)
		s.Assert().Equal(1, s.table.Size())

		s.table.Add("item2", 20)
		s.Assert().Equal(2, s.table.Size())

		s.table.Add("item1", 30)            // Update existing
		s.Assert().Equal(2, s.table.Size()) // Size shouldn't change
	})
}
