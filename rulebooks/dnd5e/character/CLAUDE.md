# D&D 5e Character Package Development Guidelines

## Module Purpose

Character creation and management for D&D 5e **using toolkit infrastructure**.

This module provides:
- Character draft workflow (build → validate → finalize)
- Choice recording and management (class, race, background, equipment)
- Character compilation (abilities, proficiencies, inventory, spells)
- Character state management (created vs finalized characters)

**We implement**: D&D 5e specific character creation rules and validation
**We use**: Core toolkit infrastructure (events, proficiencies, effects, equipment)

## Current Status: Production Ready

✅ **Core character creation workflow implemented**
✅ **Draft-based character building**
✅ **Class/Race/Background integration**
✅ **Source-level choice clearing** (Issue #344 fixed)

## Critical Architectural Patterns

### 1. Draft → Character Two-Phase Creation

**Pattern:**
```go
// Phase 1: Build draft with choices
draft := character.NewDraft(...)
draft.SetName(...)
draft.SetRace(...)
draft.SetClass(...)
draft.SetBackground(...)

// Phase 2: Finalize when complete
character, err := draft.Finalize()
```

**Why this pattern:**
- Allows incremental character building
- Validates only when complete (Finalize)
- Supports "save and continue later" workflows
- Clear separation: Draft = mutable, Character = immutable

### 2. Choice Recording and Source Tracking

**Critical understanding:** Choices are tracked with THREE identifiers:

```go
type ChoiceData struct {
    Category shared.ChoiceCategory  // WHAT: ChoiceSkills, ChoiceEquipment, etc.
    Source   shared.ChoiceSource    // WHERE: SourceClass, SourceRace, SourceBackground
    ChoiceID ChoiceID               // WHICH: "fighter-armor", "barbarian-skills"

    // One selection field populated based on Category
    SkillSelection         []skills.Skill
    EquipmentSelection     []shared.SelectionID
    // ... etc
}
```

**The three-level hierarchy:**
1. **Source** - Where did this choice come from? (class, race, background, player)
2. **Category** - What type of choice is this? (skills, equipment, languages, etc.)
3. **ChoiceID** - Which specific choice requirement? (fighter-armor vs barbarian-weapons)

**Why this matters:**
- When class changes, ALL SourceClass choices should be cleared
- Multiple equipment choices per source are allowed (armor + weapons + pack)
- Same category from different sources can coexist (class skills + background skills)

### 3. Choice Deduplication Logic (Source-Level Clearing)

**Two-level deduplication strategy:**

**Level 1: Source-level clearing (draft.go:672-685)**
```go
// clearChoicesBySource removes ALL choices from a specific source
// Called at the start of SetClass/SetRace/SetBackground before recording new choices
func (d *Draft) clearChoicesBySource(source shared.ChoiceSource) {
    filtered := make([]choices.ChoiceData, 0, len(d.choices))
    for _, choice := range d.choices {
        if choice.Source != source {
            filtered = append(filtered, choice)
        }
    }
    d.choices = filtered
}
```

**Level 2: Choice-level deduplication in `recordChoice()` (draft.go:687-707)**
```go
func (d *Draft) recordChoice(choice choices.ChoiceData) {
    filtered := make([]choices.ChoiceData, 0, len(d.choices))
    for _, c := range d.choices {
        // Equipment: Remove only if ChoiceID matches (allows multiple equipment choices)
        if choice.Category == shared.ChoiceEquipment && c.Category == shared.ChoiceEquipment {
            if c.ChoiceID != choice.ChoiceID {
                filtered = append(filtered, c)
            }
        } else {
            // Non-equipment: Remove if Category AND Source both match
            if c.Category != choice.Category || c.Source != choice.Source {
                filtered = append(filtered, c)
            }
        }
    }
    filtered = append(filtered, choice)
    d.choices = filtered
}
```

**Why two levels:**
- **Source clearing**: When changing class/race/background, remove ALL old choices from that source
- **Choice deduplication**: Within a source, allow multiple equipment choices but replace other categories

**Pattern in SetClass/SetRace/SetBackground:**
```go
// Step 1: Clear all old choices from this source
d.clearChoicesBySource(shared.SourceClass)

// Step 2: Record new choices (they won't conflict with old ones)
d.recordChoice(skillChoice)
d.recordChoice(equipmentChoice1)
d.recordChoice(equipmentChoice2)
```

**Fixed in Issue #344:** Previously, changing classes would accumulate equipment choices because different classes had different ChoiceIDs. Now, source-level clearing prevents this.

### 4. Equipment Choice Complexity

Equipment choices have **two-level structure**:

**Level 1: Choice (e.g., "fighter-armor")**
```go
Choice {
    ID: "fighter-armor",
    Category: "equipment",
    Description: "Choose armor",
    Options: [
        {ID: "fighter-armor-a", Items: [chain-mail, ...]},
        {ID: "fighter-armor-b", Items: [leather, ...]},
    ]
}
```

**Level 2: Option (e.g., "fighter-armor-a")**
The player picks ONE option, gets ALL items in that option.

**Recording pattern:**
```go
d.recordChoice(choices.ChoiceData{
    Category:           shared.ChoiceEquipment,
    Source:             shared.SourceClass,
    ChoiceID:           "fighter-armor",           // The choice
    OptionID:           "fighter-armor-a",         // The selected option
    EquipmentSelection: [...items...],             // Items from that option
})
```

**Why this matters:**
- A class has multiple equipment choices (armor, weapons, pack)
- Each choice has multiple options (option A, option B, etc.)
- All are `SourceClass` and `ChoiceEquipment`, differentiated by ChoiceID

### 5. Character Compilation Pattern

**Finalize() orchestrates compilation** (draft.go:103-159):

1. **Validate draft completeness**
2. **Compile base stats** - Race modifiers to base ability scores
3. **Compile proficiencies** - From race, class, background
4. **Compile inventory** - From equipment choices
5. **Compile cantrips/spells** - From class choices
6. **Compile features** - From race, class, background
7. **Build Character entity**

**Key compilation methods:**
- `compileAbilityScores()` - Applies racial bonuses
- `compileProficiencies()` - Merges from all sources
- `compileInventory()` - Resolves equipment choices to items
- `compileSpells()` - Extracts cantrips and spells
- `compileFeatures()` - Merges race/class/background features

**Pattern:** Each compilation method is stateless, pure function taking draft data as input.

### 6. Validation Patterns

**Two validation levels:**

**Draft validation (ongoing):**
```go
// In SetClass() - validate skill count
if len(input.Choices.Skills) != classData.SkillChoiceCount {
    return rpgerr.New(rpgerr.CodeInvalidArgument, "invalid skill count")
}
```

**Finalize validation (comprehensive):**
```go
// In Finalize() - check completeness
if d.progress < ProgressComplete {
    return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "draft not complete")
}
```

**Validation philosophy:**
- Individual setters validate their inputs (SetClass validates class exists, skill count matches)
- Finalize validates overall completeness (all required choices made)
- Missing: Cross-validation (e.g., chosen skills are actually in class skill list)

## File Organization

```
rulebooks/dnd5e/character/
├── draft.go               - Draft type and choice recording (MAIN FILE)
├── draft_data.go          - Draft serialization
├── character.go           - Finalized Character type
├── character_data.go      - Character serialization
├── choices/
│   ├── choice_data.go     - ChoiceData structure
│   ├── choice_ids.go      - Constants for choice IDs
│   └── equipment_choice.go - Equipment choice helpers
├── shared/
│   ├── types.go           - Common types (ChoiceCategory, ChoiceSource)
│   └── ability_scores.go  - Ability score calculations
└── *_test.go              - Tests (use testify suite)
```

## Common Patterns

### Creating a Character from Scratch

```go
// 1. Create draft
draft := character.NewDraft(character.NewDraftInput{
    PlayerID: "player-123",
})

// 2. Set player choices
draft.SetName("Conan")
draft.SetAbilityScores(character.SetAbilityScoresInput{
    Scores: shared.AbilityScores{STR: 15, DEX: 14, ...},
})

// 3. Set race
draft.SetRace(character.SetRaceInput{
    Race:    races.Human,
    Subrace: races.SubraceNone,
    Choices: choices.RaceChoices{
        Languages: []languages.Language{languages.Common, languages.Orcish},
    },
})

// 4. Set class
draft.SetClass(character.SetClassInput{
    Class:    classes.Barbarian,
    Subclass: classes.SubclassNone,
    Choices: choices.ClassChoices{
        Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
        Equipment: []choices.EquipmentChoiceSelection{
            {ChoiceID: "barbarian-weapons-primary", OptionID: "barbarian-weapon-a"},
            {ChoiceID: "barbarian-weapons-secondary", OptionID: "barbarian-secondary-a"},
            {ChoiceID: "barbarian-pack", OptionID: "barbarian-pack-explorer"},
        },
    },
})

// 5. Set background
draft.SetBackground(character.SetBackgroundInput{
    Background: backgrounds.Folk,
    Choices: choices.BackgroundChoices{
        Skills: []skills.Skill{skills.AnimalHandling, skills.Survival},
    },
})

// 6. Mark complete and finalize
draft.SetProgress(character.ProgressComplete)
character, err := draft.Finalize()
```

### Changing Character Class (Working Correctly)

```go
// Initial class
draft.SetClass(character.SetClassInput{
    Class: classes.Fighter,
    Choices: choices.ClassChoices{
        Skills: []skills.Skill{skills.Acrobatics, skills.Athletics},
        Equipment: []choices.EquipmentChoiceSelection{
            {ChoiceID: "fighter-armor", OptionID: "fighter-armor-a"},
            {ChoiceID: "fighter-weapons", OptionID: "fighter-weapon-a"},
        },
    },
})

// Change class - Fighter choices are automatically cleared
draft.SetClass(character.SetClassInput{
    Class: classes.Barbarian,
    Choices: choices.ClassChoices{
        Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
        Equipment: []choices.EquipmentChoiceSelection{
            {ChoiceID: "barbarian-weapons-primary", OptionID: "barbarian-weapon-a"},
        },
    },
})

// ✅ draft.choices now contains ONLY Barbarian choices
// ✅ All Fighter choices (skills + equipment) were cleared by clearChoicesBySource()
```

**How it works:**
1. `SetClass()` calls `clearChoicesBySource(SourceClass)` at the start
2. All previous class choices are removed (Fighter skills, Fighter equipment)
3. New class choices are recorded fresh (Barbarian skills, Barbarian equipment)
4. No accumulation occurs

### Testing Character Creation

**Use testify suite pattern:**

```go
type CharacterTestSuite struct {
    suite.Suite
    draft *character.Draft
}

func (s *CharacterTestSuite) SetupTest() {
    s.draft = character.NewDraft(character.NewDraftInput{
        PlayerID: "test-player",
    })
}

func (s *CharacterTestSuite) TestBarbarianCreation() {
    // Arrange
    s.draft.SetName("Test Barbarian")
    s.draft.SetAbilityScores(character.SetAbilityScoresInput{...})
    s.draft.SetRace(character.SetRaceInput{...})
    s.draft.SetClass(character.SetClassInput{
        Class: classes.Barbarian,
        // ... choices
    })
    s.draft.SetBackground(character.SetBackgroundInput{...})
    s.draft.SetProgress(character.ProgressComplete)

    // Act
    char, err := s.draft.Finalize()

    // Assert
    s.Require().NoError(err)
    s.Assert().Equal("Test Barbarian", char.GetName())
    s.Assert().Contains(char.GetProficiencies(), proficiencies.Skill(skills.Athletics))
}
```

## Common Mistakes to Avoid

1. ❌ **Not clearing old choices when changing source data**
   - Current bug in SetClass/SetRace/SetBackground
   - Choices accumulate instead of replacing

2. ❌ **Calling Finalize() on incomplete draft**
   - Always check/set Progress before finalizing
   - Finalize validates completeness

3. ❌ **Confusing ChoiceID with OptionID**
   - ChoiceID: The requirement ("fighter-armor")
   - OptionID: The selection within that requirement ("fighter-armor-a")

4. ❌ **Assuming one choice per source**
   - A class has multiple equipment choices (armor + weapons + pack)
   - Use ChoiceID to differentiate

5. ❌ **Mutating finalized Character**
   - Character is immutable
   - To modify, create new draft from character (feature not yet implemented)

6. ❌ **Not validating equipment choice selections**
   - Current code doesn't validate that chosen OptionID exists in Choice.Options
   - Validation gap (future work)

## Testing Guidelines

### What to Test

**Draft operations:**
- ✅ SetClass/SetRace/SetBackground record choices correctly
- ✅ Choice deduplication works (within category/source)
- ✅ Source-level clearing on class/race/background changes (Issue #344 - now tested)

**Character compilation:**
- ✅ Ability scores compiled with racial bonuses
- ✅ Proficiencies merged from all sources
- ✅ Inventory compiled from equipment choices
- ✅ Spells compiled from class choices

**Validation:**
- ✅ Invalid class rejected
- ✅ Wrong skill count rejected
- ✅ Incomplete draft finalization rejected
- ⚠️ **Missing:** Invalid equipment option rejected

### Test Data Patterns

**Use choice ID constants:**
```go
import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"

equipmentChoices := []choices.EquipmentChoiceSelection{
    {ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: "barbarian-weapon-a"},
}
```

**Test class-specific finalizations separately:**
- `barbarian_finalize_test.go` - Barbarian-specific validation
- `fighter_finalize_test.go` - Fighter-specific validation (doesn't exist yet)

## Integration with Other Modules

### Core Module
- Character implements `core.Entity` (GetID, GetType)
- Uses `core.EntityType` constants

### Proficiencies Module (mechanics/proficiency)
- Character has `GetProficiencies()` returning proficiency.Set
- Compiled from race/class/background during Finalize()

### Equipment Module (items - not yet implemented)
- Equipment choices reference item IDs
- Inventory compilation resolves IDs to items (when items module exists)

### Effects Module (mechanics/effects)
- Features grant effects (to be integrated)
- Racial features, class features, background features

## Known Issues and Future Work

### ✅ Issue #344: Class Change Accumulation Bug (FIXED)
**Status:** Closed
**Impact:** Medium - affected character creation workflow
**Fix:** Added `clearChoicesBySource()` helper, called in SetClass/SetRace/SetBackground
**Tests:** `ClassChangeTestSuite` in draft_test.go validates the fix

### Future: Character Modification After Creation
**Need:** Ability to level up, multiclass, gain items
**Pattern:** Create Draft from existing Character, modify, re-finalize
**Status:** Not implemented

### Future: Equipment Item Integration
**Need:** Resolve equipment SelectionIDs to actual item entities
**Blocked by:** Items module (#31)

### Future: Spell Slot Management
**Need:** Track spell slots, spell casting
**Pattern:** Integrate with resources module
**Status:** Not implemented

## Questions to Ask Before Adding Features

1. **Is this character creation or character management?**
   - Creation: Draft → Finalize workflow
   - Management: Modify existing Character (not yet supported)

2. **Is this a choice or a calculation?**
   - Choice: Player decides, recorded in ChoiceData
   - Calculation: Derived from rules, computed in compilation

3. **What is the source of this data?**
   - SourceClass, SourceRace, SourceBackground, SourcePlayer
   - Determines when/how it's cleared on changes

4. **Does this need validation?**
   - Input validation: In Set* methods
   - Completeness validation: In Finalize()
   - Cross-validation: Often missing, consider adding

## Remember

- **Draft is mutable, Character is immutable**
- **Choices have THREE identifiers: Source, Category, ChoiceID**
- **Equipment choices are multi-level: Choice → Option → Items**
- **Source changes should clear old choices (not implemented correctly yet)**
- **Finalize orchestrates compilation from all sources**
- **Use testify suite pattern for all tests**
- **Choice IDs are constants in choices/choice_ids.go**
