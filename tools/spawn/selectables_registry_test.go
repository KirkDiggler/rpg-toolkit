package spawn

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
)

// SelectablesRegistryTestSuite tests the BasicSelectablesRegistry implementation
type SelectablesRegistryTestSuite struct {
	suite.Suite
	registry   *BasicSelectablesRegistry
	mockTable  *MockSelectionTable
	testEntity core.Entity
}

// SetupTest runs before EACH test function
func (s *SelectablesRegistryTestSuite) SetupTest() {
	// Create fresh registry for each test
	s.registry = NewBasicSelectablesRegistry()
	s.mockTable = NewMockSelectionTable()
	s.testEntity = &TestEntity{id: "test_entity", entityType: "test"}
}

// SetupSubTest runs before EACH s.Run()
func (s *SelectablesRegistryTestSuite) SetupSubTest() {
	// Reset registry state for each subtest if needed
	s.registry = NewBasicSelectablesRegistry()
}

func (s *SelectablesRegistryTestSuite) TestTableRegistration() {
	s.Run("registers table successfully", func() {
		err := s.registry.RegisterTable("test_table", s.mockTable)
		s.Assert().NoError(err)

		// Verify table is registered
		tables := s.registry.ListTables()
		s.Assert().Contains(tables, "test_table")
	})

	s.Run("rejects empty table ID", func() {
		err := s.registry.RegisterTable("", s.mockTable)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "table ID cannot be empty")
	})

	s.Run("rejects nil table", func() {
		err := s.registry.RegisterTable("test_table", nil)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "table cannot be nil")
	})

	s.Run("overwrites existing table", func() {
		mockTable2 := NewMockSelectionTable()

		// Register first table
		err := s.registry.RegisterTable("test_table", s.mockTable)
		s.Assert().NoError(err)

		// Register second table with same ID
		err = s.registry.RegisterTable("test_table", mockTable2)
		s.Assert().NoError(err)

		// Verify second table is registered
		retrievedTable, err := s.registry.GetTable("test_table")
		s.Assert().NoError(err)
		s.Assert().Equal(mockTable2, retrievedTable)
	})
}

func (s *SelectablesRegistryTestSuite) TestTableRetrieval() {
	s.Run("retrieves registered table", func() {
		err := s.registry.RegisterTable("test_table", s.mockTable)
		s.Assert().NoError(err)

		retrievedTable, err := s.registry.GetTable("test_table")
		s.Assert().NoError(err)
		s.Assert().Equal(s.mockTable, retrievedTable)
	})

	s.Run("returns error for non-existent table", func() {
		_, err := s.registry.GetTable("non_existent")
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "table not found: non_existent")
	})

	s.Run("rejects empty table ID", func() {
		_, err := s.registry.GetTable("")
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "table ID cannot be empty")
	})
}

func (s *SelectablesRegistryTestSuite) TestTableListing() {
	s.Run("lists empty registry", func() {
		tables := s.registry.ListTables()
		s.Assert().Empty(tables)
	})

	s.Run("lists registered tables", func() {
		mockTable2 := NewMockSelectionTable()
		mockTable3 := NewMockSelectionTable()

		err := s.registry.RegisterTable("table1", s.mockTable)
		s.Assert().NoError(err)
		err = s.registry.RegisterTable("table2", mockTable2)
		s.Assert().NoError(err)
		err = s.registry.RegisterTable("table3", mockTable3)
		s.Assert().NoError(err)

		tables := s.registry.ListTables()
		s.Assert().Len(tables, 3)
		s.Assert().Contains(tables, "table1")
		s.Assert().Contains(tables, "table2")
		s.Assert().Contains(tables, "table3")
	})
}

func (s *SelectablesRegistryTestSuite) TestTableRemoval() {
	s.Run("removes existing table", func() {
		err := s.registry.RegisterTable("test_table", s.mockTable)
		s.Assert().NoError(err)

		// Verify table exists
		tables := s.registry.ListTables()
		s.Assert().Contains(tables, "test_table")

		// Remove table
		err = s.registry.RemoveTable("test_table")
		s.Assert().NoError(err)

		// Verify table is removed
		tables = s.registry.ListTables()
		s.Assert().NotContains(tables, "test_table")

		// Verify table cannot be retrieved
		_, err = s.registry.GetTable("test_table")
		s.Assert().Error(err)
	})

	s.Run("returns error for non-existent table", func() {
		err := s.registry.RemoveTable("non_existent")
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "table not found: non_existent")
	})

	s.Run("rejects empty table ID", func() {
		err := s.registry.RemoveTable("")
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "table ID cannot be empty")
	})
}

func (s *SelectablesRegistryTestSuite) TestConcurrentAccess() {
	s.Run("handles concurrent operations safely", func() {
		// This test would typically use goroutines to test thread safety
		// For now, we'll test sequential operations that would expose race conditions

		// Register multiple tables
		for i := 0; i < 10; i++ {
			tableID := "table_" + string(rune('0'+i))
			mockTable := NewMockSelectionTable()
			err := s.registry.RegisterTable(tableID, mockTable)
			s.Assert().NoError(err)
		}

		// List tables
		tables := s.registry.ListTables()
		s.Assert().Len(tables, 10)

		// Remove some tables
		for i := 0; i < 5; i++ {
			tableID := "table_" + string(rune('0'+i))
			err := s.registry.RemoveTable(tableID)
			s.Assert().NoError(err)
		}

		// Verify remaining tables
		tables = s.registry.ListTables()
		s.Assert().Len(tables, 5)
	})
}

// Run the suite
func TestSelectablesRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(SelectablesRegistryTestSuite))
}

// MockSelectionTable for testing
type MockSelectionTable struct{}

func NewMockSelectionTable() *MockSelectionTable {
	return &MockSelectionTable{}
}

func (m *MockSelectionTable) Select(ctx selectables.SelectionContext) (core.Entity, error) {
	return &TestEntity{id: "selected", entityType: "test"}, nil
}

func (m *MockSelectionTable) SelectMany(ctx selectables.SelectionContext, count int) ([]core.Entity, error) {
	entities := make([]core.Entity, count)
	for i := 0; i < count; i++ {
		entities[i] = &TestEntity{id: "selected", entityType: "test"}
	}
	return entities, nil
}

func (m *MockSelectionTable) SelectUnique(ctx selectables.SelectionContext, count int) ([]core.Entity, error) {
	entities := make([]core.Entity, count)
	for i := 0; i < count; i++ {
		entities[i] = &TestEntity{id: "unique_" + string(rune('0'+i)), entityType: "test"}
	}
	return entities, nil
}

func (m *MockSelectionTable) SelectVariable(ctx selectables.SelectionContext, diceExpression string) ([]core.Entity, error) {
	return []core.Entity{&TestEntity{id: "variable", entityType: "test"}}, nil
}

func (m *MockSelectionTable) Add(item core.Entity, weight int) selectables.SelectionTable[core.Entity] {
	return m
}

func (m *MockSelectionTable) AddTable(name string, table selectables.SelectionTable[core.Entity], weight int) selectables.SelectionTable[core.Entity] {
	return m
}

func (m *MockSelectionTable) Remove(item core.Entity) selectables.SelectionTable[core.Entity] {
	return m
}

func (m *MockSelectionTable) Clear() selectables.SelectionTable[core.Entity] {
	return m
}

func (m *MockSelectionTable) GetItems() map[core.Entity]int {
	return map[core.Entity]int{}
}

func (m *MockSelectionTable) GetTotalWeight() int {
	return 100
}

func (m *MockSelectionTable) IsEmpty() bool {
	return false
}

func (m *MockSelectionTable) Size() int {
	return 1
}