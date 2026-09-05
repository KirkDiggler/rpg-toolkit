package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ID is the base type for all game content identifiers.
// Domain packages alias this type for their specific content types
// (e.g., classes.Class, features.Feature, skills.Skill).
type ID = string

// Module identifies which module defined content (e.g., "dnd5e", "wildemount").
// Domain packages define their module constant using this type.
type Module = string

// Type categorizes content within a module (e.g., "features", "conditions", "classes").
// Domain packages define their type constant using this type.
type Type = string

const (
	// separatorChar is the character used to separate identifier parts
	separatorChar = ":"
	// expectedParts is the number of segments a ref string splits into:
	// module, type, and the id. The id is everything after the second
	// separator, so it may carry separators of its own.
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
	// Module identifies which module defined this Ref ("dnd5e", "wildemount", etc.)
	Module Module `json:"module"`

	// Type categorizes the identifier ("features", "conditions", "classes", etc.)
	Type Type `json:"type"`

	// ID is the unique identifier within the module namespace. It is
	// everything after the second separator, so it may itself carry
	// separator-joined parts: the id of "dnd5e:props:plushie:skeleton-dog"
	// is "plushie:skeleton-dog". The grammar requires only that every part
	// is well-formed; what the parts MEAN belongs to whoever owns the
	// content.
	ID ID `json:"id"`
}

// String returns the full identifier as module:type:id. It is the exact
// inverse of ParseString at any id depth, since the id is rejoined verbatim.
func (id *Ref) String() string {
	return fmt.Sprintf("%s:%s:%s", id.Module, id.Type, id.ID)
}

// ParseString parses module:type:id with detailed error reporting.
//
// Module and type are single identifier parts. The id is EVERYTHING after the
// second separator: one or more parts joined by it, every part non-empty and
// drawn from the identifier charset. So "dnd5e:props:plushie:skeleton-dog"
// parses, with id "plushie:skeleton-dog", and five parts read the same way as
// four. The grammar does not count the id's parts, because their structure
// belongs to the content that mints them, not to core.
func ParseString(s string) (*Ref, error) {
	if s == "" {
		return nil, NewParseError(s, "", 0, ErrEmptyString)
	}

	segments := strings.SplitN(s, separatorChar, expectedParts)
	segmentCount := len(segments)

	// Two separators are the whole shape requirement: what follows the
	// second one is the id, however many parts it carries.
	if segmentCount < expectedParts {
		return nil, NewParseError(s, "", 0,
			fmt.Errorf("%w: expected %d segments, got %d", ErrTooFewSegments, expectedParts, segmentCount))
	}

	// Create the Ref with segments
	id := &Ref{
		Module: segments[0],
		Type:   segments[1],
		ID:     segments[2],
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
		id.ID == other.ID
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
	if err := validateIDParts(id.ID); err != nil {
		return err
	}

	return nil
}

// validateIDParts holds every part of an id to the identifier charset.
//
// The id may carry separator-joined parts, and a refusal has to say WHICH one
// broke the rule — "dnd5e:props::skeleton-dog" and "dnd5e:props:plushie:" are
// different mistakes, and an author who is told only "id" has to find the gap
// themselves. A single-part id keeps the plain "id" field name it has always
// had, so the common refusal reads exactly as before.
func validateIDParts(id ID) error {
	if id == "" {
		return NewValidationError("id", id, "cannot be empty", ErrEmptyComponent)
	}

	parts := strings.Split(id, separatorChar)
	for i, part := range parts {
		field := "id"
		if len(parts) > 1 {
			field = fmt.Sprintf("id part %d", i+1)
		}

		if part == "" {
			return NewValidationError(field, id, "cannot be empty", ErrEmptyComponent)
		}
		if !isValidIdentifierPart(part) {
			return NewValidationError(field, id,
				"contains invalid characters (only letters, digits, underscore, and dash allowed)",
				ErrInvalidCharacters)
		}
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
	ID     ID     // e.g., "charm_person", "rage", "acrobatics"
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
	if input.ID == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	id := &Ref{
		Module: input.Module,
		Type:   input.Type,
		ID:     input.ID,
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
