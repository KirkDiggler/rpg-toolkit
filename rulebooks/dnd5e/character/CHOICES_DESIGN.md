# Choices and API Integration Design

## Current State

### Complex ChoiceData
```go
type ChoiceData struct {
    Category               shared.ChoiceCategory
    Source                shared.ChoiceSource  
    ChoiceID              string
    // 12+ different Selection fields!
    SkillSelection        []constants.Skill
    LanguageSelection     []constants.Language
    EquipmentSelection    []string
    // ... and more ...
}
```

This is complex because:
- Multiple typed fields for different selections
- Hard to extend with new choice types
- Not ref-based

## Proposed: Everything is Proper Refs

### Simple ChoiceData with Type Safety
```go
type ChoiceData struct {
    Ref       *core.Ref   `json:"ref"`       // Parsed ref for "dnd5e:choice:fighter_skills"
    Source    *core.Ref   `json:"source"`    // Parsed ref for "dnd5e:class:fighter"
    Selected  []*core.Ref `json:"selected"`  // Parsed refs for skills, languages, etc.
}
```

### Examples

```json
{
  "choices": [
    {
      "ref": "dnd5e:choice:race",
      "source": "dnd5e:creation",
      "selected": ["dnd5e:race:human"]
    },
    {
      "ref": "dnd5e:choice:class",
      "source": "dnd5e:creation",
      "selected": ["dnd5e:class:fighter"]
    },
    {
      "ref": "dnd5e:choice:fighter_skills",
      "source": "dnd5e:class:fighter",
      "selected": [
        "dnd5e:skill:athletics",
        "dnd5e:skill:intimidation"
      ]
    },
    {
      "ref": "dnd5e:choice:human_language",
      "source": "dnd5e:race:human",
      "selected": ["dnd5e:language:elvish"]
    },
    {
      "ref": "dnd5e:choice:fighter_fighting_style",
      "source": "dnd5e:class:fighter",
      "selected": ["dnd5e:fighting_style:defense"]
    }
  ]
}
```

## Self-Contained Character Data

With proper refs, the character data becomes fully self-contained:

```go
type Data struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Level    int    `json:"level"`
    
    // These refs are just for reference/display
    RaceRef       *core.Ref `json:"race_ref"`       // Parsed ref for "dnd5e:race:human"
    ClassRef      *core.Ref `json:"class_ref"`      // Parsed ref for "dnd5e:class:fighter"
    BackgroundRef *core.Ref `json:"background_ref"` // Parsed ref for "dnd5e:background:soldier"
    
    // Everything else is already "compiled" from choices
    AbilityScores AbilityScores         `json:"ability_scores"`
    HP           int                    `json:"hp"`
    MaxHP        int                    `json:"max_hp"`
    Speed        int                    `json:"speed"`        // From race
    Size         string                 `json:"size"`         // From race
    
    // Already resolved from all sources (with refs where appropriate)
    Skills        []*core.Ref           `json:"skills"`       // Skill refs character is proficient in
    Languages     []*core.Ref           `json:"languages"`    // Language refs character knows
    Proficiencies []*core.Ref           `json:"proficiencies"` // All proficiency refs
    Features      []json.RawMessage     `json:"features"`     // Feature data with embedded refs
    
    // Choices track what the player selected
    Choices       []ChoiceData          `json:"choices"`
    
    // Current state
    Conditions    []json.RawMessage     `json:"conditions"`   // Active conditions with refs
}
```

Note: Features and Conditions remain as `json.RawMessage` because they contain complex data that includes refs within them.

## API Client Integration

### Option 1: Toolkit-Aware API Client

Make the dnd5eapi client understand toolkit refs:

```go
// In the dnd5eapi client package
type Client interface {
    // Returns data with toolkit refs
    GetRace(id string) (*RaceData, error)
    GetClass(id string) (*ClassData, error)
    
    // New: resolve refs to data
    ResolveRef(ref string) (interface{}, error)
}

// RaceData includes refs
type RaceData struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Ref  string `json:"ref"` // "dnd5e:race:human"
    // ... other fields ...
}
```

### Option 2: Wrapper in Game Server (Recommended)

Keep API client simple, add toolkit layer in game server:

```go
// In the game server
type ToolkitWrapper struct {
    apiClient *dnd5eapi.Client
}

// Convert API data to toolkit format with proper refs
func (w *ToolkitWrapper) GetRaceWithRefs(id string) (*RaceData, error) {
    apiData, err := w.apiClient.GetRace(id)
    if err != nil {
        return nil, err
    }
    
    // Create proper ref
    raceRef := core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "race",
        Value:  apiData.Index,
    })
    
    return &RaceData{
        ID:   apiData.Index,
        Ref:  raceRef,
        Name: apiData.Name,
        // ... map other fields ...
    }, nil
}

// Compile choices into character data
func (w *ToolkitWrapper) CompileCharacter(choices []ChoiceData) (*character.Data, error) {
    // All refs are already parsed and validated
    // Apply all choices
    // Return self-contained character data
}
```

## Benefits of This Approach

1. **Simpler ChoiceData** - Just ref, source, and selected (all core.Ref)
2. **Type Safety** - Proper refs with validation, not strings
3. **Extensible** - New choice types don't require new fields
4. **Self-contained** - Character data has everything it needs
5. **API Integration** - Clean boundary between API and toolkit
6. **Validation** - Refs are validated at parse time, catching errors early

## Migration Path

1. Keep current ChoiceData for backward compatibility
2. Add new simplified choice structure alongside
3. Add conversion functions between old and new
4. Gradually migrate to ref-based approach
5. Eventually deprecate old ChoiceData

## Example: Character Creation Flow

```go
// Game server collects choices with proper refs
choices := []ChoiceData{
    {
        Ref:    core.MustParseString("dnd5e:choice:race"),
        Source: core.MustParseString("dnd5e:system:creation"),
        Selected: []*core.Ref{
            core.MustParseString("dnd5e:race:dwarf"),
        },
    },
    {
        Ref:    core.MustParseString("dnd5e:choice:class"),
        Source: core.MustParseString("dnd5e:system:creation"),
        Selected: []*core.Ref{
            core.MustParseString("dnd5e:class:barbarian"),
        },
    },
    {
        Ref:    core.MustParseString("dnd5e:choice:barbarian_skills"),
        Source: core.MustParseString("dnd5e:class:barbarian"),
        Selected: []*core.Ref{
            core.MustParseString("dnd5e:skill:athletics"),
            core.MustParseString("dnd5e:skill:survival"),
        },
    },
}

// Game server compiles to character data
charData := CompileCharacter(choices) // Everything resolved and denormalized

// Save self-contained data
database.Save(charData)

// Later: Load and play (no external deps needed!)
charData := database.Load(id)
character := character.LoadCharacterFromData(charData) // No race/class needed!
```

## Why Proper Refs Matter

Using `*core.Ref` instead of strings gives us:

1. **Compile-time safety** - Can't accidentally pass a malformed ref
2. **Parse-once** - Refs are validated when created, not on every use
3. **Rich comparison** - `ref.Equals()` instead of string comparison
4. **Module awareness** - Can check if ref is from expected module
5. **Type checking** - Can verify ref.Type is "skill" when expecting a skill

Example validation:
```go
func validateSkillChoice(ref *core.Ref) error {
    if ref.Module != "dnd5e" {
        return fmt.Errorf("expected dnd5e module, got %s", ref.Module)
    }
    if ref.Type != "skill" {
        return fmt.Errorf("expected skill type, got %s", ref.Type)
    }
    return nil
}
```