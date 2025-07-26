package core

import (
	"errors"
	"fmt"
)

// Common errors that can occur throughout the RPG toolkit.
var (
	// ErrEntityNotFound is returned when an entity cannot be found.
	ErrEntityNotFound = errors.New("entity not found")

	// ErrInvalidEntity is returned when an entity is invalid or malformed.
	ErrInvalidEntity = errors.New("invalid entity")

	// ErrDuplicateEntity is returned when attempting to create an entity with an ID that already exists.
	ErrDuplicateEntity = errors.New("duplicate entity")

	// ErrNilEntity is returned when a nil entity is provided where a valid entity is expected.
	ErrNilEntity = errors.New("nil entity")

	// ErrEmptyID is returned when an entity has an empty or invalid ID.
	ErrEmptyID = errors.New("empty entity ID")

	// ErrInvalidType is returned when an entity has an invalid or unrecognized type.
	ErrInvalidType = errors.New("invalid entity type")

	// Equipment-related errors

	// ErrInsufficientStrength is returned when a character lacks the strength requirement for an item.
	ErrInsufficientStrength = errors.New("insufficient strength")

	// ErrMissingProficiency is returned when a character lacks the required proficiency for an item.
	ErrMissingProficiency = errors.New("missing proficiency")

	// ErrIncompatibleSlot is returned when an item cannot be equipped in the specified slot.
	ErrIncompatibleSlot = errors.New("incompatible slot")

	// ErrSlotOccupied is returned when attempting to equip an item to an already occupied slot.
	ErrSlotOccupied = errors.New("slot occupied")

	// ErrRequiresAttunement is returned when attempting to use a magic item that requires attunement.
	ErrRequiresAttunement = errors.New("requires attunement")

	// ErrAttunementLimit is returned when a character has reached their attunement limit.
	ErrAttunementLimit = errors.New("attunement limit reached")

	// ErrTwoHandedConflict is returned when a two-handed item conflicts with equipped items.
	ErrTwoHandedConflict = errors.New("two-handed conflict")

	// ErrClassRestriction is returned when a character's class cannot use an item.
	ErrClassRestriction = errors.New("class restriction")

	// ErrRaceRestriction is returned when a character's race cannot use an item.
	ErrRaceRestriction = errors.New("race restriction")

	// ErrAlignmentRestriction is returned when a character's alignment prevents item use.
	ErrAlignmentRestriction = errors.New("alignment restriction")
)

// EntityError represents an error related to a specific entity.
type EntityError struct {
	EntityID   string
	EntityType string
	Op         string // Operation that caused the error
	Err        error  // Underlying error
}

// Error implements the error interface for EntityError.
func (e *EntityError) Error() string {
	if e.EntityID != "" && e.EntityType != "" {
		return fmt.Sprintf("%s %s %s: %v", e.Op, e.EntityType, e.EntityID, e.Err)
	} else if e.EntityType != "" {
		return fmt.Sprintf("%s %s: %v", e.Op, e.EntityType, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *EntityError) Unwrap() error {
	return e.Err
}

// NewEntityError creates a new EntityError.
func NewEntityError(op, entityType, entityID string, err error) *EntityError {
	return &EntityError{
		EntityID:   entityID,
		EntityType: entityType,
		Op:         op,
		Err:        err,
	}
}

// EquipmentError represents an error related to equipment validation.
type EquipmentError struct {
	CharacterID string
	ItemID      string
	Slot        string
	Op          string // Operation that caused the error
	Err         error  // Underlying error
}

// Error implements the error interface for EquipmentError.
func (e *EquipmentError) Error() string {
	if e.ItemID != "" && e.Slot != "" {
		return fmt.Sprintf("%s item %s to slot %s: %v", e.Op, e.ItemID, e.Slot, e.Err)
	} else if e.ItemID != "" {
		return fmt.Sprintf("%s item %s: %v", e.Op, e.ItemID, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *EquipmentError) Unwrap() error {
	return e.Err
}

// NewEquipmentError creates a new EquipmentError.
func NewEquipmentError(op, characterID, itemID, slot string, err error) *EquipmentError {
	return &EquipmentError{
		CharacterID: characterID,
		ItemID:      itemID,
		Slot:        slot,
		Op:          op,
		Err:         err,
	}
}
