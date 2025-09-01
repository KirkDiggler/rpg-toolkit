# D&D 5e Character Package

This package provides a clean, domain-driven character creation and management system for D&D 5e.

## Architecture Overview

The character package is organized into several key components:

- **Domain Model** (`character.go`) - Core character entity for gameplay
- **Data Model** (`data.go`) - Serialization/persistence representation
- **Draft System** (`draft.go`) - Character creation workflow
- **Choices System** (`choices/`) - Requirements, submissions, and validation
- **Input Types** (`inputs.go`) - Strongly-typed API input structures

## Key Design Principles

1. **Type Safety** - All IDs use typed constants (races.Race, classes.Class, choices.ChoiceID, etc.)
2. **Domain Separation** - Clear boundaries between creation, gameplay, and persistence
3. **Rich Data** - Equipment and choices include full descriptive information
4. **Explicit IDs** - All requirements use typed ChoiceID constants for unambiguous reference
5. **Proto-Ready** - All constants designed to map to proto enums for API type safety

## The Choices System

The choices system is the heart of character creation, handling the complex D&D 5e character creation rules.

### How It Works End-to-End

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Requirements │────>│ Submissions  │────>│ Validation   │
└──────────────┘     └──────────────┘     └──────────────┘
      What              What was            Are they
   must be chosen        chosen              valid?
```

### 1. Requirements

Requirements define what choices need to be made. Each requirement has:
- **ID** - Typed ChoiceID constant (e.g., `choices.FighterSkills`, `choices.FighterArmor`)
- **Label** - Display text (e.g., "Choose 2 skills", "Choose your armor")
- **Options** - What can be chosen (typed constants)
- **Count/Choose** - How many to select

Example:
```go
SkillRequirement{
    ID:      choices.FighterSkills,  // Typed constant, not string!
    Count:   2,
    Options: []skills.Skill{skills.Athletics, skills.History, ...},
    Label:   "Choose 2 skills",
}
```

### 2. Submissions

Submissions represent what the player actually chose:
```go
Submission{
    Category: shared.ChoiceSkills,
    Source:   shared.SourceClass,
    ChoiceID: choices.FighterSkills,  // Typed constant matching requirement ID
    Values:   []string{"athletics", "history"},
}
```

### 3. Validation

The validator checks submissions against requirements:
- Are all required choices made?
- Are the choices valid options?
- Are the counts correct?

## Character Creation Flow

### Step 1: Get Requirements

```go
// API gets requirements for class/race
classReqs := choices.GetClassRequirements(classes.Fighter)
raceReqs := choices.GetRaceRequirements(races.HalfElf)

// Requirements include full Equipment objects for rich display
for _, equipReq := range classReqs.Equipment {
    for _, option := range equipReq.Options {
        for _, item := range option.Items {
            // item.Equipment has GetDescription() method
            // "Chain Mail: AC 16, Stealth disadvantage, Str 13 required"
        }
    }
}
```

### Step 2: Player Makes Choices

The client displays requirements with full descriptions and collects choices:
```go
// Player chooses skills
draft.SetClass(&SetClassInput{
    ClassID: classes.Fighter,
    Choices: ClassChoices{
        Skills: []skills.Skill{Athletics, History},
        FightingStyle: fightingstyles.Defense,
    },
})
```

### Step 3: Validation

```go
// Draft validates all choices
err := draft.ValidateChoices()
if err != nil {
    // Missing required choices or invalid selections
}
```

### Step 4: Character Creation

```go
// Create final character from draft
character := draft.ToCharacter()

// Character has properly typed inventory
for _, item := range character.inventory {
    equipment := item.Equipment // Equipment interface
    qty := item.Quantity         // For "20 arrows"
}
```

## Equipment System

The equipment system provides a unified interface for all items:

```go
type Equipment interface {
    GetID() string
    GetType() shared.EquipmentType
    GetName() string
    GetWeight() float32
    GetValue() int
    GetDescription() string
}
```

Implemented by:
- **Weapons** (`/weapons`) - All weapon types with damage, properties
- **Armor** (`/armor`) - All armor types with AC, requirements
- **Tools** (`/tools`) - Artisan tools, instruments, gaming sets
- **Packs** (`/packs`) - Equipment bundles like Explorer's Pack

### Equipment in Choices

Equipment requirements include the actual Equipment objects:
```go
EquipmentOption{
    ID:    "fighter-armor-a",
    Label: "Chain mail",
    Items: []EquipmentItem{
        {
            Equipment: chainMail,  // Full armor.Armor object
            Quantity:  1,
        },
    },
}
```

This allows the API to send complete equipment information during character creation, not just IDs.

## Type Safety Throughout

The system uses typed constants everywhere:
- `races.Race` not string
- `classes.Class` not string  
- `skills.Skill` not string
- `fightingstyles.FightingStyle` not string
- `choices.ChoiceID` not string
- `equipment.Equipment` interface, not string IDs

This provides:
- Compile-time safety
- IDE autocomplete
- Impossible to use invalid values
- Self-documenting code
- Clean proto enum mapping for APIs

## Data Persistence

The `Data` struct provides clean serialization:
```go
type Data struct {
    // Identity
    ID       string
    PlayerID string
    Name     string
    
    // Typed IDs for core attributes
    RaceID     races.Race
    ClassID    classes.Class
    
    // Inventory stores IDs and quantities
    Inventory []InventoryItemData
}

type InventoryItemData struct {
    Type     shared.EquipmentType // weapon, armor, tool, pack
    ID       string               // "longsword", "chain-mail"
    Quantity int                  // 1, 20, etc.
}
```

## ChoiceID System

All choice requirements use typed `ChoiceID` constants defined in `choices/choice_ids.go`:

```go
const (
    // Class Skills
    FighterSkills  ChoiceID = "fighter-skills"
    RogueSkills    ChoiceID = "rogue-skills"
    
    // Fighting Styles
    FighterFightingStyle ChoiceID = "fighter-fighting-style"
    
    // Equipment
    FighterArmor           ChoiceID = "fighter-armor"
    FighterWeaponsPrimary  ChoiceID = "fighter-weapons-primary"
    
    // Race choices
    HalfElfSkills   ChoiceID = "half-elf-skills"
    HumanLanguage   ChoiceID = "human-language"
    
    // ... and many more
)
```

These constants:
- Prevent typos and invalid IDs
- Provide IDE autocomplete
- Map directly to proto enums for API type safety
- Create a single source of truth for all choice identifiers

## API Integration

The character package is designed for clean API integration:

1. **Requirements Endpoint** - Returns requirements with full Equipment data and typed ChoiceIDs
2. **Submission Endpoint** - Accepts typed ChoiceIDs and chosen item IDs
3. **Validation** - Server-side validation using typed constants
4. **Character Storage** - Efficient storage of just IDs
5. **Proto Mapping** - All constants map to proto enums for wire safety

## Future Enhancements

- [ ] Spell choices with full spell data
- [ ] Feat selection system
- [ ] Multiclass support
- [ ] Level-up choices
- [ ] Custom equipment creation

## Example Usage

See `example_test.go` for complete character creation examples.

## Testing

```bash
go test ./...
```

The package includes comprehensive tests for:
- Choice validation
- Equipment system
- Character creation flow
- Data serialization