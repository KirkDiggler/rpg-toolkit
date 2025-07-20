package spawn

import (
	"fmt"
	"math/rand"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// BasicSelectablesRegistry implements SelectablesRegistry.
// Purpose: Simple entity selection registry for Phase 1 implementation.
type BasicSelectablesRegistry struct {
	tables map[string][]core.Entity
	random *rand.Rand
}

// NewBasicSelectablesRegistry creates a new registry.
// Purpose: Constructor for entity selection table management.
func NewBasicSelectablesRegistry() *BasicSelectablesRegistry {
	return &BasicSelectablesRegistry{
		tables: make(map[string][]core.Entity),
		random: rand.New(rand.NewSource(42)), // #nosec G404 - deterministic for testing, not cryptographic
	}
}

// RegisterTable implements SelectablesRegistry.RegisterTable
func (r *BasicSelectablesRegistry) RegisterTable(tableID string, entities []core.Entity) error {
	if tableID == "" {
		return fmt.Errorf("table ID cannot be empty")
	}
	if len(entities) == 0 {
		return fmt.Errorf("entity list cannot be empty")
	}

	r.tables[tableID] = entities
	return nil
}

// GetEntities implements SelectablesRegistry.GetEntities
func (r *BasicSelectablesRegistry) GetEntities(tableID string, quantity int) ([]core.Entity, error) {
	table, exists := r.tables[tableID]
	if !exists {
		return nil, fmt.Errorf("table %s not found", tableID)
	}

	if quantity < 1 {
		return nil, fmt.Errorf("quantity must be >= 1")
	}

	result := make([]core.Entity, 0, quantity)
	for i := 0; i < quantity; i++ {
		if len(table) == 0 {
			break
		}

		// Simple random selection (Phase 1)
		index := r.random.Intn(len(table))
		result = append(result, table[index])
	}

	return result, nil
}

// ListTables implements SelectablesRegistry.ListTables
func (r *BasicSelectablesRegistry) ListTables() []string {
	tables := make([]string, 0, len(r.tables))
	for id := range r.tables {
		tables = append(tables, id)
	}
	return tables
}
