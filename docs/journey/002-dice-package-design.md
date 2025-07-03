# Dice Package Design

Date: 2025-01-03

## Goals

Create a dice package that:
1. Provides real randomness for production
2. Supports deterministic testing via mocks
3. Implements `events.ModifierValue` for event integration
4. Keeps it simple - no parsing yet

## Core Design

```go
package dice

// Roller is the interface for random number generation
type Roller interface {
    // Roll returns a random number from 1 to size (inclusive)
    Roll(size int) int
    
    // RollN rolls count dice of the given size
    RollN(count, size int) []int
}

// DefaultRoller uses crypto/rand for production
var DefaultRoller Roller = &CryptoRoller{}

// For testing
type MockRoller struct {
    results []int
    index   int
}
```

## ModifierValue Implementations

```go
// Roll implements events.ModifierValue with actual dice rolling
type Roll struct {
    Count  int
    Size   int
    roller Roller // Injected for testing
}

// NewRoll creates a dice roll modifier using DefaultRoller
func NewRoll(count, size int) *Roll {
    return &Roll{count, size, DefaultRoller}
}

// GetValue rolls the dice and returns total
func (r *Roll) GetValue() int {
    rolls := r.roller.RollN(r.Count, r.Size)
    total := 0
    for _, roll := range rolls {
        total += roll
    }
    return total
}

// GetDescription returns "2d6[4,2]=6"
func (r *Roll) GetDescription() string {
    rolls := r.roller.RollN(r.Count, r.Size)
    // Format with individual rolls shown
}
```

## Usage Example

```go
// In production
modifier := dice.NewRoll(2, 6)  // 2d6

// In tests
mockRoller := &MockRoller{results: []int{4, 2}}
modifier := &Roll{2, 6, mockRoller}
```

## What We're NOT Doing Yet

1. **No string parsing** - Use `dice.D6(2)` not `dice.Parse("2d6")`
2. **No advantage/disadvantage** - That's rulebook-specific
3. **No complex expressions** - Just simple rolls for now
4. **No roll history** - Keep it stateless

## Testing Strategy

```go
func TestDamageCalculation(t *testing.T) {
    // Arrange
    mockRoller := &MockRoller{results: []int{6, 4}} // Predictable rolls
    
    // Act
    damage := calculateDamage(mockRoller)
    
    // Assert
    assert.Equal(t, 10, damage) // We know it rolled 6 and 4
}
```

## Open Questions

1. Should `Roll` cache its result or re-roll each time `GetValue()` is called?
2. Do we need a `RollResult` type that captures the individual dice?
3. How do we handle invalid dice (d3, d7)?

## Next PR

This design is small enough for a single PR:
- Core interfaces
- Crypto implementation  
- Mock implementation
- ModifierValue implementations
- Tests showing both real and mocked usage