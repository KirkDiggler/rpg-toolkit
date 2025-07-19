package selectables

import "errors"

// Core selection errors that can occur during table operations
// These errors provide specific information about why selection operations fail

var (
	// ErrEmptyTable indicates an attempt to select from a table with no items
	// This occurs when all items have been filtered out or no items were ever added
	ErrEmptyTable = errors.New("empty table contains no items")

	// ErrInvalidCount indicates an invalid count parameter for multi-item selection
	// Count must be greater than 0 for SelectMany and SelectUnique operations
	ErrInvalidCount = errors.New("selection count must be greater than 0")

	// ErrInvalidWeight indicates an attempt to add an item with invalid weight
	// Weights must be positive integers greater than 0
	ErrInvalidWeight = errors.New("item weight must be greater than 0")

	// ErrInvalidDiceExpression indicates a dice expression that cannot be parsed
	// Used by SelectVariable when the provided expression is malformed
	ErrInvalidDiceExpression = errors.New("invalid dice expression")

	// ErrInsufficientItems indicates not enough unique items for SelectUnique operation
	// Occurs when requesting more unique items than are available in the table
	ErrInsufficientItems = errors.New("insufficient unique items available for selection")

	// ErrTableNotFound indicates a referenced nested table could not be found
	// Used by hierarchical tables when a named sub-table is missing
	ErrTableNotFound = errors.New("referenced selection table not found")

	// ErrCircularReference indicates a circular dependency in nested tables
	// Prevents infinite loops in hierarchical table structures
	ErrCircularReference = errors.New("circular reference detected in nested tables")

	// ErrContextRequired indicates an operation that requires a valid selection context
	// Some table operations need context for conditional weight modifications
	ErrContextRequired = errors.New("selection context is required for this operation")

	// ErrDiceRollerRequired indicates an operation that requires a dice roller
	// Selection operations need randomization through the dice interface
	ErrDiceRollerRequired = errors.New("dice roller is required for selection operations")

	// ErrInvalidConfiguration indicates invalid table configuration parameters
	// Used when TableConfiguration contains conflicting or invalid settings
	ErrInvalidConfiguration = errors.New("invalid table configuration")
)

// SelectionError provides detailed error information with context about failed selections
// Purpose: Rich error reporting that includes table state and selection parameters
// for better debugging and error handling in games.
type SelectionError struct {
	// Operation describes what selection operation was being performed
	Operation string

	// TableID identifies which table caused the error
	TableID string

	// Context contains the selection context when the error occurred
	Context SelectionContext

	// Cause contains the underlying error that caused the failure
	Cause error

	// Details provides additional information about the error condition
	Details map[string]interface{}
}

// Error implements the error interface for SelectionError
// Returns a descriptive error message including operation and table information
func (e *SelectionError) Error() string {
	if e.TableID != "" {
		return "selectables: " + e.Operation + " failed on table '" + e.TableID + "': " + e.Cause.Error()
	}
	return "selectables: " + e.Operation + " failed: " + e.Cause.Error()
}

// Unwrap returns the underlying error for error chain inspection
// Supports Go 1.13+ error unwrapping for better error handling
func (e *SelectionError) Unwrap() error {
	return e.Cause
}

// NewSelectionError creates a new SelectionError with the provided details
// Purpose: Standardized way to create rich selection errors with context
func NewSelectionError(operation, tableID string, ctx SelectionContext, cause error) *SelectionError {
	return &SelectionError{
		Operation: operation,
		TableID:   tableID,
		Context:   ctx,
		Cause:     cause,
		Details:   make(map[string]interface{}),
	}
}

// AddDetail adds additional context information to the selection error
// Useful for providing extra debugging information about the failure
func (e *SelectionError) AddDetail(key string, value interface{}) *SelectionError {
	e.Details[key] = value
	return e
}

// GetDetail retrieves a detail value by key from the error
// Returns the value and true if found, nil and false if not found
func (e *SelectionError) GetDetail(key string) (interface{}, bool) {
	value, exists := e.Details[key]
	return value, exists
}
