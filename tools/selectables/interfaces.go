// Package selectables provides universal weighted random selection functionality for any content type.
// This tool enables procedural content generation across the RPG toolkit by offering flexible,
// context-aware selection tables that integrate with the existing dice and events systems.
package selectables

import (
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// SelectionTable provides weighted random selection for any content type
// Purpose: Core interface for all grabbag/loot table functionality that works
// with any type T through Go generics. Supports single selection, multiple selection,
// and unique selection modes with optional context-based weight modification.
type SelectionTable[T comparable] interface {
	// Add includes an item in the selection table with the specified weight
	// Higher weights increase the probability of selection
	Add(item T, weight int) SelectionTable[T]

	// AddTable includes another selection table as a nested option with the specified weight
	// This enables hierarchical selection patterns (e.g., roll category, then roll item from category)
	AddTable(name string, table SelectionTable[T], weight int) SelectionTable[T]

	// Select performs a single weighted random selection from the table
	// Returns ErrEmptyTable if the table contains no items
	Select(ctx SelectionContext) (T, error)

	// SelectMany performs multiple weighted random selections with replacement
	// Each selection is independent and items can be selected multiple times
	// Returns ErrEmptyTable if the table contains no items
	// Returns ErrInvalidCount if count is less than 1
	SelectMany(ctx SelectionContext, count int) ([]T, error)

	// SelectUnique performs multiple weighted random selections without replacement
	// Once an item is selected, it cannot be selected again in the same operation
	// Returns ErrEmptyTable if the table contains no items
	// Returns ErrInvalidCount if count is less than 1 or greater than available items
	SelectUnique(ctx SelectionContext, count int) ([]T, error)

	// SelectVariable performs selection with quantity determined by dice expression
	// Combines quantity rolling with item selection in a single operation
	// Returns ErrEmptyTable if the table contains no items
	// Returns ErrInvalidDiceExpression if the expression cannot be parsed
	SelectVariable(ctx SelectionContext, diceExpression string) ([]T, error)

	// GetItems returns all items in the table with their weights for inspection
	// Useful for debugging and analytics
	GetItems() map[T]int

	// IsEmpty returns true if the table contains no selectable items
	IsEmpty() bool

	// Size returns the total number of items in the table
	Size() int
}

// SelectionContext provides conditional selection parameters and game state
// Purpose: Allows selection tables to modify weights based on game conditions
// like player level, environment, time of day, etc. Context values are used
// by conditional tables to dynamically adjust selection probabilities.
type SelectionContext interface {
	// Get retrieves a context value by key
	// Returns the value and true if found, nil and false if not found
	Get(key string) (interface{}, bool)

	// Set stores a context value by key
	// Returns a new context with the value set (immutable pattern)
	Set(key string, value interface{}) SelectionContext

	// GetDiceRoller returns the dice roller for this selection context
	// Used for randomization during selection operations
	GetDiceRoller() dice.Roller

	// SetDiceRoller returns a new context with the specified dice roller
	SetDiceRoller(roller dice.Roller) SelectionContext

	// Keys returns all available context keys for inspection
	Keys() []string
}

// SelectionFilter provides optional filtering logic for selection operations
// Purpose: Allows games to implement custom exclusion rules without forcing
// the selectables tool to understand game-specific logic. Used with SelectUnique
// and other operations where conditional exclusions are needed.
type SelectionFilter[T comparable] func(selected []T, candidate T) bool

// SelectionCallback provides hooks for post-selection operations
// Purpose: Allows games to implement side effects like inventory depletion,
// state tracking, or other game-specific logic after selections are made.
type SelectionCallback[T comparable] func(selected T)

// TableBuilder provides fluent interface for constructing selection tables
// Purpose: Simplifies table creation with method chaining and reduces
// boilerplate for common table construction patterns.
type TableBuilder[T comparable] interface {
	// Add includes an item with the specified weight
	Add(item T, weight int) TableBuilder[T]

	// AddTable includes a nested table with the specified weight
	AddTable(name string, table SelectionTable[T], weight int) TableBuilder[T]

	// Build creates the final selection table
	Build() SelectionTable[T]
}
