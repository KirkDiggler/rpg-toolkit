# Character System Design - Final

## Overview

The character system provides a data-driven interface between game servers and D&D 5e rules. Game servers store character data as JSON and use the toolkit to apply game mechanics.

## Core Architecture

### Layers

```
Game Server (Consumer)
    ↓ Stores JSON data
    ↓ Uses toolkit-aware wrapper
    ↓ Calls toolkit functions
Toolkit (This Package)
    ↓ Provides refs and builders
    ↓ Compiles choices to data
    ↓ Loads self-contained characters
    ↓ Implements game mechanics
```

### Key Principles

1. **Data-Driven** - Everything is data with refs
2. **Self-Contained** - Character data needs no external dependencies
3. **Domain-Organized** - Refs organized by domain packages
4. **Game Server Owns Flow** - Toolkit provides tools, not opinions

## Data Structures

### Simplified ChoiceData

```go
type ChoiceData struct {
    Ref       *core.Ref   `json:"ref"`       // What choice was made
    Source    *core.Ref   `json:"source"`    // What granted this choice
    Selected  []*core.Ref `json:"selected"`  // What was selected
}
```

### Self-Contained Character Data

```go
type Data struct {
    // Identity
    ID       string `json:"id"`
    Name     string `json:"name"`
    Level    int    `json:"level"`
    
    // Reference refs (for display only)
    RaceRef       *core.Ref `json:"race_ref"`
    ClassRef      *core.Ref `json:"class_ref"`
    BackgroundRef *core.Ref `json:"background_ref"`
    
    // Compiled attributes (no external deps needed)
    AbilityScores AbilityScores     `json:"ability_scores"`
    HP           int                `json:"hp"`
    MaxHP        int                `json:"max_hp"`
    Speed        int                `json:"speed"`        // From race
    Size         string              `json:"size"`         // From race
    
    // Refs for capabilities
    Skills        []*core.Ref        `json:"skills"`       // Skill refs
    Languages     []*core.Ref        `json:"languages"`    // Language refs
    Proficiencies []*core.Ref        `json:"proficiencies"` // Proficiency refs
    
    // Complex data with embedded refs
    Features      []json.RawMessage  `json:"features"`     // Feature data
    Conditions    []json.RawMessage  `json:"conditions"`   // Active conditions
    
    // Tracking
    Choices       []ChoiceData       `json:"choices"`      // Player selections
}
```

## Ref Organization

### Domain Packages

```
rulebooks/dnd5e/
├── races/
│   └── refs.go       # races.Ref(index), races.Human constant
├── classes/
│   └── refs.go       # classes.Ref(index), classes.Barbarian constant
├── skills/
│   └── refs.go       # skills.Ref(index), skills.Athletics constant
├── choices/
│   └── refs.go       # choices.Ref(type), choices.Race constant
├── features/
│   └── refs.go       # features.Ref(index), features.Rage constant
├── conditions/
│   └── refs.go       # conditions.Ref(index), conditions.Raging constant
└── system/
    └── refs.go       # system.Creation, system.LevelUp constants
```

### Example Usage

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
)

// Building refs
raceRef := races.Ref("dwarf")        // "dnd5e:race:dwarf"
choiceRef := choices.Race             // "dnd5e:choice:race" (constant)
```

## Toolkit Interfaces

### Character Compiler

```go
// Compiles choices and API data into self-contained character data
type Compiler interface {
    CompileCharacter(input CompileInput) (*Data, error)
}

type CompileInput struct {
    ID       string
    PlayerID string
    Choices  []ChoiceData
    
    // API data for reference during compilation
    // Everything needed gets compiled into the output
    RaceData       *RaceAPIData
    ClassData      *ClassAPIData
    BackgroundData *BackgroundAPIData
}
```

### Character Loading

```go
// Load character from self-contained data (no external deps!)
func LoadCharacterFromData(data *Data) (*Character, error)

// Character methods
type Character interface {
    // Combat
    Attack(ctx context.Context, weapon *core.Ref) AttackResult
    TakeDamage(amount int, damageType *core.Ref)
    
    // Features
    ActivateFeature(ctx context.Context, featureRef *core.Ref) error
    
    // Conditions
    HasCondition(ref *core.Ref) bool
    ApplyToEventBus(ctx context.Context, bus events.EventBus) error
    
    // Persistence
    ToData() *Data
}
```

### Choice Validation

```go
type Validator interface {
    // Validate a choice is allowed
    ValidateChoice(choice ChoiceData, current []ChoiceData) error
    
    // Get available options for a choice type
    GetAvailableOptions(choiceRef *core.Ref, current []ChoiceData) []*core.Ref
}
```

## Game Server Flow

### Character Creation

```go
// 1. Collect player choices from UI
uiChoices := collectFromUI()

// 2. Fetch API data
raceData := apiClient.GetRace(uiChoices.RaceID)
classData := apiClient.GetClass(uiChoices.ClassID)

// 3. Build choices with refs (game server wrapper does this)
choices := []character.ChoiceData{
    {
        Ref:      choices.Race,
        Source:   system.Creation,
        Selected: []*core.Ref{races.Ref(raceData.Index)},
    },
    {
        Ref:      choices.Class,
        Source:   system.Creation,
        Selected: []*core.Ref{classes.Ref(classData.Index)},
    },
}

// 4. Compile character using toolkit
compiler := character.NewCompiler()
charData, _ := compiler.CompileCharacter(character.CompileInput{
    ID:       generateID(),
    Choices:  choices,
    RaceData: convertAPIData(raceData),
    // ...
})

// 5. Save self-contained data
database.Save(charData)
```

### Gameplay

```go
// 1. Load self-contained data
charData := database.Load(characterID)

// 2. Create character (no external deps!)
character := character.LoadCharacterFromData(charData)

// 3. Apply to event bus for features/conditions
character.ApplyToEventBus(ctx, eventBus)

// 4. Play
character.Attack(ctx, weapons.Longsword)

// 5. Save changes
database.Save(character.ToData())
```

## What We're Building

### Phase 1: Core Structure
1. Domain ref packages (races, classes, skills, etc.)
2. Simplified ChoiceData with refs
3. Self-contained Data structure

### Phase 2: Compilation
1. Character compiler interface
2. Choice to data compilation
3. Remove need for external deps in LoadCharacterFromData

### Phase 3: Migration
1. Deprecate old ChoiceData with typed fields
2. Remove Draft/Builder pattern
3. Update all character creation flows

## Benefits

1. **Simple** - Just refs, no complex type switching
2. **Type-Safe** - Proper refs with validation
3. **Self-Contained** - No runtime dependencies
4. **Extensible** - New content just needs new refs
5. **Clear Boundaries** - Toolkit provides tools, game server owns flow

## Not in Scope

- ❌ Factories for specific classes
- ❌ Multi-step builders
- ❌ Database concerns
- ❌ API client implementation
- ❌ UI flow management