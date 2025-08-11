// Package identifier provides a type-safe, extensible pattern for identifying
// game mechanics like features, proficiencies, skills, and conditions.
// This allows external modules to add new identifiers while maintaining type safety.
package identifier

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

const (
	// separatorChar is the character used to separate identifier parts
	separatorChar = ":"
	// expectedParts is the number of parts in a valid identifier string
	expectedParts = 3
)

// ID represents a unique identifier for a game mechanic.
// It's designed to be extensible - external modules can create new IDs
// while core modules provide type-safe constructors for known IDs.
type ID struct {
	// Value is the unique identifier within the module namespace
	Value string `json:"value"`

	// Module identifies which module defined this ID ("core", "artificer", etc.)
	Module string `json:"module"`

	// Type categorizes the identifier ("feature", "proficiency", "skill", etc.)
	Type string `json:"type"`
}

// String returns the full identifier as module:type:value
func (id *ID) String() string {
	return fmt.Sprintf("%s:%s:%s", id.Module, id.Type, id.Value)
}

// ParseString parses the string format with detailed error reporting
func ParseString(s string) (*ID, error) {
	if s == "" {
		return nil, NewParseError(s, "", 0, ErrEmptyString)
	}
	
	segments := strings.Split(s, separatorChar)
	segmentCount := len(segments)
	
	// Validate we have exactly the right number of segments
	if segmentCount < expectedParts {
		return nil, NewParseError(s, "", 0, 
			fmt.Errorf("%w: expected %d segments, got %d", ErrTooFewSegments, expectedParts, segmentCount))
	}
	if segmentCount > expectedParts {
		return nil, NewParseError(s, "", 0,
			fmt.Errorf("%w: expected %d segments, got %d", ErrTooManySegments, expectedParts, segmentCount))
	}
	
	// Create the ID with segments
	id := &ID{
		Module: segments[0],
		Type:   segments[1],
		Value:  segments[2],
	}
	
	// Validate the ID
	if err := id.validate(); err != nil {
		return nil, err
	}
	
	return id, nil
}

// isValidIdentifierPart checks if a string contains only valid identifier characters
func isValidIdentifierPart(s string) bool {
	if s == "" {
		return false
	}
	
	for _, r := range s {
		// Allow letters, digits, underscore, and dash
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// Equals checks if two identifiers are the same
func (id *ID) Equals(other *ID) bool {
	if id == nil || other == nil {
		return id == other
	}
	return id.Module == other.Module &&
		id.Type == other.Type &&
		id.Value == other.Value
}

// IsValid checks if the identifier has all required fields
func (id *ID) IsValid() error {
	return id.validate()
}

// validate performs comprehensive validation of the identifier
func (id *ID) validate() error {
	// Check for empty components
	if id.Module == "" {
		return NewValidationError("module", id.Module, "cannot be empty", ErrEmptyComponent)
	}
	if id.Type == "" {
		return NewValidationError("type", id.Type, "cannot be empty", ErrEmptyComponent)
	}
	if id.Value == "" {
		return NewValidationError("value", id.Value, "cannot be empty", ErrEmptyComponent)
	}
	
	// Validate characters in each component
	if !isValidIdentifierPart(id.Module) {
		return NewValidationError("module", id.Module, 
			"contains invalid characters (only letters, digits, underscore, and dash allowed)", 
			ErrInvalidCharacters)
	}
	if !isValidIdentifierPart(id.Type) {
		return NewValidationError("type", id.Type,
			"contains invalid characters (only letters, digits, underscore, and dash allowed)",
			ErrInvalidCharacters)
	}
	if !isValidIdentifierPart(id.Value) {
		return NewValidationError("value", id.Value,
			"contains invalid characters (only letters, digits, underscore, and dash allowed)",
			ErrInvalidCharacters)
	}
	
	return nil
}

// MarshalJSON implements json.Marshaler
func (id *ID) MarshalJSON() ([]byte, error) {
	// Can be stored as a simple string for more compact JSON
	return json.Marshal(id.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (id *ID) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		// Try unmarshaling as object for backward compatibility
		type rawID ID
		var raw rawID
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*id = ID(raw)
		return nil
	}

	// Parse using the structured parser
	parsed, err := ParseString(str)
	if err != nil {
		return fmt.Errorf("failed to unmarshal identifier: %w", err)
	}
	
	*id = *parsed
	return nil
}

// New creates a new identifier with validation
func New(value, module, idType string) (*ID, error) {
	id := &ID{
		Value:  value,
		Module: module,
		Type:   idType,
	}

	if err := id.IsValid(); err != nil {
		return nil, err
	}

	return id, nil
}

// MustNew creates a new identifier, panicking on validation error.
// Use this for compile-time constants where you know the values are valid.
func MustNew(value, module, idType string) *ID {
	id, err := New(value, module, idType)
	if err != nil {
		panic(fmt.Sprintf("invalid identifier: %v", err))
	}
	return id
}

// WithSource bundles an identifier with its source (where it came from)
type WithSource struct {
	ID     *ID    `json:"id"`
	Source string `json:"source"` // "race:elf", "class:fighter", "background:soldier"
}

// NewWithSource creates an identifier with source information
func NewWithSource(id *ID, source string) WithSource {
	return WithSource{
		ID:     id,
		Source: source,
	}
}
