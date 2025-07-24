# D&D 5e Rulebook Example

This example demonstrates how to use the D&D 5e rulebook package for character creation, management, and gameplay.

## Running the Example

### Basic Demo Mode
```bash
go run .
```

This runs through a complete demonstration:
- Creating a character using direct creation
- Displaying character stats and abilities
- Simulating combat with conditions and effects
- Saving and loading character data
- Demonstrating the builder pattern

### Interactive CLI Mode
```bash
go run . cli
```

This launches an interactive command-line interface where you can:
- Create characters step-by-step
- Load and save characters
- Apply combat effects and conditions
- Test the builder pattern
- See real-time character state changes

## Features Demonstrated

### Character Creation
- **Direct Creation**: Quick character creation with all data at once
- **Builder Pattern**: Step-by-step creation for multi-page UIs

### Combat Mechanics
- **Conditions**: Poisoned, grappled, etc. that affect gameplay
- **Effects**: Temporary buffs like Bless, Shield, Rage
- **AC Calculation**: Dynamic calculation including effects

### Data Persistence
- Save character state including all effects and conditions
- Load characters with full state restoration
- JSON format for easy integration with APIs

## Implementation Guide

### 1. Direct Character Creation
```go
char, err := character.NewFromCreationData(character.CreationData{
    Name:           "Ragnar",
    RaceData:       raceData,
    ClassData:      classData,
    BackgroundData: backgroundData,
    AbilityScores:  scores,
    Choices:        choices,
})
```

### 2. Using the Builder
```go
builder := dnd5e.NewCharacterBuilder()
builder.SetName("Thorin")
builder.SetRaceData(raceData, "")
builder.SetClassData(classData)
// ... set other properties

if builder.Progress().CanBuild {
    char, err := builder.Build()
}
```

### 3. Managing Effects During Play
```go
// Apply conditions
char.AddCondition(conditions.Condition{
    Type:   conditions.Poisoned,
    Source: "giant_spider",
})

// Apply spell effects
char.AddEffect(effects.NewBlessEffect("cleric_spell"))

// Check state
if char.HasCondition(conditions.Poisoned) {
    // Handle disadvantage
}
```

### 4. Saving and Loading
```go
// Save
data := char.ToData()
jsonBytes, _ := json.Marshal(data)
os.WriteFile("character.json", jsonBytes, 0644)

// Load
var data character.Data
json.Unmarshal(jsonBytes, &data)
char, _ := character.LoadCharacterFromData(data, raceData, classData, backgroundData)
```

## Integration with Your Game

1. **Load Game Data**: Implement functions to load race, class, and background data from your source (API, database, files)
2. **Character Storage**: Use the CharacterData structure for persistence
3. **Event Handling**: Characters track their own state - save after changes
4. **Multi-Character**: Each character manages their own conditions and effects

## CLI Commands

When running in CLI mode:
- `1` - Create new character wizard
- `2` - Load character from file
- `3` - Show current character
- `4` - Apply combat effects menu
- `5` - Save character to file
- `6` - Demonstrate builder pattern
- `Q` - Quit

## Notes

- Character IDs are generated with timestamps (implement your own ID strategy)
- All character state is "baked in" at creation - no runtime dependencies on race/class objects
- Effects and conditions persist with the character data
- The example includes sample data for Human, Elf, Dwarf races and Fighter, Wizard, Rogue classes