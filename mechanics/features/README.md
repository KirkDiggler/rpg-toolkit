# Features Package

This package provides the infrastructure for game features (abilities, traits, feats, etc).

## What This Package Provides

- **Feature Interface**: The contract all features must implement
- **SimpleFeature**: Base implementation with common functionality
- **Error Types**: Well-defined errors for better developer communication
- **FeatureData Loader**: Extract feature refs and JSON for routing

## What This Package Does NOT Provide

- Specific feature implementations (those belong in rulebooks)
- Feature registration or routing (handled at application level)
- Game-specific logic

## Usage Pattern

The typical flow for loading features:

```go
// 1. Load feature data from your database
jsonData := loadFromDatabase()

// 2. Use features.Load to extract ref and JSON
data, err := features.Load(jsonData)
if err != nil {
    return err
}

// 3. Route based on the ref to the appropriate rulebook
var feat features.Feature
switch data.Ref().Module {
case "dnd5e":
    feat, err = dnd5e.LoadFeature(data.JSON())
case "pathfinder":
    feat, err = pathfinder.LoadFeature(data.JSON())
case "homebrew":
    feat, err = homebrew.LoadFeature(data.JSON())
default:
    return fmt.Errorf("unknown module: %s", data.Ref().Module)
}

// 4. Use the feature through the interface
feat.Apply(eventBus)
```

## Creating Features

Features should:
1. Embed `*SimpleFeature` for common functionality
2. Implement feature-specific logic
3. Live in their appropriate rulebook package

Example structure:
```
rulebooks/
  dnd5e/
    features/
      rage.go         # Implements Rage using SimpleFeature
      second_wind.go  # Implements Second Wind
      loader.go       # D&D 5e specific loading logic
```

## Error Handling

The package provides typed errors for clear communication:

- `ErrAlreadyActive`: Feature is already active
- `ErrNoUsesRemaining`: No uses left
- `ErrTargetRequired`: Feature needs a target
- `ErrInvalidRef`: Malformed feature reference

Use `errors.Is()` to check for specific conditions.