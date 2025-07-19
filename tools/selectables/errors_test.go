package selectables

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ErrorHandlingTestSuite struct {
	suite.Suite
	ctx   SelectionContext
	table SelectionTable[string]
}

// SetupTest runs before EACH test function
func (s *ErrorHandlingTestSuite) SetupTest() {
	// Create context with test roller
	testRoller := NewTestRoller([]int{50})
	s.ctx = NewSelectionContextWithRoller(testRoller)

	// Create basic table for testing
	s.table = NewBasicTable[string](BasicTableConfig{
		ID: "error_test_table",
	})
}

// SetupSubTest runs before EACH s.Run()
func (s *ErrorHandlingTestSuite) SetupSubTest() {
	// Reset table for each subtest
	s.table = NewBasicTable[string](BasicTableConfig{
		ID: "error_test_table",
	})
}

func TestErrorHandlingSuite(t *testing.T) {
	suite.Run(t, new(ErrorHandlingTestSuite))
}

func (s *ErrorHandlingTestSuite) TestSelectionError() {
	s.Run("creates selection error with all fields", func() {
		ctx := s.ctx.Set("test_key", "test_value")
		baseErr := errors.New("base error")

		selErr := NewSelectionError("test_operation", "test_table", ctx, baseErr)

		s.Assert().Equal("test_operation", selErr.Operation)
		s.Assert().Equal("test_table", selErr.TableID)
		s.Assert().Equal(ctx, selErr.Context)
		s.Assert().Equal(baseErr, selErr.Cause)
		s.Assert().NotNil(selErr.Details)
		s.Assert().Empty(selErr.Details)
	})

	s.Run("implements error interface correctly", func() {
		baseErr := errors.New("something went wrong")
		selErr := NewSelectionError("select", "my_table", s.ctx, baseErr)

		errorMsg := selErr.Error()
		s.Assert().Contains(errorMsg, "selectables:")
		s.Assert().Contains(errorMsg, "select")
		s.Assert().Contains(errorMsg, "my_table")
		s.Assert().Contains(errorMsg, "something went wrong")
	})

	s.Run("handles empty table ID gracefully", func() {
		baseErr := errors.New("test error")
		selErr := NewSelectionError("operation", "", s.ctx, baseErr)

		errorMsg := selErr.Error()
		s.Assert().Contains(errorMsg, "selectables:")
		s.Assert().Contains(errorMsg, "operation")
		s.Assert().Contains(errorMsg, "test error")
		s.Assert().NotContains(errorMsg, "table ''") // Should not include empty table reference
	})

	s.Run("supports error unwrapping", func() {
		baseErr := errors.New("root cause")
		selErr := NewSelectionError("operation", "table", s.ctx, baseErr)

		unwrapped := selErr.Unwrap()
		s.Assert().Equal(baseErr, unwrapped)

		// Test with errors.Is
		s.Assert().True(errors.Is(selErr, baseErr))
	})

	s.Run("supports adding and getting details", func() {
		selErr := NewSelectionError("operation", "table", s.ctx, errors.New("test"))

		// Add details
		_ = selErr.AddDetail("count", 5)
		_ = selErr.AddDetail("reason", "insufficient items")
		_ = selErr.AddDetail("available", []string{"item1", "item2"})

		// Get details
		count, exists := selErr.GetDetail("count")
		s.Assert().True(exists)
		s.Assert().Equal(5, count)

		reason, exists := selErr.GetDetail("reason")
		s.Assert().True(exists)
		s.Assert().Equal("insufficient items", reason)

		available, exists := selErr.GetDetail("available")
		s.Assert().True(exists)
		s.Assert().Equal([]string{"item1", "item2"}, available)

		// Non-existent detail
		_, exists = selErr.GetDetail("non_existent")
		s.Assert().False(exists)
	})

	s.Run("AddDetail returns self for chaining", func() {
		selErr := NewSelectionError("operation", "table", s.ctx, errors.New("test"))

		result := selErr.AddDetail("key1", "value1").AddDetail("key2", "value2")

		s.Assert().Equal(selErr, result, "AddDetail should return self for chaining")

		// Verify both details were added
		value1, exists1 := selErr.GetDetail("key1")
		value2, exists2 := selErr.GetDetail("key2")

		s.Assert().True(exists1)
		s.Assert().Equal("value1", value1)
		s.Assert().True(exists2)
		s.Assert().Equal("value2", value2)
	})
}

func (s *ErrorHandlingTestSuite) TestEmptyTableErrors() {
	s.Run("Select returns ErrEmptyTable", func() {
		// Don't add any items
		selected, err := s.table.Select(s.ctx)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrEmptyTable))
		s.Assert().Equal("", selected) // Zero value for string

		// Should be a SelectionError
		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select", selErr.Operation)
		s.Assert().Equal("error_test_table", selErr.TableID)
	})

	s.Run("SelectMany returns ErrEmptyTable", func() {
		selected, err := s.table.SelectMany(s.ctx, 3)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrEmptyTable))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_many", selErr.Operation)
	})

	s.Run("SelectUnique returns ErrEmptyTable", func() {
		selected, err := s.table.SelectUnique(s.ctx, 2)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrEmptyTable))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_unique", selErr.Operation)
	})

	s.Run("SelectVariable returns ErrEmptyTable", func() {
		selected, err := s.table.SelectVariable(s.ctx, "1d6")

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrEmptyTable))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_many", selErr.Operation) // SelectVariable delegates to SelectMany
	})
}

func (s *ErrorHandlingTestSuite) TestInvalidCountErrors() {
	s.Run("SelectMany returns ErrInvalidCount for zero count", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectMany(s.ctx, 0)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrInvalidCount))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_many", selErr.Operation)
	})

	s.Run("SelectMany returns ErrInvalidCount for negative count", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectMany(s.ctx, -5)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrInvalidCount))
		s.Assert().Nil(selected)
	})

	s.Run("SelectUnique returns ErrInvalidCount for zero count", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectUnique(s.ctx, 0)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrInvalidCount))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_unique", selErr.Operation)
	})

	s.Run("SelectUnique returns ErrInvalidCount for negative count", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectUnique(s.ctx, -3)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrInvalidCount))
		s.Assert().Nil(selected)
	})
}

func (s *ErrorHandlingTestSuite) TestInsufficientItemsErrors() {
	s.Run("SelectUnique returns ErrInsufficientItems when requesting more than available", func() {
		s.table.
			Add("item1", 10).
			Add("item2", 20)

		selected, err := s.table.SelectUnique(s.ctx, 5) // More than 2 available

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrInsufficientItems))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_unique", selErr.Operation)

		// Should include helpful details
		requestedCount, exists := selErr.GetDetail("requested_count")
		s.Assert().True(exists)
		s.Assert().Equal(5, requestedCount)

		availableCount, exists := selErr.GetDetail("available_count")
		s.Assert().True(exists)
		s.Assert().Equal(2, availableCount)
	})

	s.Run("SelectUnique succeeds when requesting exactly available count", func() {
		s.table.
			Add("item1", 10).
			Add("item2", 20)

		selected, err := s.table.SelectUnique(s.ctx, 2) // Exactly 2 available

		s.Assert().NoError(err)
		s.Assert().Len(selected, 2)
	})
}

func (s *ErrorHandlingTestSuite) TestContextRequiredErrors() {
	s.Run("Select returns ErrContextRequired for nil context", func() {
		s.table.Add("item", 10)

		selected, err := s.table.Select(nil)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrContextRequired))
		s.Assert().Equal("", selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select", selErr.Operation)
	})

	s.Run("SelectVariable returns ErrContextRequired for nil context", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectVariable(nil, "1d6")

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrContextRequired))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_variable", selErr.Operation)
	})
}

func (s *ErrorHandlingTestSuite) TestDiceRollerRequiredErrors() {
	s.Run("Select returns ErrDiceRollerRequired for context without roller", func() {
		s.table.Add("item", 10)

		// Create context without dice roller
		ctx := &BasicSelectionContext{
			values:     make(map[string]interface{}),
			diceRoller: nil,
		}

		selected, err := s.table.Select(ctx)

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrDiceRollerRequired))
		s.Assert().Equal("", selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select", selErr.Operation)
	})

	s.Run("SelectVariable returns ErrDiceRollerRequired for context without roller", func() {
		s.table.Add("item", 10)

		ctx := &BasicSelectionContext{
			values:     make(map[string]interface{}),
			diceRoller: nil,
		}

		selected, err := s.table.SelectVariable(ctx, "1d6")

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrDiceRollerRequired))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_variable", selErr.Operation)
	})
}

func (s *ErrorHandlingTestSuite) TestInvalidDiceExpressionErrors() {
	s.Run("SelectVariable returns ErrInvalidDiceExpression for malformed expression", func() {
		s.table.Add("item", 10)

		selected, err := s.table.SelectVariable(s.ctx, "invalid_dice_expression")

		s.Assert().Error(err)
		s.Assert().True(errors.Is(err, ErrInvalidDiceExpression))
		s.Assert().Nil(selected)

		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal("select_variable", selErr.Operation)

		// Should include the invalid expression in details
		expression, exists := selErr.GetDetail("dice_expression")
		s.Assert().True(exists)
		s.Assert().Equal("invalid_dice_expression", expression)

		// Should include the parse error details
		parseError, exists := selErr.GetDetail("parse_error")
		s.Assert().True(exists)
		s.Assert().NotEmpty(parseError)
	})

	s.Run("SelectVariable works with valid dice expressions", func() {
		s.table.Add("item", 10)

		// These should all work
		validExpressions := []string{"1d6", "2d4", "1d10+2", "3d6"}

		for _, expr := range validExpressions {
			selected, err := s.table.SelectVariable(s.ctx, expr)
			s.Assert().NoError(err, "Expression %s should be valid", expr)
			s.Assert().NotEmpty(selected, "Should select at least one item for %s", expr)
		}
	})
}

func (s *ErrorHandlingTestSuite) TestErrorChaining() {
	s.Run("errors can be unwrapped and checked with errors.Is", func() {
		s.table.Add("item", 10)

		_, err := s.table.SelectMany(s.ctx, 0) // Should cause ErrInvalidCount

		// Direct error check
		s.Assert().Error(err)

		// Unwrap and check base error
		s.Assert().True(errors.Is(err, ErrInvalidCount))

		// Check that it's wrapped in SelectionError
		var selErr *SelectionError
		s.Assert().True(errors.As(err, &selErr))
		s.Assert().Equal(ErrInvalidCount, selErr.Cause)
	})

	s.Run("error details provide debugging information", func() {
		// Create scenario with insufficient items
		s.table.
			Add("item1", 10).
			Add("item2", 20)

		_, err := s.table.SelectUnique(s.ctx, 10) // Request more than available

		var selErr *SelectionError
		s.Require().True(errors.As(err, &selErr))

		// Error should include context about the failure
		s.Assert().Equal("select_unique", selErr.Operation)
		s.Assert().Equal("error_test_table", selErr.TableID)
		s.Assert().Equal(s.ctx, selErr.Context)
		s.Assert().True(errors.Is(selErr.Cause, ErrInsufficientItems))

		// Should have details about the mismatch
		requested, exists := selErr.GetDetail("requested_count")
		s.Assert().True(exists)
		s.Assert().Equal(10, requested)

		available, exists := selErr.GetDetail("available_count")
		s.Assert().True(exists)
		s.Assert().Equal(2, available)
	})
}

func (s *ErrorHandlingTestSuite) TestErrorMessages() {
	s.Run("error messages are descriptive and helpful", func() {
		testCases := []struct {
			name          string
			setupFunc     func() error
			expectedTexts []string
		}{
			{
				name: "empty table",
				setupFunc: func() error {
					_, err := s.table.Select(s.ctx)
					return err
				},
				expectedTexts: []string{"selectables:", "select", "empty table"},
			},
			{
				name: "invalid count",
				setupFunc: func() error {
					s.table.Add("item", 10)
					_, err := s.table.SelectMany(s.ctx, -1)
					return err
				},
				expectedTexts: []string{"selectables:", "select_many", "count"},
			},
			{
				name: "insufficient items",
				setupFunc: func() error {
					s.table.Add("item", 10)
					_, err := s.table.SelectUnique(s.ctx, 5)
					return err
				},
				expectedTexts: []string{"selectables:", "select_unique", "insufficient"},
			},
			{
				name: "missing context",
				setupFunc: func() error {
					s.table.Add("item", 10)
					_, err := s.table.Select(nil)
					return err
				},
				expectedTexts: []string{"selectables:", "select", "context"},
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				// Reset table for each test case
				s.table = NewBasicTable[string](BasicTableConfig{
					ID: "error_test_table",
				})

				err := tc.setupFunc()
				s.Require().Error(err)

				errorMsg := err.Error()
				for _, expectedText := range tc.expectedTexts {
					s.Assert().Contains(errorMsg, expectedText,
						"Error message should contain '%s' for case %s", expectedText, tc.name)
				}
			})
		}
	})
}
