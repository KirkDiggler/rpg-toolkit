// Package selectables provides weighted random selection capabilities for RPG games.
package selectables

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Typed topic definitions for selectables module events
// These are defined at compile-time and connected to event bus at runtime via .On(bus)

var (
	// SelectionTableCreatedTopic publishes events when selection tables are created
	SelectionTableCreatedTopic = events.DefineTypedTopic[SelectionTableCreatedEvent]("selectables.table.created")
	// SelectionTableDestroyedTopic publishes events when selection tables are destroyed
	SelectionTableDestroyedTopic = events.DefineTypedTopic[SelectionTableDestroyedEvent]("selectables.table.destroyed")

	// ItemAddedTopic publishes events when items are added to selection tables
	ItemAddedTopic = events.DefineTypedTopic[ItemAddedEvent]("selectables.item.added")
	// ItemRemovedTopic publishes events when items are removed from selection tables
	ItemRemovedTopic = events.DefineTypedTopic[ItemRemovedEvent]("selectables.item.removed")
	// WeightChangedTopic publishes events when item weights are changed
	WeightChangedTopic = events.DefineTypedTopic[WeightChangedEvent]("selectables.weight.changed")

	// SelectionStartedTopic publishes events when selections begin
	SelectionStartedTopic = events.DefineTypedTopic[SelectionStartedEvent]("selectables.selection.started")
	// SelectionCompletedTopic publishes events when selections complete successfully
	SelectionCompletedTopic = events.DefineTypedTopic[SelectionCompletedEvent]("selectables.selection.completed")
	// SelectionFailedTopic publishes events when selections fail
	SelectionFailedTopic = events.DefineTypedTopic[SelectionFailedEvent]("selectables.selection.failed")

	// ContextModifiedTopic publishes events when selection contexts are modified
	ContextModifiedTopic = events.DefineTypedTopic[ContextModifiedEvent]("selectables.context.modified")
)

// SelectionTableCreatedEvent contains data for table creation events
type SelectionTableCreatedEvent struct {
	TableID       string    `json:"table_id"`
	TableType     string    `json:"table_type"`
	Configuration string    `json:"configuration,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// SelectionTableDestroyedEvent contains data for table destruction events
type SelectionTableDestroyedEvent struct {
	TableID     string    `json:"table_id"`
	TableType   string    `json:"table_type"`
	Reason      string    `json:"reason,omitempty"`
	DestroyedAt time.Time `json:"destroyed_at"`
}

// ItemAddedEvent contains data for item addition events
type ItemAddedEvent struct {
	TableID string    `json:"table_id"`
	ItemID  string    `json:"item_id"`
	Weight  int       `json:"weight"`
	AddedAt time.Time `json:"added_at"`
}

// ItemRemovedEvent contains data for item removal events
type ItemRemovedEvent struct {
	TableID   string    `json:"table_id"`
	ItemID    string    `json:"item_id"`
	Reason    string    `json:"reason,omitempty"`
	RemovedAt time.Time `json:"removed_at"`
}

// WeightChangedEvent contains data for weight modification events
type WeightChangedEvent struct {
	TableID   string    `json:"table_id"`
	ItemID    string    `json:"item_id"`
	OldWeight int       `json:"old_weight"`
	NewWeight int       `json:"new_weight"`
	ChangedAt time.Time `json:"changed_at"`
}

// SelectionStartedEvent contains data for selection start events
type SelectionStartedEvent struct {
	TableID       string    `json:"table_id"`
	Operation     string    `json:"operation"`
	RequestCount  int       `json:"request_count"`
	SelectionMode string    `json:"selection_mode"`
	StartedAt     time.Time `json:"started_at"`
}

// SelectionCompletedEvent contains data for successful selection events
type SelectionCompletedEvent struct {
	TableID       string    `json:"table_id"`
	Operation     string    `json:"operation"`
	RequestCount  int       `json:"request_count"`
	ActualCount   int       `json:"actual_count"`
	SelectionMode string    `json:"selection_mode"`
	DurationMs    int       `json:"duration_ms"`
	CompletedAt   time.Time `json:"completed_at"`
}

// SelectionFailedEvent contains data for failed selection events
type SelectionFailedEvent struct {
	TableID       string    `json:"table_id"`
	Operation     string    `json:"operation"`
	RequestCount  int       `json:"request_count"`
	SelectionMode string    `json:"selection_mode"`
	Error         string    `json:"error"`
	FailedAt      time.Time `json:"failed_at"`
}

// ContextModifiedEvent contains data for context modification events
type ContextModifiedEvent struct {
	TableID       string            `json:"table_id"`
	ContextType   string            `json:"context_type"`
	Modifications map[string]string `json:"modifications"`
	ModifiedAt    time.Time         `json:"modified_at"`
}
