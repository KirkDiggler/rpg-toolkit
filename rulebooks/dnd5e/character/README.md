# D&D 5e Character Creation Guide

This guide shows how to create a D&D 5e character using the rpg-toolkit character package.

## Quick Start

```go
// 1. Create a builder with a unique draft ID
builder, err := character.NewCharacterBuilder("unique-draft-id")

// 2. Set basic info
builder.SetName("Aragorn")

// 3. Set race (load race data from your source)
builder.SetRaceData(humanRace, "") // empty string for no subrace

// 4. Set class (load class data from your source)
builder.SetClassData(fighterClass, "") // empty string for no subclass at level 1

// 5. Set background (load background data from your source)  
builder.SetBackgroundData(soldierBackground)

// 6. Set ability scores
scores := shared.AbilityScores{
    constants.STR: 15,
    constants.DEX: 13,
    constants.CON: 14,
    constants.INT: 10,
    constants.WIS: 12,
    constants.CHA: 8,
}
builder.SetAbilityScores(scores)

// 7. Make choices (skills, equipment, etc)
builder.SelectSkills([]string{"perception", "survival"})
builder.SelectFightingStyle("defense") // for applicable classes
builder.SelectEquipment([]string{"Chain mail", "Shield", "Longsword"})

// 8. Build the character
character, err := builder.Build()
```

## Character Creation Flow

### Required Steps (in order)
1. **Name** - Character's name
2. **Race** - Race and optional subrace
3. **Class** - Class and optional subclass (usually chosen at level 3)
4. **Background** - Character background
5. **Ability Scores** - The six ability scores

### Optional Steps (based on class/race)
- **Skills** - Select from class skill options
- **Languages** - Additional languages from race/background
- **Fighting Style** - For martial classes
- **Spells/Cantrips** - For spellcasting classes
- **Equipment** - Starting equipment choices

## Loading Data

You need to provide race, class, and background data. These would typically come from:
- A database
- JSON files
- The D&D 5e API
- Your own data source

Example data structures are shown in the example_test.go file.

## Choice Tracking

Every choice made during character creation is tracked with:
- **Category** - What type of choice (name, skills, etc)
- **Source** - Where it came from (player, race, class, background)
- **ChoiceID** - Unique identifier for the choice
- **Selection** - The actual choice made (using typed fields)

This allows you to:
- Show players where each feature came from
- Rebuild drafts from existing characters
- Handle race/class changes cleanly
- Display tooltips with source information

## Draft vs Character

- **Draft** - Character in progress, stores all choices
- **Character** - Completed character ready for play

You can save and load drafts to allow players to resume character creation:

```go
// Save draft
draftData := builder.ToData()
// ... store draftData in your database

// Load draft
builder, err := character.LoadDraft(draftData)
// ... continue where they left off
```

## Validation

The builder validates:
- Required fields are set before building
- Skills are valid for the chosen class
- Ability scores are in valid ranges
- All required steps are completed

## Example Implementation

See `example_test.go` for complete working examples of:
- Creating a character from scratch
- Understanding choice tracking
- Working with drafts

## Common Patterns

### Checking Progress
```go
progress := builder.Progress()
fmt.Printf("%.0f%% complete\n", progress.PercentComplete)
fmt.Printf("Can build: %v\n", progress.CanBuild)
```

### Validation Before Building
```go
errors := builder.Validate()
if len(errors) > 0 {
    // Show validation errors to user
}
```

### Getting Available Options
Check the class/race data for available options:
- `classData.SkillOptions` - Skills the player can choose from
- `classData.SkillProficiencyCount` - How many to choose
- `raceData.Languages` - Languages automatically granted
- `backgroundData.SkillProficiencies` - Skills automatically granted