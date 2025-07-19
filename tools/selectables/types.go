package selectables

// SelectionMode defines the behavior for multi-item selection operations
// Purpose: Categorizes different selection behaviors to ensure consistent
// handling across different table implementations.
type SelectionMode string

const (
	// SelectionModeMultiple allows selecting the same item multiple times (with replacement)
	// Each selection is independent and previous selections don't affect future ones
	SelectionModeMultiple SelectionMode = "multiple"

	// SelectionModeUnique prevents duplicate selections in multi-item rolls (without replacement)
	// Once an item is selected, it's temporarily removed from consideration for that operation
	SelectionModeUnique SelectionMode = "unique"
)

// WeightModifierType defines how context values modify selection weights
// Purpose: Provides standardized ways for context to affect selection probability
// without requiring custom logic in every table implementation.
type WeightModifierType string

const (
	// WeightModifierMultiplier multiplies the base weight by the context value
	// Example: base weight 10 * multiplier 1.5 = effective weight 15
	WeightModifierMultiplier WeightModifierType = "multiplier"

	// WeightModifierAdditive adds the context value to the base weight
	// Example: base weight 10 + additive 5 = effective weight 15
	WeightModifierAdditive WeightModifierType = "additive"

	// WeightModifierOverride replaces the base weight with the context value
	// Example: base weight 10, override 20 = effective weight 20
	WeightModifierOverride WeightModifierType = "override"

	// WeightModifierDisable sets the effective weight to 0, preventing selection
	// Used for conditional exclusions based on game state
	WeightModifierDisable WeightModifierType = "disable"
)

// SelectionEvent represents different events that can occur during selection operations
// Purpose: Provides event types for debugging, analytics, and game integration
// through the RPG toolkit's event system.
type SelectionEvent string

const (
	// EventSelectionMade fires when a selection operation completes successfully
	EventSelectionMade SelectionEvent = "selectables.selection_made"

	// EventTableModified fires when a selection table is modified (items added/removed)
	EventTableModified SelectionEvent = "selectables.table_modified"

	// EventWeightModified fires when context causes weight modifications
	EventWeightModified SelectionEvent = "selectables.weight_modified"
)

// SelectionResult contains the outcome of a selection operation with metadata
// Purpose: Provides rich information about selections for debugging, analytics,
// and game integration. Includes provenance tracking and alternative outcomes.
type SelectionResult[T comparable] struct {
	// Selected contains the items that were chosen
	Selected []T

	// TableID identifies which table produced this result
	TableID string

	// Context contains the selection context that was used
	Context SelectionContext

	// TotalWeight represents the sum of all effective weights during selection
	TotalWeight int

	// Alternatives contains items that could have been selected with their weights
	// Useful for debugging and "what if" analysis
	Alternatives map[T]int

	// RollResults contains the actual dice roll values used for selection
	RollResults []int
}

// WeightedItem represents an item with its associated weight in a selection table
// Purpose: Internal representation of table contents that can be easily
// manipulated and inspected for debugging and analytics.
type WeightedItem[T comparable] struct {
	// Item is the actual content that can be selected
	Item T

	// Weight is the base selection weight for this item
	Weight int

	// EffectiveWeight is the final weight after context modifications
	// This is what's actually used for selection calculations
	EffectiveWeight int

	// Conditions define when this item is eligible for selection
	// Key-value pairs that must match context values for selection
	Conditions map[string]interface{}
}

// TableConfiguration provides options for customizing selection table behavior
// Purpose: Allows fine-tuning of table behavior without requiring separate
// implementations for each variation. Supports performance optimization
// and debugging features.
type TableConfiguration struct {
	// ID uniquely identifies this table for debugging and events
	ID string

	// EnableEvents determines whether selection events are published
	EnableEvents bool

	// EnableDebugging includes additional metadata in selection results
	EnableDebugging bool

	// CacheWeights improves performance by caching weight calculations
	// when the same context is used repeatedly
	CacheWeights bool

	// MinWeight sets the minimum allowed weight for items (default: 1)
	MinWeight int

	// MaxWeight sets the maximum allowed weight for items (default: unlimited)
	MaxWeight int
}
