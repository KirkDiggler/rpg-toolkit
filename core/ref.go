package core

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

// SourceCategory represents the category of an identifier
type SourceCategory string

const (
	// SourceClass represents a class source
	SourceClass SourceCategory = "class"
	// SourceRace represents a race source
	SourceRace SourceCategory = "race"
	// SourceBackground represents a background source
	SourceBackground SourceCategory = "background"
	// SourceFeat represents a feat source
	SourceFeat SourceCategory = "feat"
	// SourceItem represents an item source
	SourceItem SourceCategory = "item"
	// SourceManual represents a manual source (DM granted)
	SourceManual SourceCategory = "manual"
)

// Source represents the source of an identifier
type Source struct {
	Category SourceCategory
	Name     string
}

// String returns the source as a string in the format "category:name"
func (s *Source) String() string {
	return fmt.Sprintf("%s:%s", s.Category, s.Name)
}

// SourcedRef represents an identifier with its source
type SourcedRef struct {
	Ref    *Ref
	Source *Source // Not a string anymore!
}

// Ref represents a unique identifier for a game mechanic.
// It's designed to be extensible - external modules can create new IDs
// while core modules provide type-safe constructors for known IDs.
type Ref struct {
	// Value is the unique identifier within the module namespace
	Value string `json:"value"`

	// Module identifies which module defined this Ref ("core", "artificer", etc.)
	Module string `json:"module"`

	// Type categorizes the identifier ("feature", "proficiency", "skill", etc.)
	Type string `json:"type"`
}

// String returns the full identifier as module:type:value
func (id *Ref) String() string {
	return fmt.Sprintf("%s:%s:%s", id.Module, id.Type, id.Value)
}

// ParseString parses the string format with detailed error reporting
func ParseString(s string) (*Ref, error) {
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

	// Create the Ref with segments
	id := &Ref{
		Module: segments[0],
		Type:   segments[1],
		Value:  segments[2],
	}

	// Validate the Ref
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
func (id *Ref) Equals(other *Ref) bool {
	if id == nil || other == nil {
		return id == other
	}
	return id.Module == other.Module &&
		id.Type == other.Type &&
		id.Value == other.Value
}

// IsValid checks if the identifier has all required fields
func (id *Ref) IsValid() error {
	return id.validate()
}

// validate performs comprehensive validation of the identifier
func (id *Ref) validate() error {
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
func (id *Ref) MarshalJSON() ([]byte, error) {
	// Can be stored as a simple string for more compact JSON
	return json.Marshal(id.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (id *Ref) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		// Try unmarshaling as object for backward compatibility
		type rawID Ref
		var raw rawID
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*id = Ref(raw)
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

// RefInput provides a structured way to create a Ref with clear field names
type RefInput struct {
	Module string // e.g., "dnd5e", "core"
	Type   string // e.g., "spell", "feature", "skill"
	Value  string // e.g., "charm_person", "rage", "acrobatics"
}

// NewRef creates a new identifier with validation using RefInput
func NewRef(input RefInput) (*Ref, error) {
	// Validate all fields are provided
	if input.Module == "" {
		return nil, fmt.Errorf("module cannot be empty")
	}
	if input.Type == "" {
		return nil, fmt.Errorf("type cannot be empty")
	}
	if input.Value == "" {
		return nil, fmt.Errorf("value cannot be empty")
	}

	id := &Ref{
		Module: input.Module,
		Type:   input.Type,
		Value:  input.Value,
	}

	if err := id.IsValid(); err != nil {
		return nil, err
	}

	return id, nil
}

// MustNewRef creates a new identifier, panicking on validation error.
// Use this for compile-time constants where you know the values are valid.
func MustNewRef(input RefInput) *Ref {
	id, err := NewRef(input)
	if err != nil {
		panic(fmt.Sprintf("invalid identifier: %v", err))
	}
	return id
}

// WithSourcedRef bundles an identifier with its source (where it came from)
type WithSourcedRef struct {
	ID     *Ref    `json:"id"`
	Source *Source `json:"source"` // "race:elf", "class:fighter", "background:soldier"
}

// NewWithSourcedRef creates an identifier with source information
func NewWithSourcedRef(id *Ref, source *Source) WithSourcedRef {
	return WithSourcedRef{
		ID:     id,
		Source: source,
	}
}
