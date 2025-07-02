# RPG Toolkit Core

The core module provides fundamental interfaces and types that form the foundation of the RPG toolkit.

## Overview

This module defines the essential building blocks for creating RPG game systems, including:

- **Entity Interface**: The base interface that all game objects must implement
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

### Error Handling

The core module provides several predefined errors and an `EntityError` type for detailed error reporting:

```go
// Using predefined errors
if entity == nil {
    return core.ErrNilEntity
}

// Creating detailed entity errors
err := core.NewEntityError("create", "character", "char-123", core.ErrDuplicateEntity)
```

## Common Errors

- `ErrEntityNotFound`: Entity cannot be found
- `ErrInvalidEntity`: Entity is invalid or malformed
- `ErrDuplicateEntity`: Entity with the same ID already exists
- `ErrNilEntity`: Nil entity provided
- `ErrEmptyID`: Entity has an empty or invalid ID
- `ErrInvalidType`: Entity has an invalid or unrecognized type

## Testing

Run tests with:

```bash
go test ./...
```

## License

[Add your license information here]