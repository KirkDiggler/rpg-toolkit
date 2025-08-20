# Character Creation Modernization

## Overview

This document describes the modernization of the character creation system from a complex Draft/Builder pattern to a simplified ref-based approach.

## Problems with the Old System

1. **Complex Multi-Step Process**: Draft -> Builder -> Character with multiple validation points
2. **Duplicate Choice Types**: ChoiceData defined in multiple packages (character, class, race, shared)
3. **Heavy Validation**: Complex validator with many rules that could be simplified
4. **Data Compilation**: Complex methods to compile skills, languages, equipment from various sources
5. **Progress Tracking**: Bitwise flags for tracking creation progress
6. **Multiple Conversions**: LoadDraftFromData, ToCharacter, compileCharacter, etc.

## New Simplified System

### Core Concept: Everything is a Ref

Instead of complex choice tracking and validation, we use refs for everything:

```go
type CharacterDef struct {
    // Identity as refs
    RaceRef       string `json:"race_ref"`       // "dnd5e:race:human"
    ClassRef      string `json:"class_ref"`      // "dnd5e:class:fighter"
    BackgroundRef string `json:"background_ref"` // "dnd5e:background:soldier"
    
    // Features as refs
    Features []string `json:"features"` // ["dnd5e:features:rage"]
    
    // Skills as refs
    Skills []string `json:"skills"` // ["dnd5e:skill:athletics"]
}
```

### Benefits

1. **Direct Loading**: Load character directly from JSON with refs
2. **No Draft/Builder**: Character creation is just data + refs
3. **Simple Choices**: Choices are just arrays of refs
4. **No Progress Tracking**: If the data exists, it's valid
5. **Single Source of Truth**: Refs define everything

## Migration Path

### Phase 1: Parallel Systems (Current)
- New `CharacterDef` and `LoadFromDef()` alongside old system
- Both systems can coexist
- Conversion methods between old and new (`ToDef()`)

### Phase 2: Deprecate Draft/Builder
- Mark Draft and Builder as deprecated
- Update all creation flows to use CharacterDef
- Keep ToData/LoadFromData for backwards compatibility

### Phase 3: Clean Up
- Remove Draft, Builder, Validator
- Consolidate duplicate ChoiceData types
- Single character creation path

## Example Usage

### Old Way (Complex)
```go
// Create builder
builder, _ := NewCharacterBuilder("draft_001")

// Set each property with validation
builder.SetName("Thorin")
builder.SetRaceData(raceData, "mountain_dwarf")
builder.SetClassData(classData, "")
builder.SetBackgroundData(backgroundData)
builder.SetAbilityScores(scores)
builder.SelectSkills([]string{"athletics", "intimidation"})

// Build character
character, _ := builder.Build()
```

### New Way (Simple)
```go
// Define character with refs
def := &CharacterDef{
    ID:            "char_001",
    Name:          "Thorin",
    RaceRef:       "dnd5e:race:dwarf",
    SubraceRef:    "dnd5e:subrace:mountain_dwarf",
    ClassRef:      "dnd5e:class:barbarian",
    BackgroundRef: "dnd5e:background:soldier",
    AbilityScores: scores,
    Skills: []string{
        "dnd5e:skill:athletics",
        "dnd5e:skill:intimidation",
    },
}

// Load character
character, _ := LoadFromDef(def, eventBus)
```

## JSON Format

### Old Format
```json
{
    "race_id": "dwarf",
    "class_id": "barbarian",
    "skills": {
        "athletics": 1,
        "intimidation": 1
    },
    "choices": [
        {
            "category": "skills",
            "source": "creation",
            "selection": ["athletics", "intimidation"]
        }
    ]
}
```

### New Format
```json
{
    "race_ref": "dnd5e:race:dwarf",
    "class_ref": "dnd5e:class:barbarian",
    "skills": [
        "dnd5e:skill:athletics",
        "dnd5e:skill:intimidation"
    ],
    "choices": [
        {
            "ref": "dnd5e:choice:barbarian_skills",
            "source": "dnd5e:class:barbarian",
            "selected": [
                "dnd5e:skill:athletics",
                "dnd5e:skill:intimidation"
            ]
        }
    ]
}
```

## Implementation Status

- ✅ CharacterDef type defined
- ✅ LoadFromDef implementation
- ✅ LoadCharacterDef from JSON
- ✅ ToDef conversion method
- ✅ Tests for new system
- ⏳ Deprecate Draft/Builder
- ⏳ Update all creation flows
- ⏳ Remove old system

## Next Steps

1. Update character creation endpoints to use CharacterDef
2. Migrate existing Draft data to CharacterDef format
3. Update feature/condition loading to use refs consistently
4. Remove Draft, Builder, and Validator once migration complete
5. Consolidate duplicate ChoiceData types into single definition