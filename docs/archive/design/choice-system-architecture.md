# D&D 5e Choice System Architecture

## Problem Statement

The D&D 5e API presents choices in a complex nested structure with:
- Simple selections (choose 2 from list of skills)
- Bundle selections (choose between "sword and shield" or "two swords")
- Category lookups (choose from "simple-weapons" category)
- Nested choices (a choice that contains more choices)
- Counted items (20x arrows)

The game server needs a simplified, type-safe way to:
1. Present choices to players
2. Validate selections
3. Apply chosen options to characters

## Choice Patterns Found

### From API Analysis

**Choice Types:**
- `proficiencies` - Skill, tool, language proficiencies
- `equipment` - Starting equipment choices

**Option Types:**
- `reference` - Single item reference
- `counted_reference` - Item with quantity
- `multiple` - Bundle of items
- `choice` - Nested choice

**Option Set Types:**
- `options_array` - Explicit list of options
- `equipment_category` - Reference to category lookup

## Proposed Toolkit Design

### Core Types

```go
// rulebooks/dnd5e/choices/types.go
package choices

// Choice represents any character creation choice
type Choice struct {
    ID          string
    Category    Category
    Description string
    Choose      int        // How many to choose
    Options     []Option   // Available options
    Source      Source     // Where this choice comes from
}

// Category of choice
type Category string

const (
    CategorySkill      Category = "skill"
    CategoryLanguage   Category = "language" 
    CategoryTool       Category = "tool"
    CategoryEquipment  Category = "equipment"
    CategoryAbility    Category = "ability"
    CategorySpell      Category = "spell"
    CategoryCantrip    Category = "cantrip"
)

// Source of the choice
type Source string

const (
    SourceClass      Source = "class"
    SourceRace       Source = "race"
    SourceBackground Source = "background"
    SourceSubclass   Source = "subclass"
    SourceSubrace    Source = "subrace"
)

// Option represents a single selectable option
type Option interface {
    GetID() string
    GetType() OptionType
    Validate() error
}

type OptionType string

const (
    OptionTypeSingle   OptionType = "single"   // Single item
    OptionTypeBundle   OptionType = "bundle"   // Multiple items
    OptionTypeCategory OptionType = "category" // Choose from category
)
```

### Concrete Option Types

```go
// SingleOption - choose a single item
type SingleOption struct {
    ID    string
    Type  OptionType
    Item  Item
}

// BundleOption - choose a bundle of items
type BundleOption struct {
    ID    string
    Type  OptionType
    Items []Item
}

// CategoryOption - choose from a category
type CategoryOption struct {
    ID       string
    Type     OptionType
    Category string   // "simple-weapons", "martial-weapons"
    Choose   int      // How many from category
}

// Item in an option
type Item struct {
    Type     ItemType
    ID       string
    Quantity int
}

type ItemType string

const (
    ItemTypeSkill     ItemType = "skill"
    ItemTypeLanguage  ItemType = "language"
    ItemTypeTool      ItemType = "tool"
    ItemTypeWeapon    ItemType = "weapon"
    ItemTypeArmor     ItemType = "armor"
    ItemTypeGear      ItemType = "gear"
)
```

### Pre-defined Class Choices

```go
// rulebooks/dnd5e/classes/fighter/choices.go
package fighter

import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// SkillChoice returns the fighter's skill proficiency choices
func SkillChoice() choices.Choice {
    return choices.Choice{
        ID:          "fighter-skills",
        Category:    choices.CategorySkill,
        Description: "Choose two skills",
        Choose:      2,
        Source:      choices.SourceClass,
        Options: []choices.Option{
            choices.SingleOption{
                ID:   "acrobatics",
                Type: choices.OptionTypeSingle,
                Item: choices.Item{
                    Type: choices.ItemTypeSkill,
                    ID:   string(skills.Acrobatics),
                },
            },
            choices.SingleOption{
                ID:   "animal-handling",
                Type: choices.OptionTypeSingle,
                Item: choices.Item{
                    Type: choices.ItemTypeSkill,
                    ID:   string(skills.AnimalHandling),
                },
            },
            // ... more skills
        },
    }
}

// EquipmentChoices returns fighter's starting equipment choices
func EquipmentChoices() []choices.Choice {
    return []choices.Choice{
        {
            ID:          "fighter-armor",
            Category:    choices.CategoryEquipment,
            Description: "(a) chain mail or (b) leather armor, longbow, and 20 arrows",
            Choose:      1,
            Source:      choices.SourceClass,
            Options: []choices.Option{
                choices.SingleOption{
                    ID:   "chain-mail",
                    Type: choices.OptionTypeSingle,
                    Item: choices.Item{
                        Type: choices.ItemTypeArmor,
                        ID:   "chain-mail",
                    },
                },
                choices.BundleOption{
                    ID:   "leather-bow",
                    Type: choices.OptionTypeBundle,
                    Items: []choices.Item{
                        {Type: choices.ItemTypeArmor, ID: "leather-armor"},
                        {Type: choices.ItemTypeWeapon, ID: "longbow"},
                        {Type: choices.ItemTypeGear, ID: "arrow", Quantity: 20},
                    },
                },
            },
        },
        {
            ID:          "fighter-weapons",
            Category:    choices.CategoryEquipment,
            Description: "(a) a martial weapon and a shield or (b) two martial weapons",
            Choose:      1,
            Source:      choices.SourceClass,
            Options: []choices.Option{
                choices.BundleOption{
                    ID:   "weapon-shield",
                    Type: choices.OptionTypeBundle,
                    Items: []choices.Item{
                        {Type: choices.ItemTypeWeapon, ID: "martial-weapon-choice"},
                        {Type: choices.ItemTypeArmor, ID: "shield"},
                    },
                },
                choices.CategoryOption{
                    ID:       "two-weapons",
                    Type:     choices.OptionTypeCategory,
                    Category: "martial-weapons",
                    Choose:   2,
                },
            },
        },
    }
}
```

### Choice Builder/Loader

```go
// rulebooks/dnd5e/choices/loader.go
package choices

// LoadClassChoices returns all choices for a class
func LoadClassChoices(classID classes.Class) []Choice {
    switch classID {
    case classes.Fighter:
        return fighter.AllChoices()
    case classes.Wizard:
        return wizard.AllChoices()
    // ... other classes
    default:
        return nil
    }
}

// LoadRaceChoices returns all choices for a race
func LoadRaceChoices(raceID races.Race) []Choice {
    switch raceID {
    case races.HalfElf:
        return halfelf.AllChoices()
    // ... other races
    default:
        return nil
    }
}
```

### External Client Conversion

```go
// rpg-api/internal/clients/external/choice_mapper.go

// MapAPIChoiceToToolkit converts API choice structure to toolkit choice
func (m *ChoiceMapper) MapAPIChoiceToToolkit(apiChoice APIChoice) choices.Choice {
    choice := choices.Choice{
        ID:          generateChoiceID(apiChoice),
        Description: apiChoice.Desc,
        Choose:      apiChoice.Choose,
        Source:      m.inferSource(apiChoice),
    }
    
    // Map category
    choice.Category = m.mapCategory(apiChoice.Type)
    
    // Map options based on option_set_type
    switch apiChoice.From.OptionSetType {
    case "options_array":
        choice.Options = m.mapOptionsArray(apiChoice.From.Options)
    case "equipment_category":
        choice.Options = []choices.Option{
            choices.CategoryOption{
                ID:       apiChoice.From.EquipmentCategory.Index,
                Type:     choices.OptionTypeCategory,
                Category: apiChoice.From.EquipmentCategory.Index,
                Choose:   apiChoice.Choose,
            },
        }
    }
    
    return choice
}

// mapOptionsArray converts API options to toolkit options
func (m *ChoiceMapper) mapOptionsArray(apiOptions []APIOption) []choices.Option {
    var options []choices.Option
    
    for _, apiOpt := range apiOptions {
        switch apiOpt.OptionType {
        case "reference":
            options = append(options, m.mapReference(apiOpt))
        case "counted_reference":
            options = append(options, m.mapCountedReference(apiOpt))
        case "multiple":
            options = append(options, m.mapBundle(apiOpt))
        case "choice":
            // Nested choice - needs special handling
            options = append(options, m.mapNestedChoice(apiOpt))
        }
    }
    
    return options
}
```

## Benefits

1. **Type Safety** - Strongly typed choices prevent errors
2. **Simplified Structure** - Flattened compared to API's nested structure
3. **Validation** - Each option can validate itself
4. **Extensible** - New choice types can be added
5. **Game Server Simplicity** - Just presents choices and validates selections

## Usage Example

```go
// In game server
func (o *Orchestrator) GetCharacterCreationChoices(class classes.Class, race races.Race) []choices.Choice {
    var allChoices []choices.Choice
    
    // Get class choices
    allChoices = append(allChoices, choices.LoadClassChoices(class)...)
    
    // Get race choices
    allChoices = append(allChoices, choices.LoadRaceChoices(race)...)
    
    return allChoices
}

// Validate player selection
func (o *Orchestrator) ValidateChoice(choice choices.Choice, selections []string) error {
    if len(selections) != choice.Choose {
        return fmt.Errorf("must choose exactly %d options", choice.Choose)
    }
    
    // Validate each selection is valid
    for _, selection := range selections {
        if !choice.HasOption(selection) {
            return fmt.Errorf("invalid selection: %s", selection)
        }
    }
    
    return nil
}
```

## Implementation Phases

1. **Phase 1**: Implement core types and simple choices (skills, languages)
2. **Phase 2**: Add equipment bundles and counted items
3. **Phase 3**: Implement category lookups
4. **Phase 4**: Handle nested choices
5. **Phase 5**: Add validation and application logic