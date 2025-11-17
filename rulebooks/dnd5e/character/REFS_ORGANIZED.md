# Organized Refs by Package

## Better Structure: Domain-Specific Packages

Instead of one big `refs` package, organize by domain:

```go
// rulebooks/dnd5e/races/refs.go
package races

import "github.com/KirkDiggler/rpg-toolkit/core"

// Builder function
func Ref(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "race",
        Value:  index,
    })
}

// Common race refs as constants
var (
    Human    = Ref("human")
    Dwarf    = Ref("dwarf")
    Elf      = Ref("elf")
    Halfling = Ref("halfling")
    // ... etc
)
```

```go
// rulebooks/dnd5e/classes/refs.go
package classes

func Ref(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "class",
        Value:  index,
    })
}

var (
    Barbarian = Ref("barbarian")
    Fighter   = Ref("fighter")
    Wizard    = Ref("wizard")
    Cleric    = Ref("cleric")
    // ... etc
)
```

```go
// rulebooks/dnd5e/skills/refs.go
package skills

func Ref(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "skill",
        Value:  index,
    })
}

var (
    Athletics     = Ref("athletics")
    Acrobatics    = Ref("acrobatics")
    SleightOfHand = Ref("sleight_of_hand")
    Stealth       = Ref("stealth")
    // ... etc
)
```

```go
// rulebooks/dnd5e/choices/refs.go
package choices

func Ref(choiceType string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "choice",
        Value:  choiceType,
    })
}

var (
    Race            = Ref("race")
    Class           = Ref("class")
    Background      = Ref("background")
    AbilityScores   = Ref("ability_scores")
    BarbarianSkills = Ref("barbarian_skills")
    FighterSkills   = Ref("fighter_skills")
    WizardSkills    = Ref("wizard_skills")
    // ... etc
)
```

## Game Server Usage

The game server wrapper already knows the context from the API:

```go
// Game server wrapper - it knows what it's fetching!
func (w *ToolkitWrapper) HandleRaceData(apiRace *APIRaceData) *RaceData {
    // We KNOW this is race data, no need to parse
    return &RaceData{
        Ref:   races.Ref(apiRace.Index),  // Simple!
        Name:  apiRace.Name,
        Speed: apiRace.Speed,
        Size:  apiRace.Size,
    }
}

func (w *ToolkitWrapper) HandleSkillProficiencies(apiSkills []APISkillData) []*core.Ref {
    refs := make([]*core.Ref, len(apiSkills))
    for i, skill := range apiSkills {
        // The wrapper already knows these are skills
        // If API returns "skill-athletics", wrapper strips the prefix
        cleanIndex := strings.TrimPrefix(skill.Index, "skill-")
        refs[i] = skills.Ref(cleanIndex)
    }
    return refs
}

func (w *ToolkitWrapper) BuildCharacterChoices(uiChoices UIChoices) []character.ChoiceData {
    return []character.ChoiceData{
        {
            Ref:    choices.Race,  // Use the constant
            Source: system.Creation,
            Selected: []*core.Ref{
                races.Ref(uiChoices.RaceIndex),
            },
        },
        {
            Ref:    choices.Class,
            Source: system.Creation,
            Selected: []*core.Ref{
                classes.Ref(uiChoices.ClassIndex),
            },
        },
        {
            Ref:    choices.BarbarianSkills,  // Or build: choices.Ref("barbarian_skills")
            Source: classes.Barbarian,
            Selected: w.convertSkillIndices(uiChoices.SelectedSkills),
        },
    }
}
```

## Benefits of This Organization

1. **Clear Context** - `races.Ref()` vs `skills.Ref()` - you know what you're creating
2. **No Magic Strings** - Use constants where possible
3. **Game Server Owns Parsing** - Wrapper handles "skill-athletics" → "athletics"
4. **Toolkit Stays Clean** - Just provides the ref builders
5. **Discoverable** - Import the package you need

## Complete Example

```go
// Game server creating a character
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

func (gs *GameServer) CreateCharacter(input CreateCharacterInput) error {
    // Fetch from API
    raceData := gs.apiClient.GetRace(input.RaceID)      // Returns APIRaceData
    classData := gs.apiClient.GetClass(input.ClassID)   // Returns APIClassData
    
    // Build choices with proper refs
    characterChoices := []character.ChoiceData{
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
    
    // Add skill choices - wrapper knows these are skills
    for _, skillIndex := range input.SelectedSkills {
        // If API has prefixes, wrapper strips them
        cleanIndex := gs.cleanSkillIndex(skillIndex)
        skillRefs = append(skillRefs, skills.Ref(cleanIndex))
    }
    
    characterChoices = append(characterChoices, character.ChoiceData{
        Ref:      choices.Ref(fmt.Sprintf("%s_skills", classData.Index)),
        Source:   classes.Ref(classData.Index),
        Selected: skillRefs,
    })
    
    // Compile character
    charData := character.CompileCharacter(character.CompileInput{
        ID:      input.CharacterID,
        Choices: characterChoices,
        // ... other data
    })
    
    return gs.database.Save(charData)
}
```

## What Each Package Provides

```
rulebooks/dnd5e/
├── races/
│   └── refs.go       # races.Ref(), races.Human, races.Dwarf
├── classes/
│   └── refs.go       # classes.Ref(), classes.Barbarian
├── skills/
│   └── refs.go       # skills.Ref(), skills.Athletics
├── choices/
│   └── refs.go       # choices.Ref(), choices.Race, choices.BarbarianSkills
├── features/
│   └── refs.go       # features.Ref(), features.Rage
├── conditions/
│   └── refs.go       # conditions.Ref(), conditions.Raging
└── system/
    └── refs.go       # system.Creation, system.LevelUp
```

Each domain owns its refs. Clean and organized!