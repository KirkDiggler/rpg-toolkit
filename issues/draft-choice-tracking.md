# Fix Draft/Character Choice Tracking Structure

## Problem

We have the choice tracking backwards:
- `Draft` has flat lists (SkillChoices, LanguageChoices) with no source tracking
- `Character` has both flat lists AND rich ChoiceData tracking

This prevents game services from showing duplicate warnings or handling race/class changes during character creation.

## Current State

```go
// Draft has flat lists (WRONG - needs rich tracking)
type Draft struct {
    SkillChoices     []constants.Skill    
    LanguageChoices  []constants.Language
    // No source tracking!
}

// Character has BOTH (redundant)
type Character struct {
    skills    map[constants.Skill]shared.ProficiencyLevel  // Compiled result
    languages []constants.Language                         // Compiled result
    choices   []ChoiceData                                 // Rich tracking
}
```

## Proposed Solution

1. **Draft** should have `Choices []ChoiceData` for tracking decisions with sources
2. **Character** should have the compiled results (skills map, languages list)
3. Remove flat choice lists from Draft (they belong on Character as compiled results)

```go
// Draft tracks choices/decisions made
type Draft struct {
    ID       string
    PlayerID string
    Name     string
    
    // Rich choice tracking for game service
    Choices []ChoiceData
    
    Progress DraftProgress
}

// Character has compiled results
type Character struct {
    // Compiled from choices + grants
    skills    map[constants.Skill]shared.ProficiencyLevel
    languages []constants.Language
    
    // Keep choices for reference
    choices []ChoiceData
}
```

## Implementation Plan

Since we're in alpha1, we can make breaking changes:

1. Remove all flat choice fields from Draft (SkillChoices, LanguageChoices, etc.)
2. Add `Choices []ChoiceData` to Draft
3. Update builder to populate Draft.Choices with proper source tracking
4. Update ToCharacter() to compile choices into Character's flat structures
5. Update tests

## Example Usage

```go
// Building a draft with source tracking
draft := &Draft{
    Choices: []ChoiceData{
        {
            Category:  "skill",
            Source:    "race:half_orc",
            ChoiceID:  "",  // Automatic grant
            Selection: constants.SkillIntimidation,
        },
        {
            Category:  "skill", 
            Source:    "class:fighter",
            ChoiceID:  "fighter_skill_choice_1",
            Selection: []constants.Skill{
                constants.SkillAcrobatics,
                constants.SkillAthletics,
            },
        },
        {
            Category:  "skill",
            Source:    "background:soldier", 
            ChoiceID:  "",  // Automatic grant
            Selection: []constants.Skill{
                constants.SkillAthletics,
                constants.SkillIntimidation,
            },
        },
    },
}

// Game service can now detect duplicates
duplicates := findDuplicateSkills(draft.Choices)
// Returns: ["intimidation" from race:half_orc and background:soldier]
```

## Benefits

1. **Complete source tracking** - Know exactly where each feature came from
2. **Duplicate detection** - Can warn players about redundant choices
3. **Clean mutations** - Know what to remove when changing race/class
4. **Better UX** - Can show "You get X from Y" in the UI
5. **Simpler code** - One consistent structure instead of many flat lists

## Notes

- This aligns with the original design intent - we just put it in the wrong place
- The ChoiceData structure is already well-tested in Character
- Game services desperately need this for proper draft management
- See Journey 007 for full analysis of the problem