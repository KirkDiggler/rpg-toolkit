package spawn

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
)

// BasicSelectablesRegistry implements the SelectablesRegistry interface
// Purpose: Manages selectables integration for entity selection in spawn operations
type BasicSelectablesRegistry struct {
	tables map[string]selectables.SelectionTable[core.Entity]
	mutex  sync.RWMutex
}

// NewBasicSelectablesRegistry creates a new selectables registry
func NewBasicSelectablesRegistry() *BasicSelectablesRegistry {
	return &BasicSelectablesRegistry{
		tables: make(map[string]selectables.SelectionTable[core.Entity]),
	}
}

// RegisterTable registers a selection table with the spawn engine
func (r *BasicSelectablesRegistry) RegisterTable(tableID string, table selectables.SelectionTable[core.Entity]) error {
	if tableID == "" {
		return fmt.Errorf("table ID cannot be empty")
	}
	if table == nil {
		return fmt.Errorf("table cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.tables[tableID] = table
	return nil
}

// GetTable retrieves a registered selection table
func (r *BasicSelectablesRegistry) GetTable(tableID string) (selectables.SelectionTable[core.Entity], error) {
	if tableID == "" {
		return nil, fmt.Errorf("table ID cannot be empty")
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	table, exists := r.tables[tableID]
	if !exists {
		return nil, fmt.Errorf("table not found: %s", tableID)
	}

	return table, nil
}

// ListTables returns all registered table IDs
func (r *BasicSelectablesRegistry) ListTables() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tableIDs := make([]string, 0, len(r.tables))
	for tableID := range r.tables {
		tableIDs = append(tableIDs, tableID)
	}

	return tableIDs
}

// RemoveTable removes a selection table
func (r *BasicSelectablesRegistry) RemoveTable(tableID string) error {
	if tableID == "" {
		return fmt.Errorf("table ID cannot be empty")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.tables[tableID]; !exists {
		return fmt.Errorf("table not found: %s", tableID)
	}

	delete(r.tables, tableID)
	return nil
}