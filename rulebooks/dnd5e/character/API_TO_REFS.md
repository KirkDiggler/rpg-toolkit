# API Data to Refs: How the Game Server Bridges

## The Problem
- API returns data with `index` fields like "dwarf", "fighter", "athletics"
- Toolkit needs proper refs like "dnd5e:race:dwarf", "dnd5e:skill:athletics"
- How does the game server know which refs to create?

## Solution: Ref Builders in Toolkit

The toolkit provides helper functions to build refs from API indices:

```go
// In toolkit - rulebooks/dnd5e/refs package
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Builders for each type
func Race(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "race",
        Value:  index,
    })
}

func Class(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "class",
        Value:  index,
    })
}

func Skill(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "skill",
        Value:  index,
    })
}

func Feature(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "feature",
        Value:  index,
    })
}

func Background(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "background",
        Value:  index,
    })
}

func Language(index string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "language",
        Value:  index,
    })
}

func Choice(choiceType string) *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "choice",
        Value:  choiceType,
    })
}
```

## Game Server Usage

### Step 1: Fetch from API
```go
// API returns this structure
type APIRace struct {
    Index             string   `json:"index"`  // "dwarf"
    Name              string   `json:"name"`   // "Dwarf"
    Speed             int      `json:"speed"`  // 25
    Size              string   `json:"size"`   // "Medium"
    Languages         []APIRef `json:"languages"`
    StartingProficiencies []APIRef `json:"starting_proficiencies"`
}

type APIRef struct {
    Index string `json:"index"`
    Name  string `json:"name"`
    URL   string `json:"url"`
}

// Fetch from API
apiRace, _ := dnd5eAPIClient.GetRace("dwarf")
```

### Step 2: Convert to Toolkit Format
```go
// Game server converts API data to toolkit format with refs
func (gs *GameServer) convertRaceData(apiRace *APIRace) *RaceData {
    // Build the race ref from API index
    raceRef := refs.Race(apiRace.Index)
    
    // Convert languages
    languages := make([]*core.Ref, len(apiRace.Languages))
    for i, lang := range apiRace.Languages {
        languages[i] = refs.Language(lang.Index)
    }
    
    // Convert proficiencies
    proficiencies := make([]*core.Ref, len(apiRace.StartingProficiencies))
    for i, prof := range apiRace.StartingProficiencies {
        // API might return "skill-athletics" or just "athletics"
        // Need to parse the type from the URL or index
        profType := extractProficiencyType(prof)
        proficiencies[i] = refs.Proficiency(profType, prof.Index)
    }
    
    return &RaceData{
        Ref:           raceRef,
        Name:          apiRace.Name,
        Speed:         apiRace.Speed,
        Size:          apiRace.Size,
        Languages:     languages,
        Proficiencies: proficiencies,
    }
}
```

### Step 3: Handle Choices
```go
// When player selects race in UI
func (gs *GameServer) handleRaceSelection(raceIndex string) {
    // Create the choice with proper refs
    choice := character.ChoiceData{
        Ref:    refs.Choice("race"),
        Source: refs.System("creation"),
        Selected: []*core.Ref{
            refs.Race(raceIndex), // "dwarf" -> "dnd5e:race:dwarf"
        },
    }
    
    gs.currentChoices = append(gs.currentChoices, choice)
}

// When player selects skills from class
func (gs *GameServer) handleSkillSelection(classIndex string, selectedSkills []string) {
    // Build choice ref based on class
    choiceRef := refs.Choice(fmt.Sprintf("%s_skills", classIndex))
    
    // Convert skill indices to refs
    skillRefs := make([]*core.Ref, len(selectedSkills))
    for i, skill := range selectedSkills {
        skillRefs[i] = refs.Skill(skill) // "athletics" -> "dnd5e:skill:athletics"
    }
    
    choice := character.ChoiceData{
        Ref:      choiceRef,
        Source:   refs.Class(classIndex),
        Selected: skillRefs,
    }
    
    gs.currentChoices = append(gs.currentChoices, choice)
}
```

## Toolkit Provides Mapping Knowledge

The toolkit can also provide helpers to understand API structure:

```go
// In toolkit - helps game server understand API data
package dnd5e

// Maps API proficiency URLs to ref types
func ProficiencyTypeFromURL(url string) string {
    // "/api/proficiencies/skill-athletics" -> "skill"
    // "/api/proficiencies/medium-armor" -> "armor"
    // "/api/proficiencies/martial-weapons" -> "weapon"
    
    if strings.Contains(url, "/proficiencies/skill-") {
        return "skill"
    }
    if strings.Contains(url, "-armor") {
        return "armor"
    }
    if strings.Contains(url, "-weapons") {
        return "weapon"
    }
    return "tool"
}

// Extract the actual index from compound API indices
func ExtractSkillIndex(apiIndex string) string {
    // "skill-athletics" -> "athletics"
    return strings.TrimPrefix(apiIndex, "skill-")
}
```

## Complete Example: Creating a Character

```go
// Game Server Code
func (gs *GameServer) CreateCharacter(playerChoices PlayerChoices) (*character.Data, error) {
    // 1. Fetch API data
    apiRace := gs.apiClient.GetRace(playerChoices.RaceIndex)
    apiClass := gs.apiClient.GetClass(playerChoices.ClassIndex)
    apiBackground := gs.apiClient.GetBackground(playerChoices.BackgroundIndex)
    
    // 2. Build choices with refs
    choices := []character.ChoiceData{
        {
            Ref:      refs.Choice("race"),
            Source:   refs.System("creation"),
            Selected: []*core.Ref{refs.Race(apiRace.Index)},
        },
        {
            Ref:      refs.Choice("class"),
            Source:   refs.System("creation"),
            Selected: []*core.Ref{refs.Class(apiClass.Index)},
        },
        {
            Ref:      refs.Choice("background"),
            Source:   refs.System("creation"),
            Selected: []*core.Ref{refs.Background(apiBackground.Index)},
        },
    }
    
    // 3. Add skill choices
    skillRefs := make([]*core.Ref, len(playerChoices.SelectedSkills))
    for i, skillIndex := range playerChoices.SelectedSkills {
        // API returns "skill-athletics", we need "athletics"
        cleanIndex := dnd5e.ExtractSkillIndex(skillIndex)
        skillRefs[i] = refs.Skill(cleanIndex)
    }
    
    choices = append(choices, character.ChoiceData{
        Ref:      refs.Choice(fmt.Sprintf("%s_skills", apiClass.Index)),
        Source:   refs.Class(apiClass.Index),
        Selected: skillRefs,
    })
    
    // 4. Use toolkit to compile character
    compiler := dnd5e.NewCharacterCompiler()
    charData, err := compiler.CompileCharacter(dnd5e.CompileInput{
        ID:       gs.generateID(),
        PlayerID: playerChoices.PlayerID,
        Choices:  choices,
        
        // Pass API data for compilation
        RaceData:       gs.convertRaceData(apiRace),
        ClassData:      gs.convertClassData(apiClass),
        BackgroundData: gs.convertBackgroundData(apiBackground),
    })
    
    if err != nil {
        return nil, err
    }
    
    // 5. Save self-contained character data
    return charData, nil
}
```

## Key Insights

1. **Toolkit provides ref builders** - Functions to create refs from API indices
2. **Game server owns the mapping** - It knows how to convert API structure
3. **Toolkit provides helpers** - Functions to parse API formats
4. **Refs are built at conversion time** - Not stored as strings

## What the Toolkit Should Provide

```go
package refs

// Builders for creating refs from indices
func Race(index string) *core.Ref
func Class(index string) *core.Ref
func Skill(index string) *core.Ref
func Feature(index string) *core.Ref
func Language(index string) *core.Ref
func Background(index string) *core.Ref
func Choice(choiceType string) *core.Ref
func System(system string) *core.Ref
func Proficiency(profType, index string) *core.Ref

// Common refs as constants (for frequently used ones)
var (
    SystemCreation = System("creation")
    ChoiceRace     = Choice("race")
    ChoiceClass    = Choice("class")
    // ... etc
)

// Helpers for parsing API formats
func ExtractSkillIndex(apiIndex string) string
func ProficiencyTypeFromURL(url string) string
```

This way the game server has everything it needs to bridge API data to toolkit refs!