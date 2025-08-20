# What the Game Server Needs from the Toolkit

## Game Server Has
- Character data (from database)
- Race/class/background data (from API or cache)
- Player choices (from UI)

## Game Server Needs from Toolkit

### 1. Ref Constants
The toolkit should declare all the refs so the game server doesn't use magic strings:

```go
// In toolkit - rulebooks/dnd5e/refs package
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

var (
    // Races
    RaceHuman    = core.MustParseString("dnd5e:race:human")
    RaceDwarf    = core.MustParseString("dnd5e:race:dwarf")
    RaceElf      = core.MustParseString("dnd5e:race:elf")
    
    // Classes  
    ClassBarbarian = core.MustParseString("dnd5e:class:barbarian")
    ClassFighter   = core.MustParseString("dnd5e:class:fighter")
    ClassWizard    = core.MustParseString("dnd5e:class:wizard")
    
    // Skills
    SkillAthletics    = core.MustParseString("dnd5e:skill:athletics")
    SkillAcrobatics   = core.MustParseString("dnd5e:skill:acrobatics")
    SkillIntimidation = core.MustParseString("dnd5e:skill:intimidation")
    
    // Features
    FeatureRage         = core.MustParseString("dnd5e:features:rage")
    FeatureSecondWind   = core.MustParseString("dnd5e:features:second_wind")
    FeatureActionSurge  = core.MustParseString("dnd5e:features:action_surge")
    
    // Choices
    ChoiceRace          = core.MustParseString("dnd5e:choice:race")
    ChoiceClass         = core.MustParseString("dnd5e:choice:class")
    ChoiceBackground    = core.MustParseString("dnd5e:choice:background")
    ChoiceFighterSkills = core.MustParseString("dnd5e:choice:fighter_skills")
)
```

Game server uses:
```go
choices := []character.ChoiceData{
    {
        Ref:    refs.ChoiceClass,
        Source: refs.SystemCreation,
        Selected: []*core.Ref{refs.ClassBarbarian},
    },
}
```

### 2. Character Compiler
Convert choices + API data into self-contained character data:

```go
// In toolkit
type CharacterCompiler interface {
    // Takes choices and API data, returns self-contained character data
    CompileCharacter(input CompileInput) (*character.Data, error)
}

type CompileInput struct {
    ID            string
    PlayerID      string
    Choices       []ChoiceData
    
    // Optional: API data for reference (but everything needed gets compiled in)
    RaceData      *RaceAPIData
    ClassData     *ClassAPIData
    BackgroundData *BackgroundAPIData
}

// Returns fully compiled, self-contained data
type Data struct {
    ID            string
    Name          string
    Level         int
    
    // Just refs for reference
    RaceRef       *core.Ref
    ClassRef      *core.Ref
    
    // Everything compiled in
    HP            int
    MaxHP         int
    Speed         int
    Size          string
    AbilityScores AbilityScores
    
    // All refs
    Skills        []*core.Ref
    Languages     []*core.Ref
    Features      []FeatureData  // With refs embedded
    
    // Tracking
    Choices       []ChoiceData
}
```

### 3. Character Loading (No Dependencies)
Load a character from self-contained data:

```go
// Simple - no race/class/background needed!
func LoadCharacterFromData(data *Data) (*Character, error) {
    // Everything is already in the data
    // Just reconstruct the runtime character
}
```

### 4. Character Methods
The runtime character with game mechanics:

```go
type Character interface {
    // Combat
    Attack(ctx context.Context, targetRef *core.Ref, weaponRef *core.Ref) AttackResult
    TakeDamage(amount int, damageType *core.Ref) 
    
    // Features
    ActivateFeature(ctx context.Context, featureRef *core.Ref) error
    
    // Conditions
    HasCondition(conditionRef *core.Ref) bool
    
    // Persistence
    ToData() *Data
}
```

### 5. Choice Validation
Help the game server validate choices:

```go
// In toolkit
type ChoiceValidator interface {
    // Validate a choice is allowed given current selections
    ValidateChoice(choice ChoiceData, currentChoices []ChoiceData) error
    
    // Get available options for a choice
    GetAvailableOptions(choiceRef *core.Ref, currentChoices []ChoiceData) []*core.Ref
}

// Example: Game server checking if skill choice is valid
validator := dnd5e.NewChoiceValidator()
availableSkills := validator.GetAvailableOptions(refs.ChoiceFighterSkills, currentChoices)
// Returns: [refs.SkillAthletics, refs.SkillIntimidation, refs.SkillPerception, ...]
```

## What the Toolkit Should NOT Provide

❌ **Factories for specific classes** - Too prescriptive
❌ **Draft/Builder patterns** - Game server manages its own UI flow
❌ **Database persistence** - That's the game server's job
❌ **API client** - Game server handles data fetching

## Example Game Server Flow

```go
// 1. Player makes choices in UI
playerChoices := collectFromUI()

// 2. Game server validates using toolkit
validator := dnd5e.NewChoiceValidator()
for _, choice := range playerChoices {
    if err := validator.ValidateChoice(choice, allChoices); err != nil {
        return err // Show error in UI
    }
}

// 3. Game server fetches API data
raceData := apiClient.GetRace("dwarf")
classData := apiClient.GetClass("barbarian")

// 4. Game server compiles character using toolkit
compiler := dnd5e.NewCharacterCompiler()
charData, err := compiler.CompileCharacter(CompileInput{
    ID:       generateID(),
    Choices:  playerChoices,
    RaceData: raceData,
    ClassData: classData,
})

// 5. Game server saves self-contained data
database.Save(charData)

// 6. Later: Load and play (no deps!)
charData := database.Load(id)
character := character.LoadCharacterFromData(charData)
character.Attack(ctx, targetRef, weaponRef)
```

## Key Insights

1. **Refs as Constants** - Toolkit provides all refs as constants
2. **Compilation Service** - Toolkit compiles choices into self-contained data
3. **Simple Loading** - No external dependencies needed at runtime
4. **Pure Functions** - Most operations are pure data transformations
5. **Game Server Owns Flow** - Toolkit provides tools, not opinions

## Questions to Answer

1. Should the compiler be in the toolkit or game server?
2. How much validation should the toolkit provide?
3. Should features be compiled in or loaded dynamically?
4. What's the boundary between toolkit and game server?