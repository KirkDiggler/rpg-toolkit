# D&D 5e Rulebook

This package implements Dungeons & Dragons 5th Edition rules using the rpg-toolkit infrastructure.

## Overview

The D&D 5e rulebook provides:
- Character creation with the builder pattern
- Rich domain models with game mechanics (Attack, SaveThrow, etc.)
- Data structures for persistence
- Validation of D&D 5e rules
- Clear separation between game logic and storage

## Package Structure

The D&D 5e rulebook is organized into bounded contexts:

```
dnd5e/
├── character/     # Character creation, persistence, validation
├── combat/        # Attack rolls, damage, initiative
├── magic/         # Spells, spell slots, casting mechanics
├── equipment/     # Items, inventory, attunement
├── rules/         # Core calculations, modifiers
├── shared/        # Shared types (AbilityScores, etc.)
└── dnd5e.go       # Package facade for easy imports
```

### Usage

You can import the entire package:
```go
import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"

// Use the facade
builder := dnd5e.NewCharacterBuilder(loader)
char := &dnd5e.Character{}
```

Or import specific contexts:
```go
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)
```

## Key Concepts

### Character Data vs Game Data

The rulebook separates character state from game reference data:

1. **Character** - The runtime character with all capabilities "baked in"
   - No references to race/class objects
   - Everything extracted during creation
   - Tracks current state (HP, conditions, effects)

2. **Game Data** (RaceData, ClassData) - Reference data for character creation
   - Only needed during character creation
   - Defines available choices and features
   - Not needed during gameplay

### Conditions and Effects

Characters track their current state through typed conditions and effects:

```go
// Apply conditions
character.AddCondition(conditions.Condition{
    Type:   conditions.Poisoned,
    Source: "giant_spider",
})

// Apply spell effects
character.AddEffect(effects.NewBlessEffect("cleric_spell"))

// Effects modify calculations
ac := character.AC() // Includes any AC bonuses from effects

// Everything persists automatically
data := character.ToData() // Includes conditions and effects
```

### During Gameplay

```go
// Load character for session
character, _ := LoadCharacterFromData(savedData, raceData, classData, backgroundData)

// During combat...
character.AddEffect(effects.NewRageEffect("barbarian_rage"))
character.AddCondition(conditions.Grappled)

// Each character tracks their own state
char1.AddEffect(effects.NewBlessEffect("cleric_123"))
char2.AddEffect(effects.NewBlessEffect("cleric_123"))
char3.AddEffect(effects.NewBlessEffect("cleric_123"))

// Save state after changes
save(char1.ToData()) // Has bless
save(char2.ToData()) // Has bless
save(char3.ToData()) // Has bless
```

### Character Creation

Two ways to create characters:

#### Simple Direct Creation

```go
// Load your game data
raceData := loadRaceData("human")
classData := loadClassData("fighter")
backgroundData := loadBackgroundData("soldier")

// Create character directly
character, err := character.NewFromCreationData(character.CreationData{
    Name:           "Ragnar",
    RaceData:       raceData,
    ClassData:      classData,
    BackgroundData: backgroundData,
    AbilityScores: shared.AbilityScores{
        Strength: 15, Dexterity: 14, Constitution: 13,
        Intelligence: 12, Wisdom: 10, Charisma: 8,
    },
    Choices: map[string]any{
        "skills": []string{"athletics", "intimidation"},
        "language": "orcish",
    },
})

// Save the character
data := character.ToData()
saveToDatabase(data)
```

#### Builder Pattern (for multi-step UIs)

```go
// Create builder
builder := NewCharacterBuilder()

// Load your game data from wherever (API, files, etc.)
raceData := loadRaceData("human")     // You implement this
classData := loadClassData("wizard")   // You implement this
backgroundData := loadBackgroundData("sage") // You implement this

// Set character details with the data
builder.SetName("Gandalf")
builder.SetRaceData(raceData, "")     // Pass race data, optional subrace ID
builder.SetClassData(classData)        // Pass class data
builder.SetBackgroundData(backgroundData) // Pass background data

// Set ability scores
builder.SetAbilityScores(AbilityScores{
    Strength: 10,
    Dexterity: 14,
    Constitution: 13,
    Intelligence: 18,
    Wisdom: 15,
    Charisma: 12,
})

// Select skills from available options
builder.SelectSkills([]string{"arcana", "history"})

// Check progress
progress := builder.Progress()
fmt.Printf("%.0f%% complete\n", progress.PercentComplete)

// Build when ready
if progress.CanBuild {
    character, err := builder.Build()
    if err == nil {
        // Convert to data for persistence
        charData := character.ToData()
        saveToDatabase(charData) // You implement this
    }
}

// Or save draft to continue later
draftData := builder.ToData()
saveDraft(draftData) // You implement this

// Later, load and continue
builder2, err := LoadDraft(draftData)
```

### Data Contract

The rulebook defines what data needs persisting:

- `CharacterData` - Everything needed to recreate a character
- `CharacterDraftData` - In-progress character creation state
- Choice tracking - Player selections during creation
- Conditions and resources - Current character state

### Integration with Game Services

Game services (like rpg-api) use this rulebook by:

1. Storing the data structures we define
2. Using the builder for character creation
3. Loading characters with a DataLoader
4. Calling game mechanics methods

```go
// In your game service
type Repository struct {
    db Database
}

func (r *Repository) SaveCharacter(ctx context.Context, char *dnd5e.Character) error {
    data := char.ToData()
    return r.db.Save("character", data)
}

func (r *Repository) LoadCharacter(ctx context.Context, id string) (*dnd5e.Character, error) {
    var data dnd5e.CharacterData
    if err := r.db.Load("character", id, &data); err != nil {
        return nil, err
    }
    return dnd5e.LoadCharacter(data, r.loader)
}
```

## Future Enhancements

- [ ] Complete choice compilation logic
- [ ] Add spell casting mechanics
- [ ] Implement combat actions
- [ ] Add conditions and effects system
- [ ] Support for multiclassing
- [ ] Magic item attunement
- [ ] Feat selection

## Design Principles

1. **Game logic lives here** - Not in the API service
2. **Data/Domain separation** - Clear boundaries for persistence
3. **Validation is game rules** - We enforce D&D 5e rules
4. **No persistence logic** - We define what to store, not how