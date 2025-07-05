# Dice Roller Design Patterns

## Current Design: Global DefaultRoller

We currently use a global `DefaultRoller` variable with `SetDefaultRoller()` function, similar to Go's stdlib patterns like `http.DefaultClient`.

### Pros
- **Simple API**: `dice.D20(1)` is very clean
- **Familiar pattern**: Follows Go stdlib conventions
- **Easy mocking**: Just swap the global for tests

### Cons
- **Global state**: Can cause issues in parallel tests
- **Hidden dependency**: Not obvious from the API
- **Thread safety concerns**: Though our implementation is safe

## Alternative Designs Considered

### 1. Dice Factory Pattern
```go
type DiceFactory struct {
    roller Roller
}

func (df *DiceFactory) D20(count int) *Roll { ... }

// Usage
dice := NewDiceFactory(mockRoller)
roll := dice.D20(1)
```

**Pros**: Explicit dependencies, no global state
**Cons**: More verbose, requires passing factory around

### 2. Context-Based (like slog)
```go
func D20WithContext(ctx context.Context, count int) *Roll {
    roller := getRollerFromContext(ctx)
    // ...
}
```

**Pros**: Fits well with context-based architectures
**Cons**: Verbose, context pollution

### 3. Functional Options
```go
roll := D20(1, WithRoller(mockRoller))
```

**Pros**: Flexible, backwards compatible
**Cons**: More complex implementation

## Decision

We're keeping the global `DefaultRoller` pattern because:

1. **It matches our use case**: In game systems, you typically want one consistent source of randomness
2. **Go stdlib precedent**: Many packages use this pattern successfully
3. **Simple testing**: The pattern is well-understood for testing
4. **Clean API**: The helper functions remain simple

## Testing Best Practices

When using the global pattern:

```go
func TestSomething(t *testing.T) {
    // Save and restore
    original := dice.DefaultRoller
    defer dice.SetDefaultRoller(original)
    
    // Set mock
    dice.SetDefaultRoller(mockRoller)
    
    // Run test
}
```

For parallel tests, use `NewRollWithRoller()` instead of the global helpers.

## Future Considerations

If we find the global state becomes problematic, we could:
1. Add a factory pattern alongside the global (both APIs)
2. Use build tags for test vs production rollers
3. Add mutex protection if needed (currently not needed as CryptoRoller is stateless)