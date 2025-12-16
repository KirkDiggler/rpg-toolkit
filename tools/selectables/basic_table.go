package selectables

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicTable implements the SelectionTable interface with simple weighted selection
// Purpose: Provides a straightforward implementation of weighted random selection
// that supports all standard selection modes and integrates with the RPG toolkit's
// event system for debugging and analytics.
type BasicTable[T comparable] struct {
	// Core table identity
	id     string
	config TableConfiguration

	// Items storage with thread safety
	items map[T]int
	mutex sync.RWMutex

	// Connected typed topics for event publishing
	connectedTopics struct {
		tableCreated       events.TypedTopic[SelectionTableCreatedEvent]
		tableDestroyed     events.TypedTopic[SelectionTableDestroyedEvent]
		itemAdded          events.TypedTopic[ItemAddedEvent]
		itemRemoved        events.TypedTopic[ItemRemovedEvent]
		weightChanged      events.TypedTopic[WeightChangedEvent]
		selectionStarted   events.TypedTopic[SelectionStartedEvent]
		selectionCompleted events.TypedTopic[SelectionCompletedEvent]
		selectionFailed    events.TypedTopic[SelectionFailedEvent]
		contextModified    events.TypedTopic[ContextModifiedEvent]
	}

	// Weight calculation caching for performance
	cachedWeights    map[string]map[T]int // keyed by context hash
	weightCacheMutex sync.RWMutex
	lastModification time.Time
}

// BasicTableConfig provides configuration options for BasicTable creation
// Purpose: Follows the toolkit's config pattern for clean dependency injection
type BasicTableConfig struct {
	// ID uniquely identifies this table for debugging and events
	ID string

	// Configuration customizes table behavior
	Configuration TableConfiguration
}

// NewBasicTable creates a new basic selection table with the specified configuration
// Purpose: Standard constructor following toolkit patterns with proper initialization
func NewBasicTable[T comparable](config BasicTableConfig) SelectionTable[T] {
	if config.ID == "" {
		config.ID = generateTableID()
	}

	// Set default configuration values
	tableConfig := config.Configuration
	if tableConfig.MinWeight <= 0 {
		tableConfig.MinWeight = 1
	}
	if tableConfig.MaxWeight <= 0 {
		tableConfig.MaxWeight = 1000000 // Reasonable default
	}

	table := &BasicTable[T]{
		id:               config.ID,
		config:           tableConfig,
		items:            make(map[T]int),
		cachedWeights:    make(map[string]map[T]int),
		lastModification: time.Now(),
	}

	return table
}

// ConnectToEventBus connects all typed topics to the event bus
func (t *BasicTable[T]) ConnectToEventBus(bus events.EventBus) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Connect all typed topics
	t.connectedTopics.tableCreated = SelectionTableCreatedTopic.On(bus)
	t.connectedTopics.tableDestroyed = SelectionTableDestroyedTopic.On(bus)
	t.connectedTopics.itemAdded = ItemAddedTopic.On(bus)
	t.connectedTopics.itemRemoved = ItemRemovedTopic.On(bus)
	t.connectedTopics.weightChanged = WeightChangedTopic.On(bus)
	t.connectedTopics.selectionStarted = SelectionStartedTopic.On(bus)
	t.connectedTopics.selectionCompleted = SelectionCompletedTopic.On(bus)
	t.connectedTopics.selectionFailed = SelectionFailedTopic.On(bus)
	t.connectedTopics.contextModified = ContextModifiedTopic.On(bus)

	// Publish table creation event if events are enabled
	if t.config.EnableEvents && t.connectedTopics.tableCreated != nil {
		event := SelectionTableCreatedEvent{
			TableID:   t.id,
			TableType: "basic",
			CreatedAt: time.Now(),
		}
		_ = t.connectedTopics.tableCreated.Publish(context.Background(), event)
	}
}

// Add includes an item in the selection table with the specified weight
// Higher weights increase the probability of selection
func (t *BasicTable[T]) Add(item T, weight int) SelectionTable[T] {
	if weight < t.config.MinWeight {
		weight = t.config.MinWeight
	}
	if weight > t.config.MaxWeight {
		weight = t.config.MaxWeight
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	previousWeight, existed := t.items[item]
	t.items[item] = weight
	t.lastModification = time.Now()

	// Clear weight cache since table changed
	if t.config.CacheWeights {
		t.clearWeightCache()
	}

	// Publish item added event
	if t.config.EnableEvents {
		if existed && t.connectedTopics.weightChanged != nil {
			// Weight was changed for existing item
			event := WeightChangedEvent{
				TableID:   t.id,
				ItemID:    fmt.Sprintf("%v", item),
				OldWeight: previousWeight,
				NewWeight: weight,
				ChangedAt: time.Now(),
			}
			_ = t.connectedTopics.weightChanged.Publish(context.Background(), event)
		} else if !existed && t.connectedTopics.itemAdded != nil {
			// New item was added
			event := ItemAddedEvent{
				TableID: t.id,
				ItemID:  fmt.Sprintf("%v", item),
				Weight:  weight,
				AddedAt: time.Now(),
			}
			_ = t.connectedTopics.itemAdded.Publish(context.Background(), event)
		}
	}

	return t
}

// AddTable includes another selection table as a nested option with the specified weight
// This enables hierarchical selection patterns (e.g., roll category, then roll item from category)
// Note: For BasicTable, this converts the nested table to individual items
func (t *BasicTable[T]) AddTable(_ string, table SelectionTable[T], weight int) SelectionTable[T] {
	if weight < t.config.MinWeight {
		weight = t.config.MinWeight
	}
	if weight > t.config.MaxWeight {
		weight = t.config.MaxWeight
	}

	// For basic tables, we flatten nested tables by adding their items
	// More sophisticated hierarchical behavior is handled by specialized table types
	nestedItems := table.GetItems()
	for item, itemWeight := range nestedItems {
		// Combine weights: nested weight * table weight / total nested weight
		totalNestedWeight := 0
		for _, w := range nestedItems {
			totalNestedWeight += w
		}

		if totalNestedWeight > 0 {
			effectiveWeight := (itemWeight * weight) / totalNestedWeight
			if effectiveWeight < t.config.MinWeight {
				effectiveWeight = t.config.MinWeight
			}
			t.Add(item, effectiveWeight)
		}
	}

	return t
}

// Select performs a single weighted random selection from the table
// Returns ErrEmptyTable if the table contains no items
func (t *BasicTable[T]) Select(ctx SelectionContext) (T, error) {
	startTime := time.Now()
	var zeroValue T

	if t.IsEmpty() {
		err := NewSelectionError("select", t.id, ctx, ErrEmptyTable)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select",
				RequestCount:  1,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return zeroValue, err
	}

	if ctx == nil {
		err := NewSelectionError("select", t.id, ctx, ErrContextRequired)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select",
				RequestCount:  1,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return zeroValue, err
	}

	roller := ctx.GetDiceRoller()
	if roller == nil {
		err := NewSelectionError("select", t.id, ctx, ErrDiceRollerRequired)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select",
				RequestCount:  1,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return zeroValue, err
	}

	// Get effective weights (potentially modified by context)
	effectiveWeights, err := t.getEffectiveWeights(ctx)
	if err != nil {
		selectionErr := NewSelectionError("select", t.id, ctx, err)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select",
				RequestCount:  1,
				SelectionMode: "standard",
				Error:         selectionErr.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return zeroValue, selectionErr
	}

	// Calculate total weight
	totalWeight := 0
	for _, weight := range effectiveWeights {
		totalWeight += weight
	}

	if totalWeight <= 0 {
		err := NewSelectionError("select", t.id, ctx, ErrEmptyTable).
			AddDetail("reason", "all items have zero effective weight")
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select",
				RequestCount:  1,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return zeroValue, err
	}

	// Perform weighted random selection
	rollValue, err := roller.Roll(context.Background(), totalWeight)
	if err != nil {
		selectionErr := NewSelectionError("select", t.id, ctx, err)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select",
				RequestCount:  1,
				SelectionMode: "standard",
				Error:         selectionErr.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return zeroValue, selectionErr
	}

	currentWeight := 0
	for item, weight := range effectiveWeights {
		currentWeight += weight
		if rollValue <= currentWeight {
			// Publish successful selection event
			if t.config.EnableEvents && t.connectedTopics.selectionCompleted != nil {
				event := SelectionCompletedEvent{
					TableID:       t.id,
					Operation:     "select",
					RequestCount:  1,
					ActualCount:   1,
					SelectionMode: "standard",
					DurationMs:    int(time.Since(startTime).Milliseconds()),
					CompletedAt:   time.Now(),
				}
				_ = t.connectedTopics.selectionCompleted.Publish(context.Background(), event)
			}
			return item, nil
		}
	}

	// This should never happen, but handle it gracefully
	selectionErr := NewSelectionError("select", t.id, ctx, ErrEmptyTable).
		AddDetail("reason", "selection algorithm failed").
		AddDetail("roll_value", rollValue).
		AddDetail("total_weight", totalWeight)
	if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
		event := SelectionFailedEvent{
			TableID:       t.id,
			Operation:     "select",
			RequestCount:  1,
			SelectionMode: "standard",
			Error:         selectionErr.Error(),
			FailedAt:      time.Now(),
		}
		_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
	}
	return zeroValue, selectionErr
}

// SelectMany performs multiple weighted random selections with replacement
// Each selection is independent and items can be selected multiple times
func (t *BasicTable[T]) SelectMany(ctx SelectionContext, count int) ([]T, error) {
	startTime := time.Now()

	if count < 1 {
		err := NewSelectionError("select_many", t.id, ctx, ErrInvalidCount)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_many",
				RequestCount:  count,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return nil, err
	}

	results := make([]T, count)
	rollResults := make([]int, count)

	for i := 0; i < count; i++ {
		item, err := t.Select(ctx)
		if err != nil {
			selectionErr := NewSelectionError("select_many", t.id, ctx, err).
				AddDetail("completed_selections", i).
				AddDetail("requested_count", count)
			if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
				event := SelectionFailedEvent{
					TableID:       t.id,
					Operation:     "select_many",
					RequestCount:  count,
					SelectionMode: "standard",
					Error:         selectionErr.Error(),
					FailedAt:      time.Now(),
				}
				_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
			}
			return nil, selectionErr
		}
		results[i] = item
		// Note: In a full implementation, we'd capture the actual roll values
		rollResults[i] = 0 // Placeholder
	}

	// Publish successful selection event
	if t.config.EnableEvents && t.connectedTopics.selectionCompleted != nil {
		event := SelectionCompletedEvent{
			TableID:       t.id,
			Operation:     "select_many",
			RequestCount:  count,
			ActualCount:   len(results),
			SelectionMode: "standard",
			DurationMs:    int(time.Since(startTime).Milliseconds()),
			CompletedAt:   time.Now(),
		}
		_ = t.connectedTopics.selectionCompleted.Publish(context.Background(), event)
	}

	return results, nil
}

// SelectUnique performs multiple weighted random selections without replacement
// Once an item is selected, it cannot be selected again in the same operation
func (t *BasicTable[T]) SelectUnique(ctx SelectionContext, count int) ([]T, error) {
	startTime := time.Now()

	if count < 1 {
		err := NewSelectionError("select_unique", t.id, ctx, ErrInvalidCount)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_unique",
				RequestCount:  count,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return nil, err
	}

	if t.IsEmpty() {
		err := NewSelectionError("select_unique", t.id, ctx, ErrEmptyTable)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_unique",
				RequestCount:  count,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return nil, err
	}

	if count > t.Size() {
		err := NewSelectionError("select_unique", t.id, ctx, ErrInsufficientItems).
			AddDetail("requested_count", count).
			AddDetail("available_count", t.Size())
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_unique",
				RequestCount:  count,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return nil, err
	}

	results := make([]T, 0, count)
	used := make(map[T]bool)

	for len(results) < count {
		// Get effective weights excluding already selected items
		effectiveWeights, err := t.getEffectiveWeightsExcluding(ctx, used)
		if err != nil {
			selectionErr := NewSelectionError("select_unique", t.id, ctx, err)
			if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
				event := SelectionFailedEvent{
					TableID:       t.id,
					Operation:     "select_unique",
					RequestCount:  count,
					SelectionMode: "standard",
					Error:         selectionErr.Error(),
					FailedAt:      time.Now(),
				}
				_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
			}
			return nil, selectionErr
		}

		// Calculate total weight
		totalWeight := 0
		for _, weight := range effectiveWeights {
			totalWeight += weight
		}

		if totalWeight <= 0 {
			break // No more selectable items
		}

		// Perform selection
		roller := ctx.GetDiceRoller()
		rollValue, err := roller.Roll(context.Background(), totalWeight)
		if err != nil {
			selectionErr := NewSelectionError("select_unique", t.id, ctx, err)
			if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
				event := SelectionFailedEvent{
					TableID:       t.id,
					Operation:     "select_unique",
					RequestCount:  count,
					SelectionMode: "standard",
					Error:         selectionErr.Error(),
					FailedAt:      time.Now(),
				}
				_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
			}
			return nil, selectionErr
		}

		currentWeight := 0
		for item, weight := range effectiveWeights {
			currentWeight += weight
			if rollValue <= currentWeight && !used[item] {
				results = append(results, item)
				used[item] = true
				break
			}
		}
	}

	if len(results) < count {
		err := NewSelectionError("select_unique", t.id, ctx, ErrInsufficientItems).
			AddDetail("requested_count", count).
			AddDetail("actual_count", len(results))
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_unique",
				RequestCount:  count,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return results, err
	}

	// Publish successful selection event
	if t.config.EnableEvents && t.connectedTopics.selectionCompleted != nil {
		event := SelectionCompletedEvent{
			TableID:       t.id,
			Operation:     "select_unique",
			RequestCount:  count,
			ActualCount:   len(results),
			SelectionMode: "standard",
			DurationMs:    int(time.Since(startTime).Milliseconds()),
			CompletedAt:   time.Now(),
		}
		_ = t.connectedTopics.selectionCompleted.Publish(context.Background(), event)
	}

	return results, nil
}

// SelectVariable performs selection with quantity determined by dice expression
// Combines quantity rolling with item selection in a single operation
func (t *BasicTable[T]) SelectVariable(ctx SelectionContext, diceExpression string) ([]T, error) {
	if ctx == nil {
		err := NewSelectionError("select_variable", t.id, ctx, ErrContextRequired)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_variable",
				RequestCount:  0,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return nil, err
	}

	roller := ctx.GetDiceRoller()
	if roller == nil {
		err := NewSelectionError("select_variable", t.id, ctx, ErrDiceRollerRequired)
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_variable",
				RequestCount:  0,
				SelectionMode: "standard",
				Error:         err.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return nil, err
	}

	// Parse and roll the dice expression
	// For now, implement a simple parser for basic expressions like "1d6", "2d4", etc.
	count, err := t.parseDiceExpression(diceExpression, roller)
	if err != nil {
		selectionErr := NewSelectionError("select_variable", t.id, ctx, ErrInvalidDiceExpression).
			AddDetail("dice_expression", diceExpression).
			AddDetail("parse_error", err.Error())
		if t.config.EnableEvents && t.connectedTopics.selectionFailed != nil {
			event := SelectionFailedEvent{
				TableID:       t.id,
				Operation:     "select_variable",
				RequestCount:  0,
				SelectionMode: "standard",
				Error:         selectionErr.Error(),
				FailedAt:      time.Now(),
			}
			_ = t.connectedTopics.selectionFailed.Publish(context.Background(), event)
		}
		return nil, selectionErr
	}
	if count < 1 {
		count = 1 // Ensure at least one selection
	}

	// Delegate to SelectMany
	return t.SelectMany(ctx, count)
}

// GetItems returns all items in the table with their weights for inspection
// Useful for debugging and analytics
func (t *BasicTable[T]) GetItems() map[T]int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	result := make(map[T]int)
	for item, weight := range t.items {
		result[item] = weight
	}
	return result
}

// IsEmpty returns true if the table contains no selectable items
func (t *BasicTable[T]) IsEmpty() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return len(t.items) == 0
}

// Size returns the total number of items in the table
func (t *BasicTable[T]) Size() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return len(t.items)
}

// Helper methods for internal operations

// getEffectiveWeights calculates the effective weights for all items based on context
func (t *BasicTable[T]) getEffectiveWeights(ctx SelectionContext) (map[T]int, error) {
	// Check cache first if caching is enabled
	if t.config.CacheWeights {
		contextHash := t.hashContext(ctx)
		t.weightCacheMutex.RLock()
		if cached, exists := t.cachedWeights[contextHash]; exists {
			t.weightCacheMutex.RUnlock()
			return cached, nil
		}
		t.weightCacheMutex.RUnlock()
	}

	t.mutex.RLock()
	result := make(map[T]int)
	for item, baseWeight := range t.items {
		result[item] = baseWeight
	}
	t.mutex.RUnlock()

	// Apply context modifications (in future iterations)
	// For now, just return base weights

	// Cache the result if caching is enabled
	if t.config.CacheWeights {
		contextHash := t.hashContext(ctx)
		t.weightCacheMutex.Lock()
		t.cachedWeights[contextHash] = result
		t.weightCacheMutex.Unlock()
	}

	return result, nil
}

// getEffectiveWeightsExcluding calculates effective weights excluding specified items
func (t *BasicTable[T]) getEffectiveWeightsExcluding(ctx SelectionContext, excluded map[T]bool) (map[T]int, error) {
	allWeights, err := t.getEffectiveWeights(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[T]int)
	for item, weight := range allWeights {
		if !excluded[item] {
			result[item] = weight
		}
	}

	return result, nil
}

// hashContext creates a simple hash of the context for caching purposes
func (t *BasicTable[T]) hashContext(ctx SelectionContext) string {
	if ctx == nil {
		return "nil"
	}

	keys := ctx.Keys()
	sort.Strings(keys)

	hash := ""
	for _, key := range keys {
		if value, exists := ctx.Get(key); exists {
			hash += key + "=" + toString(value) + ";"
		}
	}

	return hash
}

// clearWeightCache clears the weight calculation cache
func (t *BasicTable[T]) clearWeightCache() {
	t.weightCacheMutex.Lock()
	defer t.weightCacheMutex.Unlock()
	t.cachedWeights = make(map[string]map[T]int)
}

// parseDiceExpression parses and rolls a simple dice expression
// For now supports basic expressions like "1d6", "2d4", etc.
func (t *BasicTable[T]) parseDiceExpression(expression string, roller dice.Roller) (int, error) {
	// Very simple parser for basic dice expressions
	// More sophisticated parsing can be added later
	ctx := context.Background()

	// Handle simple cases first
	switch expression {
	case "1d1-1":
		// Special case: 1d1-1 could result in 0, but we ensure minimum of 1
		result, err := roller.Roll(ctx, 1)
		if err != nil {
			return 0, err
		}
		result--
		if result < 1 {
			result = 1
		}
		return result, nil
	case "1d4":
		return roller.Roll(ctx, 4)
	case "1d6":
		return roller.Roll(ctx, 6)
	case "1d8":
		return roller.Roll(ctx, 8)
	case "1d10":
		return roller.Roll(ctx, 10)
	case "1d10+2":
		result, err := roller.Roll(ctx, 10)
		if err != nil {
			return 0, err
		}
		return result + 2, nil
	case "1d12":
		return roller.Roll(ctx, 12)
	case "1d20":
		return roller.Roll(ctx, 20)
	case "2d4":
		results, err := roller.RollN(ctx, 2, 4)
		if err != nil {
			return 0, err
		}
		sum := 0
		for _, r := range results {
			sum += r
		}
		return sum, nil
	case "2d6":
		results, err := roller.RollN(ctx, 2, 6)
		if err != nil {
			return 0, err
		}
		sum := 0
		for _, r := range results {
			sum += r
		}
		return sum, nil
	case "3d6":
		results, err := roller.RollN(ctx, 3, 6)
		if err != nil {
			return 0, err
		}
		sum := 0
		for _, r := range results {
			sum += r
		}
		return sum, nil
	default:
		return 0, fmt.Errorf("unsupported dice expression: %s", expression)
	}
}

// Utility functions

// generateTableID creates a unique identifier for a table
func generateTableID() string {
	return "table_" + toString(time.Now().UnixNano())
}

// toString converts various types to strings for hashing and display
func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return string(rune(v)) // Simplified conversion
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return "unknown"
	}
}

// tableEntity is a minimal implementation of core.Entity for event publishing
type tableEntity struct {
	id        string
	tableType string
}

// GetID returns the table entity's unique identifier
func (e *tableEntity) GetID() string { return e.id }

// GetType returns the table entity's type
func (e *tableEntity) GetType() core.EntityType { return core.EntityType(e.tableType) }

// Ensure tableEntity implements core.Entity
var _ core.Entity = (*tableEntity)(nil)
