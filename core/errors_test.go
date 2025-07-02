package core_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrEntityNotFound",
			err:      core.ErrEntityNotFound,
			expected: "entity not found",
		},
		{
			name:     "ErrInvalidEntity",
			err:      core.ErrInvalidEntity,
			expected: "invalid entity",
		},
		{
			name:     "ErrDuplicateEntity",
			err:      core.ErrDuplicateEntity,
			expected: "duplicate entity",
		},
		{
			name:     "ErrNilEntity",
			err:      core.ErrNilEntity,
			expected: "nil entity",
		},
		{
			name:     "ErrEmptyID",
			err:      core.ErrEmptyID,
			expected: "empty entity ID",
		},
		{
			name:     "ErrInvalidType",
			err:      core.ErrInvalidType,
			expected: "invalid entity type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Error message = %v, want %v", tt.err.Error(), tt.expected)
			}
		})
	}
}

func TestEntityError(t *testing.T) {
	tests := []struct {
		name         string
		entityError  *core.EntityError
		expectedMsg  string
		shouldUnwrap bool
		unwrappedErr error
	}{
		{
			name: "full entity error",
			entityError: core.NewEntityError(
				"create",
				"character",
				"char-123",
				core.ErrDuplicateEntity,
			),
			expectedMsg:  "create character char-123: duplicate entity",
			shouldUnwrap: true,
			unwrappedErr: core.ErrDuplicateEntity,
		},
		{
			name: "entity error without ID",
			entityError: core.NewEntityError(
				"validate",
				"item",
				"",
				core.ErrEmptyID,
			),
			expectedMsg:  "validate item: empty entity ID",
			shouldUnwrap: true,
			unwrappedErr: core.ErrEmptyID,
		},
		{
			name: "entity error without type",
			entityError: &core.EntityError{
				Op:  "delete",
				Err: core.ErrEntityNotFound,
			},
			expectedMsg:  "delete: entity not found",
			shouldUnwrap: true,
			unwrappedErr: core.ErrEntityNotFound,
		},
		{
			name: "entity error with custom error",
			entityError: core.NewEntityError(
				"load",
				"location",
				"loc-456",
				errors.New("file not found"),
			),
			expectedMsg:  "load location loc-456: file not found",
			shouldUnwrap: true,
			unwrappedErr: errors.New("file not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Error() method
			if got := tt.entityError.Error(); got != tt.expectedMsg {
				t.Errorf("Error() = %v, want %v", got, tt.expectedMsg)
			}

			// Test Unwrap() method
			if tt.shouldUnwrap {
				unwrapped := tt.entityError.Unwrap()
				if unwrapped == nil {
					t.Error("Unwrap() returned nil, expected error")
				} else if unwrapped.Error() != tt.unwrappedErr.Error() {
					t.Errorf("Unwrap() = %v, want %v", unwrapped.Error(), tt.unwrappedErr.Error())
				}
			}
		})
	}
}

func TestErrorUsagePatterns(t *testing.T) {
	t.Run("checking for specific errors", func(t *testing.T) {
		// Simulate a function that returns an EntityError
		getEntity := func(id string) error {
			if id == "" {
				return core.NewEntityError("get", "character", id, core.ErrEmptyID)
			}
			if id == "not-found" {
				return core.NewEntityError("get", "character", id, core.ErrEntityNotFound)
			}
			return nil
		}

		// Test error checking with errors.Is
		err := getEntity("")
		if !errors.Is(err, core.ErrEmptyID) {
			t.Error("Expected error to be ErrEmptyID")
		}

		err = getEntity("not-found")
		if !errors.Is(err, core.ErrEntityNotFound) {
			t.Error("Expected error to be ErrEntityNotFound")
		}

		err = getEntity("valid-id")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("error type assertions", func(t *testing.T) {
		err := core.NewEntityError("update", "item", "item-123", core.ErrInvalidEntity)

		var entityErr *core.EntityError
		if errors.As(err, &entityErr) {
			if entityErr.EntityID != "item-123" {
				t.Errorf("EntityID = %v, want item-123", entityErr.EntityID)
			}
			if entityErr.EntityType != "item" {
				t.Errorf("EntityType = %v, want item", entityErr.EntityType)
			}
			if entityErr.Op != "update" {
				t.Errorf("Op = %v, want update", entityErr.Op)
			}
		} else {
			t.Error("Expected error to be *EntityError")
		}
	})

	t.Run("error chaining", func(t *testing.T) {
		// Simulate nested error scenarios
		baseErr := errors.New("database connection failed")
		entityErr := core.NewEntityError("save", "character", "char-001", baseErr)
		wrappedErr := fmt.Errorf("failed to persist entity: %w", entityErr)

		// Check the error message contains all parts
		errMsg := wrappedErr.Error()
		if !strings.Contains(errMsg, "failed to persist entity") {
			t.Error("Error message should contain wrapper text")
		}
		if !strings.Contains(errMsg, "save character char-001") {
			t.Error("Error message should contain entity error details")
		}
		if !strings.Contains(errMsg, "database connection failed") {
			t.Error("Error message should contain base error")
		}
	})
}

func TestErrorValidation(t *testing.T) {
	t.Run("validate entity errors", func(t *testing.T) {
		validateEntity := func(e core.Entity) error {
			if e == nil {
				return core.ErrNilEntity
			}
			if e.GetID() == "" {
				return core.NewEntityError("validate", e.GetType(), "", core.ErrEmptyID)
			}
			if e.GetType() == "" {
				return core.NewEntityError("validate", "", e.GetID(), core.ErrInvalidType)
			}
			return nil
		}

		// Test nil entity
		err := validateEntity(nil)
		if !errors.Is(err, core.ErrNilEntity) {
			t.Error("Expected ErrNilEntity for nil entity")
		}

		// Test entity with empty ID
		entity := &sampleEntity{id: "", entityType: "character"}
		err = validateEntity(entity)
		if !errors.Is(err, core.ErrEmptyID) {
			t.Error("Expected ErrEmptyID for entity with empty ID")
		}

		// Test entity with empty type
		entity = &sampleEntity{id: "test-123", entityType: ""}
		err = validateEntity(entity)
		if !errors.Is(err, core.ErrInvalidType) {
			t.Error("Expected ErrInvalidType for entity with empty type")
		}

		// Test valid entity
		entity = &sampleEntity{id: "test-123", entityType: "character"}
		err = validateEntity(entity)
		if err != nil {
			t.Errorf("Expected no error for valid entity, got %v", err)
		}
	})
}