package selectables

import (
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// BasicSelectionContext implements the SelectionContext interface
// Purpose: Provides a simple, immutable context implementation that stores
// key-value pairs for conditional selection and includes dice roller integration.
// Uses copy-on-write semantics to ensure thread safety and immutability.
type BasicSelectionContext struct {
	// values stores the context key-value pairs
	values map[string]interface{}

	// diceRoller provides randomization for selection operations
	diceRoller dice.Roller
}

// NewBasicSelectionContext creates a new selection context with a default dice roller
// Purpose: Standard constructor that provides sensible defaults for most use cases
func NewBasicSelectionContext() SelectionContext {
	return &BasicSelectionContext{
		values:     make(map[string]interface{}),
		diceRoller: &dice.CryptoRoller{},
	}
}

// NewSelectionContextWithRoller creates a new selection context with a specific dice roller
// Purpose: Allows customization of randomization behavior for testing or specific game needs
func NewSelectionContextWithRoller(roller dice.Roller) SelectionContext {
	return &BasicSelectionContext{
		values:     make(map[string]interface{}),
		diceRoller: roller,
	}
}

// Get retrieves a context value by key
// Returns the value and true if found, nil and false if not found
func (c *BasicSelectionContext) Get(key string) (interface{}, bool) {
	value, exists := c.values[key]
	return value, exists
}

// Set stores a context value by key
// Returns a new context with the value set (immutable pattern)
// This ensures thread safety and prevents accidental modification
func (c *BasicSelectionContext) Set(key string, value interface{}) SelectionContext {
	newValues := make(map[string]interface{})
	for k, v := range c.values {
		newValues[k] = v
	}
	newValues[key] = value

	return &BasicSelectionContext{
		values:     newValues,
		diceRoller: c.diceRoller,
	}
}

// GetDiceRoller returns the dice roller for this selection context
// Used for randomization during selection operations
func (c *BasicSelectionContext) GetDiceRoller() dice.Roller {
	return c.diceRoller
}

// SetDiceRoller returns a new context with the specified dice roller
// Maintains immutability while allowing dice roller customization
func (c *BasicSelectionContext) SetDiceRoller(roller dice.Roller) SelectionContext {
	newValues := make(map[string]interface{})
	for k, v := range c.values {
		newValues[k] = v
	}

	return &BasicSelectionContext{
		values:     newValues,
		diceRoller: roller,
	}
}

// Keys returns all available context keys for inspection
// Useful for debugging and understanding what context is available
func (c *BasicSelectionContext) Keys() []string {
	keys := make([]string, 0, len(c.values))
	for key := range c.values {
		keys = append(keys, key)
	}
	return keys
}

// ContextBuilder provides a fluent interface for building selection contexts
// Purpose: Simplifies context creation with method chaining and type-safe value setting
type ContextBuilder struct {
	context SelectionContext
}

// NewContextBuilder creates a new context builder with a default context
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		context: NewBasicSelectionContext(),
	}
}

// NewContextBuilderWithRoller creates a new context builder with a specific dice roller
func NewContextBuilderWithRoller(roller dice.Roller) *ContextBuilder {
	return &ContextBuilder{
		context: NewSelectionContextWithRoller(roller),
	}
}

// Set adds a key-value pair to the context being built
func (b *ContextBuilder) Set(key string, value interface{}) *ContextBuilder {
	b.context = b.context.Set(key, value)
	return b
}

// SetString adds a string value with type safety
func (b *ContextBuilder) SetString(key, value string) *ContextBuilder {
	return b.Set(key, value)
}

// SetInt adds an integer value with type safety
func (b *ContextBuilder) SetInt(key string, value int) *ContextBuilder {
	return b.Set(key, value)
}

// SetBool adds a boolean value with type safety
func (b *ContextBuilder) SetBool(key string, value bool) *ContextBuilder {
	return b.Set(key, value)
}

// SetFloat adds a float64 value with type safety
func (b *ContextBuilder) SetFloat(key string, value float64) *ContextBuilder {
	return b.Set(key, value)
}

// WithDiceRoller sets the dice roller for the context
func (b *ContextBuilder) WithDiceRoller(roller dice.Roller) *ContextBuilder {
	b.context = b.context.SetDiceRoller(roller)
	return b
}

// Build returns the final selection context
func (b *ContextBuilder) Build() SelectionContext {
	return b.context
}

// ContextHelper provides utility functions for common context operations
// Purpose: Reduces boilerplate for typical context value extraction and type conversion

// GetStringValue retrieves a string value from context with default fallback
func GetStringValue(ctx SelectionContext, key, defaultValue string) string {
	if value, exists := ctx.Get(key); exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetIntValue retrieves an integer value from context with default fallback
func GetIntValue(ctx SelectionContext, key string, defaultValue int) int {
	if value, exists := ctx.Get(key); exists {
		if intVal, ok := value.(int); ok {
			return intVal
		}
	}
	return defaultValue
}

// GetBoolValue retrieves a boolean value from context with default fallback
func GetBoolValue(ctx SelectionContext, key string, defaultValue bool) bool {
	if value, exists := ctx.Get(key); exists {
		if boolVal, ok := value.(bool); ok {
			return boolVal
		}
	}
	return defaultValue
}

// GetFloatValue retrieves a float64 value from context with default fallback
func GetFloatValue(ctx SelectionContext, key string, defaultValue float64) float64 {
	if value, exists := ctx.Get(key); exists {
		if floatVal, ok := value.(float64); ok {
			return floatVal
		}
	}
	return defaultValue
}
