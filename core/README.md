# RPG Toolkit Core

The core module provides fundamental interfaces and types that form the foundation of the RPG toolkit.

## Overview

This module defines the essential building blocks for creating RPG game systems, including:

- **Entity Interface**: The base interface that all game objects must implement
- **Ref Type**: Type-safe references to game mechanics (features, skills, conditions, etc.)
- **Error Definitions**: Common error types used throughout the toolkit

## Installation

```bash
go get github.com/KirkDiggler/rpg-toolkit/core
```

## Usage

### Implementing the Entity Interface

All game objects in the RPG toolkit must implement the `Entity` interface:

```go
type Entity interface {
    GetID() string
    GetType() string
}
```

Example implementation:

```go
type Character struct {
    id   string
    name string
}

func (c *Character) GetID() string {
    return c.id
}

func (c *Character) GetType() string {
    return "character"
}
```

### Using Refs (Type References)

The `Ref` type provides a type-safe way to reference game mechanics like features, skills, and conditions:

```go
// Define compile-time constants for core features
var Rage = core.MustNewRef("rage", "core", "feature")
var SneakAttack = core.MustNewRef("sneak_attack", "core", "feature")

// Parse from string format
ref, err := core.ParseString("core:feature:rage")
if err != nil {
    return err
}

// Track where features come from
feature := core.NewSourcedRef(Rage, "class:barbarian")

// JSON serialization works automatically
// Outputs: {"id": "core:feature:rage", "source": "class:barbarian"}
```

#### Ref Structure

A Ref consists of three parts:
- **Module**: Which module defined this (`"core"`, `"artificer"`, `"homebrew"`)
- **Type**: Category of mechanic (`"feature"`, `"proficiency"`, `"skill"`, `"condition"`)
- **Value**: The specific identifier (`"rage"`, `"sneak_attack"`)

String format: `module:type:value`

### Error Handling

The core module provides several predefined errors and error types for detailed reporting:

```go
// Using predefined errors
if entity == nil {
    return core.ErrNilEntity
}

// Creating detailed entity errors
err := core.NewEntityError("create", "character", "char-123", core.ErrDuplicateEntity)

// Ref validation errors
ref, err := core.ParseString("invalid::format")
if core.IsParseError(err) {
    // Handle parse error
}
```

## Common Errors

### Entity Errors
- `ErrEntityNotFound`: Entity cannot be found
- `ErrInvalidEntity`: Entity is invalid or malformed
- `ErrDuplicateEntity`: Entity with the same ID already exists
- `ErrNilEntity`: Nil entity provided
- `ErrEmptyEntityID`: Entity has an empty or invalid ID
- `ErrInvalidType`: Entity has an invalid or unrecognized type

### Ref Errors
- `ErrEmptyString`: Ref string is empty
- `ErrInvalidFormat`: String doesn't match expected format
- `ErrEmptyComponent`: One of the Ref components is empty
- `ErrInvalidCharacters`: Component contains invalid characters
- `ErrTooManySegments`: More than 3 segments in string
- `ErrTooFewSegments`: Fewer than 3 segments in string

## Testing

Run tests with:

```bash
go test ./...
```

## License

[Add your license information here]