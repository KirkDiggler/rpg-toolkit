# Dice Package

The dice package provides cryptographically secure dice rolling for RPG mechanics.

## Features

- **Cryptographically secure randomness** using `crypto/rand`
- **Mockable interface** for deterministic testing  
- **ModifierValue implementation** for event system integration
- **Cached results** - dice are rolled once when needed
- **Support for negative dice** (penalties)
- **Proper error handling** - no panics, returns errors instead

## Installation

```bash
go get github.com/KirkDiggler/rpg-toolkit/dice
```

## Usage

### Basic Rolling

```go
// Create a 2d6 roll
roll := dice.D6(2)

// Get the total (rolls the dice if not already rolled)
total := roll.GetValue()

// Get a description showing the individual rolls
desc := roll.GetDescription() // "+2d6[4,2]=6"

// Check for errors (e.g., if crypto/rand fails)
if err := roll.Err(); err != nil {
    // Handle error
}

// Or create rolls with custom sizes (returns error for invalid sizes)
roll, err := dice.NewRoll(3, 8) // 3d8
if err != nil {
    // Handle invalid die size
}
```

### With Event System

```go
// In a damage calculation handler
e.Context().AddModifier(events.NewModifier(
    "sneak_attack",
    events.ModifierDamageBonus,
    dice.D6(3), // 3d6 sneak attack damage
    150,
))
```

### Testing with Mocks

The package uses gomock for test mocking:

```go
import (
    mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
    "go.uber.org/mock/gomock"
)

// Create a mock controller
ctrl := gomock.NewController(t)
defer ctrl.Finish()

// Create mock roller with expectations
mockRoller := mock_dice.NewMockRoller(ctrl)
mockRoller.EXPECT().RollN(2, 6).Return([]int{4, 5}, nil)

// Use it for a specific roll
roll, err := dice.NewRollWithRoller(2, 6, mockRoller)
if err != nil {
    // Handle error
}
value := roll.GetValue() // Always 9 (4+5)

// Or set as default for all rolls
original := dice.DefaultRoller
dice.SetDefaultRoller(mockRoller)
defer dice.SetDefaultRoller(original) // Restore after test
```

### Generating Mocks

To regenerate the mocks after interface changes:

```bash
go generate ./dice/...
```

## API

### Types

- `Roller` - Interface for random number generation
- `CryptoRoller` - Production implementation using crypto/rand
- `Roll` - Dice roll that implements `events.ModifierValue`

### Interfaces

```go
type Roller interface {
    Roll(size int) (int, error)           // Roll a single die of given size
    RollN(count, size int) ([]int, error) // Roll multiple dice
}
```

### Helper Functions

- `D4(count)` - Create d4 rolls
- `D6(count)` - Create d6 rolls  
- `D8(count)` - Create d8 rolls
- `D10(count)` - Create d10 rolls
- `D12(count)` - Create d12 rolls
- `D20(count)` - Create d20 rolls
- `D100(count)` - Create d100 rolls

### Negative Dice

Negative counts represent penalties:

```go
penalty := dice.D4(-1) // -1d4
value := penalty.GetValue() // Returns negative value
desc := penalty.GetDescription() // "-d4[3]=-3"
```

## Design Decisions

1. **Crypto/rand over math/rand**: Security is important for online games
2. **Cached rolls**: Once rolled, the value doesn't change (immutable)
3. **Error returns over panics**: Idiomatic Go error handling
4. **No parsing**: Use `D6(2)` not `Parse("2d6")` for simplicity
5. **ModifierValue compatibility**: Errors are handled gracefully, returning 0 on error

## Testing

The package has 96.5% test coverage, including:
- Unit tests for all dice sizes
- Mock testing with gomock
- Error handling for invalid inputs
- Cryptographic randomness distribution tests
- Error propagation through ModifierValue interface

Run tests:
```bash
go test ./dice/...
```

Check coverage:
```bash
go test -cover ./dice/...
```