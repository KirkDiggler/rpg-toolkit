// Package identifier provides a type-safe, extensible pattern for identifying
// game mechanics like features, proficiencies, skills, and conditions.
// This allows external modules to add new identifiers while maintaining type safety.
package identifier

import (
	"encoding/json"
	"fmt"
	"strings"
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
func (id ID) String() string {
	return fmt.Sprintf("%s:%s:%s", id.Module, id.Type, id.Value)
}

// Equals checks if two identifiers are the same
func (id ID) Equals(other ID) bool {
	return id.Module == other.Module &&
		id.Type == other.Type &&
		id.Value == other.Value
}

// IsValid checks if the identifier has all required fields
func (id ID) IsValid() error {
	if id.Value == "" {
		return fmt.Errorf("identifier value cannot be empty")
	}
	if id.Module == "" {
		return fmt.Errorf("identifier module cannot be empty")
	}
	if id.Type == "" {
		return fmt.Errorf("identifier type cannot be empty")
	}
	return nil
}

// MarshalJSON implements json.Marshaler
func (id ID) MarshalJSON() ([]byte, error) {
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

	// Parse the string format
	parts := strings.Split(str, ":")
	if len(parts) != 3 {
		return fmt.Errorf("invalid identifier format: %s", str)
	}

	id.Module = parts[0]
	id.Type = parts[1]
	id.Value = parts[2]
	return nil
}

// New creates a new identifier with validation
func New(value, module, idType string) (ID, error) {
	id := ID{
		Value:  value,
		Module: module,
		Type:   idType,
	}

	if err := id.IsValid(); err != nil {
		return ID{}, err
	}

	return id, nil
}

// MustNew creates a new identifier, panicking on validation error.
// Use this for compile-time constants where you know the values are valid.
func MustNew(value, module, idType string) ID {
	id, err := New(value, module, idType)
	if err != nil {
		panic(fmt.Sprintf("invalid identifier: %v", err))
	}
	return id
}

// WithSource bundles an identifier with its source (where it came from)
type WithSource struct {
	ID     ID     `json:"id"`
	Source string `json:"source"` // "race:elf", "class:fighter", "background:soldier"
}

// NewWithSource creates an identifier with source information
func NewWithSource(id ID, source string) WithSource {
	return WithSource{
		ID:     id,
		Source: source,
	}
}
