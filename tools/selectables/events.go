package selectables

import (
	"time"
)

// Event constants following the toolkit's dot notation pattern
// Format: {module}.{category}.{action}
const (
	// EventSelectionTableCreated fires when a new selection table is created
	EventSelectionTableCreated = "selectables.table.created"

	// EventSelectionTableDestroyed fires when a selection table is destroyed
	EventSelectionTableDestroyed = "selectables.table.destroyed"

	// EventItemAdded fires when an item is added to a selection table
	EventItemAdded = "selectables.item.added"

	// EventItemRemoved fires when an item is removed from a selection table
	EventItemRemoved = "selectables.item.removed"

	// EventWeightChanged fires when an item's weight is modified
	EventWeightChanged = "selectables.weight.changed"

	// EventSelectionStarted fires when a selection operation begins
	EventSelectionStarted = "selectables.selection.started"

	// EventSelectionCompleted fires when a selection operation completes successfully
	EventSelectionCompleted = "selectables.selection.completed"

	// EventSelectionFailed fires when a selection operation fails
	EventSelectionFailed = "selectables.selection.failed"

	// EventContextModified fires when selection context affects item weights
	EventContextModified = "selectables.context.modified"
)

// SelectionEventData contains data for selection-related events
// Purpose: Provides rich event data for debugging, analytics, and game integration
type SelectionEventData struct {
	// TableID identifies the table involved in the event
	TableID string `json:"table_id"`

	// Operation describes the type of selection operation
	Operation string `json:"operation"`

	// SelectedItems contains the items that were selected
	SelectedItems []interface{} `json:"selected_items,omitempty"`

	// AvailableItems contains all items that were available for selection
	AvailableItems map[interface{}]int `json:"available_items,omitempty"`

	// Context contains the selection context at the time of the event
	Context map[string]interface{} `json:"context,omitempty"`

	// RequestedCount is the number of items requested for multi-selection
	RequestedCount int `json:"requested_count,omitempty"`

	// ActualCount is the number of items actually selected
	ActualCount int `json:"actual_count,omitempty"`

	// TotalWeight is the sum of all effective weights
	TotalWeight int `json:"total_weight,omitempty"`

	// RollResults contains the dice roll values used for selection
	RollResults []int `json:"roll_results,omitempty"`

	// Duration is how long the selection operation took
	Duration time.Duration `json:"duration,omitempty"`

	// Error contains error information for failed operations
	Error string `json:"error,omitempty"`
}

// TableEventData contains data for table modification events
// Purpose: Tracks changes to selection table structure and configuration
type TableEventData struct {
	// TableID identifies the table that was modified
	TableID string `json:"table_id"`

	// TableType indicates the implementation type (e.g., "basic", "conditional")
	TableType string `json:"table_type"`

	// Operation describes what modification was made
	Operation string `json:"operation"`

	// Item contains the item that was added or removed
	Item interface{} `json:"item,omitempty"`

	// Weight contains the weight value for the operation
	Weight int `json:"weight,omitempty"`

	// PreviousWeight contains the previous weight for weight change operations
	PreviousWeight int `json:"previous_weight,omitempty"`

	// TableSize is the current number of items in the table
	TableSize int `json:"table_size"`

	// Configuration contains relevant table configuration
	Configuration interface{} `json:"configuration,omitempty"`
}

// ContextEventData contains data for context modification events
// Purpose: Tracks how selection context affects weight calculations
type ContextEventData struct {
	// TableID identifies the table where context was applied
	TableID string `json:"table_id"`

	// ContextKeys lists the context keys that were evaluated
	ContextKeys []string `json:"context_keys"`

	// WeightModifications describes how weights were changed
	WeightModifications map[interface{}]WeightModification `json:"weight_modifications"`

	// OriginalTotalWeight is the sum of base weights
	OriginalTotalWeight int `json:"original_total_weight"`

	// ModifiedTotalWeight is the sum of effective weights after context
	ModifiedTotalWeight int `json:"modified_total_weight"`
}

// WeightModification describes how context changed an item's weight
type WeightModification struct {
	// OriginalWeight is the base weight of the item
	OriginalWeight int `json:"original_weight"`

	// ModifiedWeight is the effective weight after context application
	ModifiedWeight int `json:"modified_weight"`

	// ModifierType describes how the weight was changed
	ModifierType WeightModifierType `json:"modifier_type"`

	// ModifierValue is the context value that caused the modification
	ModifierValue interface{} `json:"modifier_value"`

	// ContextKey is the context key that triggered the modification
	ContextKey string `json:"context_key"`
}
