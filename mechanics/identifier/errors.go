package identifier

import (
	"errors"
	"fmt"
)

// Sentinel errors for common identifier validation failures.
// These are used for programmatic error checking with errors.Is()
var (
	// ErrEmptyString indicates the identifier string is empty
	ErrEmptyString = errors.New("identifier string cannot be empty")
	
	// ErrInvalidFormat indicates the string doesn't match expected format
	ErrInvalidFormat = errors.New("invalid identifier format")
	
	// ErrEmptyComponent indicates one of the identifier components is empty
	ErrEmptyComponent = errors.New("identifier component cannot be empty")
	
	// ErrInvalidCharacters indicates a component contains invalid characters
	ErrInvalidCharacters = errors.New("identifier contains invalid characters")
	
	// ErrTooManySegments indicates more segments than expected
	ErrTooManySegments = errors.New("too many segments in identifier")
	
	// ErrTooFewSegments indicates fewer segments than expected
	ErrTooFewSegments = errors.New("too few segments in identifier")
)

// ParseError provides detailed information about parsing failures.
// It includes the position and component where the error occurred.
type ParseError struct {
	// Input is the original string that failed to parse
	Input string
	
	// Component indicates which part failed (module, type, or value)
	Component string
	
	// Position is the character position where the error was detected
	Position int
	
	// Err is the underlying error
	Err error
}

// Error implements the error interface
func (e *ParseError) Error() string {
	if e.Component != "" {
		return fmt.Sprintf("failed to parse identifier %q: %s %v", e.Input, e.Component, e.Err)
	}
	return fmt.Sprintf("failed to parse identifier %q: %v", e.Input, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *ParseError) Unwrap() error {
	return e.Err
}

// ValidationError provides detailed validation failure information.
// It includes which validation rule failed and the invalid value.
type ValidationError struct {
	// Field is the identifier field that failed validation (Module, Type, or Value)
	Field string
	
	// Value is the invalid value that failed validation
	Value string
	
	// Rule describes which validation rule failed
	Rule string
	
	// Err is the underlying error
	Err error
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s %q: %s", e.Field, e.Value, e.Rule)
}

// Unwrap returns the underlying error
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewParseError creates a ParseError with the given details
func NewParseError(input, component string, position int, err error) *ParseError {
	return &ParseError{
		Input:     input,
		Component: component,
		Position:  position,
		Err:       err,
	}
}

// NewValidationError creates a ValidationError with the given details
func NewValidationError(field, value, rule string, err error) *ValidationError {
	return &ValidationError{
		Field: field,
		Value: value,
		Rule:  rule,
		Err:   err,
	}
}

// Helper functions for checking specific error conditions

// IsParseError checks if an error is a ParseError
func IsParseError(err error) bool {
	var parseErr *ParseError
	return errors.As(err, &parseErr)
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	var valErr *ValidationError
	return errors.As(err, &valErr)
}

// IsEmptyString checks if the error indicates an empty string
func IsEmptyString(err error) bool {
	return errors.Is(err, ErrEmptyString)
}

// IsInvalidFormat checks if the error indicates invalid format
func IsInvalidFormat(err error) bool {
	return errors.Is(err, ErrInvalidFormat)
}

// IsEmptyComponent checks if the error indicates an empty component
func IsEmptyComponent(err error) bool {
	return errors.Is(err, ErrEmptyComponent)
}

// IsInvalidCharacters checks if the error indicates invalid characters
func IsInvalidCharacters(err error) bool {
	return errors.Is(err, ErrInvalidCharacters)
}