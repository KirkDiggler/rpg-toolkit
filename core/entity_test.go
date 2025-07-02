package core_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// sampleEntity is a test implementation of the Entity interface.
type sampleEntity struct {
	id         string
	entityType string
}

func (s *sampleEntity) GetID() string {
	return s.id
}

func (s *sampleEntity) GetType() string {
	return s.entityType
}

func TestEntity_Implementation(t *testing.T) {
	tests := []struct {
		name         string
		entity       *sampleEntity
		expectedID   string
		expectedType string
	}{
		{
			name: "character entity",
			entity: &sampleEntity{
				id:         "char-001",
				entityType: "character",
			},
			expectedID:   "char-001",
			expectedType: "character",
		},
		{
			name: "item entity",
			entity: &sampleEntity{
				id:         "item-sword-01",
				entityType: "item",
			},
			expectedID:   "item-sword-01",
			expectedType: "item",
		},
		{
			name: "location entity",
			entity: &sampleEntity{
				id:         "loc-tavern",
				entityType: "location",
			},
			expectedID:   "loc-tavern",
			expectedType: "location",
		},
		{
			name: "empty values",
			entity: &sampleEntity{
				id:         "",
				entityType: "",
			},
			expectedID:   "",
			expectedType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the entity implements the interface
			var _ core.Entity = tt.entity

			// Test GetID
			if got := tt.entity.GetID(); got != tt.expectedID {
				t.Errorf("GetID() = %v, want %v", got, tt.expectedID)
			}

			// Test GetType
			if got := tt.entity.GetType(); got != tt.expectedType {
				t.Errorf("GetType() = %v, want %v", got, tt.expectedType)
			}
		})
	}
}

// TestEntity_InterfaceCompliance ensures various entity types can implement the interface.
func TestEntity_InterfaceCompliance(t *testing.T) {
	// Define different entity types that should implement the interface
	type character struct {
		sampleEntity
		name  string
		level int
	}

	type item struct {
		sampleEntity
		name   string
		weight float64
	}

	type location struct {
		sampleEntity
		name        string
		description string
	}

	// Create instances
	char := &character{
		sampleEntity: sampleEntity{id: "char-123", entityType: "character"},
		name:         "Hero",
		level:        10,
	}

	itm := &item{
		sampleEntity: sampleEntity{id: "item-456", entityType: "item"},
		name:         "Sword of Truth",
		weight:       5.5,
	}

	loc := &location{
		sampleEntity: sampleEntity{id: "loc-789", entityType: "location"},
		name:         "Dragon's Lair",
		description:  "A dark and dangerous cave",
	}

	// Verify they all implement Entity
	entities := []core.Entity{char, itm, loc}

	for i, entity := range entities {
		if entity.GetID() == "" {
			t.Errorf("Entity %d has empty ID", i)
		}
		if entity.GetType() == "" {
			t.Errorf("Entity %d has empty type", i)
		}
	}
}

// TestEntity_NilHandling tests how implementations might handle nil scenarios.
func TestEntity_NilHandling(t *testing.T) {
	var entity *sampleEntity

	// This would panic if called on nil, demonstrating the importance of nil checks
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic when calling methods on nil entity")
		}
	}()

	// This should panic
	_ = entity.GetID()
}
