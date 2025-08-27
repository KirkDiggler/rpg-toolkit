# Character Choices Package

## Purpose

This package provides the complete character choice system for D&D 5e character creation and progression. It serves as the contract between the game server and the D&D 5e rulebook implementation.

## Core Contract

The game server needs two simple operations with two usage patterns:

### Character Creation (Step-by-Step)
```go
// 1. Discovery: "What choices for just this class/race/background?"
classReqs := choices.GetClassRequirements(classID, level)
raceReqs := choices.GetRaceRequirements(raceID)
backgroundReqs := choices.GetBackgroundRequirements(backgroundID)

// 2. Validation: "Are these class/race/background choices valid?"
result := choices.ValidateClassChoices(classID, level, submissions)
result := choices.ValidateRaceChoices(raceID, submissions)
result := choices.ValidateBackgroundChoices(backgroundID, submissions)
```

### Level Progression (Combined)
```go
// 1. Discovery: "What choices for a Level 4 Half-Elf Fighter?"
requirements := choices.GetRequirements(classID, raceID, level)

// 2. Validation: "Are all these choices valid for this character?"
result := choices.Validate(classID, raceID, level, submissions)
```

The game server **doesn't need to know**:
- What specific skills a Fighter can choose
- How many cantrips a Wizard gets
- What expertise means for a Rogue
- Any D&D 5e specific rules

## Scope

### What This Package IS

✅ **The single source of truth for character choices**
- What choices each class/race requires at each level
- Whether submitted choices are valid
- Cross-source choice interaction (e.g., duplicate skills from race and class)

✅ **A level-aware progression system**
- Level 1: Initial character creation choices
- Level 2+: Level-up choices (future)
- Multiclassing: Cross-class choice validation (future)

✅ **An intelligent validation system**
- **Errors**: Missing or invalid choices that block character creation
- **Warnings**: Valid but suboptimal choices (duplicates, missing prerequisites)

### What This Package IS NOT

❌ **Not a database or data loader**
- Requirements are code, not data files
- We prioritize readability and maintainability over data-driven design

❌ **Not a game mechanics engine**
- We don't calculate damage or apply conditions
- We just ensure choices are valid

❌ **Not a UI/presentation layer**
- We provide the data, not how to display it

## Design Principles

### 1. Simple Contract
The game server interface is minimal and clear. Two functions handle 90% of use cases.

### 2. Internal Complexity, External Simplicity
Internally, we handle all the D&D 5e complexity. Externally, the API is dead simple.

### 3. Fail Informatively
When validation fails, we explain why and suggest fixes. We're teachers, not just validators.

### 4. Code as Documentation
Requirements are readable code, not abstract data structures:
```go
// Clear and maintainable
FighterRequirements = Requirements{
    Skills: Choose(2).From(Athletics, Intimidation, Survival, ...),
    Equipment: Choose(1).From(MartialWeaponAndShield, TwoMartialWeapons),
}

// Not this
{"class": "fighter", "skills": {"count": 2, "from": ["athletics", ...]}}
```

## Package Structure

```
character/choices/
├── README.md           # This file
├── types.go           # Core types and interfaces
├── requirements.go    # What each class/race requires
├── validator.go       # Validation logic
└── *_test.go         # Comprehensive tests
```

## Usage Examples

### Example 1: Character Creation Flow (Step-by-Step)

```go
// Step 1: Player selects Fighter class
classReqs := choices.GetClassRequirements(classes.Fighter, 1)

// Returns structured requirements:
// - Choose 2 skills from [Athletics, Intimidation, ...]
// - Choose 1 fighting style from [Archery, Defense, ...]
// - Choose equipment: (a) martial weapon and shield OR (b) two martial weapons

// Player submits Fighter choices
classSubmissions := map[string][]string{
    "skills": {"athletics", "intimidation"},
    "fighting_style": {"defense"},
    "equipment": {"longsword", "shield"},
}
result := choices.ValidateClassChoices(classes.Fighter, 1, classSubmissions)

// Step 2: Player selects Half-Orc race
raceReqs := choices.GetRaceRequirements(races.HalfOrc)
// Returns: None (Half-Orc has no choices, just grants Intimidation)

// Step 3: Player selects Soldier background
backgroundReqs := choices.GetBackgroundRequirements(backgrounds.Soldier)
// Returns: None (Soldier has no choices, grants Athletics and Intimidation)
```

### Example 2: Level 4 Ability Score Improvement

```go
// Existing Level 3 Half-Orc Fighter reaches Level 4
requirements := choices.GetRequirements(classes.Fighter, races.HalfOrc, 4)
// Returns: Choose Ability Score Improvement or Feat

submissions := map[string][]string{
    "level4_choice": {"ability_score_improvement"},
    "ability_scores": {"strength", "strength"}, // +2 to Strength
}

result := choices.Validate(classes.Fighter, races.HalfOrc, 4, submissions)
// result.Valid = true
```

### Example 3: Cross-Source Duplicate Detection (3-Tier Validation)

```go  
// Player picks Fighter skills, then switches to Half-Orc (grants Intimidation)
submissions := choices.NewTypedSubmissions()

// Fighter chose Intimidation, but Half-Orc grants it automatically
submissions.AddChoice(choices.ChoiceSubmission{
    Source: choices.SourceClass,
    Field:  choices.FieldSkills,
    Values: []string{"intimidation", "athletics"},
})

result := choices.Validate(
    classes.Fighter,
    races.HalfOrc, 
    backgrounds.Soldier, // Soldier also grants Athletics
    1,
    submissions,
    nil, // no context needed for this example
)

// result.CanSave = true (can save draft)
// result.CanFinalize = true (can create character)
// result.IsOptimal = false (has warnings)
// result.Warnings = [
//   "skill 'intimidation' chosen from class but already granted by race - pick another instead",
//   "skill 'athletics' chosen from class but already granted by background - pick another instead"
// ]
```

### Example 4: Invalid Choices

```go
submissions := map[string][]string{
    "skills": {"stealth", "acrobatics"}, // Fighter can't choose these!
    // Missing fighting style!
}

result := choices.Validate(classes.Fighter, races.Human, 1, submissions)
// result.Valid = false
// result.Errors = [
//   "Invalid skills for Fighter: stealth, acrobatics",
//   "Fighter requires fighting style selection"
// ]
```

## Future Expansion

### Level Progression (Planned)
```go
// Level 4: Ability Score Improvement or Feat
requirements := choices.GetRequirements(classes.Fighter, races.Human, 4)
// Returns: Choose ASI or Feat
```

### Multiclassing (Planned)
```go
// Fighter 5 / Rogue 2
requirements := choices.GetMulticlassRequirements(
    []ClassLevel{{Fighter, 5}, {Rogue, 2}}
)
```

## Implementation Status

- [x] Core design and API contract
- [x] Basic types and interfaces (Requirements, TypedSubmissions, ValidationResult)
- [x] Requirements for all 12 PHB classes (Fighter, Wizard, Cleric, Rogue, etc.)
- [x] Requirements for racial choices (Half-Elf, Half-Orc, Dragonborn, etc.)
- [x] 3-tier validation system (Error/Incomplete/Warning)
- [x] Cross-source duplicate detection with smart warnings
- [x] TypedSubmissions with source tracking (class/race/background)
- [x] ValidationContext for expertise prerequisite checking
- [ ] Background-specific requirements (most backgrounds have no choices)
- [ ] Comprehensive test coverage (partial coverage exists)

## Contributing

When adding new content:
1. Add requirements in a clear, readable format
2. Include validation rules
3. Add tests for both valid and invalid cases
4. Document any special rules or edge cases

Remember: The game server is our customer. Keep the external API simple, even if the internal implementation is complex.